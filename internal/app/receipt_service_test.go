package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"atol-server/internal/bankrates"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/weather"
)

func TestReceiptServicePrintsDailyReceiptWithSavedPrinterConfig(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		config:    printer.Config{Host: "192.168.0.118", Port: 5555},
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		newsSettings: news.Settings{Sources: []news.SourceSettings{
			{Preset: news.PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
		}},
	}
	gateway := &fakePrinter{}
	service := NewReceiptService(
		store,
		gateway,
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "BBC Russian"}}}),
		WithMotivationProvider(&fakeMotivationProvider{
			quote:  motivation.Quote{Text: "Делай важное спокойно."},
			advice: motivation.WeatherAdvice{Text: "Возьми зонт."},
		}),
	)

	if err := service.PrintDailyReceipt(context.Background()); err != nil {
		t.Fatalf("print daily receipt: %v", err)
	}

	if gateway.printedConfig != store.config {
		t.Fatalf("expected printer config %#v, got %#v", store.config, gateway.printedConfig)
	}
	if !lineTextsContain(gateway.printedLines, "Курс доллара") {
		t.Fatalf("expected full daily receipt, got %#v", gateway.printedLines)
	}
	if !lineTextsContainSubstring(gateway.printedLines, "Делай важное") {
		t.Fatalf("expected motivation quote in daily receipt, got %#v", gateway.printedLines)
	}
	if !lineTextsContain(gateway.printedLines, "Возьми зонт.") {
		t.Fatalf("expected weather advice in daily receipt, got %#v", gateway.printedLines)
	}
}

func TestReceiptServiceAddsTonChartImageWhenMarketChartAvailable(t *testing.T) {
	weatherCode := 0
	assetsPath := t.TempDir()
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:          finance.DefaultTonPortfolio(),
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
		},
		&fakePrinter{},
		fixedClock,
		WithGeneratedAssetsPath(assetsPath),
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithTonMarketChartProvider(&fakeTonMarketChartProvider{chart: finance.TonMarketChart{Points: []finance.TonPricePoint{
			{Time: fixedClock().Add(-2 * time.Hour), USD: 1.70},
			{Time: fixedClock().Add(-1 * time.Hour), USD: 1.78},
			{Time: fixedClock(), USD: 1.74},
		}}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, err := service.BuildDailyReceipt(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt: %v", err)
	}

	chartPath := filepath.Join(assetsPath, "generated", "ton-24h.png")
	if _, err := os.Stat(chartPath); err != nil {
		t.Fatalf("expected generated chart file: %v", err)
	}
	for _, line := range lines {
		if line.ImagePath == chartPath {
			if !strings.HasPrefix(line.ImageURL, "/assets/generated/ton-24h.png?v=") {
				t.Fatalf("unexpected chart image URL: %#v", line)
			}
			if len(line.ImagePixelBuffer) != 384*96 {
				t.Fatalf("expected TON chart pixel buffer, got %#v", line)
			}
			return
		}
	}
	t.Fatalf("expected TON chart image line, got %#v", lines)
}

func TestReceiptServiceContinuesWhenTonMarketChartFails(t *testing.T) {
	weatherCode := 0
	assetsPath := t.TempDir()
	tonChange := 4.2
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:          finance.DefaultTonPortfolio(),
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
		},
		&fakePrinter{},
		fixedClock,
		WithGeneratedAssetsPath(assetsPath),
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407, USD24hChangePercent: &tonChange}}),
		WithTonMarketChartProvider(&fakeTonMarketChartProvider{err: errors.New("CoinGecko unavailable")}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt must survive chart failure: %v", err)
	}
	if !lineTextsContain(lines, "TON") {
		t.Fatalf("expected TON block without chart, got %#v", lines)
	}
	chartPath := filepath.Join(assetsPath, "generated", "ton-24h.png")
	if _, err := os.Stat(chartPath); err != nil {
		t.Fatalf("expected fallback TON chart file: %v", err)
	}
	if !receiptLinesContainPixelImage(lines, chartPath) {
		t.Fatalf("expected fallback TON chart image line, got %#v", lines)
	}
	if !containsWarning(warnings, "график TON") {
		t.Fatalf("expected chart warning, got %#v", warnings)
	}
}

