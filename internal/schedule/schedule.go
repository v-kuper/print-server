package schedule

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"atol-server/internal/receipt"
)

type Mode string

type RunProfile string

const (
	ModeInterval   Mode = "interval"
	ModeDailyTimes Mode = "daily_times"

	ProfileDefault RunProfile = "default"
	ProfileMorning RunProfile = "morning"
	ProfileDay     RunProfile = "day"
	ProfileEvening RunProfile = "evening"
	ProfileCustom  RunProfile = "custom"

	DefaultTimezone        = "Europe/Minsk"
	DefaultIntervalMinutes = 15
)

var allowedIntervals = map[int]struct{}{
	1:    {},
	5:    {},
	15:   {},
	30:   {},
	60:   {},
	120:  {},
	360:  {},
	720:  {},
	1440: {},
}

type Settings struct {
	Enabled         bool                     `json:"enabled"`
	Mode            Mode                     `json:"mode"`
	IntervalMinutes int                      `json:"intervalMinutes"`
	Times           []string                 `json:"times"`
	Timezone        string                   `json:"timezone"`
	IntervalContent *receipt.ContentSettings `json:"intervalContent,omitempty"`
	Runs            []Run                    `json:"runs,omitempty"`
}

type Run struct {
	Time    string                   `json:"time"`
	Profile RunProfile               `json:"profile"`
	Content *receipt.ContentSettings `json:"content,omitempty"`
}

