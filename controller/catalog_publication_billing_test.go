package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The real HTTP adaptor, pricing selection, wallet/token reservation, terminal
// settlement and consume log run against isolated PG. Authentication middleware
// is not claimed here; only the upstream is a loopback fixed-usage fixture.
func TestCatalogPublicationLLMBillingAcrossRefresh(t *testing.T) {
	const name = "publication-llm"
	db := catalogControllerPostgres(t, map[string]string{
		"billing_setting.billing_mode": `{"publication-llm":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"publication-llm":"tier(\"base\", p * 14 + c * 70)"}`,
		"USDExchangeRate":              "7", "GroupRatio": `{"default":1}`,
		"quota_setting.trust_quota_usd": "0", "quota_setting.pre_consume_multiplier": "1",
	})
	previousUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousUnit })
	entry := model.Model{ModelName: name, NameRule: model.NameRuleExact, BillingCurrency: "CNY"}
	require.NoError(t, entry.Insert())
	user := model.User{Username: "publication-wallet", AffCode: "publication-wallet", Quota: 1000000, Group: "default"}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "publication-token-fixture", RemainQuota: 1000000, Status: common.TokenStatusEnabled, ExpiredTime: -1}
	require.NoError(t, db.Create(&token).Error)
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	var first sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		first.Do(func() { close(entered); <-release })
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(common.RequestIdKey, "publication-upstream-fixture")
		fmt.Fprint(w, `{"id":"publication-response","object":"chat.completion","model":"publication-llm","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"total_tokens":1100}}`)
	}))
	defer func() { unblock(); upstream.Close() }()
	channel := model.Channel{Name: "publication-upstream", Key: "local-fixture", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: name, Group: "default", BaseURL: &upstream.URL}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{ChannelId: channel.Id, Model: name, Group: "default", Enabled: true}).Error)
	require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
	service.InitHttpClient()

	request := func(id string) (*ginContextAndInfo, error) {
		body := `{"model":"publication-llm","messages":[{"role":"user","content":"fixture"}],"max_tokens":100}`
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(common.RequestIdKey, id)
		c.Set("username", user.Username)
		c.Set("token_name", "publication-token")
		c.Set("token_quota", 1000000)
		common.SetContextKey(c, constant.ContextKeyOriginalModel, name)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		if apiErr := middleware.SetupContextForSelectedChannel(c, &channel, name); apiErr != nil {
			return nil, apiErr
		}
		var input dto.GeneralOpenAIRequest
		if err := common.UnmarshalJsonStr(body, &input); err != nil {
			return nil, err
		}
		info := &relaycommon.RelayInfo{OriginModelName: name, RelayMode: relayconstant.RelayModeChatCompletions, Request: &input, RequestURLPath: c.Request.URL.Path, UserId: user.Id, TokenId: token.Id, TokenKey: token.Key, UserQuota: 1000000, UserGroup: "default", UsingGroup: "default", TokenGroup: "default", ForcePreConsume: true, StartTime: time.Now(), UserSetting: dto.UserSetting{BillingPreference: "wallet_only"}}
		info.SetEstimatePromptTokens(1000)
		info.FirstResponseTime = info.StartTime
		price, err := helper.ModelPriceHelper(c, info, 1000, &types.TokenCountMeta{MaxTokens: 100})
		if err != nil {
			return nil, err
		}
		if apiErr := service.PreConsumeBilling(c, price.QuotaToPreConsume, info); apiErr != nil {
			return nil, apiErr
		}
		return &ginContextAndInfo{context: c, info: info, recorder: recorder}, nil
	}

	old, err := request("publication-old")
	require.NoError(t, err)
	assert.Equal(t, 1000, old.info.FinalPreConsumedQuota)
	finished := make(chan *types.NewAPIError, 1)
	go func() { finished <- relay.TextHelper(old.context, old.info) }()
	select {
	case <-entered:
	case apiErr := <-finished:
		t.Fatalf("request ended before local upstream: %v", apiErr)
	case <-time.After(10 * time.Second):
		t.Fatal("request did not reach loopback upstream")
	}
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"billing_setting.billing_expr": `{"publication-llm":"tier(\"base\", p * 42 + c * 210)"}`,
		"USDExchangeRate":              "14",
	}))
	unblock()
	require.Nil(t, <-finished)
	assert.Equal(t, http.StatusOK, old.recorder.Code)
	assert.Contains(t, old.recorder.Body.String(), "publication-response")

	// Three unchanged channel/config rounds, each followed by an actual LLM
	// request. The old in-flight request keeps 7 CNY/USD and the old expression;
	// all fresh requests use 14 CNY/USD and the new expression.
	for round := range 3 {
		model.InitChannelCache()
		require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
		fresh, err := request(fmt.Sprintf("publication-new-%d", round))
		require.NoError(t, err)
		assert.Equal(t, 1500, fresh.info.FinalPreConsumedQuota)
		require.Nil(t, relay.TextHelper(fresh.context, fresh.info))
		assert.Equal(t, http.StatusOK, fresh.recorder.Code)
	}
	var logs []model.Log
	require.NoError(t, db.Where("type = ? AND user_id = ?", model.LogTypeConsume, user.Id).Order("id").Find(&logs).Error)
	require.Len(t, logs, 4)
	for index, want := range []int{1500, 2250, 2250, 2250} {
		assert.Equal(t, want, logs[index].Quota)
		assert.Equal(t, 1000, logs[index].PromptTokens)
		assert.Equal(t, 100, logs[index].CompletionTokens)
		assert.Equal(t, "publication-upstream-fixture", logs[index].UpstreamRequestId)
	}
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 991750, user.Quota)
	assert.Equal(t, 8250, user.UsedQuota)
	assert.Equal(t, 991750, token.RemainQuota)
	assert.Equal(t, 8250, token.UsedQuota)
}

type ginContextAndInfo struct {
	context  *gin.Context
	info     *relaycommon.RelayInfo
	recorder *httptest.ResponseRecorder
}
