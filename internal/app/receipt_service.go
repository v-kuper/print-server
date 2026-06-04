package app

import (
	"context"
	"net/http"
	"time"

	"atol-server/internal/bankrates"
	"atol-server/internal/dailyquest"
	"atol-server/internal/denistrends"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/history"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printcoord"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/receiptsnapshot"
	"atol-server/internal/weather"
)

type SettingsStore interface {
	LoadPrinter() (printer.Config, error)
	LoadWeather() (weather.Location, error)
	LoadFinance() (finance.TonPortfolio, error)
	LoadNews() (news.Settings, error)
	LoadDenisTrends() (denistrends.Settings, error)
	LoadMotivation() (motivation.Settings, error)
	SaveMotivation(motivation.Settings) error
	LoadReceiptStyle() (receipt.StyleSettings, error)
	LoadReceiptContent() (receipt.ContentSettings, error)
	LoadReceiptSnapshotSettings() (receiptsnapshot.Settings, error)
}

type Printer interface {
	PrintReceipt(context.Context, printer.Config, []receipt.Line) error
}

type PrintCoordinator interface {
	RunUserPrint(context.Context, func(context.Context) error) error
}

type ReceiptSnapshotStore interface {
	Create(context.Context, receiptsnapshot.CreateInput) (receiptsnapshot.Snapshot, error)
	FinalizeReceiptLines(context.Context, string, []receiptsnapshot.ReceiptLine, int) error
	Publish(context.Context, string) error
	Fail(context.Context, string, error) error
}

type WeatherProvider interface {
	Current(context.Context, weather.Location) (weather.Snapshot, error)
}

type TonPriceProvider interface {
	CurrentPrice(context.Context) (finance.TonPrice, error)
}

type TonMarketChartProvider interface {
	MarketChart(context.Context) (finance.TonMarketChart, error)
}

type FiatRateProvider interface {
	CurrentRate(context.Context) (finance.FiatRate, error)
}

type FiatMarketChartProvider interface {
	MarketChart(context.Context, time.Time) (finance.FiatMarketChart, error)
}

type OilPriceProvider interface {
	CurrentPrice(context.Context) (finance.OilPrice, error)
}

type OilMarketChartProvider interface {
	MarketChart(context.Context) (finance.OilMarketChart, error)
}

type BankRatesProvider interface {
	Current(context.Context) (bankrates.Summary, error)
}

type NewsProvider interface {
	Current(context.Context, news.Settings) ([]news.Item, error)
}

type DenisTrendsProvider interface {
	Current(context.Context, denistrends.Settings, time.Time) ([]denistrends.Section, error)
}

type HistoryProvider interface {
	Current(context.Context, time.Time) ([]history.Event, error)
}

type GoogleProvider interface {
	Current(context.Context) (googleintegration.Summary, error)
}

type SelectiveGoogleProvider interface {
	CurrentSelected(context.Context, bool, bool) (googleintegration.Summary, error)
}

type MotivationProvider interface {
	Generate(context.Context, motivation.Settings) (motivation.Quote, error)
	GenerateWeatherAdvice(context.Context, motivation.Settings, motivation.WeatherContext) (motivation.WeatherAdvice, error)
	GenerateCalendarAdvice(context.Context, motivation.Settings, motivation.CalendarContext) (motivation.CalendarAdvice, error)
	GenerateHistoryFacts(context.Context, motivation.Settings, []motivation.HistoryEvent) ([]motivation.HistoryFact, error)
	GenerateDailyQuests(context.Context, motivation.Settings, []dailyquest.Quest) ([]dailyquest.DailyQuest, error)
	TranslateNewsTitles(context.Context, motivation.Settings, []motivation.NewsTitle) ([]motivation.NewsTranslation, error)
}

type ReceiptServiceOption func(*ReceiptService)

