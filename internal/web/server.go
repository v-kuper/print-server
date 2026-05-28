package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"atol-server/internal/app"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/imageeditor"
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
	PrintPixelBuffer(context.Context, printer.Config, printer.PixelBuffer) error
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
	GenerateCalendarAdvice(context.Context, motivation.Settings, motivation.CalendarContext) (motivation.CalendarAdvice, error)
	GenerateHistoryFacts(context.Context, motivation.Settings, []motivation.HistoryEvent) ([]motivation.HistoryFact, error)
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

type ImageEditorStore interface {
	State() (imageeditor.State, error)
	Save(imageeditor.SaveInput) (imageeditor.State, error)
	LoadBuffer() (printer.PixelBuffer, error)
	LoadPreviewPNG() ([]byte, error)
	Clear() error
}

type PrintJobStore interface {
	StartPrintJob(kind string, request any) (string, error)
	FinishPrintJob(id string, printErr error) error
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
	imageStore             ImageEditorStore
	clock                  func() time.Time
	assetsPath             string
	imageEditorPath        string
}

const defaultAssetsPath = "/opt/atol-server/assets"
const defaultImageEditorPath = "/data/image-editor"
const maxImageEditorHeight = 2048
const maxTextPrintFont = 9

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

type imageEditorResponse struct {
	OK      bool               `json:"ok"`
	Message string             `json:"message,omitempty"`
	Error   string             `json:"error,omitempty"`
	Data    *imageeditor.State `json:"data,omitempty"`
}

type imageEditorSaveRequest struct {
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Pixels     []int          `json:"pixels"`
	Settings   map[string]any `json:"settings"`
	PreviewPNG string         `json:"previewPng"`
}

type textPrintBlock struct {
	Text         string `json:"text"`
	Font         int    `json:"font"`
	Alignment    string `json:"alignment"`
	DoubleWidth  bool   `json:"doubleWidth"`
	DoubleHeight bool   `json:"doubleHeight"`
}

type textPrintRequest struct {
	Blocks []textPrintBlock `json:"blocks"`
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

func WithImageEditorPath(path string) ServerOption {
	return func(s *Server) {
		s.imageEditorPath = path
	}
}

func WithImageEditorStore(store ImageEditorStore) ServerOption {
	return func(s *Server) {
		s.imageStore = store
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
		imageEditorPath:        defaultImageEditorPath,
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
	mux.HandleFunc("GET /static/app.css", handleAppCSS)
	mux.HandleFunc("GET /static/app.js", handleAppJS)
	mux.Handle("GET /assets/", noStore(http.StripPrefix("/assets/", http.FileServer(http.Dir(s.assetsPathOrDefault())))))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /api/bootstrap", s.handleBootstrap)
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
	mux.HandleFunc("POST /api/print/text", s.handlePrintText)
	mux.HandleFunc("POST /api/print/weather", s.handlePrintWeather)
	mux.HandleFunc("GET /api/image-editor/state", s.handleImageEditorState)
	mux.HandleFunc("POST /api/image-editor/save", s.handleImageEditorSave)
	mux.HandleFunc("GET /api/image-editor/preview", s.handleImageEditorPreview)
	mux.HandleFunc("POST /api/image-editor/print", s.handleImageEditorPrint)
	mux.HandleFunc("DELETE /api/image-editor/image", s.handleImageEditorClear)
	return mux
}

func (s *Server) assetsPathOrDefault() string {
	if s.assetsPath == "" {
		return defaultAssetsPath
	}
	return s.assetsPath
}

func (s *Server) imageEditorPathOrDefault() string {
	if s.imageEditorPath == "" {
		return defaultImageEditorPath
	}
	return s.imageEditorPath
}

func (s *Server) imageEditorStore() ImageEditorStore {
	if s.imageStore != nil {
		return s.imageStore
	}
	return imageeditor.NewStore(s.imageEditorPathOrDefault(), s.clock)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	writeStaticClientFile(w, "text/html; charset=utf-8", indexHTML)
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
	scheduleSettings, err := s.store.LoadSchedule()
	if err != nil {
		return bootstrapData{}, err
	}
	googleStatus := googleintegration.Status{}
	if s.googleClient != nil {
		googleStatus = s.googleClient.Status()
	}
	normalizedNews := newsSettings.Normalized()
	return bootstrapData{
		Printer:           printerConfig,
		Weather:           weatherLocation,
		Finance:           portfolio,
		Motivation:        motivationSettings,
		GoogleStatus:      googleStatus,
		News:              bootstrapNewsSettings{TranslateTitles: normalizedNews.TranslateTitlesEnabled(), Sources: normalizedNews.Sources},
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

	jobID, err := s.startPrintJob("test", map[string]any{"receipt": "test"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.printer.PrintReceipt(r.Context(), config, receipt.TestReceipt(s.clock()))
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Тестовый чек напечатан.",
	})
}

func (s *Server) handlePrintText(w http.ResponseWriter, r *http.Request) {
	var request textPrintRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}
	lines, err := textPrintLines(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	jobID, err := s.startPrintJob("text", request)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.printer.PrintReceipt(r.Context(), config, lines)
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Текст напечатан.",
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
	jobID, err := s.startPrintJob("weather", map[string]any{"receipt": "daily"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.receiptService.PrintDailyReceipt(r.Context())
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, statusForBuildError(printErr), statusResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Чек напечатан.",
	})
}

func (s *Server) handleImageEditorState(w http.ResponseWriter, r *http.Request) {
	state, err := s.imageEditorStore().State()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:   true,
		Data: &state,
	})
}

func (s *Server) handleImageEditorSave(w http.ResponseWriter, r *http.Request) {
	input, err := decodeImageEditorSaveRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	state, err := s.imageEditorStore().Save(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:      true,
		Message: "Изображение сохранено.",
		Data:    &state,
	})
}

