package service

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPNGBase64() string {
	return base64.StdEncoding.EncodeToString(append([]byte("\x89PNG\r\n\x1a\n"), []byte("image-body")...))
}

func TestObjectStorageCOSUploadsBoundedReaderWithDetectedMIME(t *testing.T) {
	fakeCOS := newObjectStorageCOSTestServer(t)
	store := newObjectStorageCOSTestStore(t, fakeCOS, nil)
	png := append([]byte("\x89PNG\r\n\x1a\n"), []byte("image-body")...)
	key := ObjectKeySpec{
		ObjectKey: "users/gpt-image-2-results/42/2026/08/result.png",
		MediaType: "image",
		MIMEType:  "image/png",
		MaxBytes:  int64(len(png)),
		ExpiresAt: 1_788_131_411,
	}

	stored, created, err := store.putReaderObjectToCOSWithStatus(context.Background(), strings.NewReader(string(png)), key)

	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, key.ObjectKey, stored.ObjectKey)
	assert.Equal(t, "image/png", stored.MIMEType)
	assert.EqualValues(t, len(png), stored.Size)
	assert.Equal(t, png, fakeCOS.objects[key.ObjectKey])

	_, _, err = store.putReaderObjectToCOSWithStatus(context.Background(), strings.NewReader(string(png)), ObjectKeySpec{
		ObjectKey: "users/gpt-image-2-results/42/2026/08/too-large.png",
		MediaType: "image",
		MaxBytes:  int64(len(png) - 1),
		ExpiresAt: key.ExpiresAt,
	})
	require.ErrorContains(t, err, "maximum size")
}

func TestPersistGPTImage2ResultsRegistersCleanupBeforeUploadAndExpiresIn24Hours(t *testing.T) {
	createdAt := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	var events []string
	deps := gptImage2ResultPersistenceDeps{
		put: func(_ context.Context, reader io.Reader, key ObjectKeySpec) (*StoredObject, bool, error) {
			events = append(events, "put")
			body, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, append([]byte("\x89PNG\r\n\x1a\n"), []byte("image-body")...), body)
			return &StoredObject{ObjectKey: key.ObjectKey, MIMEType: "image/png", Size: int64(len(body)), ExpiresAt: key.ExpiresAt}, true, nil
		},
		registerCleanup: func(objectKey string, expiresAt int64) error {
			events = append(events, "cleanup")
			require.Contains(t, objectKey, "/gpt-image-2-results/42/")
			require.Equal(t, createdAt.Add(24*time.Hour).Unix(), expiresAt)
			return nil
		},
	}

	objects, err := persistGPTImage2Results(context.Background(), GPTImage2PersistenceRequest{
		UserID:    42,
		RequestID: "req-image-2",
		CreatedAt: createdAt,
		Images:    []string{testPNGBase64()},
	}, deps)

	require.NoError(t, err)
	require.Len(t, objects, 1)
	assert.Equal(t, createdAt.Add(24*time.Hour).Unix(), objects[0].ExpiresAt)
	assert.Equal(t, []string{"cleanup", "put"}, events)
	assert.True(t, IsOwnedGPTImage2ResultObject(42, objects[0].ObjectKey))
	assert.False(t, IsOwnedGPTImage2ResultObject(7, objects[0].ObjectKey))
}

func TestPersistGPTImage2ResultsRejectsInvalidBase64AndTooManyImages(t *testing.T) {
	putCalls := 0
	deps := gptImage2ResultPersistenceDeps{
		put: func(context.Context, io.Reader, ObjectKeySpec) (*StoredObject, bool, error) {
			putCalls++
			return nil, false, errors.New("unexpected put")
		},
		registerCleanup: func(string, int64) error { return nil },
	}
	request := GPTImage2PersistenceRequest{
		UserID: 42, RequestID: "req-invalid", CreatedAt: time.Now(), Images: []string{"not-base64"},
	}

	_, err := persistGPTImage2Results(context.Background(), request, deps)
	require.ErrorContains(t, err, "base64")
	assert.Zero(t, putCalls)

	request.Images = make([]string, 11)
	for index := range request.Images {
		request.Images[index] = testPNGBase64()
	}
	_, err = persistGPTImage2Results(context.Background(), request, deps)
	require.ErrorContains(t, err, "image count")
	assert.Zero(t, putCalls)
}

func TestPersistGPTImage2ResultsLeavesRegisteredObjectForCleanupWhenUploadFails(t *testing.T) {
	createdAt := time.Now().UTC()
	registered := false
	deps := gptImage2ResultPersistenceDeps{
		put: func(context.Context, io.Reader, ObjectKeySpec) (*StoredObject, bool, error) {
			return nil, false, errors.New("cos unavailable")
		},
		registerCleanup: func(string, int64) error {
			registered = true
			return nil
		},
	}

	_, err := persistGPTImage2Results(context.Background(), GPTImage2PersistenceRequest{
		UserID: 42, RequestID: "req-failed", CreatedAt: createdAt, Images: []string{testPNGBase64()},
	}, deps)

	require.Error(t, err)
	assert.True(t, registered)
}
