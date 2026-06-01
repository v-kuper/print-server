package web

import (
	"context"
	"net/http"
	"time"

	"atol-server/internal/app"
	"atol-server/internal/articlesummary"
	"atol-server/internal/dailyquest"
	"atol-server/internal/denistrends"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/imageeditor"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/receiptsnapshot"
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
	LoadDenisTrends() (denistrends.Settings, error)
	SaveDenisTrends(denistrends.Settings) error
	LoadReceiptStyle() (receipt.StyleSettings, error)
	SaveReceiptStyle(receipt.StyleSettings) error
	LoadReceiptContent() (receipt.ContentSettings, error)
	SaveReceiptContent(receipt.ContentSettings) error
	LoadReceiptSnapshotSettings() (receiptsnapshot.Settings, error)
	SaveReceiptSnapshotSettings(receiptsnapshot.Settings) error
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
	GenerateDailyQuests(context.Context, motivation.Settings, []dailyquest.Quest) ([]dailyquest.DailyQuest, error)
	TranslateNewsTitles(context.Context, motivation.Settings, []motivation.NewsTitle) ([]motivation.NewsTranslation, error)
}

type ArticleSummaryProvider interface {
	Summarize(context.Context, motivation.Settings, string) (articlesummary.Result, error)
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

type ReceiptSnapshotStore interface {
	Create(context.Context, receiptsnapshot.CreateInput) (receiptsnapshot.Snapshot, error)
	FinalizeReceiptLines(context.Context, string, []receiptsnapshot.ReceiptLine, int) error
	Publish(context.Context, string) error
	Fail(context.Context, string, error) error
	Load(context.Context, string) (receiptsnapshot.Snapshot, error)
	LoadSummary(context.Context, string, int, string) (receiptsnapshot.Summary, error)
	SaveSummary(context.Context, receiptsnapshot.SummaryInput) error
}

type DailyReceiptWarningService interface {
	BuildDailyReceiptWithWarnings(context.Context) ([]receipt.Line, []string, error)
}

type DailyReceiptPreviewWarningService interface {
	BuildDailyReceiptPreviewWithWarnings(context.Context) ([]receipt.Line, []string, error)
}

type DailyReceiptPrintWarningService interface {
	PrintDailyReceiptWithWarnings(context.Context) ([]string, error)
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
	articleSummaryProvider ArticleSummaryProvider
	receiptService         DailyReceiptService
	snapshotStore          ReceiptSnapshotStore
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

func WithArticleSummaryProvider(provider ArticleSummaryProvider) ServerOption {
	return func(s *Server) {
		s.articleSummaryProvider = provider
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

func WithReceiptSnapshotStore(store ReceiptSnapshotStore) ServerOption {
	return func(s *Server) {
		s.snapshotStore = store
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
		articleSummaryProvider: articlesummary.NewProvider(nil),
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
			app.WithReceiptSnapshotStore(server.snapshotStore),
		)
	}
	return server
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /snapshots/{id}", s.handleReceiptSnapshot)
	mux.HandleFunc("POST /api/snapshots/{id}/lines/{lineIndex}/summary", s.handleReceiptSnapshotSummary)
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
	mux.HandleFunc("POST /api/settings/denis-trends", s.handleSaveDenisTrends)
	mux.HandleFunc("POST /api/settings/receipt-snapshot", s.handleSaveReceiptSnapshot)
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
