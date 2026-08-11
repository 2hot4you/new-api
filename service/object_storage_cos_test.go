package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type objectStorageCOSTestServer struct {
	server       *httptest.Server
	client       *cos.Client
	mu           sync.Mutex
	objects      map[string][]byte
	contentTypes map[string]string
	expiresAt    map[string]string
	putCount     int
	deleteCount  int
	deleteStatus map[string]int
}

type objectStorageRoundTripFunc func(*http.Request) (*http.Response, error)

func (f objectStorageRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newObjectStorageCOSTestServer(t *testing.T) *objectStorageCOSTestServer {
	t.Helper()
	fake := &objectStorageCOSTestServer{
		objects:      make(map[string][]byte),
		contentTypes: make(map[string]string),
		expiresAt:    make(map[string]string),
		deleteStatus: make(map[string]int),
	}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := url.PathUnescape(strings.TrimPrefix(r.URL.EscapedPath(), "/"))
		require.NoError(t, err)
		fake.mu.Lock()
		defer fake.mu.Unlock()
		switch r.Method {
		case http.MethodHead:
			body, ok := fake.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.Header().Set("Content-Type", fake.contentTypes[key])
			if fake.expiresAt[key] != "" {
				w.Header().Set("x-cos-meta-expires-at", fake.expiresAt[key])
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			if strings.EqualFold(r.Header.Get("x-cos-forbid-overwrite"), "true") {
				if _, exists := fake.objects[key]; exists {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte("<Error><Code>FileAlreadyExists</Code><Message>File already exists.</Message></Error>"))
					return
				}
			}
			body, readErr := io.ReadAll(r.Body)
			require.NoError(t, readErr)
			fake.objects[key] = body
			fake.contentTypes[key] = r.Header.Get("Content-Type")
			fake.expiresAt[key] = r.Header.Get("x-cos-meta-expires-at")
			fake.putCount++
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			fake.deleteCount++
			if status := fake.deleteStatus[key]; status != 0 {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = w.Write([]byte("<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>"))
				return
			}
			delete(fake.objects, key)
			delete(fake.contentTypes, key)
			delete(fake.expiresAt, key)
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			body, ok := fake.objects[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", fake.contentTypes[key])
			w.Header().Set("Accept-Ranges", "bytes")
			if r.Header.Get("Range") == "bytes=0-3" {
				w.Header().Set("Content-Range", "bytes 0-3/"+strconv.Itoa(len(body)))
				w.Header().Set("ETag", r.Header.Get("If-Range"))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(body[:4])
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(fake.server.Close)
	bucketURL, err := url.Parse(fake.server.URL)
	require.NoError(t, err)
	fake.client = cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, fake.server.Client())
	fake.client.Conf.EnableCRC = false
	return fake
}

func TestFetchCOSObjectForwardsRangeAndIfRange(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	key := "users/grok-results/42/video/2026/08/result.mp4"
	fakeCOS.objects[key] = []byte("video-result")
	fakeCOS.contentTypes[key] = "video/mp4"

	response, err := fetchCOSObjectWithClient(context.Background(), fakeCOS.client, key, "bytes=0-3", `"etag-value"`)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusPartialContent, response.StatusCode)
	require.Equal(t, "bytes 0-3/12", response.Header.Get("Content-Range"))
	require.Equal(t, `"etag-value"`, response.Header.Get("ETag"))
	require.Equal(t, []byte("vide"), body)
}

func newObjectStorageCOSTestStore(t *testing.T, fake *objectStorageCOSTestServer, remoteClient *http.Client) *objectStorageCOS {
	t.Helper()
	return &objectStorageCOS{client: fake.client, fetchClient: remoteClient, config: operation_setting.GetCOSConfig()}
}

func TestObjectStorageCOSCopiesHTTPSObjectWithBoundedValidatedStreaming(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-body"))
	}))
	t.Cleanup(remote.Close)
	store := newObjectStorageCOSTestStore(t, fakeCOS, remote.Client())

	stored, err := store.copyRemoteObjectToCOS(context.Background(), remote.URL+"/result.png?signature=secret", ObjectKeySpec{
		ObjectKey: "users/grok-results/42/image/2026/08/result.png",
		MediaType: "image",
		MaxBytes:  16,
		ExpiresAt: 1_786_472_000,
	})
	require.NoError(t, err)
	require.Equal(t, &StoredObject{
		ObjectKey: "users/grok-results/42/image/2026/08/result.png",
		MIMEType:  "image/png",
		Size:      8,
		ExpiresAt: 1_786_472_000,
	}, stored)
	require.Equal(t, []byte("png-body"), fakeCOS.objects[stored.ObjectKey])

	_, err = store.copyRemoteObjectToCOS(context.Background(), remote.URL+"/too-large.png", ObjectKeySpec{
		ObjectKey: "users/grok-results/42/image/2026/08/too-large.png",
		MediaType: "image",
		MaxBytes:  4,
		ExpiresAt: 1_786_472_000,
	})
	require.ErrorContains(t, err, "maximum")
	require.NotContains(t, fakeCOS.objects, "users/grok-results/42/image/2026/08/too-large.png")

	_, err = store.copyRemoteObjectToCOS(context.Background(), remote.URL+"/wrong-type.png", ObjectKeySpec{
		ObjectKey: "users/grok-results/42/video/2026/08/wrong-type.mp4",
		MediaType: "video",
		MaxBytes:  16,
		ExpiresAt: 1_786_472_000,
	})
	require.ErrorContains(t, err, "content type")
}

