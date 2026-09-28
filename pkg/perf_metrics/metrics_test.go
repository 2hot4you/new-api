package perfmetrics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
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

func TestClassifyRelayOutcome(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		err  *types.NewAPIError
		want Outcome
	}{
		{"success", context.Background(), nil, OutcomeSuccess},
		{"upstream bad request", context.Background(), types.InitOpenAIError("unknown", 400), OutcomeIgnored},
		{"upstream credentials returned as 400", context.Background(), types.InitOpenAIError("invalid_api_key", 400), OutcomeFailure},
		{"wrapped credentials keep their cause", context.Background(), types.NewErrorWithStatusCode(types.InitOpenAIError("invalid_api_key", 400), types.ErrorCodeInvalidRequest, 400), OutcomeFailure},
		{"upstream context limit returned as 500", context.Background(), types.InitOpenAIError("context_length_exceeded", 500), OutcomeIgnored},
		{"local rate limit", context.Background(), types.NewErrorWithStatusCode(errors.New("limited"), types.ErrorCodeInvalidRequest, 429), OutcomeIgnored},
		{"upstream rate limit", context.Background(), types.InitOpenAIError("rate_limit_exceeded", 429), OutcomeFailure},
		{"local quota", context.Background(), types.NewError(errors.New("quota"), types.ErrorCodeInsufficientUserQuota), OutcomeIgnored},
		{"local violation fee", context.Background(), types.NewError(errors.New("csam"), types.ErrorCodeViolationFeeGrokCSAM), OutcomeIgnored},
		{"upstream quota", context.Background(), types.InitOpenAIError("insufficient_quota", 429), OutcomeFailure},
		{"upstream gateway quota", context.Background(), types.InitOpenAIError(types.ErrorCodeInsufficientUserQuota, 403), OutcomeFailure},
		{"unavailable channel", context.Background(), types.NewErrorWithStatusCode(errors.New("disabled"), types.ErrorCodeGetChannelFailed, 403), OutcomeFailure},
		{"empty upstream response", context.Background(), types.NewError(errors.New("empty"), types.ErrorCodeEmptyResponse), OutcomeFailure},
		{"network failure", context.Background(), types.NewOpenAIError(errors.New("connection refused"), types.ErrorCodeDoRequestFailed, 500), OutcomeFailure},
		{"client cancellation", canceled, types.NewOpenAIError(errors.New("context canceled"), types.ErrorCodeDoRequestFailed, 500), OutcomeIgnored},
		{"upstream deadline", context.Background(), types.NewOpenAIError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed, 504), OutcomeFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ClassifyRelayOutcome(tc.ctx, &relaycommon.RelayInfo{}, tc.err))
		})
	}
	assert.Equal(t, OutcomeIgnored, ClassifyRelayOutcome(context.Background(), &relaycommon.RelayInfo{PerformanceBusinessRejection: true}, nil))
}

