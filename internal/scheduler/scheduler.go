package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"atol-server/internal/receipt"
	"atol-server/internal/schedule"
)

type Store interface {
	LoadSchedule() (schedule.Settings, error)
	LoadScheduleState() (schedule.State, error)
	SaveScheduleState(schedule.State) error
}

type Job interface {
	PrintDailyReceipt(context.Context) error
}

type ContentJob interface {
	PrintDailyReceiptWithContent(context.Context, receipt.ContentSettings) error
}

type Service struct {
	store    Store
	job      Job
	clock    func() time.Time
	reloadCh chan struct{}

	runMu   sync.Mutex
	mu      sync.Mutex
	running bool
}

type Status struct {
	Settings      schedule.Settings `json:"settings"`
	Running       bool              `json:"running"`
	LastAttemptAt time.Time         `json:"lastAttemptAt,omitempty"`
	LastSuccessAt time.Time         `json:"lastSuccessAt,omitempty"`
	LastError     string            `json:"lastError,omitempty"`
	NextRunAt     time.Time         `json:"nextRunAt,omitempty"`
}

func NewService(store Store, job Job, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{
		store:    store,
		job:      job,
		clock:    clock,
		reloadCh: make(chan struct{}, 1),
	}
}

func (s *Service) Start(ctx context.Context) {
	for {
		wait, ok, err := s.nextWait(ctx)
		if err != nil {
			if !s.wait(ctx, time.Minute) {
				return
			}
			continue
		}
		if !ok {
			if !s.waitForReload(ctx) {
				return
			}
			continue
		}
		if wait <= 0 {
			_, _ = s.RunDue(ctx)
			continue
		}
		if !s.wait(ctx, wait) {
			return
		}
	}
}

func (s *Service) ResetFromNow(ctx context.Context) error {
	settings, err := s.store.LoadSchedule()
	if err != nil {
		return err
	}
	settings = settings.Normalized()
	if err := settings.Validate(); err != nil {
		return err
	}

	state, err := s.store.LoadScheduleState()
	if err != nil {
		return err
	}
	state.LastError = ""
	if !settings.Enabled {
		state.NextRunAt = time.Time{}
		if err := s.store.SaveScheduleState(state); err != nil {
			return err
		}
		s.Reload()
		return nil
	}

	nextRunAt, ok, err := schedule.NextAfter(settings, s.clock())
	if err != nil {
		return err
	}
	if ok {
		state.NextRunAt = nextRunAt
	}
	if err := s.store.SaveScheduleState(state); err != nil {
		return err
	}
	s.Reload()
	return nil
}

func (s *Service) RunDue(ctx context.Context) (bool, error) {
	if !s.runMu.TryLock() {
		return false, nil
	}
	defer s.runMu.Unlock()

	s.setRunning(true)
	defer s.setRunning(false)

	settings, err := s.store.LoadSchedule()
	if err != nil {
		return false, err
	}
	state, err := s.store.LoadScheduleState()
	if err != nil {
		return false, err
	}

	now := s.clock()
	scheduledAt, due, err := schedule.DueRun(settings, state, now)
	if err != nil {
		return false, err
	}
	if !due {
		return false, nil
	}

	state.LastAttemptAt = now
	state.LastError = ""
	if err := s.store.SaveScheduleState(state); err != nil {
		return false, err
	}

	printErr := s.printScheduledReceipt(ctx, settings, scheduledAt)
	finishedAt := s.clock()
	if printErr != nil {
		state.LastError = printErr.Error()
	} else {
		state.LastSuccessAt = finishedAt
		state.LastError = ""
	}
	nextRunAt, ok, err := schedule.NextAfter(settings, now)
	if err != nil {
		return true, err
	}
	if ok {
		state.NextRunAt = nextRunAt
	} else {
		state.NextRunAt = time.Time{}
	}
	if err := s.store.SaveScheduleState(state); err != nil {
		return true, err
	}
	return true, printErr
}

func (s *Service) printScheduledReceipt(ctx context.Context, settings schedule.Settings, scheduledAt time.Time) error {
	normalized := settings.Normalized()
	if normalized.Mode == schedule.ModeInterval {
		if normalized.IntervalContent == nil {
			return s.job.PrintDailyReceipt(ctx)
		}
		contentJob, ok := s.job.(ContentJob)
		if !ok {
			return fmt.Errorf("scheduled content printing is not supported")
		}
		return contentJob.PrintDailyReceiptWithContent(ctx, *normalized.IntervalContent)
	}
	if normalized.Mode != schedule.ModeDailyTimes {
		return s.job.PrintDailyReceipt(ctx)
	}
	run, ok, err := schedule.RunForScheduledAt(normalized, scheduledAt)
	if err != nil {
		return err
	}
	if !ok || run.Profile == schedule.ProfileDefault {
		return s.job.PrintDailyReceipt(ctx)
	}
	contentJob, ok := s.job.(ContentJob)
	if !ok {
		return fmt.Errorf("scheduled content printing is not supported")
	}
	return contentJob.PrintDailyReceiptWithContent(ctx, run.ResolveContent(receipt.DefaultContentSettings()))
}

func (s *Service) Reload() {
	select {
	case s.reloadCh <- struct{}{}:
	default:
	}
}

func (s *Service) Status() (Status, error) {
	settings, err := s.store.LoadSchedule()
	if err != nil {
		return Status{}, err
	}
	state, err := s.store.LoadScheduleState()
	if err != nil {
		return Status{}, err
	}
	return Status{
		Settings:      settings.Normalized(),
		Running:       s.isRunning(),
		LastAttemptAt: state.LastAttemptAt,
		LastSuccessAt: state.LastSuccessAt,
		LastError:     state.LastError,
		NextRunAt:     state.NextRunAt,
	}, nil
}

func (s *Service) nextWait(ctx context.Context) (time.Duration, bool, error) {
	settings, err := s.store.LoadSchedule()
	if err != nil {
		return 0, false, err
	}
	settings = settings.Normalized()
	if err := settings.Validate(); err != nil {
		return 0, false, err
	}
	if !settings.Enabled {
		return 0, false, nil
	}

	state, err := s.store.LoadScheduleState()
	if err != nil {
		return 0, false, err
	}
	now := s.clock()
	if state.NextRunAt.IsZero() {
		if err := s.ResetFromNow(ctx); err != nil {
			return 0, false, err
		}
		state, err = s.store.LoadScheduleState()
		if err != nil {
			return 0, false, err
		}
	}
	return state.NextRunAt.Sub(now), true, nil
}

func (s *Service) wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-s.reloadCh:
		return true
	case <-timer.C:
		return true
	}
}

func (s *Service) waitForReload(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-s.reloadCh:
		return true
	}
}

func (s *Service) setRunning(running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = running
}

func (s *Service) isRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
