package controller

import (
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/gin-gonic/gin"
)

const billingCalendarTimezone = "Asia/Shanghai"

type billingCalendarModel struct {
	ModelName   string `json:"model_name"`
	BillingExpr string `json:"billing_expr"`
}

type billingCalendarDate struct {
	Date string `json:"date"`
}

type billingCalendarMetadata struct {
	Schema         string                             `json:"schema"`
	Version        int                                `json:"version"`
	Country        string                             `json:"country"`
	Timezone       string                             `json:"timezone"`
	SupportedYears []int                              `json:"supported_years"`
	CoverageStart  string                             `json:"coverage_start"`
	CoverageEnd    string                             `json:"coverage_end"`
	Source         billingexpr.CalendarSourceMetadata `json:"source"`
	Holidays       []billingCalendarDate              `json:"holidays"`
	MakeupWorkdays []billingCalendarDate              `json:"makeup_workdays"`
}

type billingCalendarResponse struct {
	Models   []billingCalendarModel  `json:"models"`
	Calendar billingCalendarMetadata `json:"calendar"`
}

func billingCalendarDates(metadata billingexpr.CalendarMetadata) ([]billingCalendarDate, []billingCalendarDate) {
	holidays := make([]billingCalendarDate, 0)
	makeupWorkdays := make([]billingCalendarDate, 0)
	for _, year := range metadata.SupportedYears {
		for date := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC); date.Year() == year; date = date.AddDate(0, 0, 1) {
			classification, available := billingexpr.ClassifyCalendarDay(metadata.Country, date)
			if !available {
				continue
			}
			item := billingCalendarDate{Date: date.Format(time.DateOnly)}
			switch classification {
			case billingexpr.CalendarDayHoliday:
				holidays = append(holidays, item)
			case billingexpr.CalendarDayMakeupWorkday:
				makeupWorkdays = append(makeupWorkdays, item)
			}
		}
	}
	return holidays, makeupWorkdays
}

func buildBillingCalendarResponse() billingCalendarResponse {
	modes := billing_setting.GetBillingModeCopy()
	expressions := billing_setting.GetBillingExprCopy()
	models := make([]billingCalendarModel, 0, len(expressions))
	for name, expression := range expressions {
		if modes[name] != billing_setting.BillingModeTieredExpr || strings.TrimSpace(expression) == "" {
			continue
		}
		models = append(models, billingCalendarModel{ModelName: name, BillingExpr: expression})
	}
	sort.Slice(models, func(left, right int) bool {
		return models[left].ModelName < models[right].ModelName
	})

	metadata, _ := billingexpr.CalendarMetadataForCountry("CN")
	holidays, makeupWorkdays := billingCalendarDates(metadata)
	return billingCalendarResponse{
		Models: models,
		Calendar: billingCalendarMetadata{
			Schema:         metadata.Schema,
			Version:        metadata.Version,
			Country:        metadata.Country,
			Timezone:       billingCalendarTimezone,
			SupportedYears: metadata.SupportedYears,
			CoverageStart:  metadata.CoverageStart,
			CoverageEnd:    metadata.CoverageEnd,
			Source:         metadata.Source,
			Holidays:       holidays,
			MakeupWorkdays: makeupWorkdays,
		},
	}
}

// GetBillingCalendar returns only the expression and public calendar data
// required by the read-only administrator projection.
func GetBillingCalendar(c *gin.Context) {
	common.ApiSuccess(c, buildBillingCalendarResponse())
}
