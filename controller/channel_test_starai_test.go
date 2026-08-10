package controller

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSingleStarAIChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Log{}, &model.User{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedChannelForSingleStarAITest(t *testing.T, db *gorm.DB, channelType int, status int, name string) *model.Channel {
	t.Helper()
	channel := &model.Channel{
		Type:   channelType,
		Status: status,
		Name:   name,
		Key:    name + "-key",
		Models: "doubao-seedance-2-0-260128",
		Group:  "default",
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	return channel
}

func performChannelJSONRequest(t *testing.T, method string, target string, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", common.RoleRootUser)
	if id := strings.TrimPrefix(target, "/api/channel/"); id != target && !strings.Contains(id, "/") {
		ctx.Params = gin.Params{{Key: "id", Value: id}}
	}
	handler(ctx)
	return recorder
}

func TestGetUniqueEnabledChannelByTypeRequiresExactlyOne(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)

	channel, err := model.GetUniqueEnabledChannelByType(constant.ChannelTypeStarAI)
	require.Nil(t, channel)
	require.ErrorIs(t, err, model.ErrNoEnabledChannel)

	first := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "first")
	channel, err = model.GetUniqueEnabledChannelByType(constant.ChannelTypeStarAI)
	require.NoError(t, err)
	require.Equal(t, first.Id, channel.Id)

	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "second")
	channel, err = model.GetUniqueEnabledChannelByType(constant.ChannelTypeStarAI)
	require.Nil(t, channel)
	require.ErrorIs(t, err, model.ErrMultipleEnabledChannels)
}

func TestInitChannelCacheWarnsAndContinuesWhenEnabledStarAIChannelIsNotUnique(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "first")
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "second")
	common.MemoryCacheEnabled = true

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
	})

	require.NotPanics(t, model.InitChannelCache)
	assert.Contains(t, output.String(), "expected exactly one enabled channel")
	assert.Contains(t, output.String(), "startup will continue")
	_, err := model.GetUniqueEnabledChannelByType(constant.ChannelTypeStarAI)
	require.ErrorIs(t, err, model.ErrMultipleEnabledChannels)
}

func TestInitChannelCacheWarnsWhenMemoryCacheIsDisabled(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "first")
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "second")
	common.MemoryCacheEnabled = false

	var output bytes.Buffer
	common.LogWriterMu.Lock()
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultWriter = previousWriter
		common.LogWriterMu.Unlock()
	})

	require.NotPanics(t, model.InitChannelCache)
	assert.Contains(t, output.String(), "expected exactly one enabled channel")
	assert.Contains(t, output.String(), "found 2")
	assert.Contains(t, output.String(), "startup will continue")
}

func TestAddChannelRejectsSecondEnabledStarAIChannel(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "existing")

	recorder := performChannelJSONRequest(t, http.MethodPost, "/api/channel/", fmt.Sprintf(`{
		"mode":"single",
		"channel":{"type":%d,"status":%d,"name":"second","key":"second-key","models":"doubao-seedance-2-0-260128","group":"default"}
	}`, constant.ChannelTypeStarAI, common.ChannelStatusEnabled), AddChannel)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	assert.NotContains(t, recorder.Body.String(), "StarAI")
	assert.NotContains(t, recorder.Body.String(), "Molii")
	var count int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestBatchAddChannelRejectsCreatingTwoEnabledStarAIChannels(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)

	recorder := performChannelJSONRequest(t, http.MethodPost, "/api/channel/", fmt.Sprintf(`{
		"mode":"batch",
		"channel":{"type":%d,"status":%d,"name":"batch","key":"key-one\nkey-two","models":"doubao-seedance-2-0-260128","group":"default"}
	}`, constant.ChannelTypeStarAI, common.ChannelStatusEnabled), AddChannel)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var count int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestUpdateChannelRejectsChangingEnabledChannelToSecondStarAI(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "existing")
	candidate := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled, "candidate")

	recorder := performChannelJSONRequest(t, http.MethodPut, "/api/channel/", fmt.Sprintf(`{
		"id":%d,"type":%d,"name":"candidate","key":"candidate-key","models":"doubao-seedance-2-0-260128","group":"default"
	}`, candidate.Id, constant.ChannelTypeStarAI), UpdateChannel)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var saved model.Channel
	require.NoError(t, db.First(&saved, candidate.Id).Error)
	assert.Equal(t, constant.ChannelTypeOpenAI, saved.Type)
}

func TestUpdateChannelStatusRejectsSecondStarAIAndAllowsExplicitHandoff(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	oldChannel := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "old")
	newChannel := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusManuallyDisabled, "new")

	recorder := performChannelJSONRequest(t, http.MethodPut, fmt.Sprintf("/api/channel/%d", newChannel.Id), fmt.Sprintf(`{"status":%d}`, common.ChannelStatusEnabled), UpdateChannelStatus)
	assert.Contains(t, recorder.Body.String(), `"success":false`)

	recorder = performChannelJSONRequest(t, http.MethodPut, fmt.Sprintf("/api/channel/%d", oldChannel.Id), fmt.Sprintf(`{"status":%d}`, common.ChannelStatusManuallyDisabled), UpdateChannelStatus)
	assert.Contains(t, recorder.Body.String(), `"success":true`)
	recorder = performChannelJSONRequest(t, http.MethodPut, fmt.Sprintf("/api/channel/%d", newChannel.Id), fmt.Sprintf(`{"status":%d}`, common.ChannelStatusEnabled), UpdateChannelStatus)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var oldSaved, newSaved model.Channel
	require.NoError(t, db.First(&oldSaved, oldChannel.Id).Error)
	require.NoError(t, db.First(&newSaved, newChannel.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, oldSaved.Status)
	assert.Equal(t, common.ChannelStatusEnabled, newSaved.Status)
}

