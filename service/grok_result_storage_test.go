package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
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

func TestGrokResultStorageRetryAfterFailedCopyStartsFresh24HourRetention(t *testing.T) {
	useStarAIAssetRedis(t)
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	remoteCalls := 0
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		remoteCalls++
		if remoteCalls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video"))
	}))
	t.Cleanup(remote.Close)
	store := &grokResultStore{
		objectStorage:          newObjectStorageCOSTestStore(t, fakeCOS, remote.Client()),
		enqueueCleanup:         EnqueueGrokObjectCleanup,
		registerPendingCleanup: RegisterPendingGrokObjectCleanup,
	}
	t1 := time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC)
	request := GrokResultStoreRequest{
		SourceURL:      remote.URL + "/result.mp4",
		UserID:         42,
		MediaType:      "video",
		MIMEType:       "video/mp4",
		IdempotencyKey: "task_retry_retention",
		CreatedAt:      t1,
		KeyAnchor:      t1,
	}

	_, err := store.persist(context.Background(), request)
	require.Error(t, err)

	t2 := t1.Add(30 * time.Minute)
	request.CreatedAt = t2
	created, err := store.persist(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, t2.Add(grokResultRetention).Unix(), created.ExpiresAt)
	require.Equal(t, 1, fakeCOS.putCount)

	score, err := common.RDB.ZScore(context.Background(), grokCOSCleanupIndexKey, created.ObjectKey).Result()
	require.NoError(t, err)
	require.Equal(t, float64(created.ExpiresAt), score, "a stale pending registration must not shorten the successful copy retention")
	require.Equal(t, strconv.FormatInt(created.ExpiresAt, 10), fakeCOS.expiresAt[created.ObjectKey])

	request.CreatedAt = t2.Add(time.Hour)
	reused, err := store.persist(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, created, reused)
	score, err = common.RDB.ZScore(context.Background(), grokCOSCleanupIndexKey, created.ObjectKey).Result()
	require.NoError(t, err)
	require.Equal(t, float64(created.ExpiresAt), score, "reusing a stored object must not extend its retention")
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

func TestGrokResultStorageCleanupExpiresAndScrubsUserFileMetadata(t *testing.T) {
	setupUserFileServiceDB(t)
	useStarAIAssetRedis(t)
	ctx := context.Background()
	now := time.Now().Unix()
	objectKey := "users/grok-files/42/2026/08/file_cleanup.mp4"
	file := &model.MoliiFile{
		FileID: "file_cleanup", UserID: 42, ObjectKey: objectKey, Filename: "private.mp4", Purpose: "video",
		Bytes: 123, MIMEType: "video/mp4", MediaType: model.MoliiFileMediaTypeVideo,
		Width: 1280, Height: 720, DurationSeconds: 8.7, Status: model.MoliiFileStatusActive,
		CreatedAt: now - 86400, UpdatedAt: now - 86400, ExpiresAt: now - 1,
	}
	require.NoError(t, model.CreateMoliiFile(ctx, file))
	require.NoError(t, common.RDB.ZAdd(ctx, grokCOSCleanupIndexKey, &redis.Z{Score: float64(now - 1), Member: objectKey}).Err())

	cleanupExpiredGrokObjectsWithDelete(ctx, func(context.Context, string) error { return nil })

	_, err := model.GetMoliiFileForUser(ctx, 42, file.FileID, now)
	assert.ErrorIs(t, err, model.ErrMoliiFileExpired)
	record, err := model.GetMoliiFileRecordForUser(ctx, 42, file.FileID)
	require.NoError(t, err)
	assert.Equal(t, model.MoliiFileStatusExpired, record.Status)
	assert.Empty(t, record.Filename)
	assert.Zero(t, record.Bytes)
}

func TestGrokResultStoragePreEnqueueFailureNeverWritesOrDeletesSharedObject(t *testing.T) {
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video"))
	}))
	t.Cleanup(remote.Close)
	objectStorage := newObjectStorageCOSTestStore(t, fakeCOS, remote.Client())
	objectKey, err := BuildGrokResultObjectKey(42, "video", "task_enqueue_failure", time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC), "video/mp4")
	require.NoError(t, err)
	fakeCOS.deleteStatus[objectKey] = http.StatusServiceUnavailable
	enqueueObservedPutCount := -1
	store := &grokResultStore{
		objectStorage: objectStorage,
		enqueueCleanup: func(string, int64) error {
			enqueueObservedPutCount = fakeCOS.putCount
			return context.DeadlineExceeded
		},
	}

	_, err = store.persist(context.Background(), GrokResultStoreRequest{
		SourceURL:      remote.URL + "/result.mp4",
		UserID:         42,
		MediaType:      "video",
		MIMEType:       "video/mp4",
		IdempotencyKey: "task_enqueue_failure",
		CreatedAt:      time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC),
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Zero(t, enqueueObservedPutCount, "cleanup must be durably queued before any COS write")
	require.Empty(t, fakeCOS.objects)
	require.Zero(t, fakeCOS.deleteCount, "a failed pre-enqueue must not delete a key another node may own")
}

func TestEnqueueGrokObjectCleanupNeverExtendsExistingExpiry(t *testing.T) {
	useStarAIAssetRedis(t)
	ctx := context.Background()
	key := "users/grok-results/42/video/2026/08/stable.mp4"
	firstExpiry := int64(1_786_472_000)
	require.NoError(t, EnqueueGrokObjectCleanup(key, firstExpiry))
	require.NoError(t, EnqueueGrokObjectCleanup(key, firstExpiry+3600))

	score, err := common.RDB.ZScore(ctx, grokCOSCleanupIndexKey, key).Result()
	require.NoError(t, err)
	require.Equal(t, float64(firstExpiry), score)
}

