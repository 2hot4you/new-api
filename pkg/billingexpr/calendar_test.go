package billingexpr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarMetadataForCountryReturnsDefensiveCopy(t *testing.T) {
	metadata, available := CalendarMetadataForCountry(" cn ")
	require.True(t, available)
	assert.Equal(t, "billingexpr.calendar", metadata.Schema)
	assert.Equal(t, 1, metadata.Version)
	assert.Equal(t, "CN", metadata.Country)
	assert.Equal(t, "国办发明电〔2025〕7号", metadata.Source.Document)
	assert.Equal(t, "https://www.gov.cn/zhengce/zhengceku/202511/content_7047091.htm", metadata.Source.URL)
	assert.Equal(t, []int{2026}, metadata.SupportedYears)
	assert.Equal(t, "2026-10-01", metadata.CoverageStart)
	assert.Equal(t, "2026-12-31", metadata.CoverageEnd)

	metadata.SupportedYears[0] = 9999
	again, available := CalendarMetadataForCountry("CN")
	require.True(t, available)
	assert.Equal(t, []int{2026}, again.SupportedYears)

	_, available = CalendarMetadataForCountry("US")
	assert.False(t, available)
}

func TestClassifyCalendarDay(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		want      CalendarDayType
		available bool
	}{
		{name: "before coverage", date: time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC), available: false},
		{name: "holiday at coverage start", date: time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC), want: CalendarDayHoliday, available: true},
		{name: "makeup workday", date: time.Date(2026, time.October, 10, 12, 0, 0, 0, time.UTC), want: CalendarDayMakeupWorkday, available: true},
		{name: "ordinary workday at coverage end", date: time.Date(2026, time.December, 31, 12, 0, 0, 0, time.UTC), want: CalendarDayWorkday, available: true},
		{name: "unsupported year", date: time.Date(2027, time.January, 1, 12, 0, 0, 0, time.UTC), want: CalendarDayHoliday, available: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, available := ClassifyCalendarDay("CN", tt.date)
			assert.Equal(t, tt.available, available)
			if tt.available {
				assert.Equal(t, tt.want, got)
			} else {
				assert.Empty(t, got)
			}
		})
	}

	_, available := ClassifyCalendarDay("US", time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	assert.False(t, available)
}

func TestDecodeCalendarRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{name: "unknown field", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-31","years":{"2026":{"holidays":[],"makeupWorkdays":[]}},"extra":true}`},
		{name: "unsupported schema", json: `{"schema":"wrong","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-31","years":{"2026":{"holidays":[],"makeupWorkdays":[]}}}`},
		{name: "missing coverage start", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageEnd":"2026-12-31","years":{"2026":{"holidays":[],"makeupWorkdays":[]}}}`},
		{name: "invalid coverage start", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-1","coverageEnd":"2026-12-31","years":{"2026":{"holidays":[],"makeupWorkdays":[]}}}`},
		{name: "coverage order", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-12-31","coverageEnd":"2026-10-01","years":{"2026":{"holidays":[],"makeupWorkdays":[]}}}`},
		{name: "year mismatch", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-31","years":{"2027":{"holidays":[],"makeupWorkdays":[]}}}`},
		{name: "bad date", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-31","years":{"2026":{"holidays":["2026-02-30"],"makeupWorkdays":[]}}}`},
		{name: "date before coverage", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-31","years":{"2026":{"holidays":["2026-09-30"],"makeupWorkdays":[]}}}`},
		{name: "date after coverage", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-30","years":{"2026":{"holidays":[],"makeupWorkdays":["2026-12-31"]}}}`},
		{name: "overlap", json: `{"schema":"billingexpr.calendar","version":1,"country":"CN","source":{"title":"x","document":"x","url":"https://example.com"},"supportedYears":[2026],"coverageStart":"2026-10-01","coverageEnd":"2026-12-31","years":{"2026":{"holidays":["2026-10-01"],"makeupWorkdays":["2026-10-01"]}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeCalendar([]byte(tt.json))
			require.Error(t, err)
		})
	}
}

func TestHolidayExpressionUsesFrozenRequestClock(t *testing.T) {
	holiday := time.Date(2026, time.October, 1, 2, 30, 0, 0, time.UTC)
	workday := time.Date(2026, time.October, 8, 2, 30, 0, 0, time.UTC)
	expression := `tier("base", p) * (is_holiday(" cn ", "Asia/Shanghai") ? 0.5 : 1)`

	cost, trace, err := RunExprWithRequest(expression, TokenParams{P: 100}, RequestInput{Now: &holiday})
	require.NoError(t, err)
	assert.Equal(t, float64(50), cost)
	assert.Equal(t, []RequestRuleTrace{{
		Cond:       `is_holiday(" cn ", "Asia/Shanghai")`,
		Multiplier: 0.5,
		Matched:    true,
	}}, trace.RequestRules)

	// Reuse the cached program with another request clock. The function must not
	// capture the time from compilation or the previous evaluation.
	cost, trace, err = RunExprWithRequest(expression, TokenParams{P: 100}, RequestInput{Now: &workday})
	require.NoError(t, err)
	assert.Equal(t, float64(100), cost)
	assert.False(t, trace.RequestRules[0].Matched)
}

func TestHolidayExpressionCountryYearAndTimezoneFallback(t *testing.T) {
	// UTC is still outside the coverage range while Shanghai is already on the
	// Oct 1 holiday. This distinguishes invalid-timezone fallback from local
	// timezone conversion.
	now := time.Date(2026, time.September, 30, 16, 30, 0, 0, time.UTC)
	tests := []struct {
		name       string
		expression string
		want       float64
	}{
		{name: "unknown country", expression: `is_holiday("US", "UTC") ? 2 : 1`, want: 1},
		{name: "unknown year", expression: `is_holiday("CN", "UTC") ? 2 : 1`, want: 1},
		{name: "invalid timezone uses UTC", expression: `is_holiday("CN", "Invalid/Zone") ? 2 : 1`, want: 1},
		{name: "timezone selects local date", expression: `is_holiday("CN", "Asia/Shanghai") ? 2 : 1`, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestNow := now
			if tt.name == "unknown year" {
				requestNow = time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
			}
			cost, _, err := RunExprWithRequest(tt.expression, TokenParams{}, RequestInput{Now: &requestNow})
			require.NoError(t, err)
			assert.Equal(t, tt.want, cost)
		})
	}
}

func TestHolidayExpressionLocalTimezoneFallsBackToUTC(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("non-UTC local", 8*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	// UTC is still outside coverage while the process-local date is already the
	// Oct 1 holiday. "Local" must not make billing host-dependent.
	now := time.Date(2026, time.September, 30, 16, 30, 0, 0, time.UTC)
	cost, _, err := RunExprWithRequest(`is_holiday("CN", "Local") ? 2 : 1`, TokenParams{}, RequestInput{Now: &now})
	require.NoError(t, err)
	assert.Equal(t, float64(1), cost)
}

func TestHolidayExpressionMakeupAndWeekendAreNotHolidays(t *testing.T) {
	tests := []time.Time{
		time.Date(2026, time.October, 10, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
		time.Date(2026, time.October, 11, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
	}
	for _, now := range tests {
		cost, _, err := RunExprWithRequest(`is_holiday("CN", "Asia/Shanghai") ? 2 : 1`, TokenParams{}, RequestInput{Now: &now})
		require.NoError(t, err)
		assert.Equal(t, float64(1), cost)
	}
}

func TestHolidayExpressionIsAllowedAsFixedPriceDependency(t *testing.T) {
	now := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	expression := `is_holiday("CN", "Asia/Shanghai") ? tier("holiday", fixed(0.01)) : tier("standard", fixed(0.02))`

	require.True(t, UsesFixedPricing(expression))
	cost, trace, err := RunExprWithRequest(expression, TokenParams{}, RequestInput{Now: &now})
	require.NoError(t, err)
	assert.Equal(t, float64(10000), cost)
	assert.Equal(t, "holiday", trace.MatchedTier)
}

func TestTimeFunctionsUseFrozenRequestClock(t *testing.T) {
	now := time.Date(2026, time.January, 3, 16, 30, 0, 0, time.UTC)
	expression := `hour("Asia/Shanghai") == 0 && minute("Asia/Shanghai") == 30 && weekday("Asia/Shanghai") == 0 && month("Asia/Shanghai") == 1 && day("Asia/Shanghai") == 4 ? 2 : 1`

	cost, _, err := RunExprWithRequest(expression, TokenParams{}, RequestInput{Now: &now})
	require.NoError(t, err)
	assert.Equal(t, float64(2), cost)
}