func TestObjectStorageCOSReusesExistingObjectWithoutFetchingRemoteURL(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	key := "users/grok-results/42/video/2026/08/stable.mp4"
	fakeCOS.objects[key] = []byte("existing")
	fakeCOS.contentTypes[key] = "video/mp4"
	fakeCOS.expiresAt[key] = "1786472000"
	remoteCalls := 0
	remoteClient := &http.Client{Transport: objectStorageRoundTripFunc(func(*http.Request) (*http.Response, error) {
		remoteCalls++
		return nil, errors.New("remote fetch must not run")
	})}
	store := newObjectStorageCOSTestStore(t, fakeCOS, remoteClient)

	stored, err := store.copyRemoteObjectToCOS(context.Background(), "https://upstream.invalid/signed", ObjectKeySpec{
		ObjectKey: key,
		MediaType: "video",
		MaxBytes:  32,
		ExpiresAt: 1_786_472_000,
	})
	require.NoError(t, err)
	require.Equal(t, key, stored.ObjectKey)
	require.Equal(t, "video/mp4", stored.MIMEType)
	require.Equal(t, int64(8), stored.Size)
	require.Zero(t, remoteCalls)
	require.Zero(t, fakeCOS.putCount)
}

func TestObjectStorageCOSExistingObjectExpiryMetadataFailsClosed(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		expiresAt string
	}{
		{name: "missing"},
		{name: "invalid", expiresAt: "not-a-unix-time"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fakeCOS := newObjectStorageCOSTestServer(t)
			key := "users/grok-results/42/video/2026/08/stable.mp4"
			fakeCOS.objects[key] = []byte("existing")
			fakeCOS.contentTypes[key] = "video/mp4"
			fakeCOS.expiresAt[key] = testCase.expiresAt
			remoteClient := &http.Client{Transport: objectStorageRoundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("remote fetch must not run after an existing-object HEAD")
				return nil, nil
			})}
			store := newObjectStorageCOSTestStore(t, fakeCOS, remoteClient)

			_, err := store.copyRemoteObjectToCOS(context.Background(), "https://upstream.invalid/signed", ObjectKeySpec{
				ObjectKey: key,
				MediaType: "video",
				MIMEType:  "video/mp4",
				MaxBytes:  32,
				ExpiresAt: 1_786_475_600,
			})
			require.ErrorContains(t, err, "expiry metadata")
			require.Zero(t, fakeCOS.putCount)
		})
	}
}

func TestObjectStorageCOSPersistenceRequiresEnabledConfigWhileStarAIReadStillSigns(t *testing.T) {
	useStarAICOSTestConfig(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap["COSEnabled"] = "false"
	common.OptionMapRWMutex.Unlock()

	_, err := CopyRemoteObjectToCOS(context.Background(), "https://upstream.invalid/result.png", ObjectKeySpec{
		ObjectKey: "users/grok-results/42/image/2026/08/disabled.png",
		MediaType: "image",
		MIMEType:  "image/png",
		MaxBytes:  32,
		ExpiresAt: 1_786_472_000,
	})
	require.ErrorIs(t, err, ErrObjectStorageUnavailable)
	_, err = PersistGrokResult(context.Background(), GrokResultStoreRequest{
		SourceURL:      "https://upstream.invalid/result.png",
		UserID:         42,
		MediaType:      "image",
		MIMEType:       "image/png",
		IdempotencyKey: "disabled-request",
		CreatedAt:      time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC),
	})
	require.ErrorIs(t, err, ErrObjectStorageUnavailable)

	signed, err := GetStarAICOSPreviewURL(context.Background(), "users/42/starai-assets/image/existing.png")
	require.NoError(t, err)
	require.NotEmpty(t, signed)
}

