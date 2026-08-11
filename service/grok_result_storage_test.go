package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestGrokResultStorageBuildsDeterministicProviderScopedKeys(t *testing.T) {
	useStarAICOSTestConfig(t)
	createdAt := time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC)

	imageKey, err := BuildGrokResultObjectKey(42, "image", "request-123", createdAt, "image/png")
	require.NoError(t, err)
	require.Equal(t, "users/grok-results/42/image/2026/08/4c18d52386a765eb0823f784630b484e.png", imageKey)
	imageKeyAgain, err := BuildGrokResultObjectKey(42, "image", "request-123", createdAt.Add(12*time.Hour), "image/png")
	require.NoError(t, err)
	require.Equal(t, imageKey, imageKeyAgain)

	videoKey, err := BuildGrokResultObjectKey(42, "video", "task_abc", createdAt, "video/mp4")
	require.NoError(t, err)
	require.Equal(t, "users/grok-results/42/video/2026/08/2bc3df3ae352dcc1429e663890bdb3fd.mp4", videoKey)
}

func TestGrokResultStoragePersistsExactly24HoursAndReusesIdempotencyKey(t *testing.T) {
	useStarAIAssetRedis(t)
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	remoteCalls := 0
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		remoteCalls++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image"))
	}))
	t.Cleanup(remote.Close)
	objectStorage := newObjectStorageCOSTestStore(t, fakeCOS, remote.Client())
	store := &grokResultStore{objectStorage: objectStorage, enqueueCleanup: EnqueueGrokObjectCleanup}
	createdAt := time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC)
	request := GrokResultStoreRequest{
		SourceURL:      remote.URL + "/signed.png?token=upstream-secret",
		UserID:         42,
		MediaType:      "image",
		MIMEType:       "image/png",
		IdempotencyKey: "request-123",
		CreatedAt:      createdAt,
	}

	first, err := store.persist(context.Background(), request)
	require.NoError(t, err)
	secondRequest := request
	secondRequest.CreatedAt = request.CreatedAt.Add(time.Hour)
	second, err := store.persist(context.Background(), secondRequest)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, createdAt.Add(24*time.Hour).Unix(), first.ExpiresAt)
	require.Equal(t, 1, remoteCalls)
	require.Equal(t, 1, fakeCOS.putCount)

	ctx := context.Background()
	score, err := common.RDB.ZScore(ctx, grokCOSCleanupIndexKey, first.ObjectKey).Result()
	require.NoError(t, err)
	require.Equal(t, float64(first.ExpiresAt), score)
	_, err = common.RDB.ZScore(ctx, starAICOSCleanupIndexKey, first.ObjectKey).Result()
	require.ErrorIs(t, err, redis.Nil)
}

func TestGrokResultStorageCleanupKeepsTransientFailuresAndNeverTouchesStarAIQueue(t *testing.T) {
	useStarAIAssetRedis(t)
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	objectStorage := newObjectStorageCOSTestStore(t, fakeCOS, http.DefaultClient)
	ctx := context.Background()
	now := time.Now().Unix()
	grokMissing := "users/grok-results/42/image/missing.png"
	grokTransient := "users/grok-results/42/video/transient.mp4"
	starAIKey := "users/42/starai-assets/video/seedance.mp4"
	fakeCOS.deleteStatus[grokMissing] = http.StatusNotFound
	fakeCOS.deleteStatus[grokTransient] = http.StatusServiceUnavailable
	require.NoError(t, common.RDB.ZAdd(ctx, grokCOSCleanupIndexKey,
		&redis.Z{Score: float64(now - 1), Member: grokMissing},
		&redis.Z{Score: float64(now - 1), Member: grokTransient},
	).Err())
	require.NoError(t, common.RDB.ZAdd(ctx, starAICOSCleanupIndexKey,
		&redis.Z{Score: float64(now - 1), Member: starAIKey},
	).Err())

	cleanupExpiredGrokObjectsWithDelete(ctx, objectStorage.deleteObject)

	_, err := common.RDB.ZScore(ctx, grokCOSCleanupIndexKey, grokMissing).Result()
	require.ErrorIs(t, err, redis.Nil)
	_, err = common.RDB.ZScore(ctx, grokCOSCleanupIndexKey, grokTransient).Result()
	require.NoError(t, err)
	_, err = common.RDB.ZScore(ctx, starAICOSCleanupIndexKey, starAIKey).Result()
	require.NoError(t, err)
}

func TestGrokResultStorageDeletesNewObjectWhenCleanupEnqueueFails(t *testing.T) {
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video"))
	}))
	t.Cleanup(remote.Close)
	objectStorage := newObjectStorageCOSTestStore(t, fakeCOS, remote.Client())
	store := &grokResultStore{
		objectStorage: objectStorage,
		enqueueCleanup: func(string, int64) error {
			return context.DeadlineExceeded
		},
	}

	_, err := store.persist(context.Background(), GrokResultStoreRequest{
		SourceURL:      remote.URL + "/result.mp4",
		UserID:         42,
		MediaType:      "video",
		MIMEType:       "video/mp4",
		IdempotencyKey: "task_enqueue_failure",
		CreatedAt:      time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC),
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, fakeCOS.objects)
}

func TestGrokStoredResultMetadataIsOptionalAndLegacyJSONCompatible(t *testing.T) {
	var legacy model.TaskPrivateData
	require.NoError(t, common.Unmarshal([]byte(`{"upstream_task_id":"upstream-1","result_url":"legacy"}`), &legacy))
	require.Nil(t, legacy.StoredResult)

	metadata := model.TaskPrivateData{StoredResult: &model.TaskStoredResult{
		ObjectKey: "users/grok-results/42/video/2026/08/result.mp4",
		MIMEType:  "video/mp4",
		Size:      123,
		ExpiresAt: 1_786_472_000,
	}}
	body, err := common.Marshal(metadata)
	require.NoError(t, err)
	require.NotContains(t, string(body), "upstream")
}
