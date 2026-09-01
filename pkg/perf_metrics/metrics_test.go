package perfmetrics

import (
	"encoding/json"
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

func TestQuerySummaryAllExposesExactModelSampleCounts(t *testing.T) {
	setupSummaryTestState(t)
	const now = int64(2_000_001_600)
	useFixedQueryClock(t, now)
	rows := []model.PerfMetric{
		{ModelName: "model-a", Group: "default", BucketTs: now - 3600, RequestCount: 3, SuccessCount: 2},
		{ModelName: "model-a", Group: "default", BucketTs: now, RequestCount: 2, SuccessCount: 1},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	result, err := QuerySummaryAll(24, nil)

	require.NoError(t, err)
	require.Len(t, result.Models, 1)
	assert.Equal(t, int64(5), result.Models[0].RequestCount)
	assert.Equal(t, int64(3), result.Models[0].SuccessCount)
	payload, err := json.Marshal(result.Models[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"model_name":"model-a",
		"avg_latency_ms":0,
		"success_rate":60,
		"avg_tps":0,
		"recent_success_rates":[66.67,50],
		"request_count":5,
		"success_count":3
	}`, string(payload))
}

func TestQuerySummaryAllRangeUsesExactCalendarBounds(t *testing.T) {
	setupSummaryTestState(t)
	const start = int64(2_000_001_600)
	const end = start + 2*3600 + 30*60
	require.Equal(t, start, bucketStart(start))
	rows := []model.PerfMetric{
		{ModelName: "outside-before", Group: "ByteDance", BucketTs: start - 3600, RequestCount: 9, SuccessCount: 9},
		{ModelName: "seedance", Group: "ByteDance", BucketTs: start, RequestCount: 2, SuccessCount: 1},
		{ModelName: "outside-after", Group: "ByteDance", BucketTs: start + 3*3600, RequestCount: 7, SuccessCount: 0},
	}
	require.NoError(t, model.DB.Create(&rows).Error)
	insideHotKey := bucketKey{model: "seedance", group: "ByteDance", bucketTs: start + 3600}
	insideHot := &atomicBucket{}
	insideHot.addCounters(counters{requestCount: 1, successCount: 1})
	hotBuckets.Store(insideHotKey, insideHot)

	result, err := QuerySummaryAllRange(start, end, []string{"ByteDance"})

	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "ByteDance", result.Groups[0].Group)
	assert.Equal(t, int64(3), result.Groups[0].RequestCount)
	require.NotNil(t, result.Groups[0].SuccessRate)
	assert.Equal(t, 66.67, *result.Groups[0].SuccessRate)
	require.Len(t, result.Models, 1)
	assert.Equal(t, "seedance", result.Models[0].ModelName)
}

func TestQuerySummaryAllAllowsAtMostOneYear(t *testing.T) {
	setupSummaryTestState(t)
	const fixedNow = int64(2_000_001_600) // Exactly divisible by the one-hour bucket size.
	require.Equal(t, fixedNow, bucketStart(fixedNow))
	useFixedQueryClock(t, fixedNow)
	rows := []model.PerfMetric{
		{ModelName: "inside-before-boundary", Group: "default", BucketTs: fixedNow - int64(8759*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "inside-at-boundary", Group: "default", BucketTs: fixedNow - int64(8760*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "outside-after-boundary", Group: "default", BucketTs: fixedNow - int64(8761*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	result, err := QuerySummaryAll(9000, nil)

	require.NoError(t, err)
	require.Len(t, result.Models, 2)
	assert.ElementsMatch(t, []string{"inside-before-boundary", "inside-at-boundary"}, []string{
		result.Models[0].ModelName,
		result.Models[1].ModelName,
	})
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "default", result.Groups[0].Group)
	assert.Equal(t, int64(2), result.Groups[0].RequestCount)
}

func TestQueryAllowsAtMostOneYear(t *testing.T) {
	setupSummaryTestState(t)
	const fixedNow = int64(2_000_001_600) // Exactly divisible by the one-hour bucket size.
	require.Equal(t, fixedNow, bucketStart(fixedNow))
	useFixedQueryClock(t, fixedNow)
	rows := []model.PerfMetric{
		{ModelName: "query-window", Group: "default", BucketTs: fixedNow - int64(8759*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "query-window", Group: "default", BucketTs: fixedNow - int64(8760*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "query-window", Group: "default", BucketTs: fixedNow - int64(8761*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	result, err := Query(QueryParams{Model: "query-window", Hours: 9000})

	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(2), result.Groups[0].RequestCount)
	require.Len(t, result.Groups[0].Series, 2)
	assert.Equal(t, fixedNow-int64(8760*time.Hour/time.Second), result.Groups[0].Series[0].Ts)
	assert.Equal(t, fixedNow-int64(8759*time.Hour/time.Second), result.Groups[0].Series[1].Ts)
}

func useFixedQueryClock(t *testing.T, now int64) {
	t.Helper()
	previous := currentUnix
	currentUnix = func() int64 { return now }
	t.Cleanup(func() { currentUnix = previous })
}

func setupSummaryTestState(t *testing.T) {
	t.Helper()
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "perf-metrics.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
	model.DB = db
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	clearHotBuckets()
	t.Cleanup(func() {
		clearHotBuckets()
		model.DB = previousDB
		model.LOG_DB = previousLogDB
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