func TestObjectStorageCOSConcurrentSameKeyHasSingleCreateOwner(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	releaseRemote := make(chan struct{})
	remoteCalls := make(chan struct{}, 2)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		remoteCalls <- struct{}{}
		<-releaseRemote
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("same-image"))
	}))
	t.Cleanup(remote.Close)
	store := newObjectStorageCOSTestStore(t, fakeCOS, remote.Client())
	key := ObjectKeySpec{
		ObjectKey: "users/grok-results/42/image/2026/08/concurrent.jpg",
		MediaType: "image",
		MIMEType:  "image/jpeg",
		ExpiresAt: 1_786_472_000,
	}
	type copyResult struct {
		stored  *StoredObject
		created bool
		err     error
	}
	results := make(chan copyResult, 2)
	for range 2 {
		go func() {
			stored, created, err := store.copyRemoteObjectToCOSWithStatus(context.Background(), remote.URL+"/result.jpg", key)
			results <- copyResult{stored: stored, created: created, err: err}
		}()
	}
	<-remoteCalls
	<-remoteCalls
	close(releaseRemote)
	first := <-results
	second := <-results

	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first.stored, second.stored)
	createdCount := 0
	if first.created {
		createdCount++
	}
	if second.created {
		createdCount++
	}
	require.Equal(t, 1, createdCount, "COS must atomically grant ownership to only one writer")
	require.Equal(t, 1, fakeCOS.putCount)
}

func TestObjectStorageCOSHonorsCancellationWhileCopying(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	remoteClient := &http.Client{Transport: objectStorageRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"video/mp4"}},
			Body:       &cancelingObjectStorageBody{cancel: cancel},
		}, nil
	})}
	store := newObjectStorageCOSTestStore(t, fakeCOS, remoteClient)

	_, err := store.copyRemoteObjectToCOS(ctx, "https://upstream.invalid/video.mp4", ObjectKeySpec{
		ObjectKey: "users/grok-results/42/video/2026/08/cancelled.mp4",
		MediaType: "video",
		MaxBytes:  32,
		ExpiresAt: 1_786_472_000,
	})
	require.ErrorIs(t, err, context.Canceled)
	require.NotContains(t, fakeCOS.objects, "users/grok-results/42/video/2026/08/cancelled.mp4")
}

func TestObjectStorageCOSRejectsUnknownLengthBodyPastStreamingLimit(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	remoteClient := &http.Client{Transport: objectStorageRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"image/png"}},
			Body:          io.NopCloser(strings.NewReader("12345")),
			ContentLength: -1,
			Request:       request,
		}, nil
	})}
	store := newObjectStorageCOSTestStore(t, fakeCOS, remoteClient)

	_, err := store.copyRemoteObjectToCOS(context.Background(), "https://upstream.invalid/result.png", ObjectKeySpec{
		ObjectKey: "users/grok-results/42/image/2026/08/unknown-length.png",
		MediaType: "image",
		MaxBytes:  4,
		ExpiresAt: 1_786_472_000,
	})
	require.ErrorContains(t, err, "maximum")
	require.Empty(t, fakeCOS.objects)
}

type cancelingObjectStorageBody struct {
	cancel context.CancelFunc
	done   bool
}

func (b *cancelingObjectStorageBody) Read(p []byte) (int, error) {
	if b.done {
		return 0, context.Canceled
	}
	b.done = true
	copy(p, "chunk")
	b.cancel()
	return len("chunk"), nil
}

func (*cancelingObjectStorageBody) Close() error { return nil }

func TestObjectStorageCOSSignsExplicitBoundedTTLAndTreatsDeleteNotFoundAsSuccess(t *testing.T) {
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	store := newObjectStorageCOSTestStore(t, fakeCOS, http.DefaultClient)

	signed, err := store.signObjectURL(context.Background(), "users/grok-results/42/image/result.png", 90*time.Second)
	require.NoError(t, err)
	parsed, err := url.Parse(signed)
	require.NoError(t, err)
	parts := strings.Split(parsed.Query().Get("q-sign-time"), ";")
	require.Len(t, parts, 2)
	start, err := strconv.ParseInt(parts[0], 10, 64)
	require.NoError(t, err)
	end, err := strconv.ParseInt(parts[1], 10, 64)
	require.NoError(t, err)
	require.Equal(t, int64(90), end-start)
	_, err = store.signObjectURL(context.Background(), "users/key", 0)
	require.Error(t, err)
	_, err = store.signObjectURL(context.Background(), "users/key", 24*time.Hour+time.Second)
	require.Error(t, err)

	missingKey := "users/grok-results/42/image/missing.png"
	fakeCOS.deleteStatus[missingKey] = http.StatusNotFound
	require.NoError(t, store.deleteObject(context.Background(), missingKey))
	failingKey := "users/grok-results/42/image/transient.png"
	fakeCOS.deleteStatus[failingKey] = http.StatusServiceUnavailable
	require.Error(t, store.deleteObject(context.Background(), failingKey))
}