func TestReceiptServiceContinuesWhenTonPriceFails(t *testing.T) {
	weatherCode := 0
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:          finance.DefaultTonPortfolio(),
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
		},
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{err: errors.New("CoinGecko returned HTTP 429")}),
		WithTonMarketChartProvider(&fakeTonMarketChartProvider{}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt must survive TON price failure: %v", err)
	}
	if lineTextsContain(lines, "TON") {
		t.Fatalf("expected TON block to be skipped when price is unavailable, got %#v", lines)
	}
	if !containsWarning(warnings, "TON недоступен") {
		t.Fatalf("expected TON warning, got %#v", warnings)
	}
}

func TestReceiptServiceAddsUsdBynChartImageWhenMarketChartAvailable(t *testing.T) {
	weatherCode := 0
	assetsPath := t.TempDir()
	service := NewReceiptService(
		&fakeStore{
			location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio: finance.DefaultTonPortfolio(),
		},
		&fakePrinter{},
		fixedClock,
		WithGeneratedAssetsPath(assetsPath),
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithTonMarketChartProvider(&fakeTonMarketChartProvider{}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithFiatMarketChartProvider(&fakeFiatMarketChartProvider{chart: finance.FiatMarketChart{
			BaseCode:  "USD",
			QuoteCode: "BYN",
			Points: []finance.FiatRatePoint{
				{Date: fixedClock().AddDate(0, 0, -2), Rate: 3.10},
				{Date: fixedClock().AddDate(0, 0, -1), Rate: 3.13},
				{Date: fixedClock(), Rate: 3.12},
			},
		}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, err := service.BuildDailyReceipt(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt: %v", err)
	}

	chartPath := filepath.Join(assetsPath, "generated", "usd-byn-7d.png")
	if _, err := os.Stat(chartPath); err != nil {
		t.Fatalf("expected generated chart file: %v", err)
	}
	for _, line := range lines {
		if line.ImagePath == chartPath {
			if !strings.HasPrefix(line.ImageURL, "/assets/generated/usd-byn-7d.png?v=") {
				t.Fatalf("unexpected chart image URL: %#v", line)
			}
			if len(line.ImagePixelBuffer) != 384*96 {
				t.Fatalf("expected USD/BYN chart pixel buffer, got %#v", line)
			}
			return
		}
	}
	t.Fatalf("expected USD/BYN chart image line, got %#v", lines)
}

func TestReceiptServiceContinuesWhenUsdBynMarketChartFails(t *testing.T) {
	weatherCode := 0
	service := NewReceiptService(
		&fakeStore{
			location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio: finance.DefaultTonPortfolio(),
		},
		&fakePrinter{},
		fixedClock,
		WithGeneratedAssetsPath(t.TempDir()),
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithTonMarketChartProvider(&fakeTonMarketChartProvider{}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithFiatMarketChartProvider(&fakeFiatMarketChartProvider{err: errors.New("NBRB unavailable")}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt must survive chart failure: %v", err)
	}
	if !lineTextsContain(lines, "Курс доллара") {
		t.Fatalf("expected USD/BYN block without chart, got %#v", lines)
	}
	if !containsWarning(warnings, "график USD/BYN") {
		t.Fatalf("expected USD/BYN chart warning, got %#v", warnings)
	}
}

func TestReceiptServiceAddsBankRatesWhenAvailable(t *testing.T) {
	weatherCode := 0
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:          finance.DefaultTonPortfolio(),
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
		},
		&fakePrinter{},
		fixedClock,
		WithGeneratedAssetsPath(t.TempDir()),
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{summary: bankrates.Summary{
			Source: "TheMoney.by",
			SellUSD: &bankrates.Offer{
				Rate:      3.255,
				BankNames: []string{"Банк Б"},
			},
			BuyUSD: &bankrates.Offer{
				Rate:      3.279,
				BankNames: []string{"Банк Д"},
			},
		}}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{
			quote:  motivation.Quote{Text: "Спокойный фокус."},
			advice: motivation.WeatherAdvice{Text: "Погода рабочая."},
		}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !lineTextsContain(lines, "В банках") {
		t.Fatalf("expected bank rates block, got %#v", lines)
	}
	if !lineTextsContain(lines, "Продать $ 3.2550") || !lineTextsContain(lines, "Купить $ 3.2790") {
		t.Fatalf("expected bank buy/sell rates, got %#v", lines)
	}
}

func TestReceiptServiceContinuesWhenBankRatesFail(t *testing.T) {
	weatherCode := 0
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:          finance.DefaultTonPortfolio(),
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
		},
		&fakePrinter{},
		fixedClock,
		WithGeneratedAssetsPath(t.TempDir()),
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{err: errors.New("TheMoney unavailable")}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt must survive bank rates failure: %v", err)
	}
	if !lineTextsContain(lines, "Курс доллара") {
		t.Fatalf("expected official USD/BYN block to remain, got %#v", lines)
	}
	if lineTextsContain(lines, "В банках") {
		t.Fatalf("expected bank rates block to be omitted on failure, got %#v", lines)
	}
	if !containsWarning(warnings, "банковские курсы") {
		t.Fatalf("expected bank rates warning, got %#v", warnings)
	}
}

func TestReceiptServiceOmitsMotivationQuoteWhenProviderFails(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		config:             printer.Config{Host: "192.168.0.118", Port: 5555},
		location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio:          finance.DefaultTonPortfolio(),
		newsSettings:       news.Settings{},
		motivationSettings: motivation.Settings{Enabled: true},
	}
	gateway := &fakePrinter{}
	service := NewReceiptService(
		store,
		gateway,
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{err: errors.New("llama offline")}),
	)

	if err := service.PrintDailyReceipt(context.Background()); err != nil {
		t.Fatalf("print daily receipt must survive motivation failure: %v", err)
	}

	if !lineTextsContain(gateway.printedLines, "Курс доллара") {
		t.Fatalf("expected receipt to continue after motivation failure, got %#v", gateway.printedLines)
	}
	if store.motivationSettings.LastError == "" {
		t.Fatalf("expected motivation error to be saved, got %#v", store.motivationSettings)
	}
}

func TestReceiptServiceRequestsFreshMotivationQuoteForEachBuild(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		motivationSettings: motivation.Settings{
			Enabled:     true,
			CacheDate:   "2026-05-25",
			CachedQuote: "Старая цитата",
		},
	}
	provider := &fakeMotivationProvider{
		quotes: []motivation.Quote{
			{Text: "Первая новая цитата."},
			{Text: "Вторая новая цитата."},
		},
		advice: motivation.WeatherAdvice{Text: "Погода для прогулки."},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			LocationName: "Гомель",
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(provider),
	)

	first, err := service.BuildDailyReceipt(context.Background())
	if err != nil {
		t.Fatalf("build first receipt: %v", err)
	}
	second, err := service.BuildDailyReceipt(context.Background())
	if err != nil {
		t.Fatalf("build second receipt: %v", err)
	}

	if provider.quoteCalls != 2 {
		t.Fatalf("expected a fresh quote request for each build, got %d calls", provider.quoteCalls)
	}
	if !lineTextsContain(first, "Первая новая цитата.") {
		t.Fatalf("expected first fresh quote, got %#v", first)
	}
	if !lineTextsContain(second, "Вторая новая цитата.") {
		t.Fatalf("expected second fresh quote, got %#v", second)
	}
	if store.motivationSettings.CachedQuote != "Вторая новая цитата." {
		t.Fatalf("expected last generated quote to be stored for UI status, got %#v", store.motivationSettings)
	}
}