type ReceiptService struct {
	store               SettingsStore
	printer             Printer
	weatherProvider     WeatherProvider
	tonPriceProvider    TonPriceProvider
	tonChartProvider    TonMarketChartProvider
	fiatRateProvider    FiatRateProvider
	fiatChartProvider   FiatMarketChartProvider
	oilPriceProvider    OilPriceProvider
	oilChartProvider    OilMarketChartProvider
	bankRatesProvider   BankRatesProvider
	newsProvider        NewsProvider
	denisTrendsProvider DenisTrendsProvider
	historyProvider     HistoryProvider
	googleProvider      GoogleProvider
	motivationProvider  MotivationProvider
	snapshotStore       ReceiptSnapshotStore
	printCoordinator    PrintCoordinator
	generatedAssetsPath string
	clock               func() time.Time
}

type dailyReceiptBuild struct {
	Lines      []receipt.Line
	Warnings   []string
	NewsItems  []news.Item
	PaperChars int
	Style      receipt.StyleSettings
}

const defaultGeneratedAssetsPath = "/opt/atol-server/assets"

type BuildError struct {
	Status int
	Err    error
}

func (e BuildError) Error() string {
	return e.Err.Error()
}

func WithWeatherProvider(provider WeatherProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.weatherProvider = provider
	}
}

func WithTonPriceProvider(provider TonPriceProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.tonPriceProvider = provider
		if chartProvider, ok := provider.(TonMarketChartProvider); ok {
			s.tonChartProvider = chartProvider
		} else {
			s.tonChartProvider = nil
		}
	}
}

func WithTonMarketChartProvider(provider TonMarketChartProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.tonChartProvider = provider
	}
}

func WithGeneratedAssetsPath(path string) ReceiptServiceOption {
	return func(s *ReceiptService) {
		if path != "" {
			s.generatedAssetsPath = path
		}
	}
}

func WithFiatRateProvider(provider FiatRateProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.fiatRateProvider = provider
		if chartProvider, ok := provider.(FiatMarketChartProvider); ok {
			s.fiatChartProvider = chartProvider
		} else {
			s.fiatChartProvider = nil
		}
	}
}

func WithFiatMarketChartProvider(provider FiatMarketChartProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.fiatChartProvider = provider
	}
}

func WithOilPriceProvider(provider OilPriceProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.oilPriceProvider = provider
		if chartProvider, ok := provider.(OilMarketChartProvider); ok {
			s.oilChartProvider = chartProvider
		} else {
			s.oilChartProvider = nil
		}
	}
}

func WithOilMarketChartProvider(provider OilMarketChartProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.oilChartProvider = provider
	}
}

func WithBankRatesProvider(provider BankRatesProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.bankRatesProvider = provider
	}
}

func WithNewsProvider(provider NewsProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.newsProvider = provider
	}
}

func WithDenisTrendsProvider(provider DenisTrendsProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.denisTrendsProvider = provider
	}
}

func WithHistoryProvider(provider HistoryProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.historyProvider = provider
	}
}

func WithGoogleProvider(provider GoogleProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.googleProvider = provider
	}
}

func WithMotivationProvider(provider MotivationProvider) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.motivationProvider = provider
	}
}

func WithReceiptSnapshotStore(store ReceiptSnapshotStore) ReceiptServiceOption {
	return func(s *ReceiptService) {
		s.snapshotStore = store
	}
}

func WithPrintCoordinator(coordinator PrintCoordinator) ReceiptServiceOption {
	return func(s *ReceiptService) {
		if coordinator != nil {
			s.printCoordinator = coordinator
		}
	}
}