func TestStreamOutcomeClassification(t *testing.T) {
	for _, tc := range []struct {
		name string
		mark func(*relaycommon.StreamStatus)
		end  relaycommon.StreamEndReason
		want Outcome
	}{
		{"completed", (*relaycommon.StreamStatus).MarkCompleted, relaycommon.StreamEndReasonEOF, OutcomeSuccess},
		{"business rejection", func(s *relaycommon.StreamStatus) { s.MarkFailed("context_length_exceeded", "", 0) }, relaycommon.StreamEndReasonEOF, OutcomeIgnored},
		{"service error", func(s *relaycommon.StreamStatus) { s.MarkFailed("server_error", "", 0) }, relaycommon.StreamEndReasonEOF, OutcomeFailure},
		{"error after completion", func(s *relaycommon.StreamStatus) { s.MarkCompleted(); s.MarkFailed("", "server_error", 0) }, relaycommon.StreamEndReasonEOF, OutcomeFailure},
		{"output limit", func(s *relaycommon.StreamStatus) { s.MarkIncomplete("max_output_tokens") }, relaycommon.StreamEndReasonEOF, OutcomeSuccess},
		{"content filter", func(s *relaycommon.StreamStatus) { s.MarkIncomplete("content_filter") }, relaycommon.StreamEndReasonEOF, OutcomeIgnored},
		{"client cancel", (*relaycommon.StreamStatus).MarkCancelled, relaycommon.StreamEndReasonEOF, OutcomeIgnored},
		{"client gone", nil, relaycommon.StreamEndReasonClientGone, OutcomeIgnored},
		{"timeout", nil, relaycommon.StreamEndReasonTimeout, OutcomeFailure},
		{"missing terminal", nil, relaycommon.StreamEndReasonEOF, OutcomeFailure},
		{"done marker without terminal", nil, relaycommon.StreamEndReasonDone, OutcomeSuccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := relaycommon.NewStreamStatus()
			stream.RequireTerminal()
			if tc.mark != nil {
				tc.mark(stream)
			}
			stream.SetEndReason(tc.end, nil)
			assert.Equal(t, tc.want, ClassifyRelayOutcome(context.Background(), &relaycommon.RelayInfo{StreamStatus: stream}, nil))
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
		"recent_success_series":[{"ts":1999998000,"success_rate":66.67},{"ts":2000001600,"success_rate":50}],
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
		{ModelName: "inside-before-boundary", Group: "default", BucketTs: fixedNow - int64(8758*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "inside-at-boundary", Group: "default", BucketTs: fixedNow - int64(8759*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "outside-after-boundary", Group: "default", BucketTs: fixedNow - int64(8760*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
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
		{ModelName: "query-window", Group: "default", BucketTs: fixedNow - int64(8758*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "query-window", Group: "default", BucketTs: fixedNow - int64(8759*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
		{ModelName: "query-window", Group: "default", BucketTs: fixedNow - int64(8760*time.Hour/time.Second), RequestCount: 1, SuccessCount: 1},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	result, err := Query(QueryParams{Model: "query-window", Hours: 9000})

	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, int64(2), result.Groups[0].RequestCount)
	require.Len(t, result.Groups[0].Series, 2)
	assert.Equal(t, fixedNow-int64(8759*time.Hour/time.Second), result.Groups[0].Series[0].Ts)
	assert.Equal(t, fixedNow-int64(8758*time.Hour/time.Second), result.Groups[0].Series[1].Ts)
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

func TestClientCancellationDuringUpstreamRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := relaycommon.NewStreamStatus()
	stream.RequireTerminal()
	stream.SetEndReason(relaycommon.StreamEndReasonScannerErr, errors.New("reader closed"))
	assert.Equal(t, OutcomeIgnored, ClassifyRelayOutcome(ctx, &relaycommon.RelayInfo{StreamStatus: stream}, nil))

	deadline := relaycommon.NewStreamStatus()
	deadline.SetEndReason(relaycommon.StreamEndReasonClientGone, context.DeadlineExceeded)
	assert.Equal(t, OutcomeFailure, ClassifyRelayOutcome(context.Background(), &relaycommon.RelayInfo{StreamStatus: deadline}, nil))
}

func TestPerformanceWindowIncludesCurrentHour(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 37, 0, 0, time.UTC)
	start, end := queryWindow(now, 24)
	assert.Equal(t, time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC).Unix(), start)
	assert.Equal(t, now.Unix(), end)
}

func TestHourlySuccessSeriesWeightsSmallerBuckets(t *testing.T) {
	points := recentSuccessSeries(map[int64]counters{
		3600: {requestCount: 100, successCount: 100},
		3900: {requestCount: 1},
		7200: {requestCount: 1, successCount: 1},
	})
	assert.Equal(t, []SuccessRatePoint{{Ts: 3600, SuccessRate: 99.01}, {Ts: 7200, SuccessRate: 100}}, points)
}

// Terminal task sampling: success/failure counts, end-to-end latency, and
// token throughput only for successful tasks that report tokens.
func TestRecordTaskResultSamplesTerminalTasks(t *testing.T) {
	hotBuckets.Clear()
	t.Cleanup(func() { hotBuckets.Clear() })
	now := time.Now().Unix()
	RecordTaskResult(&model.Task{
		Status:     model.TaskStatusSuccess,
		Group:      "a",
		SubmitTime: now - 120,
		StartTime:  now - 100,
		FinishTime: now,
		Properties: model.Properties{OriginModelName: "video-model"},
	}, &relaycommon.TaskInfo{TotalTokens: 5000})
	RecordTaskResult(&model.Task{
		Status:     model.TaskStatusFailure,
		Group:      "a",
		SubmitTime: now - 60,
		FinishTime: now,
		Properties: model.Properties{OriginModelName: "video-model"},
	}, relaycommon.FailTaskInfo("boom"))
	RecordTaskResult(&model.Task{Status: model.TaskStatusSuccess, FinishTime: now}, nil)

	merged := map[bucketKey]counters{}
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		require.Equal(t, "video-model", k.model)
		require.Equal(t, "a", k.group)
		k.bucketTs = 0
		mergeCounters(merged, k, value.(*atomicBucket).snapshot())
		return true
	})
	assert.Equal(t, counters{
		requestCount:   2,
		successCount:   1,
		totalLatencyMs: 180000,
		outputTokens:   5000,
		generationMs:   100000,
	}, merged[bucketKey{model: "video-model", group: "a"}])
}

// TEST_PERF_MYSQL_DSN / TEST_PERF_POSTGRES_DSN optionally run the aggregation
// against isolated real MySQL/PostgreSQL databases.
func TestPerformanceAggregationAndFlush(t *testing.T) {
	for _, dialect := range []struct{ name, env string }{
		{"sqlite", ""}, {"mysql", "TEST_PERF_MYSQL_DSN"}, {"postgres", "TEST_PERF_POSTGRES_DSN"},
	} {
		t.Run(dialect.name, func(t *testing.T) {
			dsn := ""
			if dialect.env != "" {
				dsn = os.Getenv(dialect.env)
				if dsn == "" {
					t.Skip("isolated test database DSN is not configured")
				}
			}
			t.Setenv("SQL_DSN", dsn)
			oldDB, oldPath, oldMaster, oldRedis := model.DB, common.SQLitePath, common.IsMasterNode, common.RedisEnabled
			oldType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
			common.SQLitePath, common.IsMasterNode, common.RedisEnabled = filepath.Join(t.TempDir(), "perf.db"), true, false
			hotBuckets.Clear()
			t.Cleanup(func() {
				model.DB, common.SQLitePath, common.IsMasterNode, common.RedisEnabled = oldDB, oldPath, oldMaster, oldRedis
				common.SetDatabaseTypes(oldType, oldLogType)
				hotBuckets.Clear()
			})
			require.NoError(t, model.InitDB())
			db := model.DB
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			require.NoError(t, db.Migrator().DropTable(&model.PerfMetric{}))
			require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))

			now := time.Now()
			start, _ := queryWindow(now, 24)
			hour := now.Unix() - now.Unix()%3600 - 3600
			// Historical counters remain usable without reclassification or migration.
			for _, row := range []model.PerfMetric{
				{ModelName: "test-model", Group: "a", BucketTs: hour, RequestCount: 100, SuccessCount: 100, TotalLatencyMs: 100000, TtftCount: 100, TtftSumMs: 10000, OutputTokens: 200, GenerationMs: 40000},
				{ModelName: "test-model", Group: "inactive", BucketTs: hour, RequestCount: 100},
				{ModelName: "test-model", Group: "a", BucketTs: start - 3600, RequestCount: 100},
			} {
				require.NoError(t, model.UpsertPerfMetric(&row))
			}
			groups := []string{"a", "b"}

			RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: "test-model", UsingGroup: "b", StartTime: now}, types.InitOpenAIError("unknown", 400))
			businessRejected, err := QuerySummaryAll(24, groups)
			require.NoError(t, err)
			require.NotNil(t, businessRejected.Summary)
			assert.Equal(t, 100.0, businessRejected.Summary.SuccessRate)

			failure := &atomicBucket{}
			failure.add(Sample{LatencyMs: 2000})
			hotBuckets.Store(bucketKey{model: "test-model", group: "b", bucketTs: hour}, failure)
			before, err := Query(QueryParams{Model: "test-model", Hours: 24, AllowedGroups: groups})
			require.NoError(t, err)
			require.NotNil(t, before.Summary)
			assert.Equal(t, Summary{SuccessRate: 99.01, AvgLatencyMs: 1009, AvgTps: 5}, *before.Summary)
			assert.Equal(t, start, before.WindowStart)
			require.Len(t, before.Series, 1)
			assert.Equal(t, hour, before.Series[0].Ts)
			assert.InDelta(t, 99.01, before.Series[0].SuccessRate, 0.01)
			require.Len(t, before.Groups, 2)

			summary, err := QuerySummaryAll(24, groups)
			require.NoError(t, err)
			assert.Equal(t, before.Summary, summary.Summary)
			require.Len(t, summary.Models, 1)
			assert.Equal(t, 99.01, summary.Models[0].SuccessRate)
			assert.Equal(t, 99.01, summary.Models[0].RecentSuccessSeries[0].SuccessRate)
			encoded, err := common.Marshal(summary)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), `"request_count":101`)
			assert.Contains(t, string(encoded), `"success_count":100`)
			assert.Contains(t, string(encoded), `"window_start":`)

			flushCompletedBuckets()
			flushCompletedBuckets()
			after, err := Query(QueryParams{Model: "test-model", Hours: 24, AllowedGroups: groups})
			require.NoError(t, err)
			assert.Equal(t, before.Summary, after.Summary)
			assert.Equal(t, before.Series, after.Series)

			RecordRelayResult(context.Background(), &relaycommon.RelayInfo{OriginModelName: "test-model", UsingGroup: "a", StartTime: now}, types.InitOpenAIError("context_length_exceeded", 400))
			after, err = Query(QueryParams{Model: "test-model", Hours: 24, AllowedGroups: groups})
			require.NoError(t, err)
			assert.Equal(t, before.Summary, after.Summary)
			onlyA, err := Query(QueryParams{Model: "test-model", Group: "a", Hours: 24, AllowedGroups: groups})
			require.NoError(t, err)
			assert.Equal(t, 100.0, onlyA.Summary.SuccessRate)
			empty, err := Query(QueryParams{Model: "missing", Hours: 24, AllowedGroups: groups})
			require.NoError(t, err)
			assert.Nil(t, empty.Summary)
			assert.Empty(t, empty.Series)

			require.NoError(t, model.UpsertPerfMetric(&model.PerfMetric{ModelName: "second-model", Group: "a", BucketTs: hour, RequestCount: 1}))
			combined, err := QuerySummaryAll(24, groups)
			require.NoError(t, err)
			assert.Equal(t, 98.04, combined.Summary.SuccessRate)
			assert.Equal(t, 99.01, combined.Models[0].SuccessRate)
		})
	}
}
