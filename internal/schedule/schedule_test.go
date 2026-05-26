package schedule

import (
	"reflect"
	"testing"
	"time"
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
