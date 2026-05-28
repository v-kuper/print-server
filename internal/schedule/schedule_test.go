package schedule

import (
	"reflect"
	"testing"
	"time"

	"atol-server/internal/receipt"
)

func TestSettingsNormalizeSortsAndDeduplicatesDailyTimes(t *testing.T) {
	settings := Settings{
		Enabled:         true,
		Mode:            ModeDailyTimes,
		IntervalMinutes: 0,
		Times:           []string{" 09:00 ", "07:00", "09:00"},
	}

	normalized := settings.Normalized()

	if normalized.Timezone != DefaultTimezone {
		t.Fatalf("expected default timezone %q, got %q", DefaultTimezone, normalized.Timezone)
	}
	if normalized.IntervalMinutes != DefaultIntervalMinutes {
		t.Fatalf("expected default interval %d, got %d", DefaultIntervalMinutes, normalized.IntervalMinutes)
	}
	if !reflect.DeepEqual(normalized.Times, []string{"07:00", "09:00"}) {
		t.Fatalf("expected sorted unique times, got %#v", normalized.Times)
	}
	if !reflect.DeepEqual(normalized.Runs, []Run{
		{Time: "07:00", Profile: ProfileDefault},
		{Time: "09:00", Profile: ProfileDefault},
	}) {
		t.Fatalf("expected default runs from legacy times, got %#v", normalized.Runs)
	}
}

func TestSettingsNormalizeSortsAndDeduplicatesRuns(t *testing.T) {
	settings := Settings{
		Enabled: true,
		Mode:    ModeDailyTimes,
		Runs: []Run{
			{Time: " 09:00 ", Profile: ProfileEvening},
			{Time: "07:00", Profile: ProfileMorning},
			{Time: "09:00", Profile: ProfileDay},
		},
		Timezone: DefaultTimezone,
	}

	normalized := settings.Normalized()

	if !reflect.DeepEqual(normalized.Times, []string{"07:00", "09:00"}) {
		t.Fatalf("expected times from runs, got %#v", normalized.Times)
	}
	if !reflect.DeepEqual(normalized.Runs, []Run{
		{Time: "07:00", Profile: ProfileMorning},
		{Time: "09:00", Profile: ProfileEvening},
	}) {
		t.Fatalf("expected sorted unique runs, got %#v", normalized.Runs)
	}
}

func TestSettingsValidateRejectsInvalidInterval(t *testing.T) {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Mode = ModeInterval
	settings.IntervalMinutes = 2

	if err := settings.Validate(); err == nil {
		t.Fatal("expected invalid interval error")
	}
}

func TestSettingsValidateRejectsInvalidDailyTime(t *testing.T) {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Mode = ModeDailyTimes
	settings.Times = []string{"24:01"}

	if err := settings.Validate(); err == nil {
		t.Fatal("expected invalid time error")
	}
}

func TestSettingsValidateRejectsUnknownRunProfile(t *testing.T) {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Mode = ModeDailyTimes
	settings.Runs = []Run{{Time: "07:00", Profile: "weekend"}}

	if err := settings.Validate(); err == nil {
		t.Fatal("expected unknown profile error")
	}
}

func TestSettingsValidateRejectsCustomRunWithoutContent(t *testing.T) {
	settings := DefaultSettings()
	settings.Enabled = true
	settings.Mode = ModeDailyTimes
	settings.Runs = []Run{{Time: "07:00", Profile: ProfileCustom}}

	if err := settings.Validate(); err == nil {
		t.Fatal("expected missing custom content error")
	}
}