func TestReceiptServiceTranslatesEnglishNewsTitles(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		newsSettings: news.Settings{Sources: []news.SourceSettings{
			{Preset: news.PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/bbc.xml", MaxItems: 1},
			{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/reuters.xml", MaxItems: 1},
			{Preset: news.PresetHackerNews, Enabled: true, FeedURL: "https://example.com/hn.xml", MaxItems: 1},
		}},
		motivationSettings: motivation.Settings{
			Configured: true,
			Enabled:    false,
			BaseURL:    motivation.DefaultBaseURL,
			Model:      motivation.DefaultModel,
		},
	}
	provider := &fakeMotivationProvider{
		quote:  motivation.Quote{Text: "Делай важное спокойно."},
		advice: motivation.WeatherAdvice{Text: "Погода для прогулки."},
		translations: []motivation.NewsTranslation{
			{Index: 1, Title: "Reuters готовит новый обзор рынков"},
			{Index: 2, Title: "Основатель стартапа рассказал о росте"},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{
			{Title: "Русский заголовок", SourceName: "BBC Russian"},
			{Title: "Reuters prepares a new market wrap", SourceName: "Reuters"},
			{Title: "Founder mode and startup growth", SourceName: "Hacker News"},
		}}),
		WithMotivationProvider(provider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}

	if !lineTextsContainSubstring(lines, "Reuters готовит") {
		t.Fatalf("expected translated Reuters title, got %#v", lines)
	}
	if !lineTextsContainSubstring(lines, "Reuters prepares") {
		t.Fatalf("expected original Reuters title, got %#v", lines)
	}
	if !lineTextsContainSubstring(lines, "Основатель стартапа") {
		t.Fatalf("expected translated Hacker News title, got %#v", lines)
	}
	if len(provider.translatedTitles) != 2 {
		t.Fatalf("expected only English sources to be translated, got %#v", provider.translatedTitles)
	}
	if provider.translatedTitles[0].Index != 1 || provider.translatedTitles[0].SourceName != "Reuters" {
		t.Fatalf("expected Reuters title with original item index, got %#v", provider.translatedTitles)
	}
	if provider.translatedTitles[1].Index != 2 || provider.translatedTitles[1].SourceName != "Hacker News" {
		t.Fatalf("expected Hacker News title with original item index, got %#v", provider.translatedTitles)
	}
	if lineTextsContainSubstring(lines, "- Русский заголовок") && lineTextsContainSubstring(lines, "Русский заголовок Русский заголовок") {
		t.Fatalf("expected BBC Russian title not to get duplicated as original, got %#v", lines)
	}
}

func TestReceiptServiceIncludesGoogleMailAndCalendar(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		receiptContent: receipt.ContentSettings{
			Configured:          true,
			ShowWeather:         true,
			ShowWeatherAdvice:   true,
			ShowMotivationQuote: true,
			ShowTonPortfolio:    true,
			ShowUsdBynRate:      true,
			ShowBankRates:       true,
			ShowMail:            true,
			ShowCalendar:        true,
			ShowNews:            true,
		},
		motivationSettings: motivation.Settings{
			Configured: true,
			Enabled:    false,
			BaseURL:    motivation.DefaultBaseURL,
			Model:      motivation.DefaultModel,
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{
			quote:  motivation.Quote{Text: "Спокойный фокус."},
			advice: motivation.WeatherAdvice{Text: "Погода рабочая."},
		}),
		WithGoogleProvider(&fakeGoogleProvider{summary: googleintegration.Summary{
			Mail: []googleintegration.MailMessage{{From: "Alice", Subject: "Morning update"}},
			Events: []googleintegration.CalendarEvent{
				{TimeLabel: "18:30", Title: "Ветеринар"},
			},
		}}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !lineTextsContain(lines, "Почта") || !lineTextsContain(lines, "Alice") || !lineTextsContain(lines, "Morning update") {
		t.Fatalf("expected mail block, got %#v", lines)
	}
	if !lineTextsContain(lines, "Календарь") || !lineTextsContain(lines, "18:30      Ветеринар") {
		t.Fatalf("expected calendar block, got %#v", lines)
	}
}

func TestReceiptServiceDefaultContentOmitsMailAndKeepsCalendar(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio:          finance.DefaultTonPortfolio(),
		motivationSettings: motivation.Settings{Configured: true, Enabled: false},
	}
	googleProvider := &fakeGoogleProvider{summary: googleintegration.Summary{
		Mail: []googleintegration.MailMessage{{From: "Alice", Subject: "Morning update"}},
		Events: []googleintegration.CalendarEvent{
			{TimeLabel: "18:30", Title: "Ветеринар"},
		},
	}}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{
			quote:  motivation.Quote{Text: "Спокойный фокус."},
			advice: motivation.WeatherAdvice{Text: "Погода рабочая."},
		}),
		WithGoogleProvider(googleProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if lineTextsContain(lines, "Почта") || lineTextsContain(lines, "Alice") || lineTextsContain(lines, "Morning update") {
		t.Fatalf("expected default content to omit mail, got %#v", lines)
	}
	if !lineTextsContain(lines, "Календарь") || !lineTextsContain(lines, "18:30      Ветеринар") {
		t.Fatalf("expected default content to keep calendar, got %#v", lines)
	}
	if googleProvider.selectedCalls != 1 || googleProvider.includeMail || !googleProvider.includeCalendar {
		t.Fatalf("expected service to request only calendar from Google, provider=%#v", googleProvider)
	}
}

func TestReceiptServiceBuildsEveningCalendarSectionsAndAdvice(t *testing.T) {
	minsk := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2026, 5, 25, 15, 30, 0, 0, minsk)
	store := &fakeStore{
		receiptContent: receipt.ContentSettings{
			Configured:          true,
			ShowWeather:         false,
			ShowWeatherAdvice:   false,
			ShowMotivationQuote: false,
			ShowTonPortfolio:    false,
			ShowUsdBynRate:      false,
			ShowBankRates:       false,
			ShowMail:            false,
			ShowCalendar:        true,
			ShowNews:            false,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: true},
	}
	motivationProvider := &fakeMotivationProvider{
		calendarAdvice: motivation.CalendarAdvice{Text: "День плотный, держи паузы."},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		func() time.Time { return now },
		WithGoogleProvider(&fakeGoogleProvider{summary: googleintegration.Summary{
			Events: []googleintegration.CalendarEvent{
				{
					TimeLabel: "09:00",
					Title:     "Прошедший созвон",
					Start:     time.Date(2026, 5, 25, 9, 0, 0, 0, minsk),
					End:       time.Date(2026, 5, 25, 10, 0, 0, 0, minsk),
				},
				{
					TimeLabel: "16:00",
					Title:     "Синк по релизу",
					Start:     time.Date(2026, 5, 25, 16, 0, 0, 0, minsk),
					End:       time.Date(2026, 5, 25, 17, 0, 0, 0, minsk),
				},
				{
					TimeLabel: "10:00",
					Title:     "Планирование",
					Start:     time.Date(2026, 5, 26, 10, 0, 0, 0, minsk),
					End:       time.Date(2026, 5, 26, 11, 0, 0, 0, minsk),
				},
			},
		}}),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if lineTextsContain(lines, "09:00      Прошедший созвон") {
		t.Fatalf("expected elapsed today event to be omitted in evening mode, got %#v", lines)
	}
	for _, want := range []string{"Календарь", "Остаток сегодня", "16:00      Синк по релизу", "Завтра", "10:00      Планирование", "День плотный, держи паузы."} {
		if !lineTextsContain(lines, want) {
			t.Fatalf("expected %q in receipt lines, got %#v", want, lines)
		}
	}
	if motivationProvider.calendarAdviceCalls != 1 {
		t.Fatalf("expected one calendar advice call, got %d", motivationProvider.calendarAdviceCalls)
	}
	if len(motivationProvider.calendarContext.Sections) != 2 ||
		motivationProvider.calendarContext.Sections[0].Title != "Остаток сегодня" ||
		len(motivationProvider.calendarContext.Sections[0].Events) != 1 ||
		motivationProvider.calendarContext.Sections[1].Title != "Завтра" ||
		len(motivationProvider.calendarContext.Sections[1].Events) != 1 {
		t.Fatalf("unexpected calendar advice context: %#v", motivationProvider.calendarContext)
	}
}

func TestReceiptServiceCalendarAdviceFailureDoesNotBreakReceipt(t *testing.T) {
	minsk := time.FixedZone("MSK", 3*60*60)
	now := time.Date(2026, 5, 25, 9, 0, 0, 0, minsk)
	store := &fakeStore{
		receiptContent: receipt.ContentSettings{
			Configured:          true,
			ShowWeather:         false,
			ShowWeatherAdvice:   false,
			ShowMotivationQuote: false,
			ShowTonPortfolio:    false,
			ShowUsdBynRate:      false,
			ShowBankRates:       false,
			ShowMail:            false,
			ShowCalendar:        true,
			ShowNews:            false,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: true},
	}
	motivationProvider := &fakeMotivationProvider{calendarAdviceErr: errors.New("llama offline")}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		func() time.Time { return now },
		WithGoogleProvider(&fakeGoogleProvider{summary: googleintegration.Summary{
			Events: []googleintegration.CalendarEvent{
				{
					TimeLabel: "11:00",
					Title:     "Встреча",
					Start:     time.Date(2026, 5, 25, 11, 0, 0, 0, minsk),
					End:       time.Date(2026, 5, 25, 12, 0, 0, 0, minsk),
				},
			},
		}}),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if !lineTextsContain(lines, "Сегодня") || !lineTextsContain(lines, "11:00      Встреча") {
		t.Fatalf("expected calendar to remain printable after advice failure, got %#v", lines)
	}
	if !containsWarning(warnings, "AI-совет по календарю недоступен") {
		t.Fatalf("expected calendar advice warning, got %#v", warnings)
	}
}

func TestReceiptServiceSkipsDisabledFinanceContent(t *testing.T) {
	weatherCode := 0
	tonProvider := &fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}
	fiatProvider := &fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}
	bankProvider := &fakeBankRatesProvider{summary: bankrates.Summary{
		SellUSD: &bankrates.Offer{Rate: 3.255, BankNames: []string{"Банк Б"}},
		BuyUSD:  &bankrates.Offer{Rate: 3.279, BankNames: []string{"Банк Д"}},
	}}
	store := &fakeStore{
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		receiptContent: receipt.ContentSettings{
			Configured:          true,
			ShowWeather:         true,
			ShowWeatherAdvice:   false,
			ShowMotivationQuote: false,
			ShowTonPortfolio:    false,
			ShowUsdBynRate:      false,
			ShowBankRates:       false,
			ShowMail:            false,
			ShowCalendar:        false,
			ShowNews:            false,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: false},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(tonProvider),
		WithFiatRateProvider(fiatProvider),
		WithBankRatesProvider(bankProvider),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for disabled sections, got %#v", warnings)
	}
	if lineTextsContain(lines, "TON") || lineTextsContain(lines, "Курс доллара") || lineTextsContain(lines, "В банках") {
		t.Fatalf("expected disabled finance blocks to be omitted, got %#v", lines)
	}
	if tonProvider.calls != 0 || fiatProvider.calls != 0 || bankProvider.calls != 0 {
		t.Fatalf("expected disabled finance providers not to be called, got ton=%d fiat=%d bank=%d", tonProvider.calls, fiatProvider.calls, bankProvider.calls)
	}
}