func TestGrokResultStorageConcurrentFailedPreEnqueueCannotDeleteSuccessfulPeerObject(t *testing.T) {
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	var remoteCalls atomic.Int32
	releaseRemote := make(chan struct{})
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		remoteCalls.Add(1)
		<-releaseRemote
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video"))
	}))
	t.Cleanup(remote.Close)
	objectStorage := newObjectStorageCOSTestStore(t, fakeCOS, remote.Client())
	failureEnqueued := make(chan struct{})
	failingStore := &grokResultStore{
		objectStorage: objectStorage,
		enqueueCleanup: func(string, int64) error {
			close(failureEnqueued)
			return context.DeadlineExceeded
		},
	}
	successStore := &grokResultStore{
		objectStorage: objectStorage,
		enqueueCleanup: func(string, int64) error {
			<-failureEnqueued
			return nil
		},
	}
	request := GrokResultStoreRequest{
		SourceURL:      remote.URL + "/result.mp4",
		UserID:         42,
		MediaType:      "video",
		MIMEType:       "video/mp4",
		IdempotencyKey: "shared-task",
		CreatedAt:      time.Date(2026, time.August, 11, 9, 10, 11, 0, time.UTC),
	}
	type result struct {
		stored *StoredObject
		err    error
	}
	results := make(chan result, 2)
	go func() {
		stored, err := failingStore.persist(context.Background(), request)
		results <- result{stored: stored, err: err}
	}()
	go func() {
		stored, err := successStore.persist(context.Background(), request)
		results <- result{stored: stored, err: err}
	}()
	require.Eventually(t, func() bool { return remoteCalls.Load() >= 1 }, time.Second, 5*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	close(releaseRemote)
	first := <-results
	second := <-results

	errorsSeen := []error{first.err, second.err}
	require.Contains(t, errorsSeen, context.DeadlineExceeded)
	var successful *StoredObject
	if first.err == nil {
		successful = first.stored
	}
	if second.err == nil {
		successful = second.stored
	}
	require.NotNil(t, successful)
	require.Equal(t, int32(1), remoteCalls.Load())
	require.Equal(t, 1, fakeCOS.putCount)
	require.Contains(t, fakeCOS.objects, successful.ObjectKey)
	require.Zero(t, fakeCOS.deleteCount)
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

func TestGrokResultStorageUsesExplicitExpiryWithoutChangingKeyAnchor(t *testing.T) {
	useStarAICOSTestConfig(t)
	fakeCOS := newObjectStorageCOSTestServer(t)
	remote := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("video"))
	}))
	t.Cleanup(remote.Close)
	store := &grokResultStore{
		objectStorage:  newObjectStorageCOSTestStore(t, fakeCOS, remote.Client()),
		enqueueCleanup: func(string, int64) error { return nil },
	}
	keyAnchor := time.Date(2026, time.July, 31, 23, 59, 0, 0, time.UTC)
	completedAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

	stored, _, err := store.persistWithStatus(context.Background(), GrokResultStoreRequest{
		SourceURL:      remote.URL + "/result.mp4",
		UserID:         42,
		MediaType:      "video",
		MIMEType:       "video/mp4",
		IdempotencyKey: "task_public_video",
		CreatedAt:      completedAt,
		KeyAnchor:      keyAnchor,
	})

	require.NoError(t, err)
	require.Equal(t, completedAt.Add(24*time.Hour).Unix(), stored.ExpiresAt)
	require.Contains(t, stored.ObjectKey, "/2026/07/")
}

func TestIsOwnedGrokResultObjectRequiresUserAndMediaPrefix(t *testing.T) {
	useStarAICOSTestConfig(t)
	require.True(t, IsOwnedGrokResultObject(42, "video", "users/grok-results/42/video/2026/08/result.mp4"))
	require.False(t, IsOwnedGrokResultObject(7, "video", "users/grok-results/42/video/2026/08/result.mp4"))
	require.False(t, IsOwnedGrokResultObject(42, "image", "users/grok-results/42/video/2026/08/result.mp4"))
	require.False(t, IsOwnedGrokResultObject(42, "video", "users/grok-results/42/video/../image/result.mp4"))
}

func TestPersistGrokVideoResultSerializesSameTaskAcrossNodes(t *testing.T) {
	useStarAIAssetRedis(t)
	request := GrokResultStoreRequest{
		UserID: 42, MediaType: "video", MIMEType: "video/mp4",
		IdempotencyKey: "task_public_video", CreatedAt: time.Now(),
	}
	ownerEntered := make(chan struct{})
	releaseOwner := make(chan struct{})
	ownerDone := make(chan error, 1)
	persist := func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error) {
		close(ownerEntered)
		<-releaseOwner
		return &StoredObject{ObjectKey: "users/grok-results/42/video/result.mp4", MIMEType: "video/mp4", ExpiresAt: time.Now().Add(24 * time.Hour).Unix()}, true, nil
	}
	go func() {
		_, _, err := persistGrokVideoResultWithStatus(context.Background(), request, persist)
		ownerDone <- err
	}()
	<-ownerEntered

	peerEntered := false
	_, _, peerErr := persistGrokVideoResultWithStatus(context.Background(), request, func(context.Context, GrokResultStoreRequest) (*StoredObject, bool, error) {
		peerEntered = true
		return nil, false, nil
	})
	require.Error(t, peerErr)
	require.False(t, peerEntered)
	close(releaseOwner)
	require.NoError(t, <-ownerDone)
}
