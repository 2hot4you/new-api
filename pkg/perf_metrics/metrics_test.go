package perfmetrics

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

func TestQuerySummaryAllBuildsWeightedGroupSummaries(t *testing.T) {
	now := time.Now().Unix()
	previousBucket := bucketStart(now) - 3600
	olderBucket := previousBucket - 3600

	tests := []struct {
		name   string
		rows   []model.PerfMetric
		hot    map[bucketKey]counters
		groups []string
		want   []GroupSummary
	}{
		{
			name: "weights raw counts across models and persisted and hot buckets",
			rows: []model.PerfMetric{
				{ModelName: "model-a", Group: "enterprise", BucketTs: olderBucket, RequestCount: 1, SuccessCount: 1},
				{ModelName: "model-b", Group: "enterprise", BucketTs: previousBucket, RequestCount: 9, SuccessCount: 0},
				{ModelName: "model-a", Group: "default", BucketTs: previousBucket, RequestCount: 2, SuccessCount: 0},
			},
			hot: map[bucketKey]counters{
				{model: "model-a", group: "enterprise", bucketTs: bucketStart(now)}: {
					requestCount: 2,
					successCount: 2,
				},
			},
			groups: []string{"unused", "enterprise", "default"},
			want: []GroupSummary{
				{Group: "default", RequestCount: 2, SuccessRate: ratePointer(0)},
				{Group: "enterprise", RequestCount: 12, SuccessRate: ratePointer(25)},
				{Group: "unused", RequestCount: 0, SuccessRate: nil},
			},
		},
		{
			name:   "returns configured groups without samples in deterministic order",
			groups: []string{"zeta", "alpha"},
			want: []GroupSummary{
				{Group: "alpha", RequestCount: 0, SuccessRate: nil},
				{Group: "zeta", RequestCount: 0, SuccessRate: nil},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupSummaryTestState(t)
			if len(test.rows) > 0 {
				require.NoError(t, model.DB.Create(&test.rows).Error)
			}
			for key, value := range test.hot {
				bucket := &atomicBucket{}
				bucket.addCounters(value)
				hotBuckets.Store(key, bucket)
			}

			result, err := QuerySummaryAll(24, test.groups)

			require.NoError(t, err)
			assert.Equal(t, test.want, result.Groups)
		})
	}
}

func TestQuerySummaryAllAllowsAtMostOneYear(t *testing.T) {
	setupSummaryTestState(t)
	now := time.Now()
	rows := []model.PerfMetric{
		{ModelName: "inside-one-year", Group: "default", BucketTs: now.Add(-8000 * time.Hour).Unix(), RequestCount: 1, SuccessCount: 1},
		{ModelName: "outside-one-year", Group: "default", BucketTs: now.Add(-8800 * time.Hour).Unix(), RequestCount: 1, SuccessCount: 1},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	result, err := QuerySummaryAll(9000, nil)

	require.NoError(t, err)
	require.Len(t, result.Models, 1)
	assert.Equal(t, "inside-one-year", result.Models[0].ModelName)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "default", result.Groups[0].Group)
}

func TestQueryAllowsAtMostOneYear(t *testing.T) {
	setupSummaryTestState(t)
	now := time.Now()
	rows := []model.PerfMetric{
		{ModelName: "query-window", Group: "default", BucketTs: now.Add(-8000 * time.Hour).Unix(), RequestCount: 1, SuccessCount: 1},
		{ModelName: "query-window", Group: "default", BucketTs: now.Add(-8800 * time.Hour).Unix(), RequestCount: 1, SuccessCount: 1},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	result, err := Query(QueryParams{Model: "query-window", Hours: 9000})

	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(1), result.Groups[0].RequestCount)
}

func setupSummaryTestState(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "perf-metrics.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
	model.DB = db
	clearHotBuckets()
	t.Cleanup(func() {
		clearHotBuckets()
		model.DB = previousDB
	})
}

func clearHotBuckets() {
	hotBuckets.Range(func(key, _ any) bool {
		hotBuckets.Delete(key)
		return true
	})
}

func ratePointer(rate float64) *float64 {
	return &rate
}