func TestReceiptServiceSkipsDisabledAIContent(t *testing.T) {
	weatherCode := 0
	motivationProvider := &fakeMotivationProvider{
		quote:          motivation.Quote{Text: "Цитата не нужна."},
		advice:         motivation.WeatherAdvice{Text: "Совет не нужен."},
		calendarAdvice: motivation.CalendarAdvice{Text: "Календарный совет не нужен."},
	}
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			motivationSettings: motivation.Settings{Configured: true, Enabled: true},
			receiptContent: receipt.ContentSettings{
				Configured:          true,
				ShowWeather:         true,
				ShowWeatherAdvice:   false,
				ShowMotivationQuote: false,
				ShowTonPortfolio:    false,
				ShowUsdBynRate:      false,
				ShowBankRates:       false,
				ShowMail:            false,
				ShowCalendar:        false,
				ShowNews:            false,
			},
		},
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if motivationProvider.quoteCalls != 0 || motivationProvider.adviceCalls != 0 || motivationProvider.calendarAdviceCalls != 0 {
		t.Fatalf("expected disabled AI content to skip provider calls, provider=%#v", motivationProvider)
	}
	if lineTextsContain(lines, "Цитата не нужна.") || lineTextsContain(lines, "Совет не нужен.") || lineTextsContain(lines, "Календарный совет не нужен.") {
		t.Fatalf("expected disabled AI content to be omitted, got %#v", lines)
	}
}

