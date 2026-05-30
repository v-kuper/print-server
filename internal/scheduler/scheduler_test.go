package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"atol-server/internal/receipt"
	"atol-server/internal/schedule"
)

func TestResetFromNowPersistsNextIntervalRun(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
	store := &fakeStore{settings: schedule.Settings{
		Enabled:         true,
		Mode:            schedule.ModeInterval,
		IntervalMinutes: 15,
		Timezone:        schedule.DefaultTimezone,
	}}
	service := NewService(store, &fakeJob{}, func() time.Time { return now })

	if err := service.ResetFromNow(context.Background()); err != nil {
		t.Fatalf("reset from now: %v", err)
	}

	if want := now.Add(15 * time.Minute); !store.state.NextRunAt.Equal(want) {
		t.Fatalf("expected next run %s, got %s", want, store.state.NextRunAt)
	}
}

func TestRunDuePrintsMissedRunOnceAndAdvancesNextRun(t *testing.T) {
	location, err := time.LoadLocation(schedule.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, 5, 25, 7, 30, 0, 0, location)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 15,
			Times:           []string{"07:00", "09:00"},
			Timezone:        schedule.DefaultTimezone,
		},
		state: schedule.State{
			NextRunAt: time.Date(2026, 5, 25, 7, 0, 0, 0, location),
		},
	}
	job := &fakeJob{}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected due run")
	}
	if job.calls != 1 {
		t.Fatalf("expected one print call, got %d", job.calls)
	}
	wantNext := time.Date(2026, 5, 25, 9, 0, 0, 0, location)
	if !store.state.NextRunAt.Equal(wantNext) {
		t.Fatalf("expected next run %s, got %s", wantNext, store.state.NextRunAt)
	}

	didRun, err = service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("second run due: %v", err)
	}
	if didRun {
		t.Fatal("expected missed run to be caught up only once")
	}
	if job.calls != 1 {
		t.Fatalf("expected no second print call, got %d", job.calls)
	}
}

func TestRunDueUsesDailyRunPresetContent(t *testing.T) {
	location, err := time.LoadLocation(schedule.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, 5, 25, 7, 1, 0, 0, location)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 15,
			Runs: []schedule.Run{
				{Time: "07:00", Profile: schedule.ProfileMorning},
				{Time: "13:00", Profile: schedule.ProfileDay},
			},
			Timezone: schedule.DefaultTimezone,
		},
		state: schedule.State{
			NextRunAt: time.Date(2026, 5, 25, 7, 0, 0, 0, location),
		},
	}
	job := &fakeJob{}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected due run")
	}
	if job.calls != 0 {
		t.Fatalf("expected global print not to be called, got %d", job.calls)
	}
	if job.contentAtCalls != 1 || job.contentCalls != 0 {
		t.Fatalf("expected one scheduled content print call, contentAt=%d content=%d", job.contentAtCalls, job.contentCalls)
	}
	if !job.printedContent.ShowWeather || !job.printedContent.ShowTonPortfolio || job.printedContent.ShowMail {
		t.Fatalf("expected morning preset content, got %#v", job.printedContent)
	}
}

func TestRunDueUsesCustomDailyRunContent(t *testing.T) {
	location, err := time.LoadLocation(schedule.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	custom := receipt.ContentSettings{
		Configured:       true,
		ShowWeather:      true,
		ShowUsdBynRate:   true,
		ShowBankRates:    true,
		ShowCalendar:     true,
		ShowNews:         true,
		ShowTonPortfolio: false,
		DenisTrendsMode:  receipt.DenisTrendsModeAuto,
	}
	now := time.Date(2026, 5, 25, 21, 1, 0, 0, location)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 15,
			Runs: []schedule.Run{
				{Time: "21:00", Profile: schedule.ProfileCustom, Content: &custom},
			},
			Timezone: schedule.DefaultTimezone,
		},
		state: schedule.State{
			NextRunAt: time.Date(2026, 5, 25, 21, 0, 0, 0, location),
		},
	}
	job := &fakeJob{}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected due run")
	}
	if job.contentAtCalls != 1 || job.contentCalls != 0 {
		t.Fatalf("expected one scheduled content print call, contentAt=%d content=%d", job.contentAtCalls, job.contentCalls)
	}
	if job.printedContent != custom {
		t.Fatalf("expected custom content %#v, got %#v", custom, job.printedContent)
	}
}

func TestRunDuePassesScheduledTimeToContentJob(t *testing.T) {
	location, err := time.LoadLocation(schedule.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	custom := receipt.ContentSettings{Configured: true, ShowDenisTrends: true, DenisTrendsMode: receipt.DenisTrendsModeAuto}
	scheduledAt := time.Date(2026, 5, 25, 7, 0, 0, 0, location)
	now := time.Date(2026, 5, 25, 18, 30, 0, 0, location)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 15,
			Runs: []schedule.Run{
				{Time: "07:00", Profile: schedule.ProfileCustom, Content: &custom},
			},
			Timezone: schedule.DefaultTimezone,
		},
		state: schedule.State{NextRunAt: scheduledAt},
	}
	job := &fakeJob{}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected due run")
	}
	if job.contentAtCalls != 1 || job.contentCalls != 0 {
		t.Fatalf("expected scheduled content print call, contentAt=%d content=%d", job.contentAtCalls, job.contentCalls)
	}
	if !job.scheduledAt.Equal(scheduledAt) {
		t.Fatalf("expected scheduled time %s, got %s", scheduledAt, job.scheduledAt)
	}
}