type State struct {
	LastAttemptAt time.Time `json:"lastAttemptAt,omitempty"`
	LastSuccessAt time.Time `json:"lastSuccessAt,omitempty"`
	LastError     string    `json:"lastError,omitempty"`
	NextRunAt     time.Time `json:"nextRunAt,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{
		Enabled:         false,
		Mode:            ModeInterval,
		IntervalMinutes: DefaultIntervalMinutes,
		Times:           []string{"07:00"},
		Timezone:        DefaultTimezone,
	}
}

func (s Settings) Normalized() Settings {
	normalized := s
	if normalized.Mode == "" {
		normalized.Mode = ModeInterval
	}
	if normalized.IntervalMinutes == 0 {
		normalized.IntervalMinutes = DefaultIntervalMinutes
	}
	if strings.TrimSpace(normalized.Timezone) == "" {
		normalized.Timezone = DefaultTimezone
	} else {
		normalized.Timezone = strings.TrimSpace(normalized.Timezone)
	}
	normalized.Times = normalizeTimes(normalized.Times)
	if normalized.IntervalContent != nil {
		content := normalized.IntervalContent.Normalized()
		normalized.IntervalContent = &content
	}
	normalized.Runs = normalizeRuns(normalized.Runs)
	if len(normalized.Runs) > 0 {
		normalized.Times = timesFromRuns(normalized.Runs)
	} else {
		normalized.Runs = runsFromTimes(normalized.Times)
	}
	if len(normalized.Times) == 0 {
		normalized.Times = []string{"07:00"}
		normalized.Runs = runsFromTimes(normalized.Times)
	}
	return normalized
}

func (s Settings) Validate() error {
	normalized := s.Normalized()
	if normalized.Mode != ModeInterval && normalized.Mode != ModeDailyTimes {
		return fmt.Errorf("unsupported schedule mode %q", normalized.Mode)
	}
	if _, err := time.LoadLocation(normalized.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	if _, ok := allowedIntervals[normalized.IntervalMinutes]; !ok {
		return fmt.Errorf("interval must be one of 1, 5, 15, 30, 60, 120, 360, 720, 1440 minutes")
	}
	if normalized.Mode == ModeDailyTimes {
		if len(normalized.Times) == 0 {
			return errors.New("at least one daily time is required")
		}
		for _, value := range normalized.Times {
			if _, _, err := parseHHMM(value); err != nil {
				return err
			}
		}
		for _, run := range normalized.Runs {
			if _, _, err := parseHHMM(run.Time); err != nil {
				return err
			}
			if err := run.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r Run) Validate() error {
	switch r.Profile {
	case ProfileDefault, ProfileMorning, ProfileDay, ProfileEvening:
		return nil
	case ProfileCustom:
		if r.Content == nil {
			return errors.New("custom schedule run requires content")
		}
		return nil
	default:
		return fmt.Errorf("unsupported schedule run profile %q", r.Profile)
	}
}

func (r Run) ResolveContent(global receipt.ContentSettings) receipt.ContentSettings {
	switch r.Profile {
	case ProfileMorning:
		return receipt.DefaultContentSettings()
	case ProfileDay:
		return receipt.ContentSettings{
			Configured:        true,
			ShowWeather:       true,
			ShowWeatherAdvice: true,
			ShowUsdBynRate:    true,
			ShowBankRates:     true,
			ShowCalendar:      true,
			ShowHistory:       true,
		}
	case ProfileEvening:
		return receipt.ContentSettings{
			Configured:        true,
			ShowWeather:       true,
			ShowWeatherAdvice: true,
			ShowUsdBynRate:    true,
			ShowBankRates:     true,
			ShowCalendar:      true,
			ShowHistory:       true,
			ShowNews:          true,
		}
	case ProfileCustom:
		if r.Content != nil {
			return r.Content.Normalized()
		}
	}
	return global.Normalized()
}

func NextAfter(settings Settings, after time.Time) (time.Time, bool, error) {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return time.Time{}, false, err
	}
	if !normalized.Enabled {
		return time.Time{}, false, nil
	}
	if normalized.Mode == ModeInterval {
		return after.Add(time.Duration(normalized.IntervalMinutes) * time.Minute), true, nil
	}

	location, err := time.LoadLocation(normalized.Timezone)
	if err != nil {
		return time.Time{}, false, err
	}
	localAfter := after.In(location)
	for dayOffset := 0; dayOffset <= 1; dayOffset++ {
		day := localAfter.AddDate(0, 0, dayOffset)
		for _, value := range normalized.Times {
			hour, minute, err := parseHHMM(value)
			if err != nil {
				return time.Time{}, false, err
			}
			candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, location)
			if candidate.After(localAfter) {
				return candidate, true, nil
			}
		}
	}
	return time.Time{}, false, errors.New("could not calculate next schedule run")
}

func DueRun(settings Settings, state State, now time.Time) (time.Time, bool, error) {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return time.Time{}, false, err
	}
	if !normalized.Enabled || state.NextRunAt.IsZero() {
		return time.Time{}, false, nil
	}
	if state.NextRunAt.After(now) {
		return time.Time{}, false, nil
	}
	return state.NextRunAt, true, nil
}

func RunForScheduledAt(settings Settings, scheduledAt time.Time) (Run, bool, error) {
	normalized := settings.Normalized()
	if normalized.Mode != ModeDailyTimes {
		return Run{}, false, nil
	}
	location, err := time.LoadLocation(normalized.Timezone)
	if err != nil {
		return Run{}, false, err
	}
	timeKey := scheduledAt.In(location).Format("15:04")
	for _, run := range normalized.Runs {
		if run.Time == timeKey {
			return run, true, nil
		}
	}
	return Run{}, false, nil
}

func normalizeTimes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	sort.Strings(result)
	return result
}

func normalizeRuns(values []Run) []Run {
	seen := make(map[string]struct{}, len(values))
	result := make([]Run, 0, len(values))
	for _, value := range values {
		run := value
		run.Time = strings.TrimSpace(run.Time)
		if run.Time == "" {
			continue
		}
		if _, ok := seen[run.Time]; ok {
			continue
		}
		seen[run.Time] = struct{}{}
		if run.Profile == "" {
			run.Profile = ProfileDefault
		}
		if run.Profile != ProfileCustom {
			run.Content = nil
		} else if run.Content != nil {
			normalized := run.Content.Normalized()
			run.Content = &normalized
		}
		result = append(result, run)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time < result[j].Time
	})
	return result
}

func timesFromRuns(values []Run) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Time)
	}
	return result
}

func runsFromTimes(values []string) []Run {
	result := make([]Run, 0, len(values))
	for _, value := range values {
		result = append(result, Run{
			Time:    value,
			Profile: ProfileDefault,
		})
	}
	return result
}

func parseHHMM(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, 0, fmt.Errorf("time %q must use HH:MM format", value)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour in %q", value)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minute in %q", value)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("time %q must be between 00:00 and 23:59", value)
	}
	return hour, minute, nil
}