func TestModelUpdateChannelStatusRejectsAutomaticSecondStarAIEnable(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "existing")
	candidate := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusAutoDisabled, "candidate")

	changed := model.UpdateChannelStatus(candidate.Id, "", common.ChannelStatusEnabled, "automatic health recovery")

	assert.False(t, changed)
	var saved model.Channel
	require.NoError(t, db.First(&saved, candidate.Id).Error)
	assert.Equal(t, common.ChannelStatusAutoDisabled, saved.Status)
}

func TestBatchUpdateChannelStatusRejectsSecondStarAIWithoutPartialEnable(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "existing")
	starAICandidate := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusManuallyDisabled, "starai-candidate")
	otherCandidate := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeOpenAI, common.ChannelStatusManuallyDisabled, "other-candidate")

	recorder := performChannelJSONRequest(t, http.MethodPut, "/api/channel/batch", fmt.Sprintf(`{"ids":[%d,%d],"status":%d}`, starAICandidate.Id, otherCandidate.Id, common.ChannelStatusEnabled), BatchUpdateChannelStatus)
	assert.Contains(t, recorder.Body.String(), `"success":false`)

	var starAISaved, otherSaved model.Channel
	require.NoError(t, db.First(&starAISaved, starAICandidate.Id).Error)
	require.NoError(t, db.First(&otherSaved, otherCandidate.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, starAISaved.Status)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, otherSaved.Status)
}

func TestEnableTagChannelsRejectsSecondEnabledStarAIChannel(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "existing")
	candidate := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusManuallyDisabled, "candidate")
	tag := "replacement"
	require.NoError(t, db.Model(candidate).Update("tag", tag).Error)

	recorder := performChannelJSONRequest(t, http.MethodPost, "/api/channel/tag/enable", `{"tag":"replacement"}`, EnableTagChannels)

	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var saved model.Channel
	require.NoError(t, db.First(&saved, candidate.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, saved.Status)
}

func TestCopyChannelRejectsCopyingEnabledStarAIChannel(t *testing.T) {
	db := setupSingleStarAIChannelTestDB(t)
	existing := seedChannelForSingleStarAITest(t, db, constant.ChannelTypeStarAI, common.ChannelStatusEnabled, "existing")

	recorder := performChannelJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/channel/%d", existing.Id), "", CopyChannel)

	assert.Contains(t, recorder.Body.String(), `"success":false`)
	var count int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestStarAIChannelTestOnlyOpensTCPConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	receivedBytes := make(chan int, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			receivedBytes <- -1
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		buffer := make([]byte, 1)
		count, _ := conn.Read(buffer)
		receivedBytes <- count
	}()

	baseURL := "http://" + listener.Addr().String() + "/v1/video/generations"
	channel := &model.Channel{
		Id:      61,
		Type:    constant.ChannelTypeStarAI,
		Name:    "Molii Volcengine Imagine API reachability",
		Key:     "must-not-be-sent",
		BaseURL: common.GetPointer(baseURL),
	}

	result := testChannel(context.Background(), channel, 1, "doubao-seedance-2-0-260128", string(constant.EndpointTypeOpenAIVideo), false)

	require.NoError(t, result.localErr)
	assert.Nil(t, result.newAPIError)
	require.NotNil(t, result.context)
	select {
	case count := <-receivedBytes:
		assert.Zero(t, count, "reachability test must not send HTTP or authentication bytes")
	case <-time.After(2 * time.Second):
		t.Fatal("TCP reachability connection was not accepted")
	}
}

func TestStarAIChannelTestReportsClosedPortAsReachabilityFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	baseURL := "https://" + address
	channel := &model.Channel{
		Type:    constant.ChannelTypeStarAI,
		Name:    "Molii Volcengine Imagine API unreachable",
		BaseURL: common.GetPointer(baseURL),
	}

	result := testChannel(context.Background(), channel, 1, "", "", false)

	require.Error(t, result.localErr)
	require.NotNil(t, result.newAPIError)
	assert.Equal(t, types.ErrorCodeDoRequestFailed, result.newAPIError.GetErrorCode())
	assert.Equal(t, http.StatusServiceUnavailable, result.newAPIError.StatusCode)
	assert.Contains(t, result.localErr.Error(), "reachability test failed")
}

func TestStarAIReachabilityRejectsNonHTTPURL(t *testing.T) {
	baseURL := "ftp://starai.example:21"
	channel := &model.Channel{
		Type:    constant.ChannelTypeStarAI,
		BaseURL: common.GetPointer(baseURL),
	}

	err := testStarAIReachability(context.Background(), channel)

	require.ErrorContains(t, err, "unsupported URL scheme")
}