func TestRunDueUsesGlobalPrintForDefaultProfileAndInterval(t *testing.T) {
	location, err := time.LoadLocation(schedule.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, 5, 25, 7, 1, 0, 0, location)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeDailyTimes,
			IntervalMinutes: 15,
			Runs: []schedule.Run{
				{Time: "07:00", Profile: schedule.ProfileDefault},
			},
			Timezone: schedule.DefaultTimezone,
		},
		state: schedule.State{
			NextRunAt: time.Date(2026, 5, 25, 7, 0, 0, 0, location),
		},
	}
	job := &fakeJob{}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected due run")
	}
	if job.calls != 1 || job.contentCalls != 0 {
		t.Fatalf("expected default profile to use global print, got calls=%d contentCalls=%d", job.calls, job.contentCalls)
	}

	store.settings = schedule.Settings{
		Enabled:         true,
		Mode:            schedule.ModeInterval,
		IntervalMinutes: 15,
		Timezone:        schedule.DefaultTimezone,
	}
	store.state = schedule.State{NextRunAt: now.Add(-time.Minute)}
	didRun, err = service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("interval run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected interval due run")
	}
	if job.calls != 2 || job.contentCalls != 0 {
		t.Fatalf("expected interval to use global print, got calls=%d contentCalls=%d", job.calls, job.contentCalls)
	}
}

func TestRunDueUsesIntervalContentWhenConfigured(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
	content := receipt.ContentSettings{
		Configured:      true,
		ShowWeather:     true,
		ShowUsdBynRate:  true,
		ShowBankRates:   true,
		ShowNews:        true,
		DenisTrendsMode: receipt.DenisTrendsModeAuto,
	}
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeInterval,
			IntervalMinutes: 15,
			Timezone:        schedule.DefaultTimezone,
			IntervalContent: &content,
		},
		state: schedule.State{NextRunAt: now.Add(-time.Minute)},
	}
	job := &fakeJob{}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("run due: %v", err)
	}
	if !didRun {
		t.Fatal("expected interval due run")
	}
	if job.calls != 0 || job.contentAtCalls != 1 || job.contentCalls != 0 {
		t.Fatalf("expected interval scheduled content print, got calls=%d contentAt=%d content=%d", job.calls, job.contentAtCalls, job.contentCalls)
	}
	if job.printedContent != content {
		t.Fatalf("expected interval content %#v, got %#v", content, job.printedContent)
	}
}

func TestRunDueRecordsPrintErrorAndAdvancesNextRun(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeInterval,
			IntervalMinutes: 15,
			Timezone:        schedule.DefaultTimezone,
		},
		state: schedule.State{NextRunAt: now.Add(-time.Minute)},
	}
	job := &fakeJob{err: errors.New("printer offline")}
	service := NewService(store, job, func() time.Time { return now })

	didRun, err := service.RunDue(context.Background())
	if err == nil {
		t.Fatal("expected print error")
	}
	if !didRun {
		t.Fatal("expected due run")
	}
	if store.state.LastError != "printer offline" {
		t.Fatalf("expected stored error, got %q", store.state.LastError)
	}
	if !store.state.NextRunAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("expected next run to move forward, got %s", store.state.NextRunAt)
	}
}

func TestRunDueSkipsParallelExecution(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
	store := &fakeStore{
		settings: schedule.Settings{
			Enabled:         true,
			Mode:            schedule.ModeInterval,
			IntervalMinutes: 15,
			Timezone:        schedule.DefaultTimezone,
		},
		state: schedule.State{NextRunAt: now.Add(-time.Minute)},
	}
	job := &fakeJob{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(store, job, func() time.Time { return now })

	done := make(chan error, 1)
	go func() {
		_, err := service.RunDue(context.Background())
		done <- err
	}()
	<-job.started

	didRun, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("parallel run due: %v", err)
	}
	if didRun {
		t.Fatal("expected parallel due run to be skipped")
	}

	close(job.release)
	if err := <-done; err != nil {
		t.Fatalf("first run due: %v", err)
	}
	if job.calls != 1 {
		t.Fatalf("expected one print call, got %d", job.calls)
	}
}

type fakeStore struct {
	settings schedule.Settings
	state    schedule.State
}

func (s *fakeStore) LoadSchedule() (schedule.Settings, error) {
	return s.settings.Normalized(), nil
}

func (s *fakeStore) LoadScheduleState() (schedule.State, error) {
	return s.state, nil
}

func (s *fakeStore) SaveScheduleState(state schedule.State) error {
	s.state = state
	return nil
}

type fakeJob struct {
	calls          int
	contentCalls   int
	contentAtCalls int
	printedContent receipt.ContentSettings
	scheduledAt    time.Time
	err            error
	started        chan struct{}
	release        chan struct{}
}

func (j *fakeJob) PrintDailyReceipt(context.Context) error {
	j.calls++
	if j.started != nil {
		close(j.started)
		<-j.release
	}
	return j.err
}

func (j *fakeJob) PrintDailyReceiptAt(_ context.Context, scheduledAt time.Time) error {
	j.calls++
	j.scheduledAt = scheduledAt
	if j.started != nil {
		close(j.started)
		<-j.release
	}
	return j.err
}

func (j *fakeJob) PrintDailyReceiptWithContent(_ context.Context, content receipt.ContentSettings) error {
	j.contentCalls++
	j.printedContent = content
	return j.err
}

func (j *fakeJob) PrintDailyReceiptWithContentAt(_ context.Context, content receipt.ContentSettings, scheduledAt time.Time) error {
	j.contentAtCalls++
	j.printedContent = content
	j.scheduledAt = scheduledAt
	return j.err
}
