package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageResultProxyServesBytesWithoutUpstreamMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/proxy/opaque", nil)
	c.Request.Header.Set("Authorization", "Bearer client-secret")
	c.Params = gin.Params{{Key: "token", Value: "opaque"}}
	image := []byte("validated raster bytes")
	serveImageResultProxy(c,
		func(token string) (string, error) {
			assert.Equal(t, "opaque", token)
			return "https://private-provider.example/result?secret=source-secret", nil
		},
		func(ctx context.Context, source string) ([]byte, string, error) {
			assert.Equal(t, c.Request.Context(), ctx)
			assert.Equal(t, "https://private-provider.example/result?secret=source-secret", source)
			return image, "image/png", nil
		},
	)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, image, recorder.Body.Bytes())
	assert.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, recorder.Header().Get("Location"))
	assert.Empty(t, recorder.Header().Get("Set-Cookie"))
	assert.NotContains(t, recorder.Header(), "Authorization")
}

func TestImageResultProxySanitizesFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		enabled    string
		resolveErr error
		fetchErr   error
		status     int
		code       string
	}{
		{"disabled", "false", nil, nil, http.StatusNotFound, "image_link_invalid"},
		{"invalid", "true", service.ErrImageResultProxyTokenInvalid, nil, http.StatusNotFound, "image_link_invalid"},
		{"expired", "true", service.ErrImageResultProxyExpired, nil, http.StatusGone, "image_link_expired"},
		{"fetch failed", "true", nil, errors.New("Get https://private-provider.example/secret: failed"), http.StatusBadGateway, "image_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IMAGE_RESULT_PROXY_ENABLED", tc.enabled)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/proxy/token", nil)
			resolveCalled, fetchCalled := false, false
			serveImageResultProxy(c, func(string) (string, error) {
				resolveCalled = true
				return "https://private-provider.example/secret", tc.resolveErr
			}, func(context.Context, string) ([]byte, string, error) {
				fetchCalled = true
				return nil, "", tc.fetchErr
			})
			assert.Equal(t, tc.status, recorder.Code)
			assert.Contains(t, recorder.Body.String(), tc.code)
			assert.NotContains(t, recorder.Body.String(), "private-provider")
			assert.NotContains(t, recorder.Body.String(), "secret")
			assert.Empty(t, recorder.Header().Get("Location"))
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			assert.Equal(t, tc.enabled == "true", resolveCalled)
			assert.Equal(t, tc.enabled == "true" && tc.resolveErr == nil, fetchCalled)
		})
	}
}

func imageProxyConcurrencyContext(writer http.ResponseWriter) *gin.Context {
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/proxy/opaque", nil)
	return c
}

func TestImageResultProxyLimitsConcurrentDownloadsAndReleasesSlots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			var releaseOnce sync.Once
			var workers sync.WaitGroup
			defer func() { releaseOnce.Do(func() { close(release) }); workers.Wait() }()
			resolve := func(string) (string, error) { return "https://images.example/image", nil }
			recorders := []*httptest.ResponseRecorder{httptest.NewRecorder(), httptest.NewRecorder()}
			for _, recorder := range recorders {
				c := imageProxyConcurrencyContext(recorder)
				workers.Add(1)
				go func() {
					defer workers.Done()
					serveImageResultProxy(c, resolve, func(context.Context, string) ([]byte, string, error) {
						entered <- struct{}{}
						<-release
						if fail {
							return nil, "", errors.New("upstream secret")
						}
						return []byte("image bytes"), "image/png", nil
					})
				}()
			}
			<-entered
			<-entered
			rejected := httptest.NewRecorder()
			fetched := false
			serveImageResultProxy(imageProxyConcurrencyContext(rejected), resolve, func(context.Context, string) ([]byte, string, error) {
				fetched = true
				return []byte("unexpected"), "image/png", nil
			})
			assert.Equal(t, http.StatusServiceUnavailable, rejected.Code)
			assert.Contains(t, rejected.Body.String(), "image_proxy_busy")
			assert.Equal(t, "3", rejected.Header().Get("Retry-After"))
			assert.False(t, fetched)

			// Authentication errors retain their status even when all slots are occupied.
			invalid := httptest.NewRecorder()
			serveImageResultProxy(imageProxyConcurrencyContext(invalid), func(string) (string, error) { return "", service.ErrImageResultProxyTokenInvalid }, func(context.Context, string) ([]byte, string, error) {
				t.Error("invalid token must not fetch")
				return nil, "", nil
			})
			assert.Equal(t, http.StatusNotFound, invalid.Code)
			assert.Empty(t, invalid.Header().Get("Retry-After"))

			releaseOnce.Do(func() { close(release) })
			workers.Wait()
			for _, recorder := range recorders {
				if fail {
					assert.Equal(t, http.StatusBadGateway, recorder.Code)
				} else {
					assert.Equal(t, http.StatusOK, recorder.Code)
					assert.Equal(t, "image bytes", recorder.Body.String())
				}
			}
			// Both slots must be reusable after either outcome.
			for range 2 {
				next := httptest.NewRecorder()
				serveImageResultProxy(imageProxyConcurrencyContext(next), resolve, func(context.Context, string) ([]byte, string, error) { return []byte("next"), "image/png", nil })
				assert.Equal(t, http.StatusOK, next.Code)
				assert.Equal(t, "next", next.Body.String())
			}
		})
	}
}