func (s *Server) handleImageEditorPreview(w http.ResponseWriter, r *http.Request) {
	data, err := s.imageEditorStore().LoadPreviewPNG()
	if os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, imageEditorResponse{
			OK:    false,
			Error: "image editor preview is not saved",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleImageEditorPrint(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	buffer, err := s.imageEditorStore().LoadBuffer()
	if err != nil {
		writeJSON(w, http.StatusNotFound, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	jobID, err := s.startPrintJob("image", map[string]any{"source": "image_editor"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.printer.PrintPixelBuffer(r.Context(), config, buffer)
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, http.StatusBadGateway, imageEditorResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:      true,
		Message: "Изображение напечатано.",
	})
}

func (s *Server) handleImageEditorClear(w http.ResponseWriter, r *http.Request) {
	if err := s.imageEditorStore().Clear(); err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	state := imageeditor.State{Settings: map[string]any{}}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:      true,
		Message: "Изображение удалено.",
		Data:    &state,
	})
}

func (s *Server) startPrintJob(kind string, request any) (string, error) {
	store, ok := s.store.(PrintJobStore)
	if !ok {
		return "", nil
	}
	return store.StartPrintJob(kind, request)
}

func (s *Server) finishPrintJob(id string, printErr error) error {
	if id == "" {
		return nil
	}
	store, ok := s.store.(PrintJobStore)
	if !ok {
		return nil
	}
	return store.FinishPrintJob(id, printErr)
}

func decodeImageEditorSaveRequest(r *http.Request) (imageeditor.SaveInput, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		return decodeMultipartImageEditorSaveRequest(r)
	}

	var request imageEditorSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("invalid JSON: %w", err)
	}
	pixels, err := pixelsFromInts(request.Pixels)
	if err != nil {
		return imageeditor.SaveInput{}, err
	}
	preview, err := decodePreviewPNGField(request.PreviewPNG)
	if err != nil {
		return imageeditor.SaveInput{}, err
	}
	return imageeditor.SaveInput{
		Width:      request.Width,
		Height:     request.Height,
		Pixels:     pixels,
		PreviewPNG: preview,
		Settings:   request.Settings,
	}, nil
}