func NewReceiptService(store SettingsStore, printerGateway Printer, clock func() time.Time, options ...ReceiptServiceOption) *ReceiptService {
	if clock == nil {
		clock = time.Now
	}
	tonProvider := finance.NewCoinGeckoTonPriceProvider(nil)
	fiatProvider := finance.NewNbrbUsdBynRateProvider(nil)
	oilProvider := finance.NewDefaultBrentOilPriceProvider()
	service := &ReceiptService{
		store:               store,
		printer:             printerGateway,
		weatherProvider:     weather.NewOpenMeteoProvider(nil),
		tonPriceProvider:    tonProvider,
		tonChartProvider:    tonProvider,
		fiatRateProvider:    fiatProvider,
		fiatChartProvider:   fiatProvider,
		oilPriceProvider:    oilProvider,
		oilChartProvider:    oilProvider,
		bankRatesProvider:   bankrates.NewTheMoneyProvider(nil),
		newsProvider:        news.NewProvider(nil),
		denisTrendsProvider: denistrends.NewProvider(nil),
		historyProvider:     history.NewProvider(nil),
		motivationProvider:  motivation.NewOllamaProvider(nil),
		printCoordinator:    printcoord.New(),
		generatedAssetsPath: defaultGeneratedAssetsPath,
		clock:               clock,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *ReceiptService) BuildDailyReceipt(ctx context.Context) ([]receipt.Line, error) {
	lines, _, err := s.BuildDailyReceiptWithWarnings(ctx)
	return lines, err
}

func (s *ReceiptService) BuildDailyReceiptWithWarnings(ctx context.Context) ([]receipt.Line, []string, error) {
	content, err := s.store.LoadReceiptContent()
	if err != nil {
		return nil, nil, buildError(http.StatusInternalServerError, err)
	}
	return s.BuildDailyReceiptWithContentAndWarnings(ctx, content)
}

func (s *ReceiptService) BuildDailyReceiptPreviewWithWarnings(ctx context.Context) ([]receipt.Line, []string, error) {
	content, err := s.store.LoadReceiptContent()
	if err != nil {
		return nil, nil, buildError(http.StatusInternalServerError, err)
	}
	return s.BuildDailyReceiptPreviewWithContentAndWarnings(ctx, content)
}

func (s *ReceiptService) BuildDailyReceiptWithContent(ctx context.Context, content receipt.ContentSettings) ([]receipt.Line, error) {
	lines, _, err := s.BuildDailyReceiptWithContentAndWarnings(ctx, content)
	return lines, err
}

func (s *ReceiptService) BuildDailyReceiptWithContentAt(ctx context.Context, content receipt.ContentSettings, effectiveTime time.Time) ([]receipt.Line, error) {
	lines, _, err := s.BuildDailyReceiptWithContentAtAndWarnings(ctx, content, effectiveTime)
	return lines, err
}

func (s *ReceiptService) BuildDailyReceiptWithContentAndWarnings(ctx context.Context, content receipt.ContentSettings) ([]receipt.Line, []string, error) {
	return s.BuildDailyReceiptWithContentAtAndWarnings(ctx, content, s.clock())
}

func (s *ReceiptService) BuildDailyReceiptWithContentAtAndWarnings(ctx context.Context, content receipt.ContentSettings, effectiveTime time.Time) ([]receipt.Line, []string, error) {
	result, err := s.buildDailyReceiptAt(ctx, content, effectiveTime)
	if err != nil {
		return nil, nil, err
	}
	return result.Lines, result.Warnings, nil
}

func (s *ReceiptService) BuildDailyReceiptPreviewWithContentAndWarnings(ctx context.Context, content receipt.ContentSettings) ([]receipt.Line, []string, error) {
	build, err := s.buildDailyReceiptAt(ctx, content, s.clock())
	if err != nil {
		return nil, nil, err
	}
	lines, snapshotID, snapshotWarnings := s.appendNewsSnapshotQRCode(ctx, build.Lines, build.NewsItems, build.Style, build.PaperChars)
	warnings := append([]string(nil), build.Warnings...)
	warnings = append(warnings, snapshotWarnings...)
	if snapshotID != "" && s.snapshotStore != nil {
		if err := s.snapshotStore.Publish(ctx, snapshotID); err != nil {
			warnings = append(warnings, "Слепок превью создан, но не удалось подтвердить его в БД: "+err.Error())
		}
	}
	return lines, warnings, nil
}

func (s *ReceiptService) buildDailyReceipt(ctx context.Context, content receipt.ContentSettings) (dailyReceiptBuild, error) {
	return s.buildDailyReceiptAt(ctx, content, s.clock())
}

func (s *ReceiptService) buildDailyReceiptAt(ctx context.Context, content receipt.ContentSettings, effectiveTime time.Time) (dailyReceiptBuild, error) {
	if effectiveTime.IsZero() {
		effectiveTime = s.clock()
	}
	content = content.Normalized()
	unavailableSections := receipt.UnavailableSections{}

	snapshot := weather.Snapshot{
		Timezone:   motivation.DefaultTimezone,
		ObservedAt: s.clock(),
	}
	weatherAvailable := true
	var weatherWarning string
	if content.ShowWeather || content.ShowWeatherAdvice {
		location, err := s.store.LoadWeather()
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
		}

		snapshot, err = s.weatherProvider.Current(ctx, location)
		if err != nil {
			weatherAvailable = false
			weatherWarning = "погода недоступна: " + err.Error()
			if content.ShowWeather {
				unavailableSections.Weather = true
			}
		}
	}

	weatherAdvice, motivationQuote, dailyQuests, motivationWarning, err := s.resolveMotivationContent(
		ctx,
		snapshot,
		content.ShowWeatherAdvice && weatherAvailable,
		content.ShowMotivationQuote,
		content.ShowDailyQuests,
		effectiveTime,
	)
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
	}

	var tonSummary *finance.TonPortfolioSummary
	var tonChartImage *receipt.Image
	var tonPriceWarning string
	var tonChartWarning string
	if content.ShowTonPortfolio {
		portfolio, err := s.store.LoadFinance()
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
		}
		tonPrice, err := s.tonPriceProvider.CurrentPrice(ctx)
		if err != nil {
			tonPriceWarning = "TON недоступен: " + err.Error()
			unavailableSections.TonPortfolio = true
		} else {
			summary := portfolio.ValueAt(tonPrice)
			tonSummary = &summary
			tonChartImage, tonChartWarning = s.resolveTonChartImage(ctx, tonPrice)
		}
	}

	var usdBynRate *finance.FiatRate
	var usdBynChartImage *receipt.Image
	var usdBynRateWarning string
	var usdBynChartWarning string
	var oilPrice *finance.OilPrice
	var oilChartImage *receipt.Image
	var oilPriceWarning string
	var oilChartWarning string
	if content.ShowOilPrice && s.oilPriceProvider != nil {
		price, err := s.oilPriceProvider.CurrentPrice(ctx)
		if err != nil {
			oilPriceWarning = "нефть недоступна: " + err.Error()
			unavailableSections.OilPrice = true
		} else {
			oilPrice = &price
			oilChartImage, oilChartWarning = s.resolveOilChartImage(ctx)
		}
	}

	if content.ShowUsdBynRate {
		rate, err := s.fiatRateProvider.CurrentRate(ctx)
		if err != nil {
			unavailableSections.USDBYNRate = true
			usdBynRateWarning = "USD/BYN недоступен: " + err.Error()
		} else {
			usdBynRate = &rate
			usdBynChartImage, usdBynChartWarning = s.resolveUsdBynChartImage(ctx)
		}
	}

	var bankRatesSummary *bankrates.Summary
	var bankRatesWarning string
	if content.ShowBankRates {
		bankRatesSummary, bankRatesWarning = s.resolveBankRatesSummary(ctx)
		if bankRatesWarning != "" {
			unavailableSections.BankRates = true
		}
	}

	var googleSummary googleintegration.Summary
	var googleWarning string
	if content.ShowMail || content.ShowCalendar {
		googleSummary, googleWarning = s.resolveGoogleSummary(ctx, content.ShowMail, content.ShowCalendar)
		if googleWarning != "" && content.ShowCalendar {
			unavailableSections.Calendar = true
		}
	}
	if !content.ShowMail {
		googleSummary.Mail = nil
	}
	if !content.ShowCalendar {
		googleSummary.Events = nil
	}
	calendarSections := buildCalendarSections(googleSummary.Events, s.clock())
	calendarAdvice, calendarAdviceWarning, err := s.resolveCalendarAdvice(ctx, content.ShowCalendar && !unavailableSections.Calendar, calendarSections)
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
	}

	historyFacts, historyWarning, err := s.resolveHistoryFacts(ctx, content.ShowHistory)
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
	}
	if historyWarning != "" && content.ShowHistory && len(historyFacts) == 0 {
		unavailableSections.History = true
	}

	var newsItems []news.Item
	var newsTranslationWarning string
	var resolvedNewsSettings news.Settings
	var newsSettingsLoaded bool
	resolveNewsSettings := func() (news.Settings, error) {
		if content.NewsSettings != nil {
			return content.NewsSettings.Normalized(), nil
		}
		if newsSettingsLoaded {
			return resolvedNewsSettings, nil
		}
		settings, err := s.store.LoadNews()
		if err != nil {
			return news.Settings{}, err
		}
		resolvedNewsSettings = settings.Normalized()
		newsSettingsLoaded = true
		return resolvedNewsSettings, nil
	}
	if content.ShowNews {
		newsSettings, err := resolveNewsSettings()
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
		}
		newsItems, err = s.newsProvider.Current(ctx, newsSettings)
		if err != nil {
			newsTranslationWarning = "новости недоступны: " + err.Error()
			unavailableSections.News = true
		} else {
			newsItems, newsTranslationWarning, err = s.translateNewsItems(ctx, newsSettings, newsItems)
			if err != nil {
				return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
			}
		}
	}

	var denisTrendSections []denistrends.Section
	var denisTrendsTranslationWarning string
	if content.ShowDenisTrends {
		trendsSettings, err := s.resolveDenisTrendsSettings(content)
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
		}
		denisTrendSections, err = s.denisTrendsProvider.Current(ctx, trendsSettings, effectiveTime)
		if err != nil {
			denisTrendsTranslationWarning = "Denis Trends недоступны: " + err.Error()
			unavailableSections.DenisTrends = true
		} else {
			newsSettings, err := resolveNewsSettings()
			if err != nil {
				return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
			}
			denisTrendSections, denisTrendsTranslationWarning = s.translateDenisTrendSections(ctx, newsSettings, denisTrendSections)
		}
	}

	receiptStyle, err := s.store.LoadReceiptStyle()
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
	}

	lines := receipt.DailyReceiptWithStyle(receipt.DailyReceiptData{
		HideWeather:         !content.ShowWeather,
		UnavailableSections: unavailableSections,
		Weather:             snapshot,
		WeatherAdvice:       weatherAdvice,
		MotivationQuote:     motivationQuote,
		DailyQuests:         dailyQuests,
		TonPortfolio:        tonSummary,
		TonChartImage:       tonChartImage,
		OilPrice:            oilPrice,
		OilChartImage:       oilChartImage,
		USDBYNRate:          usdBynRate,
		USDBYNChartImage:    usdBynChartImage,
		BankRates:           bankRatesSummary,
		MailMessages:        googleSummary.Mail,
		CalendarSections:    calendarSections,
		CalendarAdvice:      calendarAdvice,
		HistoryFacts:        historyFacts,
		NewsItems:           newsItems,
		DenisTrendSections:  denisTrendSections,
	}, receiptStyle)
	warnings := optionalWarnings(weatherWarning, motivationWarning, tonPriceWarning, tonChartWarning, oilPriceWarning, oilChartWarning, usdBynRateWarning, usdBynChartWarning, bankRatesWarning, googleWarning, calendarAdviceWarning, historyWarning, newsTranslationWarning, denisTrendsTranslationWarning)
	return dailyReceiptBuild{
		Lines:      lines,
		Warnings:   warnings,
		NewsItems:  newsItems,
		PaperChars: receiptSnapshotPaperChars(receiptStyle),
		Style:      receiptStyle.Normalized(),
	}, nil
}

