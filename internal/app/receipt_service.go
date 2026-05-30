package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"atol-server/internal/bankrates"
	"atol-server/internal/chart"
	"atol-server/internal/denistrends"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/history"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
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
	bankRatesProvider   BankRatesProvider
	newsProvider        NewsProvider
	denisTrendsProvider DenisTrendsProvider
	historyProvider     HistoryProvider
	googleProvider      GoogleProvider
	motivationProvider  MotivationProvider
	snapshotStore       ReceiptSnapshotStore
	generatedAssetsPath string
	clock               func() time.Time
	printMu             sync.Mutex
}

type dailyReceiptBuild struct {
	Lines      []receipt.Line
	Warnings   []string
	NewsItems  []news.Item
	PaperChars int
	Style      receipt.StyleSettings
}

const defaultGeneratedAssetsPath = "/opt/atol-server/assets"
const defaultReceiptSnapshotBaseURL = "http://localhost:8080"

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

func NewReceiptService(store SettingsStore, printerGateway Printer, clock func() time.Time, options ...ReceiptServiceOption) *ReceiptService {
	if clock == nil {
		clock = time.Now
	}
	tonProvider := finance.NewCoinGeckoTonPriceProvider(nil)
	fiatProvider := finance.NewNbrbUsdBynRateProvider(nil)
	service := &ReceiptService{
		store:               store,
		printer:             printerGateway,
		weatherProvider:     weather.NewOpenMeteoProvider(nil),
		tonPriceProvider:    tonProvider,
		tonChartProvider:    tonProvider,
		fiatRateProvider:    fiatProvider,
		fiatChartProvider:   fiatProvider,
		bankRatesProvider:   bankrates.NewTheMoneyProvider(nil),
		newsProvider:        news.NewProvider(nil),
		denisTrendsProvider: denistrends.NewProvider(nil),
		historyProvider:     history.NewProvider(nil),
		motivationProvider:  motivation.NewOllamaProvider(nil),
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

	snapshot := weather.Snapshot{
		Timezone:   motivation.DefaultTimezone,
		ObservedAt: s.clock(),
	}
	if content.ShowWeather || content.ShowWeatherAdvice {
		location, err := s.store.LoadWeather()
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
		}

		snapshot, err = s.weatherProvider.Current(ctx, location)
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusBadGateway, err)
		}
	}

	weatherAdvice, motivationQuote, motivationWarning, err := s.resolveMotivationContent(
		ctx,
		snapshot,
		content.ShowWeatherAdvice,
		content.ShowMotivationQuote,
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
		} else {
			summary := portfolio.ValueAt(tonPrice)
			tonSummary = &summary
			tonChartImage, tonChartWarning = s.resolveTonChartImage(ctx, tonPrice)
		}
	}

	var usdBynRate *finance.FiatRate
	var usdBynChartImage *receipt.Image
	var usdBynChartWarning string
	if content.ShowUsdBynRate {
		rate, err := s.fiatRateProvider.CurrentRate(ctx)
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusBadGateway, err)
		}
		usdBynRate = &rate
		usdBynChartImage, usdBynChartWarning = s.resolveUsdBynChartImage(ctx)
	}

	var bankRatesSummary *bankrates.Summary
	var bankRatesWarning string
	if content.ShowBankRates {
		bankRatesSummary, bankRatesWarning = s.resolveBankRatesSummary(ctx)
	}

	var googleSummary googleintegration.Summary
	var googleWarning string
	if content.ShowMail || content.ShowCalendar {
		googleSummary, googleWarning = s.resolveGoogleSummary(ctx, content.ShowMail, content.ShowCalendar)
	}
	if !content.ShowMail {
		googleSummary.Mail = nil
	}
	if !content.ShowCalendar {
		googleSummary.Events = nil
	}
	calendarSections := buildCalendarSections(googleSummary.Events, s.clock())
	calendarAdvice, calendarAdviceWarning, err := s.resolveCalendarAdvice(ctx, content.ShowCalendar, calendarSections)
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
	}

	historyFacts, historyWarning, err := s.resolveHistoryFacts(ctx, content.ShowHistory)
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
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
			return dailyReceiptBuild{}, buildError(http.StatusBadGateway, err)
		}
		newsItems, newsTranslationWarning, err = s.translateNewsItems(ctx, newsSettings, newsItems)
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
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
			return dailyReceiptBuild{}, buildError(http.StatusBadGateway, err)
		}
		newsSettings, err := resolveNewsSettings()
		if err != nil {
			return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
		}
		denisTrendSections, denisTrendsTranslationWarning = s.translateDenisTrendSections(ctx, newsSettings, denisTrendSections)
	}

	receiptStyle, err := s.store.LoadReceiptStyle()
	if err != nil {
		return dailyReceiptBuild{}, buildError(http.StatusInternalServerError, err)
	}

	lines := receipt.DailyReceiptWithStyle(receipt.DailyReceiptData{
		HideWeather:        !content.ShowWeather,
		Weather:            snapshot,
		WeatherAdvice:      weatherAdvice,
		MotivationQuote:    motivationQuote,
		TonPortfolio:       tonSummary,
		TonChartImage:      tonChartImage,
		USDBYNRate:         usdBynRate,
		USDBYNChartImage:   usdBynChartImage,
		BankRates:          bankRatesSummary,
		MailMessages:       googleSummary.Mail,
		CalendarSections:   calendarSections,
		CalendarAdvice:     calendarAdvice,
		HistoryFacts:       historyFacts,
		NewsItems:          newsItems,
		DenisTrendSections: denisTrendSections,
	}, receiptStyle)
	warnings := optionalWarnings(motivationWarning, tonPriceWarning, tonChartWarning, usdBynChartWarning, bankRatesWarning, googleWarning, calendarAdviceWarning, historyWarning, newsTranslationWarning, denisTrendsTranslationWarning)
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

	s.printMu.Lock()
	defer s.printMu.Unlock()

	if err := s.printer.PrintReceipt(ctx, config, lines); err != nil {
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

func (s *ReceiptService) appendNewsSnapshotQRCode(ctx context.Context, lines []receipt.Line, items []news.Item, style receipt.StyleSettings, paperChars int) ([]receipt.Line, string, []string) {
	if (len(items) == 0 && !receiptLinesHaveLinks(lines)) || s.snapshotStore == nil {
		return lines, "", nil
	}
	settings, err := s.store.LoadReceiptSnapshotSettings()
	if err != nil {
		return lines, "", []string{"QR-ссылка на онлайн-слепок недоступна: " + err.Error()}
	}
	settings = settings.Normalized()
	if settings.BaseURL == "" {
		settings.BaseURL = defaultReceiptSnapshotBaseURL
	}
	if err := settings.Validate(); err != nil {
		return lines, "", []string{"QR-ссылка на онлайн-слепок некорректна: " + err.Error()}
	}
	snapshot, err := s.snapshotStore.Create(ctx, receiptsnapshot.CreateInput{
		NewsItems:    newsSnapshotItems(items),
		ReceiptLines: receiptSnapshotLines(lines, style),
		PaperChars:   paperChars,
	})
	if err != nil {
		return lines, "", []string{"Онлайн-слепок новостей не создан: " + err.Error()}
	}
	snapshotURL, ok := settings.SnapshotURL(snapshot.ID)
	if !ok {
		return lines, "", []string{"QR-ссылка на онлайн-слепок не настроена."}
	}
	finalLines := append([]receipt.Line(nil), lines...)
	finalLines = append(finalLines, qrCodeSectionSpacing(paperChars)...)
	finalLines = append(finalLines, receipt.Line{
		QRCode:            snapshotURL,
		Alignment:         receipt.AlignmentCenter,
		ImageScalePercent: 100,
	})
	if err := s.snapshotStore.FinalizeReceiptLines(ctx, snapshot.ID, receiptSnapshotLines(finalLines, style), paperChars); err != nil {
		return lines, "", []string{"Онлайн-слепок создан, но финальный чек не сохранен: " + err.Error()}
	}
	return finalLines, snapshot.ID, nil
}

func qrCodeSectionSpacing(paperChars int) []receipt.Line {
	separatorLength := paperChars / 2
	if separatorLength <= 0 {
		separatorLength = 16
	}
	return []receipt.Line{
		{Text: " ", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: strings.Repeat("-", separatorLength), Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
		{Text: " ", Alignment: receipt.AlignmentCenter, Role: receipt.RoleNormal},
	}
}

func receiptLinesHaveLinks(lines []receipt.Line) bool {
	for _, line := range lines {
		if strings.TrimSpace(line.Link) != "" {
			return true
		}
	}
	return false
}

func newsSnapshotItems(items []news.Item) []receiptsnapshot.NewsItem {
	result := make([]receiptsnapshot.NewsItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		result = append(result, receiptsnapshot.NewsItem{
			Title:         strings.TrimSpace(item.Title),
			OriginalTitle: strings.TrimSpace(item.OriginalTitle),
			SourceName:    strings.TrimSpace(item.SourceName),
			Link:          strings.TrimSpace(item.Link),
		})
	}
	return result
}

func receiptSnapshotLines(lines []receipt.Line, style receipt.StyleSettings) []receiptsnapshot.ReceiptLine {
	style = style.Normalized()
	result := make([]receiptsnapshot.ReceiptLine, 0, len(lines))
	for _, line := range lines {
		result = append(result, receiptsnapshot.ReceiptLine{
			Text:              line.Text,
			Link:              line.Link,
			QRCode:            line.QRCode,
			ImageKey:          line.ImageKey,
			ImageURL:          line.ImageURL,
			ImageWidth:        line.ImageWidth,
			ImageHeight:       line.ImageHeight,
			ImageScalePercent: line.ImageScalePercent,
			Alignment:         string(line.Alignment),
			Role:              string(line.Role),
			Font:              line.Font,
			DoubleWidth:       line.DoubleWidth,
			DoubleHeight:      line.DoubleHeight,
			LineSize:          receiptSnapshotLineSize(line, style),
		})
	}
	return result
}

type receiptSnapshotFontMetric struct {
	lineLength int
	fontWidth  int
}

var receiptSnapshotFontMetrics = map[int]receiptSnapshotFontMetric{
	0: {lineLength: 32, fontWidth: 12},
	1: {lineLength: 42, fontWidth: 9},
	2: {lineLength: 38, fontWidth: 10},
	3: {lineLength: 32, fontWidth: 12},
	4: {lineLength: 32, fontWidth: 12},
	5: {lineLength: 32, fontWidth: 12},
	6: {lineLength: 32, fontWidth: 12},
	7: {lineLength: 32, fontWidth: 12},
	8: {lineLength: 32, fontWidth: 12},
	9: {lineLength: 32, fontWidth: 12},
}

func receiptSnapshotPaperChars(style receipt.StyleSettings) int {
	metric := receiptSnapshotMetricForFont(style.Normalized().NormalFont)
	if metric.lineLength <= 0 {
		return 32
	}
	return metric.lineLength
}

func receiptSnapshotLineSize(line receipt.Line, style receipt.StyleSettings) float64 {
	normalMetric := receiptSnapshotMetricForFont(style.Normalized().NormalFont)
	metric := receiptSnapshotMetricForFont(line.Font)
	baseWidth := normalMetric.fontWidth
	if baseWidth <= 0 {
		baseWidth = 12
	}
	fontWidth := metric.fontWidth
	if fontWidth <= 0 {
		fontWidth = baseWidth
	}
	lineSize := 16 * (float64(fontWidth) / float64(baseWidth))
	if lineSize < 10 {
		return 10
	}
	if lineSize > 32 {
		return 32
	}
	return lineSize
}

func receiptSnapshotMetricForFont(font int) receiptSnapshotFontMetric {
	if metric, ok := receiptSnapshotFontMetrics[font]; ok {
		return metric
	}
	return receiptSnapshotFontMetrics[0]
}

func (s *ReceiptService) resolveMotivationContent(ctx context.Context, snapshot weather.Snapshot, includeAdvice bool, includeQuote bool) (*motivation.WeatherAdvice, *motivation.Quote, string, error) {
	if !includeAdvice && !includeQuote {
		return nil, nil, "", nil
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		return nil, nil, "", err
	}
	settings = settings.Normalized()

	var weatherAdvice *motivation.WeatherAdvice
	var warnings []string
	if includeAdvice {
		advice, adviceErr := s.motivationProvider.GenerateWeatherAdvice(ctx, settings, weatherContextFromSnapshot(snapshot))
		if adviceErr != nil {
			warnings = append(warnings, "AI-совет по погоде недоступен: "+adviceErr.Error())
		} else if advice.Text != "" {
			weatherAdvice = &advice
		}
	}

	updated := settings
	var quote *motivation.Quote
	if includeQuote {
		var err error
		updated, quote, err = motivation.ResolveDailyQuote(ctx, settings, s.clock(), s.motivationProvider)
		if err != nil {
			warnings = append(warnings, "AI-цитата недоступна: "+err.Error())
		}
	}
	if len(warnings) > 0 {
		updated.LastError = strings.Join(warnings, "; ")
	} else {
		updated.LastError = ""
	}
	if saveErr := s.store.SaveMotivation(updated); saveErr != nil {
		return nil, nil, "", saveErr
	}
	return weatherAdvice, quote, strings.Join(warnings, "\n"), nil
}

func (s *ReceiptService) resolveGoogleSummary(ctx context.Context, includeMail bool, includeCalendar bool) (googleintegration.Summary, string) {
	if s.googleProvider == nil || (!includeMail && !includeCalendar) {
		return googleintegration.Summary{}, ""
	}
	var (
		summary googleintegration.Summary
		err     error
	)
	if provider, ok := s.googleProvider.(SelectiveGoogleProvider); ok {
		summary, err = provider.CurrentSelected(ctx, includeMail, includeCalendar)
	} else {
		summary, err = s.googleProvider.Current(ctx)
	}
	if err != nil {
		return googleintegration.Summary{}, "Google недоступен: " + err.Error()
	}
	return summary, ""
}

func (s *ReceiptService) resolveCalendarAdvice(ctx context.Context, includeCalendar bool, sections []receipt.CalendarSection) (*motivation.CalendarAdvice, string, error) {
	if !includeCalendar || len(sections) == 0 || s.motivationProvider == nil {
		return nil, "", nil
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		return nil, "", err
	}
	advice, err := s.motivationProvider.GenerateCalendarAdvice(ctx, settings.Normalized(), calendarContextFromSections(s.clock(), sections))
	if err != nil {
		return nil, "AI-совет по календарю недоступен: " + err.Error(), nil
	}
	if strings.TrimSpace(advice.Text) == "" {
		return nil, "", nil
	}
	return &advice, "", nil
}

func (s *ReceiptService) resolveHistoryFacts(ctx context.Context, includeHistory bool) ([]motivation.HistoryFact, string, error) {
	if !includeHistory || s.historyProvider == nil || s.motivationProvider == nil {
		return nil, "", nil
	}
	events, err := s.historyProvider.Current(ctx, s.clock().In(minskLocation()))
	if err != nil {
		return nil, "История дня недоступна: " + err.Error(), nil
	}
	if len(events) == 0 {
		return nil, "", nil
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		return nil, "", err
	}
	facts, err := s.motivationProvider.GenerateHistoryFacts(ctx, settings.Normalized(), historyEventsForAI(events))
	if err != nil {
		return nil, "AI-история дня недоступна: " + err.Error(), nil
	}
	if len(facts) == 0 {
		return nil, "AI-история дня недоступна: пустой ответ", nil
	}
	return facts, "", nil
}

func historyEventsForAI(events []history.Event) []motivation.HistoryEvent {
	result := make([]motivation.HistoryEvent, 0, len(events))
	for _, event := range events {
		text := strings.TrimSpace(event.Text)
		if text == "" {
			continue
		}
		result = append(result, motivation.HistoryEvent{
			Year: event.Year,
			Text: text,
			Link: strings.TrimSpace(event.Link),
		})
	}
	return result
}

func buildCalendarSections(events []googleintegration.CalendarEvent, now time.Time) []receipt.CalendarSection {
	if len(events) == 0 {
		return nil
	}
	location := minskLocation()
	now = now.In(location)
	today := dayStart(now, location)
	tomorrow := today.AddDate(0, 0, 1)

	if calendarTomorrowMode(now, today) {
		return nonEmptyCalendarSections([]receipt.CalendarSection{
			{
				Title:  "Остаток сегодня",
				Date:   today,
				Events: calendarEventsForDay(events, today, now, true, true),
			},
			{
				Title:  "Завтра",
				Date:   tomorrow,
				Events: calendarEventsForDay(events, tomorrow, now, false, false),
			},
		})
	}

	return nonEmptyCalendarSections([]receipt.CalendarSection{{
		Title:  "Сегодня",
		Date:   today,
		Events: calendarEventsForDay(events, today, now, false, true),
	}})
}

func nonEmptyCalendarSections(sections []receipt.CalendarSection) []receipt.CalendarSection {
	result := make([]receipt.CalendarSection, 0, len(sections))
	for _, section := range sections {
		if len(section.Events) == 0 {
			continue
		}
		result = append(result, section)
	}
	return result
}

func calendarEventsForDay(events []googleintegration.CalendarEvent, day time.Time, now time.Time, onlyRemaining bool, includeUnknownDay bool) []googleintegration.CalendarEvent {
	result := make([]googleintegration.CalendarEvent, 0, len(events))
	for _, event := range events {
		if !calendarEventBelongsToDay(event, day, includeUnknownDay) {
			continue
		}
		if onlyRemaining && !calendarEventRemaining(event, now) {
			continue
		}
		result = append(result, event)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := calendarEventSortTime(result[i], day.Location())
		right := calendarEventSortTime(result[j], day.Location())
		if left.Equal(right) {
			return result[i].Title < result[j].Title
		}
		return left.Before(right)
	})
	return result
}

func calendarEventBelongsToDay(event googleintegration.CalendarEvent, day time.Time, includeUnknownDay bool) bool {
	if event.Start.IsZero() {
		return includeUnknownDay
	}
	start := event.Start.In(day.Location())
	end := calendarEventEnd(event, day.Location())
	if end.IsZero() {
		end = start
	}
	if !end.After(start) {
		end = start.Add(time.Nanosecond)
	}
	return start.Before(day.AddDate(0, 0, 1)) && end.After(day)
}

func calendarEventRemaining(event googleintegration.CalendarEvent, now time.Time) bool {
	if event.Start.IsZero() {
		return true
	}
	end := calendarEventEnd(event, now.Location())
	if end.IsZero() {
		return !event.Start.In(now.Location()).Before(now)
	}
	return end.After(now)
}

func calendarEventSortTime(event googleintegration.CalendarEvent, location *time.Location) time.Time {
	if event.Start.IsZero() {
		return time.Time{}
	}
	return event.Start.In(location)
}

func calendarEventEnd(event googleintegration.CalendarEvent, location *time.Location) time.Time {
	if !event.End.IsZero() {
		return event.End.In(location)
	}
	if event.Start.IsZero() {
		return time.Time{}
	}
	start := event.Start.In(location)
	if event.AllDay {
		return dayStart(start, location).AddDate(0, 0, 1)
	}
	return start
}

func calendarContextFromSections(now time.Time, sections []receipt.CalendarSection) motivation.CalendarContext {
	context := motivation.CalendarContext{
		GeneratedAt: now.In(minskLocation()),
		Mode:        "morning",
		Sections:    make([]motivation.CalendarSectionContext, 0, len(sections)),
	}
	for _, section := range sections {
		if section.Title == "Остаток сегодня" || section.Title == "Завтра" {
			context.Mode = "evening"
		}
		sectionContext := motivation.CalendarSectionContext{
			Title:  section.Title,
			Date:   section.Date,
			Events: make([]motivation.CalendarEventContext, 0, len(section.Events)),
		}
		for _, event := range section.Events {
			sectionContext.Events = append(sectionContext.Events, motivation.CalendarEventContext{
				TimeLabel: event.TimeLabel,
				Title:     event.Title,
				Start:     event.Start,
				End:       event.End,
				AllDay:    event.AllDay,
			})
		}
		context.Sections = append(context.Sections, sectionContext)
	}
	return context
}

func minskLocation() *time.Location {
	location, err := time.LoadLocation(motivation.DefaultTimezone)
	if err != nil {
		return time.Local
	}
	return location
}

func dayStart(value time.Time, location *time.Location) time.Time {
	if location == nil {
		location = value.Location()
	}
	value = value.In(location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}

func calendarTomorrowMode(now time.Time, today time.Time) bool {
	return !now.Before(today.Add(15 * time.Hour))
}

func (s *ReceiptService) resolveTonChartImage(ctx context.Context, price finance.TonPrice) (*receipt.Image, string) {
	if s.tonChartProvider == nil {
		return nil, ""
	}
	chartData, err := s.tonChartProvider.MarketChart(ctx)
	if err != nil {
		image, fallbackErr := s.renderTonChartImage(fallbackTonMarketChart(price, s.clock()))
		if fallbackErr != nil {
			return nil, "график TON недоступен: " + err.Error()
		}
		return image, "график TON построен по запасным данным: " + err.Error()
	}

	image, err := s.renderTonChartImage(chartData)
	if err != nil {
		return nil, "график TON недоступен: " + err.Error()
	}
	return image, ""
}

func (s *ReceiptService) renderTonChartImage(chartData finance.TonMarketChart) (*receipt.Image, error) {
	path := filepath.Join(s.generatedAssetsPathOrDefault(), "generated", "ton-24h.png")
	chartImage, err := chart.RenderTonPriceChartPixelBuffer(chartData, chart.Options{Width: 384, Height: 96})
	if err != nil {
		return nil, err
	}
	if err := chart.SaveMonoPNG(path, chartImage); err != nil {
		return nil, err
	}

	return &receipt.Image{
		Path:        path,
		URL:         fmt.Sprintf("/assets/generated/ton-24h.png?v=%d", s.clock().UnixNano()),
		Width:       chartImage.Width,
		Height:      chartImage.Height,
		PixelBuffer: chartImage.Pixels,
	}, nil
}

func fallbackTonMarketChart(price finance.TonPrice, now time.Time) finance.TonMarketChart {
	currentPrice := price.USD
	previousPrice := currentPrice
	if price.USD24hChangePercent != nil {
		denominator := 1 + (*price.USD24hChangePercent / 100)
		if denominator > 0.001 {
			previousPrice = currentPrice / denominator
		}
	}
	if now.IsZero() {
		now = time.Now()
	}
	return finance.TonMarketChart{Points: []finance.TonPricePoint{
		{Time: now.Add(-24 * time.Hour), USD: previousPrice},
		{Time: now, USD: currentPrice},
	}}
}

func (s *ReceiptService) resolveUsdBynChartImage(ctx context.Context) (*receipt.Image, string) {
	if s.fiatChartProvider == nil {
		return nil, ""
	}
	chartData, err := s.fiatChartProvider.MarketChart(ctx, s.clock())
	if err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}

	path := filepath.Join(s.generatedAssetsPathOrDefault(), "generated", "usd-byn-7d.png")
	chartImage, err := chart.RenderFiatRateChartPixelBuffer(chartData, chart.Options{Width: 384, Height: 96})
	if err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}
	if err := chart.SaveMonoPNG(path, chartImage); err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}

	return &receipt.Image{
		Path:        path,
		URL:         fmt.Sprintf("/assets/generated/usd-byn-7d.png?v=%d", s.clock().UnixNano()),
		Width:       chartImage.Width,
		Height:      chartImage.Height,
		PixelBuffer: chartImage.Pixels,
	}, ""
}

