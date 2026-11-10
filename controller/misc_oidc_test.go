package controller

import (
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type controlledStatusResponseWriter struct {
	*httptest.ResponseRecorder
	phase   string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *controlledStatusResponseWriter) Header() http.Header {
	if w.phase == "headers" {
		w.once.Do(func() { close(w.entered) })
		<-w.release
	}
	return w.ResponseRecorder.Header()
}

func (w *controlledStatusResponseWriter) Write(data []byte) (int, error) {
	if w.phase == "body" {
		w.once.Do(func() { close(w.entered) })
		<-w.release
	}
	return w.ResponseRecorder.Write(data)
}

func TestGetStatusReleasesOptionMapBeforeResponseIO(t *testing.T) {
	for _, tc := range []struct {
		phase       string
		contentType string
	}{
		{phase: "headers"},
		{phase: "body"},
		{phase: "headers", contentType: "application/custom+json"},
		{phase: "body", contentType: "application/custom+json"},
	} {
		t.Run(tc.phase+"/"+tc.contentType, func(t *testing.T) {
			common.OptionMapRWMutex.Lock()
			originalOptionMap, originalChats := common.OptionMap, setting.Chats
			common.OptionMap = map[string]string{"HeaderNavModules": "old-header", "SidebarModulesAdmin": "old-sidebar"}
			setting.Chats = []map[string]string{{"old-chat": "https://old.example/?a=1&b=<old>"}}
			common.OptionMapRWMutex.Unlock()
			t.Cleanup(func() {
				common.OptionMapRWMutex.Lock()
				common.OptionMap, setting.Chats = originalOptionMap, originalChats
				common.OptionMapRWMutex.Unlock()
			})

			baseline := httptest.NewRecorder()
			if tc.contentType != "" {
				baseline.Header().Set("Content-Type", tc.contentType)
			}
			baselineContext, _ := gin.CreateTestContext(baseline)
			baselineContext.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
			GetStatus(baselineContext)

			writer := &controlledStatusResponseWriter{
				ResponseRecorder: httptest.NewRecorder(), phase: tc.phase,
				entered: make(chan struct{}), release: make(chan struct{}),
			}
			if tc.contentType != "" {
				writer.ResponseRecorder.Header().Set("Content-Type", tc.contentType)
			}
			context, _ := gin.CreateTestContext(writer)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
			done := make(chan struct{})
			go func() {
				defer close(done)
				GetStatus(context)
			}()
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(writer.release) }) }
			t.Cleanup(func() {
				release()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("GetStatus did not finish after response writer release")
				}
			})
			select {
			case <-writer.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("GetStatus did not reach controlled response I/O")
			}

			writable := common.OptionMapRWMutex.TryLock()
			if assert.True(t, writable, "GetStatus retained OptionMap read lock during response %s", tc.phase) {
				common.OptionMap["HeaderNavModules"] = "new-header"
				common.OptionMap["SidebarModulesAdmin"] = "new-sidebar"
				setting.Chats[0]["old-chat"] = "https://new.example/"
				common.OptionMapRWMutex.Unlock()
			}
			release()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("GetStatus did not finish after response writer release")
			}

			assert.Equal(t, http.StatusOK, writer.Code)
			assert.Equal(t, baseline.Header(), writer.ResponseRecorder.Header())
			wantContentType := tc.contentType
			if wantContentType == "" {
				wantContentType = "application/json; charset=utf-8"
			}
			assert.Equal(t, wantContentType, writer.ResponseRecorder.Header().Get("Content-Type"))
			assert.Empty(t, writer.ResponseRecorder.Header().Get("Content-Length"))
			assert.Equal(t, baseline.Body.String(), writer.Body.String(), "the entire response must retain its pre-I/O snapshot")
			var payload struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
				Data    struct {
					HeaderNavModules    string              `json:"HeaderNavModules"`
					SidebarModulesAdmin string              `json:"SidebarModulesAdmin"`
					Chats               []map[string]string `json:"chats"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(writer.Body.Bytes(), &payload))
			assert.True(t, payload.Success)
			assert.Empty(t, payload.Message)
			assert.Equal(t, "old-header", payload.Data.HeaderNavModules)
			assert.Equal(t, "old-sidebar", payload.Data.SidebarModulesAdmin)
			assert.Equal(t, []map[string]string{{"old-chat": "https://old.example/?a=1&b=<old>"}}, payload.Data.Chats)
			t.Logf("response status=%d headers=%v bytes=%s", writer.Code, writer.ResponseRecorder.Header(), writer.Body.String())
		})
	}
}

func TestGetStatusMarshalFailureReleasesOptionMapBeforeResponseHandling(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = math.NaN()
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	writer := &controlledStatusResponseWriter{
		ResponseRecorder: httptest.NewRecorder(), phase: "headers",
		entered: make(chan struct{}), release: make(chan struct{}),
	}
	context, _ := gin.CreateTestContext(writer)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		GetStatus(context)
	}()
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(writer.release) }) }
	t.Cleanup(func() {
		release()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("GetStatus marshal failure did not finish")
		}
	})
	select {
	case <-writer.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("GetStatus marshal failure did not reach response handling")
	}
	writable := common.OptionMapRWMutex.TryLock()
	if writable {
		common.OptionMapRWMutex.Unlock()
	}
	assert.True(t, writable, "GetStatus retained OptionMap read lock while handling marshal failure")
	release()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("GetStatus marshal failure did not finish")
	}
	assert.Equal(t, http.StatusOK, context.Writer.Status())
	assert.Equal(t, "application/json; charset=utf-8", writer.ResponseRecorder.Header().Get("Content-Type"))
	assert.Empty(t, writer.Body.String())
	assert.Empty(t, writer.ResponseRecorder.Header().Get("Content-Length"))
	assert.True(t, context.IsAborted())
	require.Len(t, context.Errors, 1)
	assert.Contains(t, context.Errors[0].Error(), "unsupported value: NaN")
	require.True(t, common.OptionMapRWMutex.TryLock(), "marshal failure leaked the OptionMap lock")
	common.OptionMapRWMutex.Unlock()
	t.Logf("marshal failure preserved status=%d headers=%v empty body and Gin abort/error; OptionMap writable", context.Writer.Status(), writer.ResponseRecorder.Header())
}

func TestGetStatusReturnsEffectiveOIDCDisplayName(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	originalDisplayName := settings.DisplayName
	originalOptionMap := common.OptionMap
	t.Cleanup(func() {
		settings.DisplayName = originalDisplayName
		common.OptionMap = originalOptionMap
	})
	common.OptionMap = map[string]string{}

	tests := []struct {
		name        string
		displayName string
		want        string
	}{
		{
			name:        "custom name is trimmed",
			displayName: "  Acme SSO  ",
			want:        "Acme SSO",
		},
		{
			name:        "whitespace-only name falls back",
			displayName: "   ",
			want:        "OIDC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.DisplayName = tt.displayName
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

			GetStatus(context)

			var payload struct {
				Success bool           `json:"success"`
				Data    map[string]any `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.True(t, payload.Success)
			assert.Equal(t, tt.want, payload.Data["oidc_display_name"])
		})
	}
}