func (s *ReceiptService) resolveDenisTrendsSettings(content receipt.ContentSettings) (denistrends.Settings, error) {
	if content.DenisTrendsSettings != nil {
		return content.DenisTrendsSettings.Normalized(), nil
	}
	settings, err := s.store.LoadDenisTrends()
	if err != nil {
		return denistrends.Settings{}, err
	}
	return settings.Normalized(), nil
}

func (s *ReceiptService) PrintDailyReceipt(ctx context.Context) error {
	_, err := s.PrintDailyReceiptWithWarnings(ctx)
	return err
}

func (s *ReceiptService) PrintDailyReceiptAt(ctx context.Context, effectiveTime time.Time) error {
	content, err := s.store.LoadReceiptContent()
	if err != nil {
		return buildError(http.StatusInternalServerError, err)
	}
	return s.PrintDailyReceiptWithContentAt(ctx, content, effectiveTime)
}

func (s *ReceiptService) PrintDailyReceiptWithWarnings(ctx context.Context) ([]string, error) {
	content, err := s.store.LoadReceiptContent()
	if err != nil {
		return nil, buildError(http.StatusInternalServerError, err)
	}
	return s.PrintDailyReceiptWithContentAndWarnings(ctx, content)
}

func (s *ReceiptService) PrintDailyReceiptWithContent(ctx context.Context, content receipt.ContentSettings) error {
	_, err := s.PrintDailyReceiptWithContentAndWarnings(ctx, content)
	return err
}

