package web

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"atol-server/internal/app"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/schedule"
	schedulerruntime "atol-server/internal/scheduler"
	"atol-server/internal/weather"
)

type SettingsStore interface {
	LoadPrinter() (printer.Config, error)
	SavePrinter(printer.Config) error
	LoadWeather() (weather.Location, error)
	SaveWeather(weather.Location) error
	LoadFinance() (finance.TonPortfolio, error)
	SaveFinance(finance.TonPortfolio) error
	LoadMotivation() (motivation.Settings, error)
	SaveMotivation(motivation.Settings) error
	LoadNews() (news.Settings, error)
	SaveNews(news.Settings) error
	LoadReceiptStyle() (receipt.StyleSettings, error)
	SaveReceiptStyle(receipt.StyleSettings) error
	LoadReceiptContent() (receipt.ContentSettings, error)
	SaveReceiptContent(receipt.ContentSettings) error
	LoadSchedule() (schedule.Settings, error)
	SaveSchedule(schedule.Settings) error
	LoadScheduleState() (schedule.State, error)
	SaveScheduleState(schedule.State) error
}

type PrinterGateway interface {
	CheckConnection(context.Context, printer.Config) (string, error)
	PrintReceipt(context.Context, printer.Config, []receipt.Line) error
	FontMetrics(context.Context, printer.Config) ([]printer.FontMetric, error)
	DriverVersion() (string, error)
}

type WeatherProvider interface {
	Current(context.Context, weather.Location) (weather.Snapshot, error)
}

type LocationSearchProvider interface {
	Search(context.Context, string) ([]weather.LocationCandidate, error)
}

type TonPriceProvider interface {
	CurrentPrice(context.Context) (finance.TonPrice, error)
}

type FiatRateProvider interface {
	CurrentRate(context.Context) (finance.FiatRate, error)
}

type NewsProvider interface {
	Current(context.Context, news.Settings) ([]news.Item, error)
}

type MotivationProvider interface {
	Generate(context.Context, motivation.Settings) (motivation.Quote, error)
	GenerateWeatherAdvice(context.Context, motivation.Settings, motivation.WeatherContext) (motivation.WeatherAdvice, error)
	TranslateNewsTitles(context.Context, motivation.Settings, []motivation.NewsTitle) ([]motivation.NewsTranslation, error)
}

type GoogleClient interface {
	Current(context.Context) (googleintegration.Summary, error)
	Status() googleintegration.Status
	AuthURL(redirectURI string, state string) (string, error)
	ExchangeCode(context.Context, string, string) error
	Disconnect() error
}

type DailyReceiptService interface {
	BuildDailyReceipt(context.Context) ([]receipt.Line, error)
	PrintDailyReceipt(context.Context) error
}

type DailyReceiptWarningService interface {
	BuildDailyReceiptWithWarnings(context.Context) ([]receipt.Line, []string, error)
}

type Scheduler interface {
	ResetFromNow(context.Context) error
	Status() (schedulerruntime.Status, error)
}

type ServerOption func(*Server)

type Server struct {
	store                  SettingsStore
	printer                PrinterGateway
	weatherProvider        WeatherProvider
	locationSearchProvider LocationSearchProvider
	tonPriceProvider       TonPriceProvider
	fiatRateProvider       FiatRateProvider
	newsProvider           NewsProvider
	googleClient           GoogleClient
	motivationProvider     MotivationProvider
	receiptService         DailyReceiptService
	scheduler              Scheduler
	clock                  func() time.Time
	assetsPath             string
}

const defaultAssetsPath = "/opt/atol-server/assets"

type statusResponse struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
	Config  *printer.Config `json:"config,omitempty"`
}

type receiptPreviewResponse struct {
	OK       bool           `json:"ok"`
	Message  string         `json:"message,omitempty"`
	Error    string         `json:"error,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
	Lines    []receipt.Line `json:"lines,omitempty"`
}

type fontMetricsResponse struct {
	OK      bool                 `json:"ok"`
	Message string               `json:"message,omitempty"`
	Error   string               `json:"error,omitempty"`
	Fonts   []printer.FontMetric `json:"fonts,omitempty"`
}

type schedulerStatusResponse struct {
	OK      bool                     `json:"ok"`
	Message string                   `json:"message,omitempty"`
	Error   string                   `json:"error,omitempty"`
	Status  *schedulerruntime.Status `json:"status,omitempty"`
}

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

type googleStatusResponse struct {
	OK      bool                      `json:"ok"`
	Message string                    `json:"message,omitempty"`
	Error   string                    `json:"error,omitempty"`
	Status  *googleintegration.Status `json:"status,omitempty"`
}

func WithWeatherProvider(provider WeatherProvider) ServerOption {
	return func(s *Server) {
		s.weatherProvider = provider
	}
}

func WithLocationSearchProvider(provider LocationSearchProvider) ServerOption {
	return func(s *Server) {
		s.locationSearchProvider = provider
	}
}

func WithTonPriceProvider(provider TonPriceProvider) ServerOption {
	return func(s *Server) {
		s.tonPriceProvider = provider
	}
}

func WithFiatRateProvider(provider FiatRateProvider) ServerOption {
	return func(s *Server) {
		s.fiatRateProvider = provider
	}
}

func WithNewsProvider(provider NewsProvider) ServerOption {
	return func(s *Server) {
		s.newsProvider = provider
	}
}

func WithGoogleClient(client GoogleClient) ServerOption {
	return func(s *Server) {
		s.googleClient = client
	}
}

func WithMotivationProvider(provider MotivationProvider) ServerOption {
	return func(s *Server) {
		s.motivationProvider = provider
	}
}

func WithAssetsPath(path string) ServerOption {
	return func(s *Server) {
		s.assetsPath = path
	}
}

func WithReceiptService(service DailyReceiptService) ServerOption {
	return func(s *Server) {
		s.receiptService = service
	}
}

func WithScheduler(scheduler Scheduler) ServerOption {
	return func(s *Server) {
		s.scheduler = scheduler
	}
}

func NewServer(store SettingsStore, gateway PrinterGateway, clock func() time.Time, options ...ServerOption) *Server {
	if clock == nil {
		clock = time.Now
	}
	server := &Server{
		store:                  store,
		printer:                gateway,
		weatherProvider:        weather.NewOpenMeteoProvider(nil),
		locationSearchProvider: weather.NewGeocodingProvider(nil),
		tonPriceProvider:       finance.NewCoinGeckoTonPriceProvider(nil),
		fiatRateProvider:       finance.NewNbrbUsdBynRateProvider(nil),
		newsProvider:           news.NewProvider(nil),
		motivationProvider:     motivation.NewOllamaProvider(nil),
		clock:                  clock,
		assetsPath:             defaultAssetsPath,
	}
	for _, option := range options {
		option(server)
	}
	if server.receiptService == nil {
		server.receiptService = app.NewReceiptService(
			store,
			gateway,
			clock,
			app.WithGeneratedAssetsPath(server.assetsPathOrDefault()),
			app.WithWeatherProvider(server.weatherProvider),
			app.WithTonPriceProvider(server.tonPriceProvider),
			app.WithFiatRateProvider(server.fiatRateProvider),
			app.WithNewsProvider(server.newsProvider),
			app.WithGoogleProvider(server.googleClient),
			app.WithMotivationProvider(server.motivationProvider),
		)
	}
	return server
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(s.assetsPathOrDefault()))))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /api/settings/printer", s.handleSavePrinter)
	mux.HandleFunc("POST /api/settings/weather", s.handleSaveWeather)
	mux.HandleFunc("GET /api/weather/locations", s.handleWeatherLocationSearch)
	mux.HandleFunc("POST /api/settings/finance", s.handleSaveFinance)
	mux.HandleFunc("POST /api/settings/motivation", s.handleSaveMotivation)
	mux.HandleFunc("POST /api/motivation/test", s.handleMotivationTest)
	mux.HandleFunc("POST /api/settings/news", s.handleSaveNews)
	mux.HandleFunc("GET /api/google/status", s.handleGoogleStatus)
	mux.HandleFunc("GET /api/google/auth/start", s.handleGoogleAuthStart)
	mux.HandleFunc("GET /oauth/google/callback", s.handleGoogleCallback)
	mux.HandleFunc("POST /api/google/disconnect", s.handleGoogleDisconnect)
	mux.HandleFunc("POST /api/settings/receipt-style", s.handleSaveReceiptStyle)
	mux.HandleFunc("POST /api/settings/receipt-content", s.handleSaveReceiptContent)
	mux.HandleFunc("POST /api/settings/schedule", s.handleSaveSchedule)
	mux.HandleFunc("POST /api/settings/schedule/stop", s.handleStopSchedule)
	mux.HandleFunc("GET /api/scheduler/status", s.handleSchedulerStatus)
	mux.HandleFunc("POST /api/printer/check", s.handlePrinterCheck)
	mux.HandleFunc("POST /api/printer/fonts", s.handlePrinterFonts)
	mux.HandleFunc("POST /api/receipt/preview", s.handleReceiptPreview)
	mux.HandleFunc("POST /api/print/test", s.handlePrintTest)
	mux.HandleFunc("POST /api/print/weather", s.handlePrintWeather)
	return mux
}

func (s *Server) assetsPathOrDefault() string {
	if s.assetsPath == "" {
		return defaultAssetsPath
	}
	return s.assetsPath
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	page, err := s.indexPage()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, page); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) indexPage() (indexPageData, error) {
	printerConfig, err := s.store.LoadPrinter()
	if err != nil {
		return indexPageData{}, err
	}
	weatherLocation, err := s.store.LoadWeather()
	if err != nil {
		return indexPageData{}, err
	}
	portfolio, err := s.store.LoadFinance()
	if err != nil {
		return indexPageData{}, err
	}
	newsSettings, err := s.store.LoadNews()
	if err != nil {
		return indexPageData{}, err
	}
	motivationSettings, err := s.store.LoadMotivation()
	if err != nil {
		return indexPageData{}, err
	}
	receiptStyle, err := s.store.LoadReceiptStyle()
	if err != nil {
		return indexPageData{}, err
	}
	receiptContent, err := s.store.LoadReceiptContent()
	if err != nil {
		return indexPageData{}, err
	}
	scheduleSettings, err := s.store.LoadSchedule()
	if err != nil {
		return indexPageData{}, err
	}
	googleStatus := googleintegration.Status{}
	if s.googleClient != nil {
		googleStatus = s.googleClient.Status()
	}
	return indexPageData{
		Printer:           printerConfig,
		Weather:           weatherLocation,
		Finance:           portfolio,
		Motivation:        motivationSettings,
		GoogleStatus:      googleStatus,
		News:              newsSettings,
		ReceiptStyle:      receiptStyle,
		ReceiptContent:    receiptContent,
		Schedule:          scheduleSettings,
		NewsPresets:       news.Presets(),
		FontOptions:       []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		ScheduleIntervals: scheduleIntervalOptions(),
	}, nil
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

func (s *Server) handleGoogleStatus(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		writeJSON(w, http.StatusOK, googleStatusResponse{
			OK:     true,
			Status: &googleintegration.Status{},
		})
		return
	}
	status := s.googleClient.Status()
	writeJSON(w, http.StatusOK, googleStatusResponse{
		OK:     true,
		Status: &status,
	})
}

func (s *Server) handleGoogleAuthStart(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		http.Error(w, "Google integration is not configured", http.StatusServiceUnavailable)
		return
	}
	authURL, err := s.googleClient.AuthURL(s.googleRedirectURI(r), "atol-google")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		http.Error(w, "Google integration is not configured", http.StatusServiceUnavailable)
		return
	}
	if message := strings.TrimSpace(r.URL.Query().Get("error")); message != "" {
		http.Error(w, "Google authorization failed: "+message, http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Error(w, "Google authorization code is missing", http.StatusBadRequest)
		return
	}
	if err := s.googleClient.ExchangeCode(r.Context(), code, s.googleRedirectURI(r)); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>Google подключен</title></head><body><h1>Google подключен</h1><p>Можно закрыть эту вкладку и вернуться к ATOL Go Server.</p><p><a href="/">Вернуться в приложение</a></p></body></html>`))
}