func (s *ReceiptService) resolveBankRatesSummary(ctx context.Context) (*bankrates.Summary, string) {
	if s.bankRatesProvider == nil {
		return nil, ""
	}
	summary, err := s.bankRatesProvider.Current(ctx)
	if err != nil {
		return nil, "банковские курсы недоступны: " + err.Error()
	}
	return &summary, ""
}

func (s *ReceiptService) generatedAssetsPathOrDefault() string {
	if s.generatedAssetsPath != "" {
		return s.generatedAssetsPath
	}
	return defaultGeneratedAssetsPath
}

func (s *ReceiptService) translateNewsItems(ctx context.Context, newsSettings news.Settings, items []news.Item) ([]news.Item, string, error) {
	if !newsSettings.TranslateTitlesEnabled() || len(items) == 0 {
		return items, "", nil
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		return nil, "", err
	}
	settings = settings.Normalized()

	titles := make([]motivation.NewsTitle, 0, len(items))
	for index, item := range items {
		if shouldTranslateNewsItem(item) {
			titles = append(titles, motivation.NewsTitle{
				Index:      index,
				SourceName: item.SourceName,
				Title:      item.Title,
			})
		}
	}
	if len(titles) == 0 {
		return items, "", nil
	}

	translations, err := s.motivationProvider.TranslateNewsTitles(ctx, settings, titles)
	if err != nil {
		return items, "AI-перевод новостей недоступен: " + err.Error(), nil
	}

	translationByIndex := make(map[int]string, len(translations))
	for _, translation := range translations {
		translationByIndex[translation.Index] = strings.TrimSpace(translation.Title)
	}

	translatedItems := append([]news.Item(nil), items...)
	for _, title := range titles {
		translation := strings.TrimSpace(translationByIndex[title.Index])
		if translation == "" || strings.EqualFold(translation, strings.TrimSpace(title.Title)) {
			continue
		}
		translatedItems[title.Index].OriginalTitle = strings.TrimSpace(title.Title)
		translatedItems[title.Index].Title = translation
	}
	return translatedItems, "", nil
}

