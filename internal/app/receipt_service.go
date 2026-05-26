package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"atol-server/internal/bankrates"
	"atol-server/internal/chart"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/weather"
)

type SettingsStore interface {
	LoadPrinter() (printer.Config, error)
	LoadWeather() (weather.Location, error)
	LoadFinance() (finance.TonPortfolio, error)
	LoadNews() (news.Settings, error)
	LoadMotivation() (motivation.Settings, error)
	SaveMotivation(motivation.Settings) error
	LoadReceiptStyle() (receipt.StyleSettings, error)
	LoadReceiptContent() (receipt.ContentSettings, error)
}

type Printer interface {
	PrintReceipt(context.Context, printer.Config, []receipt.Line) error
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

type GoogleProvider interface {
	Current(context.Context) (googleintegration.Summary, error)
}

type SelectiveGoogleProvider interface {
	CurrentSelected(context.Context, bool, bool) (googleintegration.Summary, error)
}

type MotivationProvider interface {
	Generate(context.Context, motivation.Settings) (motivation.Quote, error)
	GenerateWeatherAdvice(context.Context, motivation.Settings, motivation.WeatherContext) (motivation.WeatherAdvice, error)
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
	googleProvider      GoogleProvider
	motivationProvider  MotivationProvider
	generatedAssetsPath string
	clock               func() time.Time
	printMu             sync.Mutex
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
	content = content.Normalized()

	snapshot := weather.Snapshot{
		Timezone:   motivation.DefaultTimezone,
		ObservedAt: s.clock(),
	}
	if content.ShowWeather || content.ShowWeatherAdvice {
		location, err := s.store.LoadWeather()
		if err != nil {
			return nil, nil, buildError(http.StatusInternalServerError, err)
		}

		snapshot, err = s.weatherProvider.Current(ctx, location)
		if err != nil {
			return nil, nil, buildError(http.StatusBadGateway, err)
		}
	}

	weatherAdvice, motivationQuote, motivationWarning, err := s.resolveMotivationContent(
		ctx,
		snapshot,
		content.ShowWeatherAdvice,
		content.ShowMotivationQuote,
	)
	if err != nil {
		return nil, nil, buildError(http.StatusInternalServerError, err)
	}

	var tonSummary *finance.TonPortfolioSummary
	var tonChartImage *receipt.Image
	var tonChartWarning string
	if content.ShowTonPortfolio {
		portfolio, err := s.store.LoadFinance()
		if err != nil {
			return nil, nil, buildError(http.StatusInternalServerError, err)
		}
		tonPrice, err := s.tonPriceProvider.CurrentPrice(ctx)
		if err != nil {
			return nil, nil, buildError(http.StatusBadGateway, err)
		}
		summary := portfolio.ValueAt(tonPrice)
		tonSummary = &summary
		tonChartImage, tonChartWarning = s.resolveTonChartImage(ctx, tonPrice)
	}

	var usdBynRate *finance.FiatRate
	var usdBynChartImage *receipt.Image
	var usdBynChartWarning string
	if content.ShowUsdBynRate {
		rate, err := s.fiatRateProvider.CurrentRate(ctx)
		if err != nil {
			return nil, nil, buildError(http.StatusBadGateway, err)
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

	var newsItems []news.Item
	var newsTranslationWarning string
	if content.ShowNews {
		newsSettings, err := s.store.LoadNews()
		if err != nil {
			return nil, nil, buildError(http.StatusInternalServerError, err)
		}
		newsItems, err = s.newsProvider.Current(ctx, newsSettings)
		if err != nil {
			return nil, nil, buildError(http.StatusBadGateway, err)
		}
		newsItems, newsTranslationWarning, err = s.translateNewsItems(ctx, newsSettings, newsItems)
		if err != nil {
			return nil, nil, buildError(http.StatusInternalServerError, err)
		}
	}

	receiptStyle, err := s.store.LoadReceiptStyle()
	if err != nil {
		return nil, nil, buildError(http.StatusInternalServerError, err)
	}

	lines := receipt.DailyReceiptWithStyle(receipt.DailyReceiptData{
		HideWeather:      !content.ShowWeather,
		Weather:          snapshot,
		WeatherAdvice:    weatherAdvice,
		MotivationQuote:  motivationQuote,
		TonPortfolio:     tonSummary,
		TonChartImage:    tonChartImage,
		USDBYNRate:       usdBynRate,
		USDBYNChartImage: usdBynChartImage,
		BankRates:        bankRatesSummary,
		MailMessages:     googleSummary.Mail,
		CalendarEvents:   googleSummary.Events,
		NewsItems:        newsItems,
	}, receiptStyle)
	warnings := optionalWarnings(motivationWarning, tonChartWarning, usdBynChartWarning, bankRatesWarning, googleWarning, newsTranslationWarning)
	return lines, warnings, nil
}

func (s *ReceiptService) PrintDailyReceipt(ctx context.Context) error {
	config, err := s.store.LoadPrinter()
	if err != nil {
		return buildError(http.StatusInternalServerError, err)
	}

	lines, err := s.BuildDailyReceipt(ctx)
	if err != nil {
		return err
	}

	s.printMu.Lock()
	defer s.printMu.Unlock()

	if err := s.printer.PrintReceipt(ctx, config, lines); err != nil {
		return buildError(http.StatusBadGateway, err)
	}
	return nil
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
	if err := chart.RenderTonPriceChart(path, chartData, chart.Options{Width: 384, Height: 96}); err != nil {
		return nil, err
	}

	return &receipt.Image{
		Path:         path,
		URL:          fmt.Sprintf("/assets/generated/ton-24h.png?v=%d", s.clock().UnixNano()),
		Width:        384,
		Height:       96,
		ScalePercent: 100,
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
	if err := chart.RenderFiatRateChart(path, chartData, chart.Options{Width: 384, Height: 96}); err != nil {
		return nil, "график USD/BYN недоступен: " + err.Error()
	}

	return &receipt.Image{
		Path:         path,
		URL:          fmt.Sprintf("/assets/generated/usd-byn-7d.png?v=%d", s.clock().UnixNano()),
		Width:        384,
		Height:       96,
		ScalePercent: 100,
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
		LocationName:         snapshot.LocationName,
		ObservedAt:           snapshot.ObservedAt.In(snapshot.TimeLocation()),
		Condition:            weather.ConditionLabel(snapshot),
		TemperatureC:         snapshot.TemperatureC,
		ApparentTemperatureC: snapshot.ApparentTemperatureC,
		PrecipitationMm:      snapshot.PrecipitationMm,
		WindSpeedMs:          snapshot.WindSpeedMs,
		SurfacePressureHpa:   snapshot.SurfacePressureHpa,
		DayTemperatureC:      snapshot.DayTemperatureC,
		NightTemperatureC:    snapshot.NightTemperatureC,
	}
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
