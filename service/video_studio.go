package service

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const videoStudioRequestContextKey = "video_studio_request_snapshot"

func SetVideoStudioRequestSnapshot(c *gin.Context, snapshot *model.VideoStudioRequestSnapshot) {
	if c != nil && snapshot != nil {
		c.Set(videoStudioRequestContextKey, snapshot)
	}
}

func VideoStudioRequestSnapshotFromContext(c *gin.Context) *model.VideoStudioRequestSnapshot {
	if c == nil {
		return nil
	}
	value, exists := c.Get(videoStudioRequestContextKey)
	if !exists {
		return nil
	}
	snapshot, _ := value.(*model.VideoStudioRequestSnapshot)
	return snapshot
}
