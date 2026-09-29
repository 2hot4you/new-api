package billingexpr

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	calendarSchema  = "billingexpr.calendar"
	calendarVersion = 1
)

// CalendarDayType is the classification of a date in a supported calendar.
type CalendarDayType string

const (
	CalendarDayHoliday       CalendarDayType = "holiday"
	CalendarDayMakeupWorkday CalendarDayType = "makeup_workday"
	CalendarDayWorkday       CalendarDayType = "workday"
	CalendarDayWeekend       CalendarDayType = "weekend"
)

// CalendarSourceMetadata identifies the official document backing a calendar.
type CalendarSourceMetadata struct {
	Title    string `json:"title"`
	Document string `json:"document"`
	URL      string `json:"url"`
}

// CalendarMetadata describes an embedded billing calendar. Returned values are
// copies and can be changed by callers without mutating the shared calendar.
type CalendarMetadata struct {
	Schema         string                 `json:"schema"`
	Version        int                    `json:"version"`
	Country        string                 `json:"country"`
	Source         CalendarSourceMetadata `json:"source"`
	SupportedYears []int                  `json:"supportedYears"`
	CoverageStart  string                 `json:"coverageStart"`
	CoverageEnd    string                 `json:"coverageEnd"`
}

type calendarYearData struct {
	Holidays       []string `json:"holidays"`
	MakeupWorkdays []string `json:"makeupWorkdays"`
}

type calendarDocument struct {
	Schema         string                      `json:"schema"`
	Version        int                         `json:"version"`
	Country        string                      `json:"country"`
	Source         CalendarSourceMetadata      `json:"source"`
	SupportedYears []int                       `json:"supportedYears"`
	CoverageStart  string                      `json:"coverageStart"`
	CoverageEnd    string                      `json:"coverageEnd"`
	Years          map[string]calendarYearData `json:"years"`
}

type calendarYear struct {
	holidays       map[string]struct{}
	makeupWorkdays map[string]struct{}
}

type billingCalendar struct {
	metadata      CalendarMetadata
	coverageStart string
	coverageEnd   string
	years         map[int]calendarYear
}

//go:embed calendars/cn.v1.json
var cnCalendarJSON []byte

var billingCalendars = mustLoadCalendars()

func mustLoadCalendars() map[string]*billingCalendar {
	document, err := decodeCalendar(cnCalendarJSON)
	if err != nil {
		panic(fmt.Sprintf("billingexpr: invalid embedded CN calendar: %v", err))
	}
	calendar := newBillingCalendar(document)
	return map[string]*billingCalendar{calendar.metadata.Country: calendar}
}

func decodeCalendar(data []byte) (*calendarDocument, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document calendarDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode calendar: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode calendar: multiple JSON values")
		}
		return nil, fmt.Errorf("decode calendar trailing data: %w", err)
	}
	if err := validateCalendar(&document); err != nil {
		return nil, err
	}
	return &document, nil
}

func validateCalendar(document *calendarDocument) error {
	if document.Schema != calendarSchema {
		return fmt.Errorf("calendar schema must be %q", calendarSchema)
	}
	if document.Version != calendarVersion {
		return fmt.Errorf("calendar version must be %d", calendarVersion)
	}
	if document.Country == "" || document.Country != strings.ToUpper(strings.TrimSpace(document.Country)) {
		return fmt.Errorf("calendar country must be a normalized uppercase code")
	}
	if strings.TrimSpace(document.Source.Title) == "" || strings.TrimSpace(document.Source.Document) == "" {
		return fmt.Errorf("calendar source title and document are required")
	}
	sourceURL, err := url.ParseRequestURI(document.Source.URL)
	if err != nil || sourceURL.Scheme != "https" || sourceURL.Host == "" {
		return fmt.Errorf("calendar source URL must be an absolute HTTPS URL")
	}
	if len(document.SupportedYears) == 0 {
		return fmt.Errorf("calendar must declare at least one supported year")
	}
	if len(document.Years) != len(document.SupportedYears) {
		return fmt.Errorf("calendar years must exactly match supportedYears")
	}
	coverageStart, err := parseCalendarDate("coverageStart", document.CoverageStart)
	if err != nil {
		return err
	}
	coverageEnd, err := parseCalendarDate("coverageEnd", document.CoverageEnd)
	if err != nil {
		return err
	}
	if coverageStart.After(coverageEnd) {
		return fmt.Errorf("calendar coverageStart must not be after coverageEnd")
	}

	for index, year := range document.SupportedYears {
		if year < 1 || year > 9999 {
			return fmt.Errorf("unsupported calendar year %d", year)
		}
		if index > 0 && document.SupportedYears[index-1] >= year {
			return fmt.Errorf("supportedYears must be sorted and unique")
		}
		data, ok := document.Years[strconv.Itoa(year)]
		if !ok {
			return fmt.Errorf("supported year %d has no data", year)
		}
		holidaySet, err := validateCalendarDates(year, "holidays", data.Holidays, document.CoverageStart, document.CoverageEnd)
		if err != nil {
			return err
		}
		makeupSet, err := validateCalendarDates(year, "makeupWorkdays", data.MakeupWorkdays, document.CoverageStart, document.CoverageEnd)
		if err != nil {
			return err
		}
		for date := range holidaySet {
			if _, overlaps := makeupSet[date]; overlaps {
				return fmt.Errorf("calendar date %s is both a holiday and makeup workday", date)
			}
		}
	}
	for yearKey := range document.Years {
		year, err := strconv.Atoi(yearKey)
		if err != nil || strconv.Itoa(year) != yearKey {
			return fmt.Errorf("calendar year key %q is not canonical", yearKey)
		}
		if index := sort.SearchInts(document.SupportedYears, year); index >= len(document.SupportedYears) || document.SupportedYears[index] != year {
			return fmt.Errorf("calendar year %d is not declared in supportedYears", year)
		}
	}
	return nil
}