type imageProxyBlockingWriter struct {
	*httptest.ResponseRecorder
	entered chan struct{}
	release chan struct{}
}

func (w *imageProxyBlockingWriter) Write(body []byte) (int, error) {
	close(w.entered)
	<-w.release
	return w.ResponseRecorder.Write(body)
}

func TestImageResultProxyHoldsSlotUntilResponseWriteCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	writers := []*imageProxyBlockingWriter{
		{httptest.NewRecorder(), make(chan struct{}), make(chan struct{})},
		{httptest.NewRecorder(), make(chan struct{}), make(chan struct{})},
	}
	var workers sync.WaitGroup
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			for _, writer := range writers {
				close(writer.release)
			}
		})
	}
	defer func() { release(); workers.Wait() }()
	resolve := func(string) (string, error) { return "https://images.example/image", nil }
	for _, writer := range writers {
		c := imageProxyConcurrencyContext(writer)
		workers.Add(1)
		go func() {
			defer workers.Done()
			serveImageResultProxy(c, resolve, func(context.Context, string) ([]byte, string, error) { return []byte("image"), "image/png", nil })
		}()
	}
	for _, writer := range writers {
		<-writer.entered
	}
	rejected := httptest.NewRecorder()
	fetched := false
	serveImageResultProxy(imageProxyConcurrencyContext(rejected), resolve, func(context.Context, string) ([]byte, string, error) { fetched = true; return nil, "", nil })
	assert.Equal(t, http.StatusServiceUnavailable, rejected.Code)
	assert.False(t, fetched)
	release()
	workers.Wait()
	for _, writer := range writers {
		assert.Equal(t, "image", writer.Body.String())
	}
}

type imageProxyDeadlineWriter struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
	events    []string
	writeErr  error
}

func (w *imageProxyDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlines = append(w.deadlines, deadline)
	w.events = append(w.events, "deadline")
	return nil
}

func (w *imageProxyDeadlineWriter) Write(body []byte) (int, error) {
	w.events = append(w.events, "write")
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.ResponseRecorder.Write(body)
}

func TestImageResultProxySetsAndClearsResponseWriteDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("IMAGE_RESULT_PROXY_ENABLED", "true")
	for _, tc := range []struct {
		name     string
		writeErr error
	}{
		{"success", nil},
		{"write fails", errors.New("write failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &imageProxyDeadlineWriter{ResponseRecorder: httptest.NewRecorder(), writeErr: tc.writeErr}
			before := time.Now()
			serveImageResultProxy(imageProxyConcurrencyContext(writer), func(string) (string, error) { return "https://images.example/image", nil }, func(context.Context, string) ([]byte, string, error) { return []byte("image"), "image/png", nil })
			after := time.Now()
			assert.Equal(t, []string{"deadline", "write", "deadline"}, writer.events)
			require.Len(t, writer.deadlines, 2)
			assert.False(t, writer.deadlines[0].Before(before.Add(60*time.Second)))
			assert.False(t, writer.deadlines[0].After(after.Add(60*time.Second)))
			assert.True(t, writer.deadlines[1].IsZero())
		})
	}
}
