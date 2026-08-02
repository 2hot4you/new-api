package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const starAIResultCleanupBatchSize = 500

type StarAIResultCleanupResult struct {
	ClearedCount   int64 `json:"cleared_count"`
	RetentionHours int   `json:"retention_hours"`
}

func CleanupExpiredStarAIResultMetadata(ctx context.Context) (StarAIResultCleanupResult, error) {
	result := StarAIResultCleanupResult{RetentionHours: constant.StarAIResultRetentionHours}
	if constant.StarAIResultRetentionHours <= 0 {
		return result, nil
	}
	cutoff := time.Now().Add(-time.Duration(constant.StarAIResultRetentionHours) * time.Hour).Unix()
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		cleared, err := model.ClearExpiredStarAIResultMetadata(cutoff, starAIResultCleanupBatchSize)
		result.ClearedCount += cleared
		if err != nil || cleared < starAIResultCleanupBatchSize {
			return result, err
		}
	}
}
