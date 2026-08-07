package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"molii-aigc-demo/internal/upstream"
)

func fixtureCatalog() Catalog {
	return Catalog{
		GroupRatios: map[string]decimal.Decimal{"default": decimal.RequireFromString("1.5")},
		Models: map[string]ModelPricing{
			"doubao-seedance-2-0-260128": {
				ModelName: "doubao-seedance-2-0-260128",
				VideoPricing: &SeedancePricing{FPS: 24, ExtraFrames: 1, Rows: []SeedancePriceRow{
					{Resolutions: []string{"480p", "720p"}, WithoutVideo: decimal.NewFromInt(46), WithVideo: decimal.NewFromInt(28)},
				}},
			},
			"grok-imagine-image-quality": {
				ModelName:        "grok-imagine-image-quality",
				MoliiGrokPricing: &GrokPricing{Kind: "image", OutputPrices: map[string]decimal.Decimal{"1k": decimal.RequireFromString("0.05"), "2k": decimal.RequireFromString("0.07")}, ImageInputPrice: decimal.RequireFromString("0.01")},
			},
			"grok-imagine-video-1.5-preview": {
				ModelName:        "grok-imagine-video-1.5-preview",
				MoliiGrokPricing: &GrokPricing{Kind: "video", OutputPrices: map[string]decimal.Decimal{"1080p": decimal.RequireFromString("0.25")}, ImageInputPrice: decimal.RequireFromString("0.01")},
			},
			"grok-imagine-video": {
				ModelName:        "grok-imagine-video",
				MoliiGrokPricing: &GrokPricing{Kind: "video", OutputPrices: map[string]decimal.Decimal{"480p": decimal.RequireFromString("0.05"), "720p": decimal.RequireFromString("0.07")}, VideoInputPrice: decimal.RequireFromString("0.01")},
			},
		},
	}
}

func TestFetchCatalogAndActualUseAuthenticatedServerSideCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer sk-fixture", r.Header.Get("Authorization"))
		require.Empty(t, r.URL.Query().Get("key"), "API key must never be put in a query string")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/pricing":
			_, _ = w.Write([]byte(`{"success":true,"pricing_version":"v1","group_ratio":{"default":1.25},"data":[{"model_name":"grok-imagine-image","molii_grok_pricing":{"kind":"image","output_unit":"image","output_prices":{"1k":0.02},"image_input_unit":"image","image_input_price":0.002}}]}`))
		case "/api/log/token":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"created_at":1,"type":2,"model_name":"grok-imagine-image","quota":10000,"request_id":"req_1","other":"{}"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := upstream.NewClient(time.Second, 1<<20, nil)
	catalog, err := FetchCatalog(context.Background(), client, server.URL, "sk-fixture")
	require.NoError(t, err)
	require.Equal(t, "v1", catalog.Version)
	require.True(t, catalog.GroupRatios["default"].Equal(decimal.RequireFromString("1.25")))

	actual, err := FetchActual(context.Background(), client, server.URL, "sk-fixture", Reference{Kind: "image", RequestID: "req_1"})
	require.NoError(t, err)
	require.True(t, actual.Amount.Equal(decimal.RequireFromString("0.02")))
}

func TestSeedanceEstimateUsesDecimalAndFlagsAdaptiveInputs(t *testing.T) {
	catalog := fixtureCatalog()
	estimate := catalog.Estimate(EstimateInput{Model: "doubao-seedance-2-0-260128", Resolution: "480p", Ratio: "16:9", Duration: 4, Group: "default"})
	require.True(t, estimate.Available)
	require.Equal(t, int64(40595), estimate.TokenCount)
	require.True(t, estimate.Amount.Equal(decimal.NewFromInt(40595).Mul(decimal.NewFromInt(46)).Div(decimal.NewFromInt(1_000_000)).Mul(decimal.RequireFromString("1.5"))))

	adaptive := catalog.Estimate(EstimateInput{Model: "doubao-seedance-2-0-260128", Resolution: "720p", Ratio: "adaptive", Duration: 5})
	require.False(t, adaptive.Available)
	require.True(t, adaptive.Adaptive)
	require.Contains(t, adaptive.Reason, "dimensions")

	smart := catalog.Estimate(EstimateInput{Model: "doubao-seedance-2-0-260128", Resolution: "720p", Ratio: "16:9", Duration: -1})
	require.False(t, smart.Available)
	require.True(t, smart.Adaptive)
}

func TestGrokEstimatesIncludeInputsAndPreview(t *testing.T) {
	catalog := fixtureCatalog()
	image := catalog.Estimate(EstimateInput{Model: "grok-imagine-image-quality", Operation: "grok.image.edit", Resolution: "2k", OutputCount: 2, InputImageCount: 3})
	require.True(t, image.Amount.Equal(decimal.RequireFromString("0.17")))

	preview := catalog.Estimate(EstimateInput{Model: "grok-imagine-video-1.5-preview", Operation: "grok.video.generate", Resolution: "1080p", Duration: 4, InputImageCount: 1})
	require.True(t, preview.Available)
	require.True(t, preview.Amount.Equal(decimal.RequireFromString("1.01")))

	edit := catalog.Estimate(EstimateInput{Model: "grok-imagine-video", Operation: "grok.video.edit"})
	require.True(t, edit.Available)
	require.True(t, edit.Adaptive)
	require.True(t, edit.Amount.Equal(decimal.RequireFromString("0.696")))
}

func TestReconcileMatchesRequestIDAndOtherTaskID(t *testing.T) {
	otherString, err := json.Marshal(`{"task_id":"task_1","grok_video_billing":{"final_cost":1.2345}}`)
	require.NoError(t, err)
	logs := []Log{
		{CreatedAt: 20, Type: 2, ModelName: "grok-imagine-video", Quota: json.Number("999"), Other: otherString},
		{CreatedAt: 10, Type: 2, ModelName: "grok-imagine-image", RequestID: "req_1", Quota: json.Number("25000"), Other: json.RawMessage(`{}`)},
	}
	video := Reconcile(logs, Reference{Kind: "video", TaskID: "task_1"})
	require.True(t, video.Found)
	require.True(t, video.Amount.Equal(decimal.RequireFromString("1.2345")))

	image := Reconcile(logs, Reference{Kind: "image", RequestID: "req_1"})
	require.True(t, image.Amount.Equal(decimal.RequireFromString("0.05")))

	pending := Reconcile(logs, Reference{Kind: "video", TaskID: "task_missing"})
	require.True(t, pending.Pending)
}
