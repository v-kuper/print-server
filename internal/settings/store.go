package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"atol-server/internal/finance"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/schedule"
	"atol-server/internal/weather"
)

type Store struct {
	path string
	mu   sync.Mutex
}

type fileSettings struct {
	Printer        printer.Config          `json:"printer"`
	Weather        weather.Location        `json:"weather"`
	Finance        finance.TonPortfolio    `json:"finance"`
	News           news.Settings           `json:"news"`
	Motivation     motivation.Settings     `json:"motivation"`
	Receipt        receipt.StyleSettings   `json:"receipt"`
	ReceiptContent receipt.ContentSettings `json:"receiptContent"`
	Schedule       schedule.Settings       `json:"schedule"`
	ScheduleState  schedule.State          `json:"scheduleState"`
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) LoadPrinter() (printer.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return printer.Config{}, err
	}
	return settings.Printer.Normalized(), nil
}

func (s *Store) SavePrinter(config printer.Config) error {
	normalized := config.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.Printer = normalized
	return s.save(settings)
}

func (s *Store) LoadWeather() (weather.Location, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return weather.Location{}, err
	}
	return settings.Weather.Normalized(), nil
}

func (s *Store) SaveWeather(location weather.Location) error {
	normalized := location.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.Weather = normalized
	return s.save(settings)
}

func (s *Store) LoadFinance() (finance.TonPortfolio, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return finance.TonPortfolio{}, err
	}
	return settings.Finance.Normalized(), nil
}

func (s *Store) SaveFinance(portfolio finance.TonPortfolio) error {
	normalized := portfolio.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.Finance = normalized
	return s.save(settings)
}

func (s *Store) LoadNews() (news.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return news.Settings{}, err
	}
	return settings.News.Normalized(), nil
}

func (s *Store) LoadMotivation() (motivation.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return motivation.Settings{}, err
	}
	return settings.Motivation.Normalized(), nil
}

func (s *Store) SaveMotivation(motivationSettings motivation.Settings) error {
	normalized := motivationSettings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.Motivation = normalized
	return s.save(settings)
}

func (s *Store) SaveNews(newsSettings news.Settings) error {
	if err := newsSettings.Validate(); err != nil {
		return err
	}
	normalized := newsSettings.Normalized()
	for _, source := range normalized.Sources {
		if err := source.Validate(); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.News = normalized
	return s.save(settings)
}

func (s *Store) LoadReceiptStyle() (receipt.StyleSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return receipt.StyleSettings{}, err
	}
	return settings.Receipt.Normalized(), nil
}

func (s *Store) SaveReceiptStyle(style receipt.StyleSettings) error {
	normalized := style.Normalized()

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.Receipt = normalized
	return s.save(settings)
}

func (s *Store) LoadReceiptContent() (receipt.ContentSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return receipt.ContentSettings{}, err
	}
	return settings.ReceiptContent.Normalized(), nil
}

func (s *Store) SaveReceiptContent(content receipt.ContentSettings) error {
	normalized := content.Normalized()

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.ReceiptContent = normalized
	return s.save(settings)
}

func (s *Store) LoadSchedule() (schedule.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return schedule.Settings{}, err
	}
	return settings.Schedule.Normalized(), nil
}

func (s *Store) SaveSchedule(scheduleSettings schedule.Settings) error {
	normalized := scheduleSettings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.Schedule = normalized
	return s.save(settings)
}

func (s *Store) LoadScheduleState() (schedule.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return schedule.State{}, err
	}
	return settings.ScheduleState, nil
}

func (s *Store) SaveScheduleState(state schedule.State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	settings, err := s.load()
	if err != nil {
		return err
	}
	settings.ScheduleState = state
	return s.save(settings)
}

func (s *Store) load() (fileSettings, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSettings{
			Printer:        printer.DefaultConfig(),
			Weather:        weather.DefaultLocation(),
			Finance:        finance.DefaultTonPortfolio(),
			News:           news.DefaultSettings(),
			Motivation:     motivation.DefaultSettings(),
			Receipt:        receipt.DefaultStyleSettings(),
			ReceiptContent: receipt.DefaultContentSettings(),
			Schedule:       schedule.DefaultSettings(),
		}, nil
	}
	if err != nil {
		return fileSettings{}, err
	}

	var settings fileSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fileSettings{}, err
	}
	if settings.Printer.Host == "" && settings.Printer.Port == 0 {
		settings.Printer = printer.DefaultConfig()
	}
	if settings.Weather.Name == "" && settings.Weather.Latitude == 0 && settings.Weather.Longitude == 0 {
		settings.Weather = weather.DefaultLocation()
	}
	if settings.Finance.AmountTon == 0 && settings.Finance.InvestedUSD == 0 {
		settings.Finance = finance.DefaultTonPortfolio()
	}
	if len(settings.News.Sources) == 0 {
		settings.News = news.DefaultSettings()
	}
	settings.Motivation = settings.Motivation.Normalized()
	if !settings.Receipt.Configured {
		settings.Receipt = receipt.DefaultStyleSettings()
	}
	if !settings.ReceiptContent.Configured {
		settings.ReceiptContent = receipt.DefaultContentSettings()
	}
	if isZeroSchedule(settings.Schedule) {
		settings.Schedule = schedule.DefaultSettings()
	} else {
		settings.Schedule = settings.Schedule.Normalized()
	}
	return settings, nil
}

func isZeroSchedule(settings schedule.Settings) bool {
	return !settings.Enabled &&
		settings.Mode == "" &&
		settings.IntervalMinutes == 0 &&
		len(settings.Times) == 0 &&
		settings.Timezone == ""
}

func (s *Store) save(settings fileSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}