func shouldTranslateNewsItem(item news.Item) bool {
	title := strings.TrimSpace(item.Title)
	if title == "" || containsCyrillic(title) {
		return false
	}
	sourceName := strings.ToLower(strings.TrimSpace(item.SourceName))
	return strings.Contains(sourceName, "reuters") ||
		strings.Contains(sourceName, "economist") ||
		strings.Contains(sourceName, "hacker news")
}

func (s *ReceiptService) translateDenisTrendSections(ctx context.Context, newsSettings news.Settings, sections []denistrends.Section) ([]denistrends.Section, string) {
	if !newsSettings.TranslateTitlesEnabled() || len(sections) == 0 {
		return sections, ""
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		return sections, "AI-перевод Denis Trends недоступен: " + err.Error()
	}
	settings = settings.Normalized()

	type trendTitleRef struct {
		sectionIndex int
		itemIndex    int
	}
	refs := make(map[int]trendTitleRef)
	titles := make([]motivation.NewsTitle, 0)
	for sectionIndex, section := range sections {
		for itemIndex, item := range section.Items {
			title := strings.TrimSpace(item.Title)
			if title == "" || containsCyrillic(title) {
				continue
			}
			translationIndex := len(titles)
			refs[translationIndex] = trendTitleRef{sectionIndex: sectionIndex, itemIndex: itemIndex}
			titles = append(titles, motivation.NewsTitle{
				Index:      translationIndex,
				SourceName: strings.TrimSpace(item.SourceName),
				Title:      title,
			})
		}
	}
	if len(titles) == 0 {
		return sections, ""
	}

	translations, err := s.motivationProvider.TranslateNewsTitles(ctx, settings, titles)
	if err != nil {
		return sections, "AI-перевод Denis Trends недоступен: " + err.Error()
	}
	translationByIndex := make(map[int]string, len(translations))
	for _, translation := range translations {
		translationByIndex[translation.Index] = strings.TrimSpace(translation.Title)
	}

	translatedSections := append([]denistrends.Section(nil), sections...)
	for sectionIndex := range translatedSections {
		translatedSections[sectionIndex].Items = append([]denistrends.Item(nil), sections[sectionIndex].Items...)
	}
	for _, title := range titles {
		translation := strings.TrimSpace(translationByIndex[title.Index])
		if translation == "" || strings.EqualFold(translation, strings.TrimSpace(title.Title)) {
			continue
		}
		ref := refs[title.Index]
		item := &translatedSections[ref.sectionIndex].Items[ref.itemIndex]
		item.OriginalTitle = strings.TrimSpace(title.Title)
		item.Title = translation
	}
	return translatedSections, ""
}