func decodeMultipartImageEditorSaveRequest(r *http.Request) (imageeditor.SaveInput, error) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("invalid multipart form: %w", err)
	}
	width, err := strconv.Atoi(strings.TrimSpace(r.FormValue("width")))
	if err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("width is required")
	}
	height, err := strconv.Atoi(strings.TrimSpace(r.FormValue("height")))
	if err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("height is required")
	}
	pixels, err := multipartFileBytes(r, "pixels")
	if err != nil {
		field := strings.TrimSpace(r.FormValue("pixels"))
		if field == "" {
			return imageeditor.SaveInput{}, err
		}
		var values []int
		if jsonErr := json.Unmarshal([]byte(field), &values); jsonErr != nil {
			return imageeditor.SaveInput{}, fmt.Errorf("pixels must be a binary file or JSON array: %w", jsonErr)
		}
		pixels, err = pixelsFromInts(values)
		if err != nil {
			return imageeditor.SaveInput{}, err
		}
	}
	preview, err := multipartFileBytes(r, "previewPng")
	if err != nil {
		preview, err = decodePreviewPNGField(r.FormValue("previewPng"))
		if err != nil {
			return imageeditor.SaveInput{}, err
		}
	}
	settings := map[string]any{}
	if rawSettings := strings.TrimSpace(r.FormValue("settings")); rawSettings != "" {
		if err := json.Unmarshal([]byte(rawSettings), &settings); err != nil {
			return imageeditor.SaveInput{}, fmt.Errorf("settings must be valid JSON: %w", err)
		}
	}
	return imageeditor.SaveInput{
		Width:      width,
		Height:     height,
		Pixels:     pixels,
		PreviewPNG: preview,
		Settings:   settings,
	}, nil
}

func multipartFileBytes(r *http.Request, field string) ([]byte, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("%s file is required", field)
	}
	defer file.Close()
	return io.ReadAll(file)
}

func pixelsFromInts(values []int) ([]byte, error) {
	pixels := make([]byte, len(values))
	for index, value := range values {
		if value != 0 && value != 255 {
			return nil, fmt.Errorf("pixel value at index %d must be 0 or 255, got %d", index, value)
		}
		pixels[index] = byte(value)
	}
	return pixels, nil
}

func decodePreviewPNGField(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("previewPng is required")
	}
	const prefix = "data:image/png;base64,"
	if strings.HasPrefix(value, prefix) {
		value = strings.TrimPrefix(value, prefix)
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("previewPng must be base64 PNG: %w", err)
	}
	return data, nil
}

func textPrintLines(request textPrintRequest) ([]receipt.Line, error) {
	if len(request.Blocks) == 0 {
		return nil, fmt.Errorf("blocks is required")
	}
	hasContent := false
	for _, b := range request.Blocks {
		if strings.TrimSpace(b.Text) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return nil, fmt.Errorf("at least one block must have text")
	}
	var lines []receipt.Line
	for _, block := range request.Blocks {
		font, err := validatedTextPrintFont(block.Font, "font")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(block.Alignment) == "" {
			block.Alignment = string(receipt.AlignmentLeft)
		}
		alignment, err := validatedTextPrintAlignment(block.Alignment, "alignment")
		if err != nil {
			return nil, err
		}
		text := strings.ReplaceAll(block.Text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		for _, lineText := range strings.Split(text, "\n") {
			lines = append(lines, receipt.Line{
				Text:         lineText,
				Alignment:    alignment,
				Role:         receipt.RoleNormal,
				Font:         font,
				DoubleWidth:  block.DoubleWidth,
				DoubleHeight: block.DoubleHeight,
			})
		}
	}
	return lines, nil
}

func validatedTextPrintFont(font int, field string) (int, error) {
	if font < 0 || font > maxTextPrintFont {
		return 0, fmt.Errorf("%s must be between 0 and %d", field, maxTextPrintFont)
	}
	return font, nil
}

func validatedTextPrintAlignment(value string, field string) (receipt.Alignment, error) {
	switch receipt.Alignment(strings.TrimSpace(value)) {
	case receipt.AlignmentLeft:
		return receipt.AlignmentLeft, nil
	case receipt.AlignmentCenter, "":
		return receipt.AlignmentCenter, nil
	case receipt.AlignmentRight:
		return receipt.AlignmentRight, nil
	default:
		return "", fmt.Errorf("%s must be left, center, or right", field)
	}
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
	ReceiptStyle      receipt.StyleSettings    `json:"receiptStyle"`
	ReceiptContent    receipt.ContentSettings  `json:"receiptContent"`
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
