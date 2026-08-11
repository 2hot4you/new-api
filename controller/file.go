package controller

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	apiDTO "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const moliiFileMultipartMaxBytes = int64(34 * 1024 * 1024)

var (
	createMoliiUserFile   = service.CreateUserFile
	fetchMoliiFileContent = service.FetchCOSObject
)

func fileAPIError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message, "type": "invalid_request_error"}})
}

func newFileObject(file *model.MoliiFile) apiDTO.FileObject {
	if file == nil {
		return apiDTO.FileObject{Object: "file"}
	}
	return apiDTO.FileObject{
		ID: file.FileID, Object: "file", Bytes: file.Bytes, CreatedAt: file.CreatedAt,
		ExpiresAt: file.ExpiresAt, Filename: file.Filename, Purpose: file.Purpose, MIMEType: file.MIMEType,
	}
}

func handleFileServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrMoliiFileNotFound):
		fileAPIError(c, http.StatusNotFound, "file_not_found", "File not found")
	case errors.Is(err, model.ErrMoliiFileExpired):
		fileAPIError(c, http.StatusGone, "file_expired", "File has expired")
	case errors.Is(err, service.ErrMoliiFileInvalidPurpose):
		fileAPIError(c, http.StatusBadRequest, "invalid_purpose", "purpose must contain only letters, numbers, dot, underscore or hyphen")
	case errors.Is(err, service.ErrMoliiFileUnsupportedMedia):
		fileAPIError(c, http.StatusBadRequest, "unsupported_file", "Only PNG, JPEG, WebP and MP4 files are supported")
	case errors.Is(err, service.ErrMoliiFileEmpty):
		fileAPIError(c, http.StatusBadRequest, "empty_file", "File is empty")
	case errors.Is(err, service.ErrMoliiFileTooLarge):
		fileAPIError(c, http.StatusRequestEntityTooLarge, "file_too_large", "File exceeds the maximum supported size")
	default:
		fileAPIError(c, http.StatusServiceUnavailable, "file_service_unavailable", "File service is temporarily unavailable")
	}
}

func CreateFile(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, moliiFileMultipartMaxBytes)
	header, err := c.FormFile("file")
	if err != nil {
		fileAPIError(c, http.StatusBadRequest, "file_required", "file is required")
		return
	}
	purpose := strings.TrimSpace(c.PostForm("purpose"))
	if purpose == "" {
		fileAPIError(c, http.StatusBadRequest, "purpose_required", "purpose is required")
		return
	}
	content, err := header.Open()
	if err != nil {
		fileAPIError(c, http.StatusBadRequest, "invalid_file", "File could not be read")
		return
	}
	defer content.Close()
	file, err := createMoliiUserFile(c.Request.Context(), c.GetInt("id"), header.Filename, purpose, content)
	if err != nil {
		handleFileServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, newFileObject(file))
}

func ListFiles(c *gin.Context) {
	files, err := service.ListUserFiles(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		handleFileServiceError(c, err)
		return
	}
	data := make([]apiDTO.FileObject, 0, len(files))
	for _, file := range files {
		data = append(data, newFileObject(file))
	}
	c.JSON(http.StatusOK, apiDTO.FileList{Object: "list", Data: data})
}

func RetrieveFile(c *gin.Context) {
	file, err := model.GetMoliiFileForUser(c.Request.Context(), c.GetInt("id"), c.Param("id"), time.Now().Unix())
	if err != nil {
		handleFileServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, newFileObject(file))
}

func DeleteFile(c *gin.Context) {
	file, err := service.DeleteUserFile(c.Request.Context(), c.GetInt("id"), c.Param("id"))
	if err != nil {
		handleFileServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, apiDTO.FileDeleted{ID: file.FileID, Object: "file", Deleted: true})
}

func DownloadFile(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	file, err := model.GetMoliiFileForUser(c.Request.Context(), c.GetInt("id"), c.Param("id"), time.Now().Unix())
	if err != nil {
		handleFileServiceError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	response, err := fetchMoliiFileContent(ctx, file.ObjectKey, c.GetHeader("Range"), c.GetHeader("If-Range"))
	if err != nil || response == nil || response.Body == nil {
		fileAPIError(c, http.StatusBadGateway, "file_content_unavailable", "File content is temporarily unavailable")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent && response.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		fileAPIError(c, http.StatusBadGateway, "file_content_unavailable", "File content is temporarily unavailable")
		return
	}
	for _, key := range []string{"Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified"} {
		if value := response.Header.Get(key); value != "" {
			c.Header(key, value)
		}
	}
	c.Header("Content-Type", file.MIMEType)
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": file.Filename})
	if disposition == "" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", disposition)
	c.Status(response.StatusCode)
	if response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return
	}
	_, _ = io.Copy(c.Writer, response.Body)
}
