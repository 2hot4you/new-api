package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFilesControllerDB(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "files-controller.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.MoliiFile{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })
}

func newFileControllerContext(method, target string, body io.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, body)
	c.Set("id", 42)
	return c, recorder
}

func TestFilesAPICreateUsesAuthenticatedUserAndReturnsOpenAIShape(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("purpose", "vision"))
	part, err := writer.CreateFormFile("file", "cat.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("image bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	original := createMoliiUserFile
	createMoliiUserFile = func(_ context.Context, userID int, filename, purpose string, _ io.Reader) (*model.MoliiFile, error) {
		assert.Equal(t, 42, userID)
		assert.Equal(t, "cat.png", filename)
		assert.Equal(t, "vision", purpose)
		return &model.MoliiFile{FileID: "file_created", Filename: filename, Purpose: purpose, Bytes: 11, MIMEType: "image/png", MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive, CreatedAt: 100, ExpiresAt: 200}, nil
	}
	t.Cleanup(func() { createMoliiUserFile = original })

	c, recorder := newFileControllerContext(http.MethodPost, "/v1/files", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	CreateFile(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"id":"file_created"`)
	assert.Contains(t, recorder.Body.String(), `"object":"file"`)
}

func TestFilesAPIRetrieveHidesForeignFilesAndMarksExpiredGone(t *testing.T) {
	setupFilesControllerDB(t)
	now := time.Now().Unix()
	for _, file := range []*model.MoliiFile{
		{FileID: "file_owned", UserID: 42, ObjectKey: "p/grok-files/42/file.png", Filename: "file.png", Purpose: "vision", Bytes: 1, MIMEType: "image/png", MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 60},
		{FileID: "file_expired", UserID: 42, ObjectKey: "p/grok-files/42/expired.png", Filename: "expired.png", Purpose: "vision", Bytes: 1, MIMEType: "image/png", MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive, CreatedAt: now - 120, UpdatedAt: now - 120, ExpiresAt: now - 1},
		{FileID: "file_foreign", UserID: 7, ObjectKey: "p/grok-files/7/file.png", Filename: "file.png", Purpose: "vision", Bytes: 1, MIMEType: "image/png", MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 60},
	} {
		require.NoError(t, model.CreateMoliiFile(context.Background(), file))
	}

	c, recorder := newFileControllerContext(http.MethodGet, "/v1/files/file_owned", nil)
	c.Params = gin.Params{{Key: "id", Value: "file_owned"}}
	RetrieveFile(c)
	assert.Equal(t, http.StatusOK, recorder.Code)

	c, recorder = newFileControllerContext(http.MethodGet, "/v1/files/file_foreign", nil)
	c.Params = gin.Params{{Key: "id", Value: "file_foreign"}}
	RetrieveFile(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	c, recorder = newFileControllerContext(http.MethodGet, "/v1/files/file_expired", nil)
	c.Params = gin.Params{{Key: "id", Value: "file_expired"}}
	RetrieveFile(c)
	assert.Equal(t, http.StatusGone, recorder.Code)
}

func TestFilesAPIContentStreamsRangesWithPrivateCache(t *testing.T) {
	setupFilesControllerDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.CreateMoliiFile(context.Background(), &model.MoliiFile{FileID: "file_video", UserID: 42, ObjectKey: "p/grok-files/42/file.mp4", Filename: "file.mp4", Purpose: "vision", Bytes: 4, MIMEType: "video/mp4", MediaType: model.MoliiFileMediaTypeVideo, Status: model.MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 60}))

	original := fetchMoliiFileContent
	fetchMoliiFileContent = func(_ context.Context, key, rangeHeader, _ string) (*http.Response, error) {
		assert.Equal(t, "p/grok-files/42/file.mp4", key)
		assert.Equal(t, "bytes=0-3", rangeHeader)
		return &http.Response{StatusCode: http.StatusPartialContent, Header: http.Header{"Content-Range": []string{"bytes 0-3/4"}, "Content-Length": []string{"4"}}, Body: io.NopCloser(strings.NewReader("data"))}, nil
	}
	t.Cleanup(func() { fetchMoliiFileContent = original })

	c, recorder := newFileControllerContext(http.MethodGet, "/v1/files/file_video/content", nil)
	c.Params = gin.Params{{Key: "id", Value: "file_video"}}
	c.Request.Header.Set("Range", "bytes=0-3")
	DownloadFile(c)

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	assert.Equal(t, "data", recorder.Body.String())
}

func TestFilesAPIListReturnsOnlyCurrentActiveFiles(t *testing.T) {
	setupFilesControllerDB(t)
	now := time.Now().Unix()
	require.NoError(t, model.CreateMoliiFile(context.Background(), &model.MoliiFile{FileID: "file_current", UserID: 42, ObjectKey: "p/grok-files/42/current.png", Filename: "current.png", Purpose: "vision", Bytes: 1, MIMEType: "image/png", MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 60}))
	require.NoError(t, model.CreateMoliiFile(context.Background(), &model.MoliiFile{FileID: "file_other", UserID: 7, ObjectKey: "p/grok-files/7/other.png", Filename: "other.png", Purpose: "vision", Bytes: 1, MIMEType: "image/png", MediaType: model.MoliiFileMediaTypeImage, Status: model.MoliiFileStatusActive, CreatedAt: now, UpdatedAt: now, ExpiresAt: now + 60}))

	c, recorder := newFileControllerContext(http.MethodGet, "/v1/files", nil)
	ListFiles(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	data := payload["data"].([]any)
	require.Len(t, data, 1)
	assert.Equal(t, "file_current", data[0].(map[string]any)["id"])
}