func TestRunResolvesPresetContent(t *testing.T) {
	global := Run{Profile: ProfileDefault}.ResolveContent(receiptContent(false, false, true, false, false, false, false, false, false))
	if global.ShowWeather || !global.ShowMotivationQuote {
		t.Fatalf("expected default profile to use global content, got %#v", global)
	}

	morning := Run{Profile: ProfileMorning}.ResolveContent(receiptContent(false, false, false, false, false, false, true, false, false))
	if !morning.ShowWeather || !morning.ShowWeatherAdvice || !morning.ShowMotivationQuote ||
		!morning.ShowTonPortfolio || !morning.ShowUsdBynRate || !morning.ShowBankRates ||
		!morning.ShowCalendar || !morning.ShowNews || morning.ShowMail {
		t.Fatalf("expected morning preset to enable all daily sections except mail, got %#v", morning)
	}

	day := Run{Profile: ProfileDay}.ResolveContent(receiptContent(false, false, false, false, false, false, false, false, false))
	if !day.ShowWeather || !day.ShowWeatherAdvice || !day.ShowUsdBynRate || !day.ShowBankRates || !day.ShowCalendar ||
		day.ShowMotivationQuote || day.ShowTonPortfolio || day.ShowMail || day.ShowNews {
		t.Fatalf("expected day preset to include weather, rates, calendar only, got %#v", day)
	}

	evening := Run{Profile: ProfileEvening}.ResolveContent(receiptContent(false, false, false, false, false, false, false, false, false))
	if !evening.ShowWeather || !evening.ShowWeatherAdvice || !evening.ShowUsdBynRate || !evening.ShowBankRates ||
		!evening.ShowCalendar || !evening.ShowNews || evening.ShowMotivationQuote || evening.ShowTonPortfolio || evening.ShowMail {
		t.Fatalf("expected evening preset to include day sections plus news, got %#v", evening)
	}
}

func TestNextAfterUsesEuropeMinskDailyTimes(t *testing.T) {
	now := time.Date(2026, 5, 25, 6, 30, 0, 0, time.UTC)
	settings := Settings{
		Enabled:         true,
		Mode:            ModeDailyTimes,
		IntervalMinutes: 15,
		Times:           []string{"09:00", "10:00"},
		Timezone:        DefaultTimezone,
	}

	next, ok, err := NextAfter(settings, now)
	if err != nil {
		t.Fatalf("next after: %v", err)
	}
	if !ok {
		t.Fatal("expected next run")
	}

	location, err := time.LoadLocation(DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	want := time.Date(2026, 5, 25, 10, 0, 0, 0, location)
	if !next.Equal(want) {
		t.Fatalf("expected next %s, got %s", want, next)
	}
}

func TestDueRunCatchesUpPersistedMissedRunOnce(t *testing.T) {
	location, err := time.LoadLocation(DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	settings := Settings{
		Enabled:         true,
		Mode:            ModeDailyTimes,
		IntervalMinutes: 15,
		Times:           []string{"07:00", "09:00"},
		Timezone:        DefaultTimezone,
	}
	state := State{
		NextRunAt: time.Date(2026, 5, 25, 7, 0, 0, 0, location),
	}
	now := time.Date(2026, 5, 25, 7, 30, 0, 0, location)

	scheduledAt, due, err := DueRun(settings, state, now)
	if err != nil {
		t.Fatalf("due run: %v", err)
	}
	if !due {
		t.Fatal("expected one missed run to be due")
	}
	if !scheduledAt.Equal(state.NextRunAt) {
		t.Fatalf("expected due scheduled time %s, got %s", state.NextRunAt, scheduledAt)
	}

	advanced, ok, err := NextAfter(settings, scheduledAt)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if !ok {
		t.Fatal("expected advanced next run")
	}
	want := time.Date(2026, 5, 25, 9, 0, 0, 0, location)
	if !advanced.Equal(want) {
		t.Fatalf("expected advanced next %s, got %s", want, advanced)
	}
}

func TestNextAfterForIntervalCountsFromReferenceTime(t *testing.T) {
	settings := Settings{
		Enabled:         true,
		Mode:            ModeInterval,
		IntervalMinutes: 15,
		Timezone:        DefaultTimezone,
	}
	now := time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)

	next, ok, err := NextAfter(settings, now)
	if err != nil {
		t.Fatalf("next after: %v", err)
	}
	if !ok {
		t.Fatal("expected next interval run")
	}
	if want := now.Add(15 * time.Minute); !next.Equal(want) {
		t.Fatalf("expected %s, got %s", want, next)
	}
}

func receiptContent(weather, weatherAdvice, motivation, ton, usdByn, bankRates, mail, calendar, news bool) receipt.ContentSettings {
	return receipt.ContentSettings{
		Configured:          true,
		ShowWeather:         weather,
		ShowWeatherAdvice:   weatherAdvice,
		ShowMotivationQuote: motivation,
		ShowTonPortfolio:    ton,
		ShowUsdBynRate:      usdByn,
		ShowBankRates:       bankRates,
		ShowMail:            mail,
		ShowCalendar:        calendar,
		ShowNews:            news,
	}
}
