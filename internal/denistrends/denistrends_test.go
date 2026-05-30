package denistrends

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSettingsNormalizeDefaultsAndValidateCounts(t *testing.T) {
	settings := Settings{}.Normalized()

	if !settings.Periods[PeriodNow].Enabled || !settings.Periods[PeriodDay].Enabled || !settings.Periods[PeriodWeek].Enabled || !settings.Periods[PeriodMonth].Enabled {
		t.Fatalf("expected all periods enabled by default, got %#v", settings.Periods)
	}
	if settings.Periods[PeriodNow].MaxItems != DefaultMaxItems {
		t.Fatalf("expected default now count %d, got %d", DefaultMaxItems, settings.Periods[PeriodNow].MaxItems)
	}
	if settings.Periods[PeriodDay].MaxItems != DefaultMaxItems {
		t.Fatalf("expected default day count %d, got %d", DefaultMaxItems, settings.Periods[PeriodDay].MaxItems)
	}

	invalid := settings
	invalid.Periods[PeriodDay] = PeriodSettings{Enabled: true, MaxItems: 0}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected enabled period without valid count to fail validation")
	}
}

func TestActivePeriodsReturnsEnabledPeriodsInFixedOrder(t *testing.T) {
	settings := DefaultSettings()
	location := time.FixedZone("Europe/Minsk", 3*60*60)

	morning := time.Date(2026, 5, 29, 9, 0, 0, 0, location)
	if got := settings.ActivePeriods(morning); len(got) != 4 || got[0] != PeriodNow || got[1] != PeriodDay || got[2] != PeriodWeek || got[3] != PeriodMonth {
		t.Fatalf("expected all enabled periods in fixed order, got %#v", got)
	}

	weekdayEvening := time.Date(2026, 5, 29, 18, 0, 0, 0, location)
	if got := settings.ActivePeriods(weekdayEvening); len(got) != 4 || got[0] != PeriodNow || got[1] != PeriodDay || got[2] != PeriodWeek || got[3] != PeriodMonth {
		t.Fatalf("expected time of day not to affect enabled periods, got %#v", got)
	}

	disabled := DefaultSettings()
	disabled.Periods[PeriodNow] = PeriodSettings{Enabled: false, MaxItems: 20}
	disabled.Periods[PeriodWeek] = PeriodSettings{Enabled: false, MaxItems: 20}
	if got := disabled.ActivePeriods(morning); len(got) != 2 || got[0] != PeriodDay || got[1] != PeriodMonth {
		t.Fatalf("expected disabled periods to be skipped, got %#v", got)
	}
}

func TestProviderParsesRSSItemsInRankOrderAndUsesPeriodEndpoints(t *testing.T) {
	var requested []string
	client := &http.Client{Transport: trendsRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requested = append(requested, request.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/rss+xml"}},
			Body: io.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
				<rss version="2.0" xmlns:sm="https://shir-man.com/rss/ns#">
					<channel>
						<item>
							<title>HN one</title>
							<link>https://example.com/hn1</link>
							<sm:source>hackerNews</sm:source>
							<sm:rank>1</sm:rank>
						</item>
						<item>
							<title>LessWrong kept</title>
							<link>https://example.com/lw</link>
							<sm:source>lessWrong</sm:source>
							<sm:rank>2</sm:rank>
						</item>
						<item>
							<title>Repo one</title>
							<link>https://github.com/acme/one</link>
							<sm:source>github</sm:source>
							<sm:rank>3</sm:rank>
						</item>
					</channel>
				</rss>`)),
		}, nil
	})}

	settings := DefaultSettings()
	settings.Periods[PeriodDay] = PeriodSettings{Enabled: false, MaxItems: 20}
	settings.Periods[PeriodWeek] = PeriodSettings{Enabled: false, MaxItems: 20}
	settings.Periods[PeriodMonth] = PeriodSettings{Enabled: false, MaxItems: 20}
	settings.Periods[PeriodNow] = PeriodSettings{Enabled: true, MaxItems: 2}
	sections, err := NewProvider(client).Current(context.Background(), settings, time.Date(2026, 5, 29, 9, 0, 0, 0, time.FixedZone("Europe/Minsk", 3*60*60)))
	if err != nil {
		t.Fatalf("load trends: %v", err)
	}

	if len(requested) != 1 || requested[0] != "https://shir-man.com/api/rss" {
		t.Fatalf("unexpected requests: %#v", requested)
	}
	if len(sections) != 1 || sections[0].Period != PeriodNow || sections[0].Title != "Top now" {
		t.Fatalf("expected one active section, got %#v", sections)
	}
	items := sections[0].Items
	if len(items) != 2 {
		t.Fatalf("expected limited RSS items, got %#v", items)
	}
	if items[0].Title != "HN one" || items[0].SourceName != "Hacker News" || items[1].Title != "LessWrong kept" || items[1].SourceName != "LessWrong" {
		t.Fatalf("expected ranking order to be preserved, got %#v", items)
	}
}

func TestProviderUsesSortQueryForDayWeekMonthRSS(t *testing.T) {
	var requested []string
	client := &http.Client{Transport: trendsRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requested = append(requested, request.URL.String())
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/rss+xml"}},
			Body:       io.NopCloser(strings.NewReader(`<rss version="2.0"><channel><item><title>One</title><link>https://example.com/one</link></item></channel></rss>`)),
		}, nil
	})}

	settings := DefaultSettings()
	_, err := NewProvider(client).Current(context.Background(), settings, time.Date(2026, 5, 31, 18, 0, 0, 0, time.FixedZone("Europe/Minsk", 3*60*60)))
	if err != nil {
		t.Fatalf("load trends: %v", err)
	}

	want := []string{
		"https://shir-man.com/api/rss",
		"https://shir-man.com/api/rss?sort=day",
		"https://shir-man.com/api/rss?sort=week",
		"https://shir-man.com/api/rss?sort=month",
	}
	if len(requested) != len(want) {
		t.Fatalf("expected requests %#v, got %#v", want, requested)
	}
	for index := range want {
		if requested[index] != want[index] {
			t.Fatalf("expected requests %#v, got %#v", want, requested)
		}
	}
}

type trendsRoundTripFunc func(*http.Request) (*http.Response, error)

func (f trendsRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
