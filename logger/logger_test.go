package logger

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func TestLogHelperConcurrentState(t *testing.T) {
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	previousErrorWriter := gin.DefaultErrorWriter
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		gin.DefaultErrorWriter = previousErrorWriter
		common.LogWriterMu.Unlock()
	})

	previousCount := logCount.Load()
	previousSetupWorking := setupLogWorking.Load()
	logCount.Store(0)
	setupLogWorking.Store(false)
	t.Cleanup(func() {
		logCount.Store(previousCount)
		setupLogWorking.Store(previousSetupWorking)
	})
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				LogInfo(context.Background(), "concurrent logger state")
			}
		}()
	}
	wg.Wait()
}