func TestReceiptServicePrintsWeatherAdviceWhenLegacyMotivationDisabled(t *testing.T) {
	weatherCode := 0
	motivationProvider := &fakeMotivationProvider{
		quote:  motivation.Quote{Text: "Цитата не нужна."},
		advice: motivation.WeatherAdvice{Text: "Возьми зонт."},
	}
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
			receiptContent: receipt.ContentSettings{
				Configured:          true,
				ShowWeather:         true,
				ShowWeatherAdvice:   true,
				ShowMotivationQuote: false,
				ShowTonPortfolio:    false,
				ShowUsdBynRate:      false,
				ShowBankRates:       false,
				ShowMail:            false,
				ShowCalendar:        false,
				ShowNews:            false,
			},
		},
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if motivationProvider.adviceCalls != 1 || motivationProvider.quoteCalls != 0 {
		t.Fatalf("expected only weather advice generation, provider=%#v", motivationProvider)
	}
	if !lineTextsContain(lines, "Возьми зонт.") {
		t.Fatalf("expected weather advice in receipt, got %#v", lines)
	}
	if lineTextsContain(lines, "Цитата не нужна.") {
		t.Fatalf("expected quote to stay hidden, got %#v", lines)
	}
}

func TestReceiptServicePrintsQuoteWhenLegacyMotivationDisabled(t *testing.T) {
	weatherCode := 0
	motivationProvider := &fakeMotivationProvider{
		quote:  motivation.Quote{Text: "Сегодня достаточно одного шага."},
		advice: motivation.WeatherAdvice{Text: "Совет не нужен."},
	}
	service := NewReceiptService(
		&fakeStore{
			location:           weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			motivationSettings: motivation.Settings{Configured: true, Enabled: false},
			receiptContent: receipt.ContentSettings{
				Configured:          true,
				ShowWeather:         true,
				ShowWeatherAdvice:   false,
				ShowMotivationQuote: true,
				ShowTonPortfolio:    false,
				ShowUsdBynRate:      false,
				ShowBankRates:       false,
				ShowMail:            false,
				ShowCalendar:        false,
				ShowNews:            false,
			},
		},
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if motivationProvider.quoteCalls != 1 || motivationProvider.adviceCalls != 0 {
		t.Fatalf("expected only quote generation, provider=%#v", motivationProvider)
	}
	if !lineTextsContain(lines, "Сегодня достаточно одного шага.") {
		t.Fatalf("expected quote in receipt, got %#v", lines)
	}
	if lineTextsContain(lines, "Совет не нужен.") {
		t.Fatalf("expected weather advice to stay hidden, got %#v", lines)
	}
}

func TestReceiptServiceHidesWeatherBlockWhenDisabled(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		receiptContent: receipt.ContentSettings{
			Configured:          true,
			ShowWeather:         false,
			ShowWeatherAdvice:   false,
			ShowMotivationQuote: false,
			ShowTonPortfolio:    false,
			ShowUsdBynRate:      false,
			ShowBankRates:       false,
			ShowMail:            false,
			ShowCalendar:        false,
			ShowNews:            false,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: false},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	for _, unwanted := range []string{"25 Мая", "Понедельник", "Ясно", "+22 C"} {
		if lineTextsContain(lines, unwanted) {
			t.Fatalf("expected weather block to omit %q, got %#v", unwanted, lines)
		}
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
}

type fakeStore struct {
	config             printer.Config
	location           weather.Location
	portfolio          finance.TonPortfolio
	newsSettings       news.Settings
	receiptStyle       receipt.StyleSettings
	receiptContent     receipt.ContentSettings
	motivationSettings motivation.Settings
}

func (s *fakeStore) LoadPrinter() (printer.Config, error) {
	return s.config, nil
}

func (s *fakeStore) LoadWeather() (weather.Location, error) {
	return s.location, nil
}

func (s *fakeStore) LoadFinance() (finance.TonPortfolio, error) {
	return s.portfolio, nil
}

func (s *fakeStore) LoadNews() (news.Settings, error) {
	return s.newsSettings, nil
}

func (s *fakeStore) LoadReceiptStyle() (receipt.StyleSettings, error) {
	if s.receiptStyle == (receipt.StyleSettings{}) {
		return receipt.DefaultStyleSettings(), nil
	}
	return s.receiptStyle.Normalized(), nil
}

func (s *fakeStore) LoadReceiptContent() (receipt.ContentSettings, error) {
	return s.receiptContent.Normalized(), nil
}

func (s *fakeStore) LoadMotivation() (motivation.Settings, error) {
	if s.motivationSettings == (motivation.Settings{}) {
		return motivation.DefaultSettings(), nil
	}
	return s.motivationSettings.Normalized(), nil
}

func (s *fakeStore) SaveMotivation(settings motivation.Settings) error {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	s.motivationSettings = normalized
	return nil
}

type fakePrinter struct {
	printedConfig printer.Config
	printedLines  []receipt.Line
}

func (p *fakePrinter) PrintReceipt(_ context.Context, config printer.Config, lines []receipt.Line) error {
	p.printedConfig = config
	p.printedLines = append([]receipt.Line(nil), lines...)
	return nil
}

type fakeWeatherProvider struct {
	snapshot weather.Snapshot
}

func (p *fakeWeatherProvider) Current(context.Context, weather.Location) (weather.Snapshot, error) {
	return p.snapshot, nil
}

type fakeTonProvider struct {
	price finance.TonPrice
	err   error
	calls int
}

func (p *fakeTonProvider) CurrentPrice(context.Context) (finance.TonPrice, error) {
	p.calls++
	if p.err != nil {
		return finance.TonPrice{}, p.err
	}
	return p.price, nil
}

type fakeTonMarketChartProvider struct {
	chart finance.TonMarketChart
	err   error
}

func (p *fakeTonMarketChartProvider) MarketChart(context.Context) (finance.TonMarketChart, error) {
	return p.chart, p.err
}

type fakeFiatProvider struct {
	rate  finance.FiatRate
	calls int
}

func (p *fakeFiatProvider) CurrentRate(context.Context) (finance.FiatRate, error) {
	p.calls++
	return p.rate, nil
}

type fakeFiatMarketChartProvider struct {
	chart finance.FiatMarketChart
	err   error
}

func (p *fakeFiatMarketChartProvider) MarketChart(context.Context, time.Time) (finance.FiatMarketChart, error) {
	return p.chart, p.err
}

type fakeBankRatesProvider struct {
	summary bankrates.Summary
	err     error
	calls   int
}

func (p *fakeBankRatesProvider) Current(context.Context) (bankrates.Summary, error) {
	p.calls++
	return p.summary, p.err
}

type fakeNewsProvider struct {
	items []news.Item
}

func (p *fakeNewsProvider) Current(context.Context, news.Settings) ([]news.Item, error) {
	return p.items, nil
}

type fakeGoogleProvider struct {
	summary         googleintegration.Summary
	err             error
	calls           int
	selectedCalls   int
	includeMail     bool
	includeCalendar bool
}

func (p *fakeGoogleProvider) Current(context.Context) (googleintegration.Summary, error) {
	p.calls++
	return p.summary, p.err
}

func (p *fakeGoogleProvider) CurrentSelected(_ context.Context, includeMail bool, includeCalendar bool) (googleintegration.Summary, error) {
	p.selectedCalls++
	p.includeMail = includeMail
	p.includeCalendar = includeCalendar
	if p.err != nil {
		return googleintegration.Summary{}, p.err
	}
	summary := p.summary
	if !includeMail {
		summary.Mail = nil
	}
	if !includeCalendar {
		summary.Events = nil
	}
	return summary, nil
}

type fakeMotivationProvider struct {
	quote               motivation.Quote
	quotes              []motivation.Quote
	advice              motivation.WeatherAdvice
	calendarAdvice      motivation.CalendarAdvice
	translations        []motivation.NewsTranslation
	err                 error
	calendarAdviceErr   error
	translationErr      error
	weatherContext      motivation.WeatherContext
	calendarContext     motivation.CalendarContext
	translatedTitles    []motivation.NewsTitle
	quoteCalls          int
	adviceCalls         int
	calendarAdviceCalls int
}

func (p *fakeMotivationProvider) Generate(context.Context, motivation.Settings) (motivation.Quote, error) {
	p.quoteCalls++
	if len(p.quotes) > 0 {
		quote := p.quotes[0]
		p.quotes = p.quotes[1:]
		return quote, p.err
	}
	return p.quote, p.err
}

func (p *fakeMotivationProvider) GenerateWeatherAdvice(_ context.Context, _ motivation.Settings, weatherContext motivation.WeatherContext) (motivation.WeatherAdvice, error) {
	p.adviceCalls++
	p.weatherContext = weatherContext
	return p.advice, p.err
}

func (p *fakeMotivationProvider) GenerateCalendarAdvice(_ context.Context, _ motivation.Settings, calendarContext motivation.CalendarContext) (motivation.CalendarAdvice, error) {
	p.calendarAdviceCalls++
	p.calendarContext = calendarContext
	if p.calendarAdviceErr != nil {
		return motivation.CalendarAdvice{}, p.calendarAdviceErr
	}
	return p.calendarAdvice, p.err
}

func (p *fakeMotivationProvider) TranslateNewsTitles(_ context.Context, _ motivation.Settings, titles []motivation.NewsTitle) ([]motivation.NewsTranslation, error) {
	p.translatedTitles = append([]motivation.NewsTitle(nil), titles...)
	return append([]motivation.NewsTranslation(nil), p.translations...), p.translationErr
}

func lineTextsContain(lines []receipt.Line, want string) bool {
	for _, line := range lines {
		if line.Text == want {
			return true
		}
	}
	return false
}

func lineTextsContainSubstring(lines []receipt.Line, want string) bool {
	for _, line := range lines {
		if strings.Contains(line.Text, want) {
			return true
		}
	}
	return false
}

func receiptLinesContainImage(lines []receipt.Line, path string) bool {
	for _, line := range lines {
		if line.ImagePath == path {
			return true
		}
	}
	return false
}

func receiptLinesContainPixelImage(lines []receipt.Line, path string) bool {
	for _, line := range lines {
		if line.ImagePath == path && len(line.ImagePixelBuffer) == line.ImageWidth*line.ImageHeight {
			return true
		}
	}
	return false
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
