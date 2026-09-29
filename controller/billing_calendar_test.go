package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBillingCalendarReturnsOnlyExpressionModelsAndCalendarMetadata(t *testing.T) {
	previousModes := billing_setting.GetBillingModeCopy()
	previousExpressions := billing_setting.GetBillingExprCopy()
	previousModesJSON, err := json.Marshal(previousModes)
	require.NoError(t, err)
	previousExpressionsJSON, err := json.Marshal(previousExpressions)
	require.NoError(t, err)
	t.Cleanup(func() {
		config.UpdateConfigFromMap(config.GlobalConfig.Get("billing_setting"), map[string]string{
			"billing_mode": string(previousModesJSON),
			"billing_expr": string(previousExpressionsJSON),
		})
	})

	config.UpdateConfigFromMap(config.GlobalConfig.Get("billing_setting"), map[string]string{
		"billing_mode": `{"calendar-expr":"tiered_expr","legacy-ratio":"ratio"}`,
		"billing_expr": `{"calendar-expr":"tier(\"base\", p * 2)","orphan-expression":"tier(\"hidden\", p * 9)"}`,
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/billing-calendar", nil)

	GetBillingCalendar(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Models []struct {
				ModelName   string `json:"model_name"`
				BillingExpr string `json:"billing_expr"`
			} `json:"models"`
			Calendar struct {
				Country        string `json:"country"`
				Timezone       string `json:"timezone"`
				SupportedYears []int  `json:"supported_years"`
				CoverageStart  string `json:"coverage_start"`
				CoverageEnd    string `json:"coverage_end"`
				Source         struct {
					Title string `json:"title"`
					URL   string `json:"url"`
				} `json:"source"`
				Holidays       []map[string]any `json:"holidays"`
				MakeupWorkdays []map[string]any `json:"makeup_workdays"`
			} `json:"calendar"`
		} `json:"data"`
	}
	require.NoError(t, common.DecodeJson(recorder.Body, &response))
	require.True(t, response.Success)
	assert.Contains(t, response.Data.Models, struct {
		ModelName   string `json:"model_name"`
		BillingExpr string `json:"billing_expr"`
	}{ModelName: "calendar-expr", BillingExpr: `tier("base", p * 2)`})
	for _, item := range response.Data.Models {
		assert.NotEqual(t, "legacy-ratio", item.ModelName)
		assert.NotEqual(t, "orphan-expression", item.ModelName)
	}
	assert.Equal(t, "CN", response.Data.Calendar.Country)
	assert.Equal(t, "Asia/Shanghai", response.Data.Calendar.Timezone)
	assert.Contains(t, response.Data.Calendar.SupportedYears, 2026)
	assert.Equal(t, "2026-10-01", response.Data.Calendar.CoverageStart)
	assert.Equal(t, "2026-12-31", response.Data.Calendar.CoverageEnd)
	assert.NotEmpty(t, response.Data.Calendar.Source.Title)
	assert.NotEmpty(t, response.Data.Calendar.Source.URL)
	assert.Equal(t, []map[string]any{
		{"date": "2026-10-01"},
		{"date": "2026-10-02"},
		{"date": "2026-10-03"},
		{"date": "2026-10-04"},
		{"date": "2026-10-05"},
		{"date": "2026-10-06"},
		{"date": "2026-10-07"},
	}, response.Data.Calendar.Holidays)
	assert.Equal(t, []map[string]any{{"date": "2026-10-10"}}, response.Data.Calendar.MakeupWorkdays)
}
