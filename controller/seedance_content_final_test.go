package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestByteDanceSeedanceDashboardNativeArtifactPlayback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/videos/task_molii_public/content", r.URL.Path)
		require.Equal(t, "Bearer instance-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = io.WriteString(w, "native-video")
	}))
	defer upstream.Close()
	task := setupByteDanceSeedanceVideoProxy(t, upstream.URL, "https://private.invalid/signed")
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/task/"+task.TaskID+"/artifacts", nil)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Set("id", 999)
	c.Set("role", common.RoleAdminUser)
	GetDashboardTaskArtifacts(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Artifacts []taskArtifactResponse `json:"artifacts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data.Artifacts, 1)
	require.Equal(t, "video", response.Data.Artifacts[0].Key)
	require.NotContains(t, recorder.Body.String(), "private.invalid")
	require.NotContains(t, recorder.Body.String(), "task_molii_public")
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, response.Data.Artifacts[0].ContentURL, nil)
	c.Params = gin.Params{{Key: "key", Value: task.TaskID}, {Key: "artifact_key", Value: "video"}}
	c.Set("id", 999)
	c.Set("role", common.RoleAdminUser)
	TaskArtifactContent(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "native-video", recorder.Body.String())
	// A type-64 task never needs the legacy signed/result URL.
	task.PrivateData.ResultURL = ""
	require.NoError(t, model.DB.Save(task).Error)
	artifacts, err := projectTaskArtifacts(task)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
}

func TestByteDanceSeedanceContentDispositionGETAndHEAD(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, download := range []bool{false, true} {
			t.Run(method+map[bool]string{false: "inline", true: "download"}[download], func(t *testing.T) {
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, method, r.Method)
					w.Header().Set("Content-Type", "video/mp4")
					w.Header().Set("Content-Disposition", `attachment; filename="task_molii_private.mp4"`)
					_, _ = io.WriteString(w, "video")
				}))
				defer upstream.Close()
				task := setupByteDanceSeedanceVideoProxy(t, upstream.URL, "")
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				path := "/v1/videos/" + task.TaskID + "/content"
				if download {
					path += "?download=1"
				}
				c.Request = httptest.NewRequest(method, path, nil)
				c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
				c.Set("id", task.UserId)
				VideoProxy(c)
				require.Equal(t, http.StatusOK, recorder.Code)
				want := `inline; filename="video.mp4"`
				if download {
					want = `attachment; filename="video.mp4"`
				}
				require.Equal(t, want, recorder.Header().Get("Content-Disposition"))
				if method == http.MethodHead {
					require.Empty(t, recorder.Body.String())
				}
			})
		}
	}
}
