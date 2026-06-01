package web

import (
	"encoding/json"
	"net/http"

	"atol-server/internal/denistrends"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/receiptsnapshot"
	"atol-server/internal/schedule"
	"atol-server/internal/weather"
)

type locationSearchResponse struct {
	OK      bool                        `json:"ok"`
	Message string                      `json:"message,omitempty"`
	Error   string                      `json:"error,omitempty"`
	Results []weather.LocationCandidate `json:"results,omitempty"`
}

type motivationResponse struct {
	OK       bool                 `json:"ok"`
	Message  string               `json:"message,omitempty"`
	Error    string               `json:"error,omitempty"`
	Quote    *motivation.Quote    `json:"quote,omitempty"`
	Settings *motivation.Settings `json:"settings,omitempty"`
}

type bootstrapResponse struct {
	OK    bool           `json:"ok"`
	Error string         `json:"error,omitempty"`
	Data  *bootstrapData `json:"data,omitempty"`
}

type bootstrapData struct {
	Printer           printer.Config           `json:"printer"`
	Weather           weather.Location         `json:"weather"`
	Finance           finance.TonPortfolio     `json:"finance"`
	Motivation        motivation.Settings      `json:"motivation"`
	GoogleStatus      googleintegration.Status `json:"googleStatus"`
	News              bootstrapNewsSettings    `json:"news"`
	DenisTrends       denistrends.Settings     `json:"denisTrends"`
	ReceiptStyle      receipt.StyleSettings    `json:"receiptStyle"`
	ReceiptContent    receipt.ContentSettings  `json:"receiptContent"`
	ReceiptSnapshot   receiptsnapshot.Settings `json:"receiptSnapshot"`
	Schedule          schedule.Settings        `json:"schedule"`
	NewsPresets       []news.PresetInfo        `json:"newsPresets"`
	FontOptions       []int                    `json:"fontOptions"`
	ScheduleIntervals []scheduleIntervalOption `json:"scheduleIntervals"`
}

type bootstrapNewsSettings struct {
	TranslateTitles bool                  `json:"translateTitles"`
	Sources         []news.SourceSettings `json:"sources"`
}

type scheduleIntervalOption struct {
	Minutes int    `json:"minutes"`
	Label   string `json:"label"`
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	data, err := s.bootstrapData()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, bootstrapResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, bootstrapResponse{
		OK:   true,
		Data: &data,
	})
}

func (s *Server) bootstrapData() (bootstrapData, error) {
	printerConfig, err := s.store.LoadPrinter()
	if err != nil {
		return bootstrapData{}, err
	}
	weatherLocation, err := s.store.LoadWeather()
	if err != nil {
		return bootstrapData{}, err
	}
	portfolio, err := s.store.LoadFinance()
	if err != nil {
		return bootstrapData{}, err
	}
	newsSettings, err := s.store.LoadNews()
	if err != nil {
		return bootstrapData{}, err
	}
	denisTrendsSettings, err := s.store.LoadDenisTrends()
	if err != nil {
		return bootstrapData{}, err
	}
	motivationSettings, err := s.store.LoadMotivation()
	if err != nil {
		return bootstrapData{}, err
	}
	receiptStyle, err := s.store.LoadReceiptStyle()
	if err != nil {
		return bootstrapData{}, err
	}
	receiptContent, err := s.store.LoadReceiptContent()
	if err != nil {
		return bootstrapData{}, err
	}
	receiptSnapshot, err := s.store.LoadReceiptSnapshotSettings()
	if err != nil {
		return bootstrapData{}, err
	}
	scheduleSettings, err := s.store.LoadSchedule()
	if err != nil {
		return bootstrapData{}, err
	}
	googleStatus := googleintegration.Status{}
	if s.googleClient != nil {
		googleStatus = s.googleClient.Status()
	}
	normalizedNews := newsSettings.Normalized()
	normalizedDenisTrends := denisTrendsSettings.Normalized()
	scheduleSettings = seedScheduleContentSettings(scheduleSettings, normalizedNews, normalizedDenisTrends)
	return bootstrapData{
		Printer:           printerConfig,
		Weather:           weatherLocation,
		Finance:           portfolio,
		Motivation:        motivationSettings,
		GoogleStatus:      googleStatus,
		News:              bootstrapNewsSettings{TranslateTitles: normalizedNews.TranslateTitlesEnabled(), Sources: normalizedNews.Sources},
		DenisTrends:       normalizedDenisTrends,
		ReceiptStyle:      receiptStyle,
		ReceiptContent:    receiptContent,
		ReceiptSnapshot:   receiptSnapshot,
		Schedule:          scheduleSettings,
		NewsPresets:       news.Presets(),
		FontOptions:       []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		ScheduleIntervals: scheduleIntervalOptions(),
	}, nil
}

func seedScheduleContentSettings(settings schedule.Settings, newsSettings news.Settings, denisTrendsSettings denistrends.Settings) schedule.Settings {
	seeded := settings.Normalized()
	seedContentSettings(seeded.IntervalContent, newsSettings, denisTrendsSettings)
	for index := range seeded.Runs {
		seedContentSettings(seeded.Runs[index].Content, newsSettings, denisTrendsSettings)
	}
	return seeded
}