func (s *Server) handleGoogleDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.googleClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, statusResponse{
			OK:    false,
			Error: "Google integration is not configured",
		})
		return
	}
	if err := s.googleClient.Disconnect(); err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Google отключен.",
	})
}

func (s *Server) googleRedirectURI(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + r.Host + "/oauth/google/callback"
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

func (s *Server) handleSaveSchedule(w http.ResponseWriter, r *http.Request) {
	var settings schedule.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}

	if err := s.store.SaveSchedule(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if s.scheduler != nil {
		if err := s.scheduler.ResetFromNow(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, statusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Расписание сохранено.",
	})
}

func (s *Server) handleStopSchedule(w http.ResponseWriter, r *http.Request) {
	settings := schedule.DefaultSettings()
	settings.Enabled = false

	if err := s.store.SaveSchedule(settings); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	if s.scheduler != nil {
		if err := s.scheduler.ResetFromNow(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, statusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Расписание остановлено.",
	})
}

func (s *Server) handleSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		settings, err := s.store.LoadSchedule()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, schedulerStatusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		state, err := s.store.LoadScheduleState()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, schedulerStatusResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}
		status := schedulerruntime.Status{
			Settings:      settings.Normalized(),
			LastAttemptAt: state.LastAttemptAt,
			LastSuccessAt: state.LastSuccessAt,
			LastError:     state.LastError,
			NextRunAt:     state.NextRunAt,
		}
		writeJSON(w, http.StatusOK, schedulerStatusResponse{
			OK:     true,
			Status: &status,
		})
		return
	}

	status, err := s.scheduler.Status()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, schedulerStatusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, schedulerStatusResponse{
		OK:     true,
		Status: &status,
	})
}

func (s *Server) handlePrinterCheck(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	message, err := s.printer.CheckConnection(r.Context(), config)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: message,
	})
}

func (s *Server) handlePrinterFonts(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, fontMetricsResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	fonts, err := s.printer.FontMetrics(r.Context(), config)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, fontMetricsResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, fontMetricsResponse{
		OK:      true,
		Message: "Метрики шрифтов ККТ загружены.",
		Fonts:   fonts,
	})
}

func (s *Server) handlePrintTest(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	if err := s.printer.PrintReceipt(r.Context(), config, receipt.TestReceipt(s.clock())); err != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Тестовый чек напечатан.",
	})
}