func (s *ReceiptService) PrintDailyReceiptWithContentAt(ctx context.Context, content receipt.ContentSettings, effectiveTime time.Time) error {
	_, err := s.PrintDailyReceiptWithContentAtAndWarnings(ctx, content, effectiveTime)
	return err
}

func (s *ReceiptService) PrintDailyReceiptWithContentAndWarnings(ctx context.Context, content receipt.ContentSettings) ([]string, error) {
	return s.PrintDailyReceiptWithContentAtAndWarnings(ctx, content, s.clock())
}

func (s *ReceiptService) PrintDailyReceiptWithContentAtAndWarnings(ctx context.Context, content receipt.ContentSettings, effectiveTime time.Time) ([]string, error) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		return nil, buildError(http.StatusInternalServerError, err)
	}

	build, err := s.buildDailyReceiptAt(ctx, content, effectiveTime)
	if err != nil {
		return nil, err
	}
	lines, snapshotID, snapshotWarnings := s.appendNewsSnapshotQRCode(ctx, build.Lines, build.NewsItems, build.Style, build.PaperChars)
	warnings := append([]string(nil), build.Warnings...)
	warnings = append(warnings, snapshotWarnings...)

	if err := s.printCoordinator.RunUserPrint(ctx, func(ctx context.Context) error {
		return s.printer.PrintReceipt(ctx, config, lines)
	}); err != nil {
		if snapshotID != "" && s.snapshotStore != nil {
			_ = s.snapshotStore.Fail(ctx, snapshotID, err)
		}
		return warnings, buildError(http.StatusBadGateway, err)
	}
	if snapshotID != "" && s.snapshotStore != nil {
		if err := s.snapshotStore.Publish(ctx, snapshotID); err != nil {
			warnings = append(warnings, "Слепок создан, но печать не удалось подтвердить в БД: "+err.Error())
		}
	}
	return warnings, nil
}