func seedContentSettings(content *receipt.ContentSettings, newsSettings news.Settings, denisTrendsSettings denistrends.Settings) {
	if content == nil {
		return
	}
	if content.ShowNews && content.NewsSettings == nil {
		settingsCopy := newsSettings.Normalized()
		content.NewsSettings = &settingsCopy
	}
	if content.ShowDenisTrends && content.DenisTrendsSettings == nil {
		settingsCopy := denisTrendsSettings.Normalized()
		content.DenisTrendsSettings = &settingsCopy
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	version, err := s.printer.DriverVersion()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "ATOL driver loaded: " + version,
	})
}

func (s *Server) handleSavePrinter(w http.ResponseWriter, r *http.Request) {
	var config printer.Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SavePrinter(config); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	normalized := config.Normalized()
	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки сохранены.",
		Config:  &normalized,
	})
}

func (s *Server) handleSaveWeather(w http.ResponseWriter, r *http.Request) {
	var location weather.Location
	if err := json.NewDecoder(r.Body).Decode(&location); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveWeather(location); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки погоды сохранены.",
	})
}

func (s *Server) handleWeatherLocationSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	results, err := s.locationSearchProvider.Search(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, locationSearchResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, locationSearchResponse{
		OK:      true,
		Results: results,
	})
}

func (s *Server) handleSaveFinance(w http.ResponseWriter, r *http.Request) {
	var portfolio finance.TonPortfolio
	if err := json.NewDecoder(r.Body).Decode(&portfolio); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveFinance(portfolio); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки портфеля сохранены.",
	})
}

func (s *Server) handleSaveMotivation(w http.ResponseWriter, r *http.Request) {
	var settings motivation.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	settings.Configured = true
	settings.Enabled = true
	merged, err := s.mergeMotivationCache(settings)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if err := s.store.SaveMotivation(merged); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки AI-модели сохранены.",
	})
}

func (s *Server) handleMotivationTest(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.LoadMotivation()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, motivationResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	settings = settings.Normalized()
	settings.CacheDate = ""
	settings.CachedQuote = ""
	settings.LastError = ""

	updated, quote, err := motivation.ResolveDailyQuote(r.Context(), settings, s.clock(), s.motivationProvider)
	if saveErr := s.store.SaveMotivation(updated); saveErr != nil {
		writeJSON(w, http.StatusInternalServerError, motivationResponse{
			OK:    false,
			Error: saveErr.Error(),
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, motivationResponse{
			OK:       false,
			Error:    err.Error(),
			Settings: &updated,
		})
		return
	}

	writeJSON(w, http.StatusOK, motivationResponse{
		OK:       true,
		Message:  "AI-модель проверена.",
		Quote:    quote,
		Settings: &updated,
	})
}

func (s *Server) mergeMotivationCache(next motivation.Settings) (motivation.Settings, error) {
	normalized := next.Normalized()
	current, err := s.store.LoadMotivation()
	if err != nil {
		return motivation.Settings{}, err
	}
	current = current.Normalized()
	if normalized.BaseURL == current.BaseURL && normalized.Model == current.Model {
		normalized.CacheDate = current.CacheDate
		normalized.CachedQuote = current.CachedQuote
		normalized.LastError = current.LastError
	}
	return normalized, nil
}

func (s *Server) handleSaveNews(w http.ResponseWriter, r *http.Request) {
	var settings news.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveNews(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки новостей сохранены.",
	})
}

func (s *Server) handleSaveDenisTrends(w http.ResponseWriter, r *http.Request) {
	var settings denistrends.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveDenisTrends(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки Denis Trends сохранены.",
	})
}

func (s *Server) handleSaveReceiptSnapshot(w http.ResponseWriter, r *http.Request) {
	var settings receiptsnapshot.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveReceiptSnapshotSettings(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки онлайн-слепка сохранены.",
	})
}

func (s *Server) handleSaveReceiptStyle(w http.ResponseWriter, r *http.Request) {
	var style receipt.StyleSettings
	if err := json.NewDecoder(r.Body).Decode(&style); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveReceiptStyle(style); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Настройки чека сохранены.",
	})
}

func (s *Server) handleSaveReceiptContent(w http.ResponseWriter, r *http.Request) {
	var content receipt.ContentSettings
	if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveReceiptContent(content); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Состав чека сохранен.",
	})
}

func scheduleIntervalOptions() []scheduleIntervalOption {
	return []scheduleIntervalOption{
		{Minutes: 1, Label: "1 минута"},
		{Minutes: 5, Label: "5 минут"},
		{Minutes: 15, Label: "15 минут"},
		{Minutes: 30, Label: "30 минут"},
		{Minutes: 60, Label: "1 час"},
		{Minutes: 120, Label: "2 часа"},
		{Minutes: 360, Label: "6 часов"},
		{Minutes: 720, Label: "12 часов"},
		{Minutes: 1440, Label: "24 часа"},
	}
}
