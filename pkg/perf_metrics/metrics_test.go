package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildQueryResultIncludesRequestCounts(t *testing.T) {
	result := buildQueryResult("grok-imagine-image", map[bucketKey]counters{
		{model: "grok-imagine-image", group: "default", bucketTs: 100}: {
			requestCount: 3, successCount: 2, totalLatencyMs: 900,
		},
		{model: "grok-imagine-image", group: "default", bucketTs: 200}: {
			requestCount: 2, successCount: 2, totalLatencyMs: 400,
		},
	})

	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(5), result.Groups[0].RequestCount)
	require.Len(t, result.Groups[0].Series, 2)
	assert.Equal(t, int64(3), result.Groups[0].Series[0].RequestCount)
	assert.Equal(t, int64(2), result.Groups[0].Series[1].RequestCount)
}
