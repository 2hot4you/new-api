package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
	var requestCount atomic.Int32
	var roundEntered, roundRelease [3]chan struct{}
	var releaseRounds [3]func()
	for round := range 3 {
		roundEntered[round], roundRelease[round] = make(chan struct{}), make(chan struct{})
		releaseRounds[round] = sync.OnceFunc(func() { close(roundRelease[round]) })
		defer releaseRounds[round]()
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sequence := requestCount.Add(1)
		first.Do(func() { close(entered); <-release })
		if sequence >= 2 && sequence <= 6 && sequence%2 == 0 {
			round := int(sequence/2) - 1
			close(roundEntered[round])
			<-roundRelease[round]
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(common.RequestIdKey, "publication-upstream-fixture")
		fmt.Fprint(w, `{"id":"publication-response","object":"chat.completion","model":"publication-llm","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1000,"completion_tokens":100,"total_tokens":1100}}`)
	}))
	defer func() {
		unblock()
		for _, releaseRound := range releaseRounds {
			releaseRound()
		}
		upstream.Close()
	}()
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
	require.NotNil(t, old.info.TieredBillingSnapshot)
	assert.Equal(t, "CNY", old.info.TieredBillingSnapshot.SourceCurrency)
	assert.Equal(t, float64(7), old.info.TieredBillingSnapshot.CNYPerUSD)
	finished := make(chan *types.NewAPIError, 1)
	oldExited := make(chan struct{})
	go func() { defer close(oldExited); finished <- relay.TextHelper(old.context, old.info) }()
	defer func() {
		unblock()
		select {
		case <-oldExited:
		case <-time.After(10 * time.Second):
			t.Error("old request did not finish before database fixture cleanup")
		}
	}()
	select {
	case <-entered:
	case apiErr := <-finished:
		t.Fatalf("request ended before local upstream: %v", apiErr)
	case <-time.After(10 * time.Second):
		t.Fatal("request did not reach loopback upstream")
	}
	// Exercise the real public status handler and complete ordinary publisher,
	// not only a lock wrapper: a blocked response must not retain publication's
	// OptionMap lock, and the upstream request must keep its old quote.
	statusWriter := &controlledStatusResponseWriter{ResponseRecorder: httptest.NewRecorder(), phase: "body", entered: make(chan struct{}), release: make(chan struct{})}
	releaseStatus := sync.OnceFunc(func() { close(statusWriter.release) })
	defer releaseStatus()
	statusContext, _ := gin.CreateTestContext(statusWriter)
	statusContext.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	statusDone := make(chan struct{})
	go func() { defer close(statusDone); GetStatus(statusContext) }()
	select {
	case <-statusWriter.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("status handler did not reach controlled response I/O")
	}
	published := make(chan error, 1)
	go func() {
		published <- model.UpdateOptionsBulk(map[string]string{
			"billing_setting.billing_expr": `{"publication-llm":"tier(\"base\", p * 42 + c * 210)"}`,
			"USDExchangeRate":              "14",
		})
	}()
	select {
	case err := <-published:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		releaseStatus()
		select {
		case <-published:
		case <-time.After(35 * time.Second):
			t.Fatal("publication failed to exit after releasing status response")
		}
		t.Fatal("real price publication waited for blocked status network I/O")
	}
	releaseStatus()
	select {
	case <-statusDone:
	case <-time.After(10 * time.Second):
		t.Fatal("status handler did not finish after response release")
	}
	unblock()
	require.Nil(t, <-finished)
	assert.Equal(t, http.StatusOK, old.recorder.Code)
	assert.Contains(t, old.recorder.Body.String(), "publication-response")

	// Three real publications while LLM requests are already at the upstream,
	// each followed by unchanged channel/config refresh and a fresh request.
	// Independently hand-calculated quota = (1000*p + 100*c)/1e6 * 500000/FX.
	nextExpressions := []string{
		`{"publication-llm":"tier(\"base\", p * 56 + c * 280)"}`,
		`{"publication-llm":"tier(\"base\", p * 98 + c * 490)"}`,
		`{"publication-llm":"tier(\"base\", p * 112 + c * 560)"}`,
	}
	nextFX := []string{"14", "14", "28"}
	heldPreconsume := []int{1500, 2000, 3500}
	freshPreconsume := []int{2000, 3500, 2000}
	heldExpressions := []string{`tier("base", p * 42 + c * 210)`, `tier("base", p * 56 + c * 280)`, `tier("base", p * 98 + c * 490)`}
	freshExpressions := []string{`tier("base", p * 56 + c * 280)`, `tier("base", p * 98 + c * 490)`, `tier("base", p * 112 + c * 560)`}
	freshFX := []float64{14, 14, 28}
	for round := range 3 {
		held, err := request(fmt.Sprintf("publication-held-%d", round))
		require.NoError(t, err)
		assert.Equal(t, heldPreconsume[round], held.info.FinalPreConsumedQuota)
		heldDone := make(chan *types.NewAPIError, 1)
		heldExited := make(chan struct{})
		go func() { defer close(heldExited); heldDone <- relay.TextHelper(held.context, held.info) }()
		defer func() {
			releaseRounds[round]()
			select {
			case <-heldExited:
			case <-time.After(10 * time.Second):
				t.Error("held request did not finish before database fixture cleanup")
			}
		}()
		select {
		case <-roundEntered[round]:
		case apiErr := <-heldDone:
			t.Fatalf("held request ended before upstream rendezvous: %v", apiErr)
		case <-time.After(10 * time.Second):
			t.Fatal("held LLM request did not reach upstream")
		}
		require.NoError(t, model.UpdateOptionsBulk(map[string]string{"billing_setting.billing_expr": nextExpressions[round], "USDExchangeRate": nextFX[round]}))
		require.NotNil(t, held.info.TieredBillingSnapshot)
		assert.Equal(t, "CNY", held.info.TieredBillingSnapshot.SourceCurrency)
		assert.Equal(t, float64(14), held.info.TieredBillingSnapshot.CNYPerUSD, "in-flight old money cannot follow the new FX")
		assert.Equal(t, heldExpressions[round], held.info.TieredBillingSnapshot.ExprString)
		releaseRounds[round]()
		require.Nil(t, <-heldDone)
		assert.Equal(t, http.StatusOK, held.recorder.Code)
		model.InitChannelCache()
		require.NoError(t, model.RecoverCatalogSyncRuntime(context.Background()))
		fresh, err := request(fmt.Sprintf("publication-new-%d", round))
		require.NoError(t, err)
		assert.Equal(t, freshPreconsume[round], fresh.info.FinalPreConsumedQuota)
		require.NotNil(t, fresh.info.TieredBillingSnapshot)
		assert.Equal(t, "CNY", fresh.info.TieredBillingSnapshot.SourceCurrency)
		assert.Equal(t, freshFX[round], fresh.info.TieredBillingSnapshot.CNYPerUSD)
		assert.Equal(t, freshExpressions[round], fresh.info.TieredBillingSnapshot.ExprString)
		require.Nil(t, relay.TextHelper(fresh.context, fresh.info))
		assert.Equal(t, http.StatusOK, fresh.recorder.Code)
	}
	var logs []model.Log
	require.NoError(t, db.Where("type = ? AND user_id = ?", model.LogTypeConsume, user.Id).Order("id").Find(&logs).Error)
	require.Len(t, logs, 7)
	wantLogFX := []float64{7, 14, 14, 14, 14, 14, 28}
	for index, want := range []int{1500, 2250, 3000, 3000, 5250, 5250, 3000} {
		assert.Equal(t, want, logs[index].Quota)
		assert.Equal(t, 1000, logs[index].PromptTokens)
		assert.Equal(t, 100, logs[index].CompletionTokens)
		assert.Equal(t, "publication-upstream-fixture", logs[index].UpstreamRequestId)
		var money struct {
			SourceCurrency string  `json:"source_currency"`
			CNYPerUSD      float64 `json:"cny_per_usd"`
		}
		require.NoError(t, common.UnmarshalJsonStr(logs[index].Other, &money))
		assert.Equal(t, "CNY", money.SourceCurrency)
		assert.Equal(t, wantLogFX[index], money.CNYPerUSD)
	}
	require.NoError(t, db.First(&user, user.Id).Error)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, 976750, user.Quota)
	assert.Equal(t, 23250, user.UsedQuota)
	assert.Equal(t, 976750, token.RemainQuota)
	assert.Equal(t, 23250, token.UsedQuota)
}

type ginContextAndInfo struct {
	context  *gin.Context
	info     *relaycommon.RelayInfo
	recorder *httptest.ResponseRecorder
}