func parseCalendarDate(field, date string) (time.Time, error) {
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil || parsed.Format(time.DateOnly) != date {
		return time.Time{}, fmt.Errorf("calendar %s must be an ISO date", field)
	}
	return parsed, nil
}

func validateCalendarDates(year int, field string, dates []string, coverageStart, coverageEnd string) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(dates))
	for index, date := range dates {
		parsed, err := time.Parse(time.DateOnly, date)
		if err != nil || parsed.Format(time.DateOnly) != date || parsed.Year() != year {
			return nil, fmt.Errorf("calendar %s contains invalid date %q for year %d", field, date, year)
		}
		if date < coverageStart || date > coverageEnd {
			return nil, fmt.Errorf("calendar %s date %s is outside coverage range", field, date)
		}
		if index > 0 && dates[index-1] >= date {
			return nil, fmt.Errorf("calendar %s dates must be sorted and unique", field)
		}
		seen[date] = struct{}{}
	}
	return seen, nil
}

func newBillingCalendar(document *calendarDocument) *billingCalendar {
	calendar := &billingCalendar{
		metadata: CalendarMetadata{
			Schema:         document.Schema,
			Version:        document.Version,
			Country:        document.Country,
			Source:         document.Source,
			SupportedYears: append([]int(nil), document.SupportedYears...),
			CoverageStart:  document.CoverageStart,
			CoverageEnd:    document.CoverageEnd,
		},
		coverageStart: document.CoverageStart,
		coverageEnd:   document.CoverageEnd,
		years:         make(map[int]calendarYear, len(document.Years)),
	}
	for _, year := range document.SupportedYears {
		data := document.Years[strconv.Itoa(year)]
		holidays := make(map[string]struct{}, len(data.Holidays))
		for _, date := range data.Holidays {
			holidays[date] = struct{}{}
		}
		makeupWorkdays := make(map[string]struct{}, len(data.MakeupWorkdays))
		for _, date := range data.MakeupWorkdays {
			makeupWorkdays[date] = struct{}{}
		}
		calendar.years[year] = calendarYear{holidays: holidays, makeupWorkdays: makeupWorkdays}
	}
	return calendar
}

// CalendarMetadataForCountry returns metadata for a supported country.
func CalendarMetadataForCountry(country string) (CalendarMetadata, bool) {
	calendar, ok := billingCalendars[normalizeCalendarCountry(country)]
	if !ok {
		return CalendarMetadata{}, false
	}
	metadata := calendar.metadata
	metadata.SupportedYears = append([]int(nil), calendar.metadata.SupportedYears...)
	return metadata, true
}

// ClassifyCalendarDay classifies a date for a country. available is false when
// either the country or the exact date is not covered by embedded data.
func ClassifyCalendarDay(country string, date time.Time) (dayType CalendarDayType, available bool) {
	calendar, ok := billingCalendars[normalizeCalendarCountry(country)]
	if !ok {
		return "", false
	}
	dateKey := date.Format(time.DateOnly)
	if dateKey < calendar.coverageStart || dateKey > calendar.coverageEnd {
		return "", false
	}
	year, ok := calendar.years[date.Year()]
	if !ok {
		return "", false
	}
	if _, ok := year.holidays[dateKey]; ok {
		return CalendarDayHoliday, true
	}
	if _, ok := year.makeupWorkdays[dateKey]; ok {
		return CalendarDayMakeupWorkday, true
	}
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return CalendarDayWeekend, true
	}
	return CalendarDayWorkday, true
}

func normalizeCalendarCountry(country string) string {
	return strings.ToUpper(strings.TrimSpace(country))
}