func containsCyrillic(value string) bool {
	for _, current := range value {
		if unicode.Is(unicode.Cyrillic, current) {
			return true
		}
	}
	return false
}

func weatherContextFromSnapshot(snapshot weather.Snapshot) motivation.WeatherContext {
	return motivation.WeatherContext{
		LocationName:                   snapshot.LocationName,
		ObservedAt:                     snapshot.ObservedAt.In(snapshot.TimeLocation()),
		Condition:                      weather.ConditionLabel(snapshot),
		TemperatureC:                   snapshot.TemperatureC,
		ApparentTemperatureC:           snapshot.ApparentTemperatureC,
		RelativeHumidityPct:            snapshot.RelativeHumidityPct,
		PrecipitationMm:                snapshot.PrecipitationMm,
		WindSpeedMs:                    snapshot.WindSpeedMs,
		WindGustsMs:                    snapshot.WindGustsMs,
		WindDirectionDeg:               snapshot.WindDirectionDeg,
		UVIndex:                        snapshot.UVIndex,
		UVIndexMax:                     snapshot.UVIndexMax,
		PrecipitationProbabilityMaxPct: snapshot.PrecipitationProbabilityMaxPct,
		VisibilityM:                    snapshot.VisibilityM,
		DewPointC:                      snapshot.DewPointC,
		SurfacePressureHpa:             snapshot.SurfacePressureHpa,
		DayTemperatureC:                snapshot.DayTemperatureC,
		NightTemperatureC:              snapshot.NightTemperatureC,
		Forecast:                       weatherForecastContext(snapshot.Forecast),
	}
}

func weatherForecastContext(points []weather.ForecastPoint) []motivation.WeatherForecastPoint {
	if len(points) == 0 {
		return nil
	}
	result := make([]motivation.WeatherForecastPoint, 0, len(points))
	for _, point := range points {
		result = append(result, motivation.WeatherForecastPoint{
			ObservedAt:                  point.ObservedAt,
			TemperatureC:                point.TemperatureC,
			ApparentTemperatureC:        point.ApparentTemperatureC,
			PrecipitationProbabilityPct: point.PrecipitationProbabilityPct,
			PrecipitationMm:             point.PrecipitationMm,
			WindSpeedMs:                 point.WindSpeedMs,
			WindGustsMs:                 point.WindGustsMs,
			WeatherCode:                 point.WeatherCode,
		})
	}
	return result
}

func optionalWarnings(values ...string) []string {
	var warnings []string
	for _, value := range values {
		if value != "" {
			warnings = append(warnings, value)
		}
	}
	return warnings
}

func buildError(status int, err error) error {
	return BuildError{Status: status, Err: err}
}