func (s *Server) handleReceiptPreview(w http.ResponseWriter, r *http.Request) {
	var lines []receipt.Line
	var warnings []string
	var err error
	if service, ok := s.receiptService.(DailyReceiptWarningService); ok {
		lines, warnings, err = service.BuildDailyReceiptWithWarnings(r.Context())
	} else {
		lines, err = s.receiptService.BuildDailyReceipt(r.Context())
	}
	if err != nil {
		writeJSON(w, statusForBuildError(err), receiptPreviewResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	message := "Превью собрано."
	if len(warnings) > 0 {
		message += "\n" + strings.Join(warnings, "\n")
	}
	writeJSON(w, http.StatusOK, receiptPreviewResponse{
		OK:       true,
		Message:  message,
		Warnings: warnings,
		Lines:    lines,
	})
}

func (s *Server) handlePrintWeather(w http.ResponseWriter, r *http.Request) {
	if err := s.receiptService.PrintDailyReceipt(r.Context()); err != nil {
		writeJSON(w, statusForBuildError(err), statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Чек напечатан.",
	})
}

func statusForBuildError(err error) int {
	var buildErr app.BuildError
	if errors.As(err, &buildErr) {
		return buildErr.Status
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type indexPageData struct {
	Printer           printer.Config
	Weather           weather.Location
	Finance           finance.TonPortfolio
	Motivation        motivation.Settings
	GoogleStatus      googleintegration.Status
	News              news.Settings
	ReceiptStyle      receipt.StyleSettings
	ReceiptContent    receipt.ContentSettings
	Schedule          schedule.Settings
	NewsPresets       []news.PresetInfo
	FontOptions       []int
	ScheduleIntervals []scheduleIntervalOption
}

type scheduleIntervalOption struct {
	Minutes int
	Label   string
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

var indexTemplate = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ATOL Go Server</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f4f6f8;
      --panel: #ffffff;
      --text: #1d2330;
      --muted: #697386;
      --primary: #315994;
      --primary-hover: #24497d;
      --border: #d8deea;
      --soft-border: #e8edf5;
      --error: #b42318;
      --ok: #067647;
      --shadow: 0 12px 32px rgba(29, 35, 48, 0.08);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: var(--bg);
      color: var(--text);
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    main {
      width: min(1160px, 100%);
      margin: 0 auto;
      padding: 28px 20px 40px;
    }
    h1 {
      margin: 0 0 18px;
      font-size: 30px;
      font-weight: 650;
      letter-spacing: 0;
    }
    .layout {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 380px;
      gap: 24px;
      align-items: start;
    }
    .layout > *,
    .settings-column,
    #settings-form,
    .section,
    .section-grid,
    .section-grid > *,
    .weather-search-row,
    .weather-search-row > *,
    .actions,
    .preview-panel,
    #status {
      min-width: 0;
    }
    .settings-column {
      display: grid;
      gap: 24px;
    }
    #settings-form {
      display: grid;
      gap: 24px;
    }
    .section {
      display: grid;
      gap: 16px;
      background: var(--panel);
      border: 1px solid var(--soft-border);
      border-radius: 10px;
      padding: 16px;
      box-shadow: var(--shadow);
    }
    .section-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }
    .section-grid .wide {
      grid-column: 1 / -1;
    }
    label {
      display: grid;
      gap: 6px;
      color: var(--muted);
      font-size: 13px;
      font-weight: 600;
    }
    h2 {
      margin: 0;
      color: var(--text);
      font-size: 17px;
      font-weight: 750;
    }
    input,
    select {
      width: 100%;
      min-width: 0;
      min-height: 40px;
      border: 1px solid var(--border);
      border-radius: 7px;
      background: var(--panel);
      color: var(--text);
      padding: 8px 10px;
      font-size: 15px;
    }
    input:focus,
    select:focus {
      border-color: var(--primary);
      outline: 3px solid rgba(49, 89, 148, 0.14);
    }
    input.invalid {
      border-color: var(--error);
      outline: 3px solid rgba(180, 35, 24, 0.14);
    }
    input[readonly] {
      background: #f8fafc;
      color: var(--muted);
      cursor: default;
    }
    input[type="checkbox"] {
      width: 18px;
      min-height: 18px;
      padding: 0;
    }
    .news-source {
      display: grid;
      grid-template-columns: 24px 1fr 92px;
      gap: 8px;
      align-items: center;
    }
    .news-source input {
      font-size: 14px;
      min-height: 38px;
    }
    .weather-search-row {
      display: grid;
      grid-template-columns: 1fr auto;
      gap: 8px;
      align-items: end;
    }
    .location-results {
      display: grid;
      gap: 8px;
    }
    .location-result {
      width: 100%;
      min-height: auto;
      padding: 10px 12px;
      background: #fff;
      border-color: var(--border);
      color: var(--text);
      text-align: left;
      font-weight: 650;
    }
    .location-result:hover {
      background: #f8fafc;
    }
    .location-meta {
      display: block;
      margin-top: 3px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 500;
    }
    .style-row {
      display: grid;
      grid-template-columns: minmax(96px, 0.65fr) minmax(220px, 1.45fr) minmax(142px, 0.9fr) minmax(142px, 0.9fr);
      gap: 12px 14px;
      align-items: center;
    }
    .style-row strong {
      align-self: center;
      font-size: 14px;
    }
    .style-row label {
      min-width: 0;
    }
    .toggle-label {
      display: flex;
      gap: 8px;
      align-items: center;
      color: var(--text);
      font-weight: 500;
      line-height: 1.25;
    }
    .content-options {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 14px 18px;
    }
    .content-group {
      display: grid;
      gap: 8px;
      align-content: start;
    }
    .content-group-title {
      color: var(--text);
      font-size: 13px;
      font-weight: 750;
    }
    .switch-label {
      display: flex;
      align-items: center;
      gap: 10px;
      color: var(--text);
      font-weight: 600;
      line-height: 1.25;
    }
    .switch-input {
      position: absolute;
      opacity: 0;
      pointer-events: none;
    }
    .switch-track {
      position: relative;
      flex: 0 0 auto;
      width: 42px;
      height: 24px;
      border-radius: 999px;
      background: #cbd5e1;
      border: 1px solid #b6c1d1;
      transition: background 0.16s ease, border-color 0.16s ease;
    }
    .switch-track::after {
      content: "";
      position: absolute;
      top: 2px;
      left: 2px;
      width: 18px;
      height: 18px;
      border-radius: 50%;
      background: #fff;
      box-shadow: 0 1px 3px rgba(15, 23, 42, 0.22);
      transition: transform 0.16s ease;
    }
    .switch-input:checked + .switch-track {
      background: var(--primary);
      border-color: var(--primary);
    }
    .switch-input:checked + .switch-track::after {
      transform: translateX(18px);
    }
    .switch-input:focus + .switch-track {
      outline: 3px solid rgba(49, 89, 148, 0.14);
    }
    .font-settings {
      gap: 14px;
    }
    .font-summary {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      cursor: pointer;
      color: var(--text);
      font-size: 17px;
      font-weight: 750;
      list-style: none;
    }
    .font-summary::-webkit-details-marker {
      display: none;
    }
    .font-summary::after {
      content: "Показать";
      color: var(--primary);
      font-size: 13px;
      font-weight: 700;
    }
    .font-settings[open] .font-summary::after {
      content: "Скрыть";
    }
    .section-note {
      margin: -4px 0 0;
      color: var(--muted);
      font-size: 13px;
      line-height: 1.45;
      overflow-wrap: anywhere;
    }
    .font-tools {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      align-items: flex-start;
    }
    .font-metrics {
      flex: 1 1 280px;
      color: var(--muted);
      font-size: 13px;
      line-height: 1.45;
      white-space: pre-wrap;
    }
    .mode-options {
      display: flex;
      flex-wrap: wrap;
      gap: 10px 18px;
      align-items: center;
    }
    .mode-options label {
      display: flex;
      gap: 8px;
      align-items: center;
      color: var(--text);
      font-size: 14px;
      font-weight: 600;
    }
    .mode-options input[type="radio"] {
      width: 16px;
      min-height: 16px;
    }
    .schedule-time-list {
      display: grid;
      gap: 8px;
    }
    .schedule-time-row {
      display: grid;
      grid-template-columns: minmax(140px, 180px) auto;
      gap: 8px;
      align-items: center;
      justify-content: start;
    }
    .scheduler-status {
      color: var(--muted);
      font-size: 13px;
      line-height: 1.45;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }
    .motivation-status {
      color: var(--muted);
      font-size: 13px;
      line-height: 1.45;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }
    .motivation-status.error {
      color: var(--error);
    }
    .motivation-status.ok {
      color: var(--ok);
    }
    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 2px;
    }
    .printer-actions {
      margin-top: 0;
    }
    .printer-actions button {
      flex: 1 1 170px;
    }
    .main-actions {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      margin-top: 0;
    }
    .primary-print {
      width: 100%;
      min-height: 52px;
      font-size: 16px;
    }
    .section-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
    }
    .section-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      justify-content: flex-end;
    }
    .hidden {
      display: none !important;
    }
    [hidden] {
      display: none !important;
    }
    button,
    .button-link {
      min-width: 0;
      min-height: 40px;
      border: 1px solid transparent;
      border-radius: 7px;
      background: var(--primary);
      color: #fff;
      padding: 0 14px;
      font-size: 14px;
      font-weight: 700;
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      text-decoration: none;
    }
    button:hover,
    .button-link:hover { background: var(--primary-hover); }
    button.secondary,
    .button-link.secondary {
      background: #fff;
      border-color: var(--border);
      color: var(--text);
    }
    button.secondary:hover,
    .button-link.secondary:hover { background: #f8fafc; }
    button:disabled {
      opacity: 0.64;
      cursor: wait;
    }
    #status {
      margin-top: 12px;
      min-height: 28px;
      font-size: 14px;
      line-height: 1.45;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
    }
    #status.ok { color: var(--ok); }
    #status.error { color: var(--error); }
    .preview-panel {
      position: sticky;
      top: 20px;
      display: grid;
      gap: 16px;
    }
    .receipt-preview {
      overflow: auto;
      max-height: calc(100vh - 150px);
      background: #fdfdfb;
      border: 1px solid #d6d0bf;
      border-radius: 6px;
      padding: 18px 16px;
      box-shadow: var(--shadow);
    }
    .receipt-empty {
      color: var(--muted);
      font-size: 14px;
      line-height: 1.45;
      overflow-wrap: anywhere;
    }
    .receipt-paper {
      width: min(100%, calc(var(--paper-chars, 32) * 1ch));
      margin: 0 auto;
      color: #111;
      font-family: "Courier New", ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 16px;
      line-height: 1.22;
      letter-spacing: 0;
      white-space: pre-wrap;
    }
    .receipt-line {
      --line-size: 16px;
      --line-scale-x: 1;
      --line-scale-y: 1;
      display: flex;
      align-items: center;
      justify-content: flex-start;
      width: calc(100% / var(--line-scale-x));
      margin: 0 auto;
      min-height: calc(var(--line-size) * 1.22 * var(--line-scale-y));
      line-height: 1.22;
      overflow: visible;
    }
    .receipt-line-text {
      display: inline-block;
      max-width: calc(100% / var(--line-scale-x));
      font-size: var(--line-size);
      line-height: 1.22;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
      transform: scale(var(--line-scale-x), var(--line-scale-y));
      transform-origin: center center;
    }
    .align-left { text-align: left; justify-content: flex-start; }
    .align-center { text-align: center; justify-content: center; }
    .align-right { text-align: right; justify-content: flex-end; }
    .align-left .receipt-line-text { transform-origin: left center; }
    .align-right .receipt-line-text { transform-origin: right center; }
    .receipt-image-line {
      width: 100%;
      min-height: var(--image-line-height, 84px);
      padding: 4px 0;
    }
    .receipt-image {
      display: block;
      width: 76px;
      height: 76px;
      object-fit: contain;
      image-rendering: auto;
    }
    @media (max-width: 920px) {
      .layout { grid-template-columns: minmax(0, 1fr); }
      .preview-panel { position: static; }
    }
    @media (max-width: 640px) {
      main { padding: 20px 14px 28px; }
      h1 { font-size: 26px; }
      .section-grid { grid-template-columns: minmax(0, 1fr); }
      .content-options { grid-template-columns: minmax(0, 1fr); }
      .weather-search-row { grid-template-columns: minmax(0, 1fr); }
      .style-row { grid-template-columns: 1fr; }
      .schedule-time-row { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <main>
    <h1>ATOL Go Server</h1>
    <div class="layout">
      <div class="settings-column">
        <form id="settings-form">
          <section class="section" data-section="printer">
            <h2>Касса</h2>
            <div class="section-grid">
              <label>
                IP address
                <input id="host" name="host" autocomplete="off" inputmode="decimal" value="{{ .Printer.Host }}">
              </label>
              <label>
                Port
                <input id="port" name="port" autocomplete="off" inputmode="numeric" value="{{ .Printer.Port }}">
              </label>
            </div>
            <div class="actions printer-actions">
              <button type="button" class="secondary" data-action="check">Проверить связь</button>
              <button type="button" class="secondary" data-action="print">Тестовый чек</button>
            </div>
          </section>

          <section class="section">
            <h2>Состав чека</h2>
            <div class="content-options">
              <div class="content-group">
                <div class="content-group-title">Основное</div>
                <label class="toggle-label">
                  <input id="content-weather" type="checkbox" {{ if .ReceiptContent.ShowWeather }}checked{{ end }}>
                  Погода
                </label>
                <label class="toggle-label">
                  <input id="content-weather-advice" type="checkbox" {{ if .ReceiptContent.ShowWeatherAdvice }}checked{{ end }}>
                  AI-совет
                </label>
                <label class="toggle-label">
                  <input id="content-motivation-quote" type="checkbox" {{ if .ReceiptContent.ShowMotivationQuote }}checked{{ end }}>
                  AI-цитата
                </label>
              </div>
              <div class="content-group">
                <div class="content-group-title">Финансы</div>
                <label class="toggle-label">
                  <input id="content-ton-portfolio" type="checkbox" {{ if .ReceiptContent.ShowTonPortfolio }}checked{{ end }}>
                  TON
                </label>
                <label class="toggle-label">
                  <input id="content-usd-byn-rate" type="checkbox" {{ if .ReceiptContent.ShowUsdBynRate }}checked{{ end }}>
                  USD/BYN
                </label>
                <label class="toggle-label">
                  <input id="content-bank-rates" type="checkbox" {{ if .ReceiptContent.ShowBankRates }}checked{{ end }}>
                  Курсы в банках
                </label>
              </div>
              <div class="content-group">
                <div class="content-group-title">Google</div>
                <label class="toggle-label">
                  <input id="content-mail" type="checkbox" {{ if .ReceiptContent.ShowMail }}checked{{ end }}>
                  Почта
                </label>
                <label class="toggle-label">
                  <input id="content-calendar" type="checkbox" {{ if .ReceiptContent.ShowCalendar }}checked{{ end }}>
                  Календарь
                </label>
              </div>
              <div class="content-group">
                <div class="content-group-title">Новости</div>
                <label class="toggle-label">
                  <input id="content-news" type="checkbox" {{ if .ReceiptContent.ShowNews }}checked{{ end }}>
                  Коротко о мире
                </label>
              </div>
            </div>
          </section>

          <section class="section">
            <h2>Погода</h2>
            <div class="section-grid">
              <label class="wide">
                Город
                <div class="weather-search-row">
                  <input id="weather-name" name="weather-name" autocomplete="off" value="{{ .Weather.Name }}" data-weather-location-selected="true">
                  <button type="button" class="secondary" data-action="search-weather-location">Найти</button>
                </div>
              </label>
              <div id="weather-location-results" class="location-results wide"></div>
              <p id="weather-location-help" class="section-note wide">Начни вводить город и выбери подходящий вариант из списка. Координаты заполнятся автоматически.</p>
              <label>
                Latitude
                <input id="weather-latitude" name="weather-latitude" autocomplete="off" inputmode="decimal" value="{{ .Weather.Latitude }}" readonly>
              </label>
              <label>
                Longitude
                <input id="weather-longitude" name="weather-longitude" autocomplete="off" inputmode="decimal" value="{{ .Weather.Longitude }}" readonly>
              </label>
            </div>
          </section>

          <section class="section">
            <div class="section-header">
              <h2>AI-модель</h2>
              <button type="button" class="secondary" data-action="test-motivation">Проверить AI</button>
            </div>
            <div class="section-grid">
              <label>
                Base URL
                <input id="motivation-base-url" autocomplete="off" value="{{ .Motivation.BaseURL }}">
              </label>
              <label>
                Model
                <input id="motivation-model" autocomplete="off" value="{{ .Motivation.Model }}">
              </label>
            </div>
            <div id="motivation-status" class="motivation-status">{{ if .Motivation.LastError }}Ошибка: {{ .Motivation.LastError }}{{ else if .Motivation.CachedQuote }}Последняя проверка: {{ .Motivation.CachedQuote }}{{ else }}Модель будет использоваться для включенных AI-блоков в составе чека.{{ end }}</div>
          </section>

          <section class="section">
            <div class="section-header">
              <h2>Google</h2>
              <div class="section-actions">
                <a class="button-link secondary" href="/api/google/auth/start">Авторизовать</a>
                <button type="button" class="secondary" data-action="google-disconnect">Отключить</button>
              </div>
            </div>
            <div id="google-status" class="motivation-status {{ if .GoogleStatus.Authorized }}ok{{ else if not .GoogleStatus.CredentialsAvailable }}error{{ end }}">
              {{ if not .GoogleStatus.CredentialsAvailable }}
              Положи OAuth credentials.json в {{ .GoogleStatus.CredentialsPath }}.
              {{ else if .GoogleStatus.Authorized }}
              Google подключен. В чек попадут включенные Google-блоки.
              {{ else }}
              credentials.json найден. Нажми «Авторизовать», чтобы подключить почту и календарь.
              {{ end }}
            </div>
            <p class="section-note">credentials.json: {{ .GoogleStatus.CredentialsPath }}</p>
            <p class="section-note">token.json: {{ .GoogleStatus.TokenPath }}</p>
          </section>

          <section class="section">
            <div class="section-header">
              <h2>TON</h2>
              <div class="section-actions">
                <button type="button" class="secondary" data-action="edit-finance">Редактировать</button>
                <button type="button" data-action="save-finance" class="hidden">Сохранить TON</button>
                <button type="button" class="secondary hidden" data-action="cancel-finance">Отмена</button>
              </div>
            </div>
            <div class="section-grid">
              <label>
                Количество TON
                <input id="ton-amount" name="ton-amount" autocomplete="off" inputmode="decimal" value="{{ .Finance.AmountTon }}" data-finance-input readonly>
              </label>
              <label>
                Куплено на, USD
                <input id="ton-invested" name="ton-invested" autocomplete="off" inputmode="decimal" value="{{ .Finance.InvestedUSD }}" data-finance-input readonly>
              </label>
            </div>
          </section>

          <details class="section font-settings">
            <summary class="font-summary">Шрифт чека</summary>
            <p class="section-note">Эти поля напрямую соответствуют параметрам ATOL printText: LIBFPTR_PARAM_FONT, LIBFPTR_PARAM_FONT_DOUBLE_WIDTH и LIBFPTR_PARAM_FONT_DOUBLE_HEIGHT. Метрики ниже читаются через LIBFPTR_DT_FONT_INFO.</p>
            <div class="style-row">
              <strong>Дата</strong>
              <label>
                LIBFPTR_PARAM_FONT
                <select id="calendar-font" data-font-select>
                  {{ range .FontOptions }}
                  <option value="{{ . }}" {{ if eq $.ReceiptStyle.CalendarFont . }}selected{{ end }}>Шрифт {{ . }}</option>
                  {{ end }}
                </select>
              </label>
              <label class="toggle-label">
                <input id="calendar-double-width" type="checkbox" {{ if .ReceiptStyle.CalendarDoubleWidth }}checked{{ end }}>
                Двойная ширина
              </label>
              <label class="toggle-label">
                <input id="calendar-double-height" type="checkbox" {{ if .ReceiptStyle.CalendarDoubleHeight }}checked{{ end }}>
                Двойная высота
              </label>
            </div>
            <div class="style-row">
              <strong>Температура</strong>
              <label>
                LIBFPTR_PARAM_FONT
                <select id="temperature-font" data-font-select>
                  {{ range .FontOptions }}
                  <option value="{{ . }}" {{ if eq $.ReceiptStyle.TemperatureFont . }}selected{{ end }}>Шрифт {{ . }}</option>
                  {{ end }}
                </select>
              </label>
              <label class="toggle-label">
                <input id="temperature-double-width" type="checkbox" {{ if .ReceiptStyle.TemperatureDoubleWidth }}checked{{ end }}>
                Двойная ширина
              </label>
              <label class="toggle-label">
                <input id="temperature-double-height" type="checkbox" {{ if .ReceiptStyle.TemperatureDoubleHeight }}checked{{ end }}>
                Двойная высота
              </label>
            </div>
            <div class="style-row">
              <strong>Информация</strong>
              <label>
                LIBFPTR_PARAM_FONT
                <select id="normal-font" data-font-select>
                  {{ range .FontOptions }}
                  <option value="{{ . }}" {{ if eq $.ReceiptStyle.NormalFont . }}selected{{ end }}>Шрифт {{ . }}</option>
                  {{ end }}
                </select>
              </label>
              <span></span>
              <span></span>
            </div>
            <div class="font-tools">
              <button type="button" class="secondary" data-action="fonts">Загрузить шрифты ККТ</button>
              <div id="font-metrics" class="font-metrics">Пока используется приблизительная сетка. После загрузки сервер спросит у ККТ длину строки и ширину символа для каждого шрифта.</div>
            </div>
          </details>

          <section class="section">
            <h2>Коротко о мире</h2>
            <label class="switch-label">
              <input id="news-translate" class="switch-input" type="checkbox" {{ if .News.TranslateTitlesEnabled }}checked{{ end }}>
              <span class="switch-track" aria-hidden="true"></span>
              <span>Переводить английские заголовки через AI</span>
            </label>
            {{ range .News.Sources }}
            <div class="news-source" data-news-source>
              <input type="checkbox" data-news-enabled {{ if .Enabled }}checked{{ end }}>
              <input type="hidden" data-news-preset value="{{ .Preset }}">
              <input type="hidden" data-news-url value="{{ .FeedURL }}">
              <label>
                {{ .DisplayName }}
                <input data-news-url-visible autocomplete="off" value="{{ .FeedURL }}">
              </label>
              <label>
                Кол-во
                <input data-news-count autocomplete="off" inputmode="numeric" value="{{ .MaxItems }}" min="1" max="100">
              </label>
            </div>
            {{ end }}
          </section>

          <section class="section">
            <h2>Расписание</h2>
            <div class="mode-options">
              <label>
                <input type="radio" name="schedule-mode" value="interval" {{ if eq .Schedule.Mode "interval" }}checked{{ end }}>
                Интервал
              </label>
              <label>
                <input type="radio" name="schedule-mode" value="daily_times" {{ if eq .Schedule.Mode "daily_times" }}checked{{ end }}>
                Точное время
              </label>
            </div>
            <div id="schedule-interval-panel" class="section-grid">
              <label>
                Печатать каждые
                <select id="schedule-interval">
                  {{ range .ScheduleIntervals }}
                  <option value="{{ .Minutes }}" {{ if eq $.Schedule.IntervalMinutes .Minutes }}selected{{ end }}>{{ .Label }}</option>
                  {{ end }}
                </select>
              </label>
            </div>
            <div id="schedule-times-panel">
              <div id="schedule-time-list" class="schedule-time-list">
                {{ range .Schedule.Times }}
                <div class="schedule-time-row" data-schedule-time-row>
                  <input type="time" data-schedule-time value="{{ . }}">
                  <button type="button" class="secondary" data-action="remove-schedule-time">Удалить</button>
                </div>
                {{ end }}
              </div>
              <div class="actions">
                <button type="button" class="secondary" data-action="add-schedule-time">Добавить время</button>
              </div>
            </div>
            <div class="actions">
              <button type="button" data-action="save-schedule">Сохранить расписание</button>
              <button type="button" class="secondary" data-action="stop-schedule">Остановить расписание</button>
            </div>
            <div id="scheduler-status" class="scheduler-status">Статус расписания загружается...</div>
          </section>
        </form>

        <div class="actions main-actions">
          <button type="button" class="primary-print" data-action="weather">Напечатать чек</button>
        </div>
        <div id="status"></div>
      </div>

      <aside class="preview-panel">
        <section class="section">
          <div class="section-header">
            <h2>Превью</h2>
            <button type="button" class="secondary" data-action="preview">Показать превью</button>
          </div>
          <div id="receipt-preview" class="receipt-preview">
            <div class="receipt-empty">Нажми «Показать превью», чтобы собрать чек с включенными блоками без печати на кассе.</div>
          </div>
        </section>
      </aside>
    </div>
  </main>
  <script>
    const statusEl = document.querySelector("#status");
    const previewEl = document.querySelector("#receipt-preview");
    const fontMetricsEl = document.querySelector("#font-metrics");
    const schedulerStatusEl = document.querySelector("#scheduler-status");
    const motivationStatusEl = document.querySelector("#motivation-status");
    const googleStatusEl = document.querySelector("#google-status");
    const weatherNameInput = document.querySelector("#weather-name");
    const weatherLatitudeInput = document.querySelector("#weather-latitude");
    const weatherLongitudeInput = document.querySelector("#weather-longitude");
    const weatherLocationResultsEl = document.querySelector("#weather-location-results");
    const weatherLocationHelpEl = document.querySelector("#weather-location-help");
    let lastPreviewLines = [];
    let weatherSearchTimer = null;
    let financeDraft = {
      amountTon: document.querySelector("#ton-amount").value,
      investedUsd: document.querySelector("#ton-invested").value
    };
    const fallbackFontMetrics = [
      { font: 0, lineLength: 32, fontWidth: 12 },
      { font: 1, lineLength: 42, fontWidth: 9 },
      { font: 2, lineLength: 38, fontWidth: 10 },
      { font: 3, lineLength: 32, fontWidth: 12 },
      { font: 4, lineLength: 32, fontWidth: 12 },
      { font: 5, lineLength: 32, fontWidth: 12 },
      { font: 6, lineLength: 32, fontWidth: 12 },
      { font: 7, lineLength: 32, fontWidth: 12 },
      { font: 8, lineLength: 32, fontWidth: 12 },
      { font: 9, lineLength: 32, fontWidth: 12 }
    ];
    let fontMetrics = new Map(fallbackFontMetrics.map(metric => [metric.font, metric]));

    function setBusy(busy) {
      document.querySelectorAll("button").forEach(button => button.disabled = busy);
    }

    function setStatus(kind, text) {
      statusEl.className = kind;
      statusEl.textContent = text;
    }

    function setWeatherLocationSelected(selected) {
      weatherNameInput.dataset.weatherLocationSelected = selected ? "true" : "false";
      weatherNameInput.classList.toggle("invalid", !selected);
      weatherLatitudeInput.classList.toggle("invalid", !selected);
      weatherLongitudeInput.classList.toggle("invalid", !selected);
      if (!selected) {
        weatherNameInput.setAttribute("aria-invalid", "true");
      } else {
        weatherNameInput.removeAttribute("aria-invalid");
      }
    }

    function validateWeatherLocation() {
      const latitude = Number.parseFloat(weatherLatitudeInput.value);
      const longitude = Number.parseFloat(weatherLongitudeInput.value);
      const selected = weatherNameInput.dataset.weatherLocationSelected === "true";
      if (!selected || !Number.isFinite(latitude) || !Number.isFinite(longitude)) {
        setWeatherLocationSelected(false);
        throw new Error("Выбери город из списка, чтобы координаты погоды обновились автоматически.");
      }
    }

    function renderWeatherLocationResults(results) {
      weatherLocationResultsEl.replaceChildren();
      if (!results || results.length === 0) {
        weatherLocationHelpEl.textContent = "Город не найден. Уточни название или попробуй вариант на другом языке.";
        return;
      }
      weatherLocationHelpEl.textContent = "Выбери город из списка, чтобы обновить координаты.";
      for (const result of results) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "location-result";
        button.dataset.locationName = result.name || "";
        button.dataset.locationLatitude = String(result.latitude);
        button.dataset.locationLongitude = String(result.longitude);
        button.textContent = result.displayName || result.name || "Без названия";
        const meta = document.createElement("span");
        meta.className = "location-meta";
        meta.textContent = [
          Number.isFinite(result.latitude) ? Number(result.latitude).toFixed(4) : "",
          Number.isFinite(result.longitude) ? Number(result.longitude).toFixed(4) : "",
          result.timezone || ""
        ].filter(Boolean).join(" · ");
        button.appendChild(meta);
        weatherLocationResultsEl.appendChild(button);
      }
    }

    async function searchWeatherLocations() {
      const query = weatherNameInput.value.trim();
      if (query.length < 2) {
        weatherLocationResultsEl.replaceChildren();
        weatherLocationHelpEl.textContent = "Введи минимум 2 символа для поиска города.";
        return;
      }
      weatherLocationHelpEl.textContent = "Ищу город...";
      const response = await fetch("/api/weather/locations?q=" + encodeURIComponent(query));
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось найти город.");
      }
      renderWeatherLocationResults(payload.results || []);
    }

    function queueWeatherLocationSearch() {
      setWeatherLocationSelected(false);
      window.clearTimeout(weatherSearchTimer);
      weatherSearchTimer = window.setTimeout(() => {
        searchWeatherLocations().catch(error => {
          weatherLocationHelpEl.textContent = error.message;
        });
      }, 350);
    }

    async function saveSettings() {
      const port = Number.parseInt(document.querySelector("#port").value, 10);
      const response = await fetch("/api/settings/printer", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          host: document.querySelector("#host").value,
          port: Number.isFinite(port) ? port : 0
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить настройки.");
      }
      return payload;
    }

    async function saveWeatherSettings() {
      validateWeatherLocation();
      const latitude = Number.parseFloat(document.querySelector("#weather-latitude").value);
      const longitude = Number.parseFloat(document.querySelector("#weather-longitude").value);
      const response = await fetch("/api/settings/weather", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: document.querySelector("#weather-name").value,
          latitude: Number.isFinite(latitude) ? latitude : 999,
          longitude: Number.isFinite(longitude) ? longitude : 999
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить настройки погоды.");
      }
      return payload;
    }

    async function saveFinanceSettings() {
      const amountTon = Number.parseFloat(document.querySelector("#ton-amount").value);
      const investedUsd = Number.parseFloat(document.querySelector("#ton-invested").value);
      const response = await fetch("/api/settings/finance", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          amountTon: Number.isFinite(amountTon) ? amountTon : -1,
          investedUsd: Number.isFinite(investedUsd) ? investedUsd : -1
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить настройки TON.");
      }
      return payload;
    }

    async function saveMotivationSettings() {
      const response = await fetch("/api/settings/motivation", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          configured: true,
          baseUrl: document.querySelector("#motivation-base-url").value,
          model: document.querySelector("#motivation-model").value
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить настройки AI-модели.");
      }
      return payload;
    }

    function setMotivationStatus(kind, text) {
      motivationStatusEl.className = ["motivation-status", kind].filter(Boolean).join(" ");
      motivationStatusEl.textContent = text;
    }

    async function testMotivation() {
      setBusy(true);
      setStatus("", "Проверяю AI-модель...");
      setMotivationStatus("", "Отправляю тестовый запрос...");
      try {
        await saveMotivationSettings();
        const response = await fetch("/api/motivation/test", { method: "POST" });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось проверить AI-модель.");
        }
        const text = payload.quote && payload.quote.text ? "Модель вернула: " + payload.quote.text : "AI-модель отвечает.";
        setMotivationStatus("ok", text);
        setStatus("ok", payload.message || "AI-модель проверена.");
      } catch (error) {
        setMotivationStatus("error", "Ошибка: " + error.message);
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    function setGoogleStatus(kind, text) {
      googleStatusEl.className = ["motivation-status", kind].filter(Boolean).join(" ");
      googleStatusEl.textContent = text;
    }

    function renderGoogleStatus(status) {
      if (!status || !status.credentialsAvailable) {
        setGoogleStatus("error", "Положи OAuth credentials.json в " + ((status && status.credentialsPath) || "/data/google/credentials.json") + ".");
        return;
      }
      if (status.authorized) {
        setGoogleStatus("ok", "Google подключен. В чек попадут включенные Google-блоки.");
        return;
      }
      setGoogleStatus("", "credentials.json найден. Нажми «Авторизовать», чтобы подключить почту и календарь.");
    }

    async function loadGoogleStatus() {
      const response = await fetch("/api/google/status");
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось загрузить статус Google.");
      }
      renderGoogleStatus(payload.status);
      return payload.status;
    }

    async function disconnectGoogle() {
      setBusy(true);
      setStatus("", "Отключаю Google...");
      try {
        const response = await fetch("/api/google/disconnect", { method: "POST" });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось отключить Google.");
        }
        await loadGoogleStatus();
        setStatus("ok", payload.message || "Google отключен.");
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    function setFinanceEditing(editing) {
      document.querySelectorAll("[data-finance-input]").forEach(input => {
        if (editing) {
          input.removeAttribute("readonly");
        } else {
          input.setAttribute("readonly", "");
        }
      });
      document.querySelector('[data-action="edit-finance"]').classList.toggle("hidden", editing);
      document.querySelector('[data-action="save-finance"]').classList.toggle("hidden", !editing);
      document.querySelector('[data-action="cancel-finance"]').classList.toggle("hidden", !editing);
    }

    async function saveFinanceExplicitly() {
      setBusy(true);
      setStatus("", "Сохраняю TON...");
      try {
        await saveFinanceSettings();
        financeDraft = {
          amountTon: document.querySelector("#ton-amount").value,
          investedUsd: document.querySelector("#ton-invested").value
        };
        setFinanceEditing(false);
        setStatus("ok", "Настройки TON сохранены.");
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    function cancelFinanceEditing() {
      document.querySelector("#ton-amount").value = financeDraft.amountTon;
      document.querySelector("#ton-invested").value = financeDraft.investedUsd;
      setFinanceEditing(false);
      setStatus("", "");
    }

    function validateNewsSettings() {
      const errors = [];
      document.querySelectorAll("[data-news-source]").forEach(source => {
        const enabled = source.querySelector("[data-news-enabled]").checked;
        const countInput = source.querySelector("[data-news-count]");
        countInput.classList.remove("invalid");
        countInput.removeAttribute("aria-invalid");
        if (!enabled) {
          return;
        }
        const raw = countInput.value.trim();
        const count = Number.parseInt(raw, 10);
        const validInteger = /^\d+$/.test(raw);
        if (!validInteger || !Number.isFinite(count) || count < 1 || count > 100) {
          countInput.classList.add("invalid");
          countInput.setAttribute("aria-invalid", "true");
          const name = source.querySelector("label")?.textContent.trim() || "RSS";
          errors.push(name + ": укажи количество от 1 до 100");
        }
      });
      if (errors.length > 0) {
        throw new Error(errors.join("\n"));
      }
    }

    async function saveNewsSettings() {
      validateNewsSettings();
      const sources = Array.from(document.querySelectorAll("[data-news-source]")).map(source => {
        const countInput = source.querySelector("[data-news-count]");
        const count = Number.parseInt(countInput.value, 10);
        return {
          preset: source.querySelector("[data-news-preset]").value,
          enabled: source.querySelector("[data-news-enabled]").checked,
          feedUrl: source.querySelector("[data-news-url-visible]").value,
          maxItems: Number.isFinite(count) && count > 0 ? count : 1
        };
      });
      const response = await fetch("/api/settings/news", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          translateTitles: document.querySelector("#news-translate").checked,
          sources
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить настройки новостей.");
      }
      return payload;
    }

    function readFont(selector) {
      const value = Number.parseInt(document.querySelector(selector).value, 10);
      return Number.isFinite(value) ? value : 0;
    }

    function metricForFont(font) {
      return fontMetrics.get(font) || fontMetrics.get(0) || fallbackFontMetrics[0];
    }

    function fontLabel(metric) {
      const lineLength = metric.lineLength ? metric.lineLength + " симв/стр" : "длина ?";
      const fontWidth = metric.fontWidth ? metric.fontWidth + " px" : "ширина ?";
      return "Шрифт " + metric.font + " · " + lineLength + " · " + fontWidth;
    }

    function updateFontControls() {
      const metrics = Array.from(fontMetrics.values()).sort((a, b) => a.font - b.font);
      const visibleMetrics = metrics.length > 0 ? metrics : fallbackFontMetrics;
      document.querySelectorAll("[data-font-select]").forEach(select => {
        const selected = select.value;
        select.replaceChildren(...visibleMetrics.map(metric => {
          const option = document.createElement("option");
          option.value = String(metric.font);
          option.textContent = fontLabel(metric);
          return option;
        }));
        if (visibleMetrics.some(metric => String(metric.font) === selected)) {
          select.value = selected;
        }
      });
      fontMetricsEl.textContent = visibleMetrics.map(fontLabel).join("\n");
    }

    async function saveReceiptStyle() {
      const response = await fetch("/api/settings/receipt-style", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          configured: true,
          normalFont: readFont("#normal-font"),
          emphasisFont: readFont("#calendar-font"),
          calendarFont: readFont("#calendar-font"),
          temperatureFont: readFont("#temperature-font"),
          calendarDoubleWidth: document.querySelector("#calendar-double-width").checked,
          calendarDoubleHeight: document.querySelector("#calendar-double-height").checked,
          temperatureDoubleWidth: document.querySelector("#temperature-double-width").checked,
          temperatureDoubleHeight: document.querySelector("#temperature-double-height").checked
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить настройки чека.");
      }
      return payload;
    }

    function checked(selector) {
      return document.querySelector(selector).checked;
    }

    function readReceiptContentSettings() {
      return {
        configured: true,
        showWeather: checked("#content-weather"),
        showWeatherAdvice: checked("#content-weather-advice"),
        showMotivationQuote: checked("#content-motivation-quote"),
        showTonPortfolio: checked("#content-ton-portfolio"),
        showUsdBynRate: checked("#content-usd-byn-rate"),
        showBankRates: checked("#content-bank-rates"),
        showMail: checked("#content-mail"),
        showCalendar: checked("#content-calendar"),
        showNews: checked("#content-news")
      };
    }

    async function saveReceiptContentSettings(content = readReceiptContentSettings()) {
      const response = await fetch("/api/settings/receipt-content", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(content)
      });
      const payload = await response.json();
      if (!response.ok || !payload.ok) {
        throw new Error(payload.error || "Не удалось сохранить состав чека.");
      }
      return payload;
    }

    async function saveAllSettings() {
      const receiptContent = readReceiptContentSettings();
      await saveSettings();
      await saveReceiptContentSettings(receiptContent);
      if (receiptContent.showWeather || receiptContent.showWeatherAdvice) {
        await saveWeatherSettings();
      }
      await saveMotivationSettings();
      await saveNewsSettings();
      await saveReceiptStyle();
    }

    function scheduleMode() {
      const checked = document.querySelector('input[name="schedule-mode"]:checked');
      return checked ? checked.value : "interval";
    }

    function updateScheduleMode() {
      const mode = scheduleMode();
      document.querySelector("#schedule-interval-panel").hidden = mode !== "interval";
      document.querySelector("#schedule-times-panel").hidden = mode !== "daily_times";
    }

    function addScheduleTime(value = "07:00") {
      const row = document.createElement("div");
      row.className = "schedule-time-row";
      row.dataset.scheduleTimeRow = "";
      const input = document.createElement("input");
      input.type = "time";
      input.dataset.scheduleTime = "";
      input.value = value;
      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "secondary";
      remove.dataset.action = "remove-schedule-time";
      remove.textContent = "Удалить";
      row.append(input, remove);
      document.querySelector("#schedule-time-list").appendChild(row);
    }

    function readScheduleSettings(enabled) {
      const interval = Number.parseInt(document.querySelector("#schedule-interval").value, 10);
      const times = Array.from(document.querySelectorAll("[data-schedule-time]"))
        .map(input => input.value.trim())
        .filter(Boolean);
      return {
        enabled,
        mode: scheduleMode(),
        intervalMinutes: Number.isFinite(interval) ? interval : 15,
        times,
        timezone: "Europe/Minsk"
      };
    }

    function formatDateTime(value) {
      if (!value || value.startsWith("0001-01-01")) {
        return "—";
      }
      const date = new Date(value);
      if (Number.isNaN(date.getTime())) {
        return "—";
      }
      return date.toLocaleString("ru-RU", {
        day: "2-digit",
        month: "2-digit",
        hour: "2-digit",
        minute: "2-digit"
      });
    }

    function renderSchedulerStatus(status) {
      if (!status) {
        schedulerStatusEl.textContent = "Статус расписания недоступен.";
        return;
      }
      const enabled = status.settings && status.settings.enabled ? "включено" : "выключено";
      const mode = status.settings && status.settings.mode === "daily_times" ? "точное время" : "интервал";
      schedulerStatusEl.textContent = [
        "Состояние: " + enabled + " · " + mode + (status.running ? " · печатает сейчас" : ""),
        "Следующий запуск: " + formatDateTime(status.nextRunAt),
        "Последний запуск: " + formatDateTime(status.lastSuccessAt),
        "Ошибка: " + (status.lastError || "—")
      ].join("\n");
    }

    async function loadSchedulerStatus() {
      try {
        const response = await fetch("/api/scheduler/status");
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось загрузить статус расписания.");
        }
        renderSchedulerStatus(payload.status);
      } catch (error) {
        schedulerStatusEl.textContent = error.message;
      }
    }

    async function saveScheduleSettings() {
      setBusy(true);
      setStatus("", "Сохраняю расписание...");
      try {
        await saveAllSettings();
        const response = await fetch("/api/settings/schedule", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(readScheduleSettings(true))
        });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось сохранить расписание.");
        }
        await loadSchedulerStatus();
        setStatus("ok", payload.message || "Расписание сохранено.");
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    async function stopSchedule() {
      setBusy(true);
      setStatus("", "Останавливаю расписание...");
      try {
        const response = await fetch("/api/settings/schedule/stop", { method: "POST" });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось остановить расписание.");
        }
        await loadSchedulerStatus();
        setStatus("ok", payload.message || "Расписание остановлено.");
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    function renderPreview(lines) {
      const paper = document.createElement("div");
      paper.className = "receipt-paper";
      const normalMetric = metricForFont(readFont("#normal-font"));
      paper.style.setProperty("--paper-chars", normalMetric.lineLength || 32);
      for (const line of lines) {
        const node = document.createElement("div");
        const imageURL = line.ImageURL || (line.ImageKey ? "/assets/weather-icons/print/" + encodeURIComponent(line.ImageKey) + ".png" : "");
        if (imageURL) {
          node.className = [
            "receipt-line",
            "receipt-image-line",
            "align-" + (line.Alignment || "center"),
            line.Role ? "role-" + line.Role : ""
          ].filter(Boolean).join(" ");
          const image = document.createElement("img");
          image.className = "receipt-image";
          image.src = imageURL;
          image.alt = line.ImageKey || "receipt image";
          image.title = line.ImageKey || "receipt image";
          const imageWidth = Number.isFinite(line.ImageWidth) && line.ImageWidth > 0 ? line.ImageWidth : 96;
          const imageHeight = Number.isFinite(line.ImageHeight) && line.ImageHeight > 0 ? line.ImageHeight : imageWidth;
          const previewWidth = Math.max(48, Math.min(320, imageWidth * 0.8));
          const previewHeight = Math.max(32, previewWidth * imageHeight / imageWidth);
          image.style.width = previewWidth.toFixed(0) + "px";
          image.style.height = previewHeight.toFixed(0) + "px";
          node.style.setProperty("--image-line-height", (previewHeight + 8).toFixed(0) + "px");
          node.appendChild(image);
          paper.appendChild(node);
          continue;
        }
        const font = Number.isFinite(line.Font) ? line.Font : 0;
        const metric = metricForFont(font);
        const baseWidth = normalMetric.fontWidth || fallbackFontMetrics[0].fontWidth;
        const fontWidth = metric.fontWidth || baseWidth;
        const lineSize = Math.max(10, Math.min(32, 16 * (fontWidth / baseWidth)));
        node.className = [
          "receipt-line",
          "align-" + (line.Alignment || "left"),
          line.Role ? "role-" + line.Role : ""
        ].filter(Boolean).join(" ");
        node.style.setProperty("--line-size", lineSize.toFixed(2) + "px");
        node.style.setProperty("--line-scale-x", line.DoubleWidth ? 2 : 1);
        node.style.setProperty("--line-scale-y", line.DoubleHeight ? 2 : 1);
        node.title = fontLabel(metric) + (line.DoubleWidth ? " · double width" : "") + (line.DoubleHeight ? " · double height" : "");
        const text = document.createElement("span");
        text.className = "receipt-line-text";
        text.textContent = line.Text || " ";
        node.appendChild(text);
        paper.appendChild(node);
      }
      previewEl.replaceChildren(paper);
    }

    function styledPreviewLines(lines) {
      return lines.map(line => {
        const next = { ...line };
        switch (next.Role) {
          case "calendar":
            next.Font = readFont("#calendar-font");
            next.DoubleWidth = document.querySelector("#calendar-double-width").checked;
            next.DoubleHeight = document.querySelector("#calendar-double-height").checked;
            break;
          case "temperature":
            next.Font = readFont("#temperature-font");
            next.DoubleWidth = document.querySelector("#temperature-double-width").checked;
            next.DoubleHeight = document.querySelector("#temperature-double-height").checked;
            break;
          case "original":
            next.Font = 1;
            next.DoubleWidth = false;
            next.DoubleHeight = false;
            break;
          default:
            next.Font = readFont("#normal-font");
            next.DoubleWidth = false;
            next.DoubleHeight = false;
            break;
        }
        return next;
      });
    }

    function refreshPreviewStyle() {
      if (lastPreviewLines.length === 0) {
        return;
      }
      renderPreview(styledPreviewLines(lastPreviewLines));
    }

    async function runAction(path, successPrefix) {
      setBusy(true);
      setStatus("", "Работаю...");
      try {
        await saveAllSettings();
        const response = await fetch(path, { method: "POST" });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Операция не выполнена.");
        }
        setStatus("ok", successPrefix ? successPrefix + "\n" + payload.message : payload.message);
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    async function previewReceipt() {
      setBusy(true);
      setStatus("", "Собираю превью...");
      try {
        await saveAllSettings();
        const response = await fetch("/api/receipt/preview", { method: "POST" });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось собрать превью.");
        }
        lastPreviewLines = (payload.lines || []).map(line => ({ ...line }));
        renderPreview(styledPreviewLines(lastPreviewLines));
        setStatus(payload.warnings && payload.warnings.length > 0 ? "" : "ok", payload.message || "Превью собрано.");
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    async function loadFontMetrics() {
      setBusy(true);
      setStatus("", "Читаю параметры шрифтов ККТ...");
      try {
        await saveSettings();
        const response = await fetch("/api/printer/fonts", { method: "POST" });
        const payload = await response.json();
        if (!response.ok || !payload.ok) {
          throw new Error(payload.error || "Не удалось загрузить параметры шрифтов.");
        }
        if (!payload.fonts || payload.fonts.length === 0) {
          throw new Error("ККТ не вернула параметры шрифтов.");
        }
        fontMetrics = new Map(payload.fonts.map(metric => [metric.font, metric]));
        updateFontControls();
        refreshPreviewStyle();
        setStatus("ok", payload.message || "Метрики шрифтов ККТ загружены.");
      } catch (error) {
        setStatus("error", error.message);
      } finally {
        setBusy(false);
      }
    }

    document.querySelector('[data-action="check"]').addEventListener("click", () => {
      runAction("/api/printer/check", "");
    });
    document.querySelector('[data-action="fonts"]').addEventListener("click", () => {
      loadFontMetrics();
    });
    document.querySelector('[data-action="search-weather-location"]').addEventListener("click", () => {
      searchWeatherLocations().catch(error => {
        setStatus("error", error.message);
      });
    });
    document.querySelector('[data-action="test-motivation"]').addEventListener("click", () => {
      testMotivation();
    });
    document.querySelector('[data-action="google-disconnect"]').addEventListener("click", () => {
      disconnectGoogle();
    });
    weatherNameInput.addEventListener("input", queueWeatherLocationSearch);
    weatherLocationResultsEl.addEventListener("click", event => {
      const button = event.target.closest(".location-result");
      if (!button) {
        return;
      }
      weatherNameInput.value = button.dataset.locationName || button.textContent.trim();
      weatherLatitudeInput.value = button.dataset.locationLatitude || "";
      weatherLongitudeInput.value = button.dataset.locationLongitude || "";
      weatherLocationResultsEl.replaceChildren();
      weatherLocationHelpEl.textContent = "Координаты обновлены из выбранного города.";
      setWeatherLocationSelected(true);
    });
    document.querySelector('[data-action="edit-finance"]').addEventListener("click", () => {
      financeDraft = {
        amountTon: document.querySelector("#ton-amount").value,
        investedUsd: document.querySelector("#ton-invested").value
      };
      setFinanceEditing(true);
    });
    document.querySelector('[data-action="save-finance"]').addEventListener("click", () => {
      saveFinanceExplicitly();
    });
    document.querySelector('[data-action="cancel-finance"]').addEventListener("click", () => {
      cancelFinanceEditing();
    });
    document.querySelector('[data-action="print"]').addEventListener("click", () => {
      runAction("/api/print/test", "");
    });
    document.querySelector('[data-action="preview"]').addEventListener("click", () => {
      previewReceipt();
    });
    document.querySelector('[data-action="weather"]').addEventListener("click", () => {
      runAction("/api/print/weather", "");
    });
    document.querySelectorAll("[data-news-count], [data-news-enabled], #news-translate").forEach(input => {
      input.addEventListener("input", () => {
        const row = input.closest("[data-news-source]");
        const countInput = row?.querySelector("[data-news-count]");
        countInput?.classList.remove("invalid");
        countInput?.removeAttribute("aria-invalid");
      });
      input.addEventListener("change", () => {
        const row = input.closest("[data-news-source]");
        const countInput = row?.querySelector("[data-news-count]");
        countInput?.classList.remove("invalid");
        countInput?.removeAttribute("aria-invalid");
      });
    });
    document.querySelector('[data-action="save-schedule"]').addEventListener("click", () => {
      saveScheduleSettings();
    });
    document.querySelector('[data-action="stop-schedule"]').addEventListener("click", () => {
      stopSchedule();
    });
    document.querySelector('[data-action="add-schedule-time"]').addEventListener("click", () => {
      addScheduleTime("");
    });
    document.querySelector("#schedule-time-list").addEventListener("click", event => {
      const button = event.target.closest('[data-action="remove-schedule-time"]');
      if (!button) {
        return;
      }
      const row = button.closest("[data-schedule-time-row]");
      if (row) {
        row.remove();
      }
      if (document.querySelectorAll("[data-schedule-time-row]").length === 0) {
        addScheduleTime("");
      }
    });
    document.querySelectorAll('input[name="schedule-mode"]').forEach(input => {
      input.addEventListener("change", updateScheduleMode);
    });

    [
      "#calendar-font",
      "#calendar-double-width",
      "#calendar-double-height",
      "#temperature-font",
      "#temperature-double-width",
      "#temperature-double-height",
      "#normal-font"
    ].forEach(selector => {
      document.querySelector(selector).addEventListener("input", refreshPreviewStyle);
      document.querySelector(selector).addEventListener("change", refreshPreviewStyle);
    });
    updateFontControls();
    updateScheduleMode();
    loadSchedulerStatus();
    loadGoogleStatus().catch(error => {
      setGoogleStatus("error", error.message);
    });
  </script>
</body>
</html>`))
