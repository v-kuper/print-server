package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"atol-server/internal/bankrates"
	"atol-server/internal/dailyquest"
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

func TestReceiptServicePrintsDailyReceiptWithSavedPrinterConfig(t *testing.T) {
	weatherCode := 0
	store := &fakeStore{
		config:    printer.Config{Host: "192.168.0.118", Port: 5555},
		location:  weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		portfolio: finance.DefaultTonPortfolio(),
		newsSettings: news.Settings{Sources: []news.SourceSettings{
			{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
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
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "Reuters"}}}),
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

func TestReceiptServiceCreatesNewsSnapshotAndPrintsQRCode(t *testing.T) {
	translateTitles := false
	store := &fakeStore{
		config: printer.Config{Host: "192.168.0.118", Port: 5555},
		newsSettings: news.Settings{
			TranslateTitles: &translateTitles,
			Sources: []news.SourceSettings{
				{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
			},
		},
		snapshotSettings: receiptsnapshot.Settings{BaseURL: "http://192.168.0.25:8080"},
	}
	gateway := &fakePrinter{}
	snapshotStore := &fakeReceiptSnapshotStore{id: "snapshot-1"}
	service := NewReceiptService(
		store,
		gateway,
		fixedClock,
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{
			Title:         "Переведенный заголовок",
			OriginalTitle: "Original title",
			SourceName:    "Reuters",
			Link:          "https://example.com/news",
		}}}),
		WithReceiptSnapshotStore(snapshotStore),
	)

	warnings, err := service.PrintDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured: true,
		ShowNews:   true,
	})
	if err != nil {
		t.Fatalf("print receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !receiptLinesContainQRCode(gateway.printedLines, "http://192.168.0.25:8080/snapshots/snapshot-1") {
		t.Fatalf("expected printed QR line, got %#v", gateway.printedLines)
	}
	requireQRCodeSeparatedFromReceiptContent(t, gateway.printedLines, "http://192.168.0.25:8080/snapshots/snapshot-1")
	wantItems := []receiptsnapshot.NewsItem{{
		Title:         "Переведенный заголовок",
		OriginalTitle: "Original title",
		SourceName:    "Reuters",
		Link:          "https://example.com/news",
	}}
	if !reflect.DeepEqual(snapshotStore.createdItems, wantItems) {
		t.Fatalf("expected snapshot items %#v, got %#v", wantItems, snapshotStore.createdItems)
	}
	if !snapshotReceiptLinesContain(snapshotStore.createdLines, "Коротко о мире:") {
		t.Fatalf("expected snapshot receipt lines to include printed receipt, got %#v", snapshotStore.createdLines)
	}
	if !snapshotReceiptLinesContain(snapshotStore.finalizedLines, "Коротко о мире:") {
		t.Fatalf("expected finalized snapshot receipt lines to include printed receipt, got %#v", snapshotStore.finalizedLines)
	}
	if !snapshotReceiptLinesContainQRCode(snapshotStore.finalizedLines, "http://192.168.0.25:8080/snapshots/snapshot-1") {
		t.Fatalf("expected finalized snapshot receipt lines to include QR, got %#v", snapshotStore.finalizedLines)
	}
	requireSnapshotQRCodeSeparatedFromReceiptContent(t, snapshotStore.finalizedLines, "http://192.168.0.25:8080/snapshots/snapshot-1")
	if !snapshotReceiptLinesContainLinkedText(snapshotStore.finalizedLines, "Переведенный заголовок", "https://example.com/news") {
		t.Fatalf("expected finalized snapshot receipt lines to include linked news title, got %#v", snapshotStore.finalizedLines)
	}
	if snapshotStore.publishedID != "snapshot-1" {
		t.Fatalf("expected snapshot to be published, got %q", snapshotStore.publishedID)
	}
}

func TestReceiptServicePrintsNewsWithDefaultQRCodeWhenSnapshotBaseURLIsEmpty(t *testing.T) {
	translateTitles := false
	store := &fakeStore{
		config: printer.Config{Host: "192.168.0.118", Port: 5555},
		newsSettings: news.Settings{
			TranslateTitles: &translateTitles,
			Sources: []news.SourceSettings{
				{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
			},
		},
	}
	gateway := &fakePrinter{}
	snapshotStore := &fakeReceiptSnapshotStore{id: "snapshot-1"}
	service := NewReceiptService(
		store,
		gateway,
		fixedClock,
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "Reuters"}}}),
		WithReceiptSnapshotStore(snapshotStore),
	)

	warnings, err := service.PrintDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured: true,
		ShowNews:   true,
	})
	if err != nil {
		t.Fatalf("print receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !receiptLinesContainQRCode(gateway.printedLines, "http://localhost:8080/snapshots/snapshot-1") {
		t.Fatalf("expected default QR line, got %#v", gateway.printedLines)
	}
	if len(snapshotStore.createdItems) == 0 {
		t.Fatalf("expected snapshot items to be created")
	}
}

func TestReceiptServiceSkipsQRCodeWhenSnapshotFinalizationFails(t *testing.T) {
	translateTitles := false
	store := &fakeStore{
		config: printer.Config{Host: "192.168.0.118", Port: 5555},
		newsSettings: news.Settings{
			TranslateTitles: &translateTitles,
			Sources: []news.SourceSettings{
				{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
			},
		},
		snapshotSettings: receiptsnapshot.Settings{BaseURL: "http://192.168.0.25:8080"},
	}
	gateway := &fakePrinter{}
	snapshotStore := &fakeReceiptSnapshotStore{id: "snapshot-1", finalizeErr: errors.New("database timeout")}
	service := NewReceiptService(
		store,
		gateway,
		fixedClock,
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "Reuters"}}}),
		WithReceiptSnapshotStore(snapshotStore),
	)

	warnings, err := service.PrintDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured: true,
		ShowNews:   true,
	})
	if err != nil {
		t.Fatalf("print receipt: %v", err)
	}
	if len(warnings) == 0 || !strings.Contains(warnings[0], "не сохранен") {
		t.Fatalf("expected snapshot finalization warning, got %#v", warnings)
	}
	if receiptLinesContainAnyQRCode(gateway.printedLines) {
		t.Fatalf("QR must not be printed when finalized snapshot lines are not saved, got %#v", gateway.printedLines)
	}
	if snapshotStore.publishedID != "" {
		t.Fatalf("snapshot without final lines must not be published, got %q", snapshotStore.publishedID)
	}
}

func TestReceiptServiceMarksNewsSnapshotFailedWhenPrinterFails(t *testing.T) {
	translateTitles := false
	store := &fakeStore{
		config: printer.Config{Host: "192.168.0.118", Port: 5555},
		newsSettings: news.Settings{
			TranslateTitles: &translateTitles,
			Sources: []news.SourceSettings{
				{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
			},
		},
		snapshotSettings: receiptsnapshot.Settings{BaseURL: "http://192.168.0.25:8080"},
	}
	snapshotStore := &fakeReceiptSnapshotStore{id: "snapshot-1"}
	service := NewReceiptService(
		store,
		&fakePrinter{printErr: errors.New("paper empty")},
		fixedClock,
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Заголовок", SourceName: "Reuters"}}}),
		WithReceiptSnapshotStore(snapshotStore),
	)

	warnings, err := service.PrintDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured: true,
		ShowNews:   true,
	})

	if err == nil {
		t.Fatal("expected printer error")
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings on printer error, got %#v", warnings)
	}
	if snapshotStore.failedID != "snapshot-1" || snapshotStore.failedErr == nil || snapshotStore.failedErr.Error() != "paper empty" {
		t.Fatalf("expected failed snapshot, got id=%q err=%v", snapshotStore.failedID, snapshotStore.failedErr)
	}
	if snapshotStore.publishedID != "" {
		t.Fatalf("failed snapshot must not be published, got %q", snapshotStore.publishedID)
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

func TestReceiptServiceAddsOilChartImageWhenMarketChartAvailable(t *testing.T) {
	weatherCode := 0
	assetsPath := t.TempDir()
	content := receipt.DefaultContentSettings()
	service := NewReceiptService(
		&fakeStore{
			location:       weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:      finance.DefaultTonPortfolio(),
			receiptContent: content,
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
		WithOilPriceProvider(&fakeOilProvider{
			price: finance.OilPrice{Name: "Brent", Currency: "USD", Unit: "barrel", Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), ValueUSD: 98.29},
			chart: finance.OilMarketChart{
				Name:     "Brent",
				Currency: "USD",
				Points: []finance.OilPricePoint{
					{Date: fixedClock().AddDate(0, 0, -2), ValueUSD: 102.75},
					{Date: fixedClock().AddDate(0, 0, -1), ValueUSD: 95.47},
					{Date: fixedClock(), ValueUSD: 98.29},
				},
			},
		}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithFiatMarketChartProvider(&fakeFiatMarketChartProvider{}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, err := service.BuildDailyReceipt(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt: %v", err)
	}

	chartPath := filepath.Join(assetsPath, "generated", "oil-brent-7d.png")
	if _, err := os.Stat(chartPath); err != nil {
		t.Fatalf("expected generated oil chart file: %v", err)
	}
	if !lineTextsContain(lines, "Цена нефти") || !lineTextsContain(lines, "Brent $98.29/барр (01.06)") {
		t.Fatalf("expected oil block, got %#v", lines)
	}
	if !receiptLinesContainPixelImage(lines, chartPath) {
		t.Fatalf("expected oil chart image line, got %#v", lines)
	}
}

func TestReceiptServiceContinuesWhenOilMarketChartFails(t *testing.T) {
	weatherCode := 0
	content := receipt.DefaultContentSettings()
	service := NewReceiptService(
		&fakeStore{
			location:       weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:      finance.DefaultTonPortfolio(),
			receiptContent: content,
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
		WithOilPriceProvider(&fakeOilProvider{
			price:    finance.OilPrice{Name: "Brent", Currency: "USD", Unit: "barrel", Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), ValueUSD: 98.29},
			chartErr: errors.New("FRED unavailable"),
		}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithFiatMarketChartProvider(&fakeFiatMarketChartProvider{}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt must survive oil chart failure: %v", err)
	}
	if !lineTextsContain(lines, "Цена нефти") {
		t.Fatalf("expected oil block without chart, got %#v", lines)
	}
	if !containsWarning(warnings, "график нефти") {
		t.Fatalf("expected oil chart warning, got %#v", warnings)
	}
}

func TestReceiptServiceContinuesWhenOilPriceFails(t *testing.T) {
	weatherCode := 0
	content := receipt.DefaultContentSettings()
	service := NewReceiptService(
		&fakeStore{
			location:       weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
			portfolio:      finance.DefaultTonPortfolio(),
			receiptContent: content,
		},
		&fakePrinter{},
		fixedClock,
		WithWeatherProvider(&fakeWeatherProvider{snapshot: weather.Snapshot{
			Timezone:     "Europe/Minsk",
			ObservedAt:   fixedClock(),
			TemperatureC: 22.2,
			WeatherCode:  &weatherCode,
		}}),
		WithTonPriceProvider(&fakeTonProvider{price: finance.TonPrice{USD: 1.7435687405482407}}),
		WithTonMarketChartProvider(&fakeTonMarketChartProvider{}),
		WithOilPriceProvider(&fakeOilProvider{priceErr: errors.New("FRED returned HTTP 503")}),
		WithFiatRateProvider(&fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.1234}}),
		WithFiatMarketChartProvider(&fakeFiatMarketChartProvider{}),
		WithBankRatesProvider(&fakeBankRatesProvider{}),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build daily receipt must survive oil price failure: %v", err)
	}
	if lineTextsContain(lines, "Цена нефти") {
		t.Fatalf("expected oil block to be skipped when price is unavailable, got %#v", lines)
	}
	if !containsWarning(warnings, "нефть недоступна") {
		t.Fatalf("expected oil warning, got %#v", warnings)
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
		WithHistoryProvider(&fakeHistoryProvider{}),
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
			{Preset: news.PresetEconomist, Enabled: true, FeedURL: "https://example.com/economist.xml", MaxItems: 1},
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
			{Title: "Русский заголовок", SourceName: "Reuters"},
			{Title: "Reuters prepares a new market wrap", SourceName: "Reuters"},
			{Title: "Founder mode and startup growth", SourceName: "Hacker News"},
		}}),
		WithHistoryProvider(&fakeHistoryProvider{}),
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
		t.Fatalf("expected Reuters title not to get duplicated as original, got %#v", lines)
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
		WithHistoryProvider(&fakeHistoryProvider{}),
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

func TestReceiptServicePrintsHistoryBeforeNews(t *testing.T) {
	store := &fakeStore{
		newsSettings: news.Settings{Sources: []news.SourceSettings{
			{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
		}},
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
			ShowHistory:         true,
			ShowNews:            true,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: true},
	}
	historyProvider := &fakeHistoryProvider{events: []history.Event{
		{Year: 1961, Text: "Venera 1 became the first spacecraft to fly by Venus.", Link: "https://en.example/venera"},
		{Year: -585, Text: "A solar eclipse interrupted a battle.", Link: "https://en.example/eclipse"},
	}}
	motivationProvider := &fakeMotivationProvider{historyFacts: []motivation.HistoryFact{
		{Year: 1961, Text: "запущена первая автоматическая станция к Венере."},
		{Year: -585, Text: "солнечное затмение остановило битву на Галисе."},
	}}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithHistoryProvider(historyProvider),
		WithMotivationProvider(motivationProvider),
		WithNewsProvider(&fakeNewsProvider{items: []news.Item{{Title: "Новость", SourceName: "Reuters"}}}),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if historyProvider.calls != 1 {
		t.Fatalf("expected one history provider call, got %d", historyProvider.calls)
	}
	if got := historyProvider.date.Location().String(); got != "Europe/Minsk" {
		t.Fatalf("expected history lookup in Europe/Minsk date, got %q", got)
	}
	if motivationProvider.historyFactCalls != 1 {
		t.Fatalf("expected one history AI call, got %d", motivationProvider.historyFactCalls)
	}
	if len(motivationProvider.historyEvents) != 2 || motivationProvider.historyEvents[0].Link != "https://en.example/venera" {
		t.Fatalf("expected history events passed to AI, got %#v", motivationProvider.historyEvents)
	}
	if !lineTextsContain(lines, "История дня") || !lineTextsContainSubstring(lines, "1961 — запущена первая") {
		t.Fatalf("expected history block, got %#v", lines)
	}
	historyIndex := indexOfLineText(lines, "История дня")
	newsIndex := indexOfLineText(lines, "Коротко о мире:")
	if historyIndex < 0 || newsIndex < 0 || historyIndex > newsIndex {
		t.Fatalf("expected history before news, got %#v", lines)
	}
}

func TestReceiptServiceHistoryFailureDoesNotBreakReceipt(t *testing.T) {
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
			ShowCalendar:        false,
			ShowHistory:         true,
			ShowNews:            false,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: true},
	}
	historyProvider := &fakeHistoryProvider{err: errors.New("wikimedia offline")}
	motivationProvider := &fakeMotivationProvider{historyFacts: []motivation.HistoryFact{{Year: 1961, Text: "не должно печататься"}}}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithHistoryProvider(historyProvider),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "История дня недоступна: wikimedia offline") {
		t.Fatalf("expected history warning, got %#v", warnings)
	}
	if lineTextsContain(lines, "История дня") {
		t.Fatalf("expected history block to be omitted, got %#v", lines)
	}
	if motivationProvider.historyFactCalls != 0 {
		t.Fatalf("expected AI to be skipped when history API fails, got %d calls", motivationProvider.historyFactCalls)
	}
}

func TestReceiptServiceSkipsDisabledHistory(t *testing.T) {
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
			ShowCalendar:        false,
			ShowHistory:         false,
			ShowNews:            false,
		},
		motivationSettings: motivation.Settings{Configured: true, Enabled: true},
	}
	historyProvider := &fakeHistoryProvider{events: []history.Event{{Year: 1961, Text: "Venera"}}}
	motivationProvider := &fakeMotivationProvider{historyFacts: []motivation.HistoryFact{{Year: 1961, Text: "не должно печататься"}}}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithHistoryProvider(historyProvider),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithWarnings(context.Background())
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if historyProvider.calls != 0 || motivationProvider.historyFactCalls != 0 {
		t.Fatalf("expected disabled history to skip providers, history=%d ai=%d", historyProvider.calls, motivationProvider.historyFactCalls)
	}
	if lineTextsContain(lines, "История дня") {
		t.Fatalf("expected no history block, got %#v", lines)
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
		WithHistoryProvider(&fakeHistoryProvider{}),
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
	oilProvider := &fakeOilProvider{price: finance.OilPrice{Name: "Brent", Currency: "USD", Unit: "barrel", Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), ValueUSD: 98.29}}
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
			ShowOilPrice:        false,
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
		WithOilPriceProvider(oilProvider),
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
	if lineTextsContain(lines, "TON") || lineTextsContain(lines, "Цена нефти") || lineTextsContain(lines, "Курс доллара") || lineTextsContain(lines, "В банках") {
		t.Fatalf("expected disabled finance blocks to be omitted, got %#v", lines)
	}
	if tonProvider.calls != 0 || oilProvider.priceCalls != 0 || oilProvider.chartCalls != 0 || fiatProvider.calls != 0 || bankProvider.calls != 0 {
		t.Fatalf("expected disabled finance providers not to be called, got ton=%d oilPrice=%d oilChart=%d fiat=%d bank=%d", tonProvider.calls, oilProvider.priceCalls, oilProvider.chartCalls, fiatProvider.calls, bankProvider.calls)
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
	if motivationProvider.quoteCalls != 0 || motivationProvider.adviceCalls != 0 || motivationProvider.calendarAdviceCalls != 0 || motivationProvider.historyFactCalls != 0 {
		t.Fatalf("expected disabled AI content to skip provider calls, provider=%#v", motivationProvider)
	}
	if lineTextsContain(lines, "Цитата не нужна.") || lineTextsContain(lines, "Совет не нужен.") || lineTextsContain(lines, "Календарный совет не нужен.") {
		t.Fatalf("expected disabled AI content to be omitted, got %#v", lines)
	}
}

func TestReceiptServicePrintsDailyReceiptWithExplicitContent(t *testing.T) {
	weatherCode := 0
	tonProvider := &fakeTonProvider{price: finance.TonPrice{USD: 1.7}}
	fiatProvider := &fakeFiatProvider{rate: finance.FiatRate{BaseCode: "USD", QuoteCode: "BYN", Scale: 1, Rate: 3.12}}
	bankProvider := &fakeBankRatesProvider{}
	store := &fakeStore{
		config:         printer.Config{Host: "192.168.0.118", Port: 5555},
		location:       weather.Location{Name: "Гомель", Latitude: 52.4345, Longitude: 30.9754},
		receiptContent: receipt.DefaultContentSettings(),
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
		WithTonPriceProvider(tonProvider),
		WithFiatRateProvider(fiatProvider),
		WithBankRatesProvider(bankProvider),
		WithNewsProvider(&fakeNewsProvider{}),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)
	content := receipt.ContentSettings{
		Configured:       true,
		ShowWeather:      true,
		ShowUsdBynRate:   false,
		ShowBankRates:    false,
		ShowTonPortfolio: false,
	}

	if err := service.PrintDailyReceiptWithContent(context.Background(), content); err != nil {
		t.Fatalf("print daily receipt with content: %v", err)
	}

	if store.receiptContentLoadCalls != 0 {
		t.Fatalf("expected explicit content path not to load global receipt content, got %d calls", store.receiptContentLoadCalls)
	}
	if tonProvider.calls != 0 || fiatProvider.calls != 0 || bankProvider.calls != 0 {
		t.Fatalf("expected disabled finance providers not to be called, got ton=%d fiat=%d bank=%d", tonProvider.calls, fiatProvider.calls, bankProvider.calls)
	}
	if gateway.printedConfig != store.config {
		t.Fatalf("expected printer config %#v, got %#v", store.config, gateway.printedConfig)
	}
	if lineTextsContain(gateway.printedLines, "TON") || lineTextsContain(gateway.printedLines, "Курс доллара") || lineTextsContain(gateway.printedLines, "В банках") {
		t.Fatalf("expected explicit content to omit finance sections, got %#v", gateway.printedLines)
	}
}

func TestReceiptServiceBuildsDenisTrendsBlockWhenEnabled(t *testing.T) {
	store := &fakeStore{
		denisTrends: denistrends.DefaultSettings(),
	}
	provider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodDay,
				Title:  "Top day",
				Items: []denistrends.Item{
					{Title: "HN title", SourceName: "Hacker News", Link: "https://example.com/hn"},
				},
			},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithDenisTrendsProvider(provider),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, err := service.BuildDailyReceiptWithContent(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDenisTrends: true,
	})
	if err != nil {
		t.Fatalf("build daily receipt: %v", err)
	}

	if !provider.now.Equal(fixedClock()) {
		t.Fatalf("expected provider to receive service clock, got %v", provider.now)
	}
	if !lineTextsContain(lines, "Denis Trends") || !lineTextsContain(lines, "Hacker News: HN title") {
		t.Fatalf("expected Denis Trends receipt lines, got %#v", lines)
	}
	if link := receiptLinkForText(lines, "Hacker News: HN title"); link != "https://example.com/hn" {
		t.Fatalf("expected trend link on receipt line, got %q", link)
	}
}

func TestReceiptServiceUsesEmbeddedNewsSettingsForScheduledContent(t *testing.T) {
	translateTitles := false
	embedded := news.Settings{
		TranslateTitles: &translateTitles,
		Sources: []news.SourceSettings{
			{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/schedule.xml", MaxItems: 7},
			{Preset: news.PresetEconomist, Enabled: false, FeedURL: "https://example.com/economist.xml", MaxItems: 2},
		},
	}
	global := news.DefaultSettings()
	global.Sources[0].MaxItems = 1
	provider := &fakeNewsProvider{items: []news.Item{{Title: "Scheduled title", SourceName: "Reuters"}}}
	service := NewReceiptService(
		&fakeStore{newsSettings: global},
		&fakePrinter{},
		fixedClock,
		WithNewsProvider(provider),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	_, err := service.BuildDailyReceiptWithContent(context.Background(), receipt.ContentSettings{
		Configured:   true,
		ShowNews:     true,
		NewsSettings: &embedded,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}

	got := provider.settings.Normalized()
	want := embedded.Normalized()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected embedded news settings %#v, got %#v", want, got)
	}
}

func TestReceiptServiceUsesEmbeddedDenisTrendsSettingsWithoutMode(t *testing.T) {
	embedded := denistrends.DefaultSettings()
	embedded.Periods[denistrends.PeriodNow] = denistrends.PeriodSettings{Enabled: false, MaxItems: 20}
	embedded.Periods[denistrends.PeriodDay] = denistrends.PeriodSettings{Enabled: false, MaxItems: 20}
	embedded.Periods[denistrends.PeriodWeek] = denistrends.PeriodSettings{Enabled: true, MaxItems: 2}
	embedded.Periods[denistrends.PeriodMonth] = denistrends.PeriodSettings{Enabled: true, MaxItems: 3}
	global := denistrends.DefaultSettings()
	global.Periods[denistrends.PeriodMonth] = denistrends.PeriodSettings{Enabled: true, MaxItems: 99}
	provider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodMonth,
				Title:  "Top month",
				Items:  []denistrends.Item{{Title: "Monthly title", SourceName: "GitHub"}},
			},
		},
	}
	service := NewReceiptService(
		&fakeStore{denisTrends: global},
		&fakePrinter{},
		fixedClock,
		WithDenisTrendsProvider(provider),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	_, err := service.BuildDailyReceiptWithContent(context.Background(), receipt.ContentSettings{
		Configured:          true,
		ShowDenisTrends:     true,
		DenisTrendsSettings: &embedded,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}

	got := provider.settings.Normalized()
	want := embedded.Normalized()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected embedded Denis Trends settings %#v, got %#v", want, got)
	}
}

func TestReceiptServiceTranslatesEnglishDenisTrendTitles(t *testing.T) {
	translateTitles := true
	store := &fakeStore{
		denisTrends:  denistrends.DefaultSettings(),
		newsSettings: news.Settings{TranslateTitles: &translateTitles},
		motivationSettings: motivation.Settings{
			Configured: true,
			Enabled:    false,
			BaseURL:    motivation.DefaultBaseURL,
			Model:      motivation.DefaultModel,
		},
	}
	trendsProvider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodDay,
				Title:  "Top day",
				Items: []denistrends.Item{
					{Title: "Founder mode and startup growth", SourceName: "Hacker News", Link: "https://example.com/founder"},
					{Title: "Русский заголовок", SourceName: "GitHub", Link: "https://example.com/ru"},
				},
			},
		},
	}
	motivationProvider := &fakeMotivationProvider{
		translations: []motivation.NewsTranslation{
			{Index: 0, Title: "Рост стартапа"},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithDenisTrendsProvider(trendsProvider),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDenisTrends: true,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !lineTextsContain(lines, "Hacker News: Рост стартапа") {
		t.Fatalf("expected translated Denis Trends title, got %#v", lines)
	}
	if !lineTextsContain(lines, "Founder mode and startup growth") {
		t.Fatalf("expected original Denis Trends title, got %#v", lines)
	}
	if link := receiptLinkForText(lines, "Hacker News: Рост стартапа"); link != "https://example.com/founder" {
		t.Fatalf("expected translated trend link on title line, got %q", link)
	}
	if link := receiptLinkForText(lines, "Founder mode and startup growth"); link != "" {
		t.Fatalf("expected original trend title to stay plain, got link %q", link)
	}
	if len(motivationProvider.translatedTitles) != 1 {
		t.Fatalf("expected only non-cyrillic Denis Trends title to be translated, got %#v", motivationProvider.translatedTitles)
	}
	if got := motivationProvider.translatedTitles[0]; got.Index != 0 || got.SourceName != "Hacker News" || got.Title != "Founder mode and startup growth" {
		t.Fatalf("unexpected Denis Trends translation request: %#v", got)
	}
}

func TestReceiptServiceUsesEmbeddedNewsTranslationToggleForDenisTrends(t *testing.T) {
	globalTranslateTitles := false
	embeddedTranslateTitles := true
	embeddedNewsSettings := news.Settings{TranslateTitles: &embeddedTranslateTitles}
	store := &fakeStore{
		denisTrends:  denistrends.DefaultSettings(),
		newsSettings: news.Settings{TranslateTitles: &globalTranslateTitles},
	}
	trendsProvider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodDay,
				Title:  "Top day",
				Items: []denistrends.Item{
					{Title: "New database engine reaches beta", SourceName: "GitHub", Link: "https://example.com/db"},
				},
			},
		},
	}
	motivationProvider := &fakeMotivationProvider{
		translations: []motivation.NewsTranslation{
			{Index: 0, Title: "Новый движок базы данных вышел в бета-версию"},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithDenisTrendsProvider(trendsProvider),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowNews:        false,
		ShowDenisTrends: true,
		NewsSettings:    &embeddedNewsSettings,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !lineTextsContainSubstring(lines, "GitHub: Новый движок базы данных") || !lineTextsContain(lines, "вышел в бета-версию") {
		t.Fatalf("expected embedded translation toggle to translate Denis Trends, got %#v", lines)
	}
	if len(motivationProvider.translatedTitles) != 1 {
		t.Fatalf("expected Denis Trends title to be sent for translation, got %#v", motivationProvider.translatedTitles)
	}
}

func TestReceiptServiceDisablesDenisTrendsTranslationFromEmbeddedNewsSettings(t *testing.T) {
	globalTranslateTitles := true
	embeddedTranslateTitles := false
	embeddedNewsSettings := news.Settings{TranslateTitles: &embeddedTranslateTitles}
	store := &fakeStore{
		denisTrends:  denistrends.DefaultSettings(),
		newsSettings: news.Settings{TranslateTitles: &globalTranslateTitles},
	}
	trendsProvider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodDay,
				Title:  "Top day",
				Items: []denistrends.Item{
					{Title: "New database engine reaches beta", SourceName: "GitHub", Link: "https://example.com/db"},
				},
			},
		},
	}
	motivationProvider := &fakeMotivationProvider{
		translations: []motivation.NewsTranslation{
			{Index: 0, Title: "Новый движок базы данных вышел в бета-версию"},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithDenisTrendsProvider(trendsProvider),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowNews:        false,
		ShowDenisTrends: true,
		NewsSettings:    &embeddedNewsSettings,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if !lineTextsContainSubstring(lines, "GitHub: New database engine") || !lineTextsContain(lines, "reaches beta") {
		t.Fatalf("expected original Denis Trends title when embedded translation is off, got %#v", lines)
	}
	if len(motivationProvider.translatedTitles) != 0 {
		t.Fatalf("expected translation provider not to be called, got %#v", motivationProvider.translatedTitles)
	}
}

func TestReceiptServiceKeepsDenisTrendsOriginalWhenTranslationFails(t *testing.T) {
	translateTitles := true
	store := &fakeStore{
		denisTrends:  denistrends.DefaultSettings(),
		newsSettings: news.Settings{TranslateTitles: &translateTitles},
	}
	trendsProvider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodDay,
				Title:  "Top day",
				Items: []denistrends.Item{
					{Title: "GitHub releases a faster runner", SourceName: "GitHub", Link: "https://example.com/runner"},
				},
			},
		},
	}
	motivationProvider := &fakeMotivationProvider{translationErr: errors.New("ollama offline")}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithDenisTrendsProvider(trendsProvider),
		WithMotivationProvider(motivationProvider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDenisTrends: true,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected translation warning, got none")
	}
	if !lineTextsContainSubstring(lines, "GitHub releases a faster") || !lineTextsContain(lines, "runner") {
		t.Fatalf("expected original Denis Trends title after translation failure, got %#v", lines)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "AI-перевод Denis Trends недоступен") {
		t.Fatalf("expected Denis Trends translation warning, got %#v", warnings)
	}
}

func TestReceiptServiceBuildsDenisTrendsUsingScheduledTime(t *testing.T) {
	location, err := time.LoadLocation(denistrends.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	scheduledAt := time.Date(2026, 5, 25, 7, 0, 0, 0, location)
	store := &fakeStore{
		denisTrends: denistrends.DefaultSettings(),
	}
	provider := &fakeDenisTrendsProvider{
		sections: []denistrends.Section{
			{
				Period: denistrends.PeriodNow,
				Title:  "Top now",
				Items:  []denistrends.Item{{Title: "Morning title", SourceName: "GitHub", Link: "https://example.com/morning"}},
			},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		func() time.Time { return time.Date(2026, 5, 25, 18, 30, 0, 0, location) },
		WithDenisTrendsProvider(provider),
		WithMotivationProvider(&fakeMotivationProvider{}),
	)

	lines, err := service.BuildDailyReceiptWithContentAt(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDenisTrends: true,
	}, scheduledAt)
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}

	if !lineTextsContain(lines, "Top now") {
		t.Fatalf("expected scheduled morning Top now section, got %#v", lines)
	}
	if !provider.now.Equal(scheduledAt) {
		t.Fatalf("expected provider to receive scheduled time %s, got %s", scheduledAt, provider.now)
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

func TestReceiptServicePrintsDailyQuestsAfterMotivationQuote(t *testing.T) {
	store := &fakeStore{
		motivationSettings: motivation.Settings{
			Configured: true,
			Enabled:    true,
			BaseURL:    motivation.DefaultBaseURL,
			Model:      motivation.DefaultModel,
		},
	}
	provider := &fakeMotivationProvider{
		quote: motivation.Quote{Text: "Сегодня достаточно одного честного шага."},
		dailyQuests: []dailyquest.DailyQuest{
			{ID: 7, Text: "Составь карту любимых мест района."},
			{ID: 23, Text: "Почини одну небольшую вещь дома."},
			{ID: 48, Text: "Проживи день без жалоб."},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithMotivationProvider(provider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:          true,
		ShowWeather:         false,
		ShowWeatherAdvice:   false,
		ShowMotivationQuote: true,
		ShowDailyQuests:     true,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}

	quoteIndex := indexOfLineText(lines, "Сегодня достаточно одного")
	questsIndex := indexOfLineText(lines, "Квест на день")
	if quoteIndex < 0 || questsIndex < 0 || quoteIndex > questsIndex {
		t.Fatalf("expected daily quests after quote, got %#v", lines)
	}
	for _, want := range []string{
		"1. Составь карту любимых мест",
		"2. Почини одну небольшую вещь",
		"3. Проживи день без жалоб.",
	} {
		if !lineTextsContain(lines, want) {
			t.Fatalf("expected daily quest line %q, got %#v", want, lines)
		}
	}
	if provider.dailyQuestCalls != 1 || len(provider.dailyQuestInput) != 3 {
		t.Fatalf("expected one AI daily quest generation with three selected IDs, provider=%#v", provider)
	}
	if store.motivationSettings.QuestCacheDate != motivation.CacheDate(fixedClock()) ||
		len(store.motivationSettings.CachedDailyQuests) != 3 {
		t.Fatalf("expected generated daily quests to be cached, got %#v", store.motivationSettings)
	}
}

func TestReceiptServiceSkipsDailyQuestsWhenDisabled(t *testing.T) {
	provider := &fakeMotivationProvider{
		dailyQuests: []dailyquest.DailyQuest{{ID: 7, Text: "Составь карту района."}},
	}
	service := NewReceiptService(
		&fakeStore{},
		&fakePrinter{},
		fixedClock,
		WithMotivationProvider(provider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:          true,
		ShowWeather:         false,
		ShowWeatherAdvice:   false,
		ShowMotivationQuote: false,
		ShowDailyQuests:     false,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if lineTextsContain(lines, "Квест на день") {
		t.Fatalf("daily quests must not be printed when disabled, got %#v", lines)
	}
	if provider.dailyQuestCalls != 0 {
		t.Fatalf("expected daily quest provider not to be called, got %d", provider.dailyQuestCalls)
	}
}

func TestReceiptServiceUsesDailyQuestCacheForSameMinskDate(t *testing.T) {
	store := &fakeStore{
		motivationSettings: motivation.Settings{
			Configured:     true,
			Enabled:        true,
			BaseURL:        motivation.DefaultBaseURL,
			Model:          motivation.DefaultModel,
			QuestCacheDate: motivation.CacheDate(fixedClock()),
			CachedDailyQuests: []dailyquest.DailyQuest{
				{ID: 7, Text: "Кэшированный квест один."},
				{ID: 23, Text: "Кэшированный квест два."},
				{ID: 48, Text: "Кэшированный квест три."},
			},
		},
	}
	provider := &fakeMotivationProvider{
		dailyQuests: []dailyquest.DailyQuest{
			{ID: 7, Text: "Новый квест не нужен."},
			{ID: 23, Text: "Новый квест два."},
			{ID: 48, Text: "Новый квест три."},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithMotivationProvider(provider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDailyQuests: true,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if provider.dailyQuestCalls != 0 {
		t.Fatalf("expected cached daily quests without AI call, got %d calls", provider.dailyQuestCalls)
	}
	if !lineTextsContain(lines, "1. Кэшированный квест один.") ||
		!lineTextsContain(lines, "2. Кэшированный квест два.") ||
		!lineTextsContain(lines, "3. Кэшированный квест три.") {
		t.Fatalf("expected cached daily quests in receipt, got %#v", lines)
	}
}

func TestReceiptServiceRegeneratesDailyQuestsForEffectiveScheduledDate(t *testing.T) {
	location, err := time.LoadLocation(motivation.DefaultTimezone)
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	scheduledAt := time.Date(2026, 5, 26, 7, 0, 0, 0, location)
	store := &fakeStore{
		motivationSettings: motivation.Settings{
			Configured:     true,
			Enabled:        true,
			BaseURL:        motivation.DefaultBaseURL,
			Model:          motivation.DefaultModel,
			QuestCacheDate: motivation.CacheDate(fixedClock()),
			CachedDailyQuests: []dailyquest.DailyQuest{
				{ID: 7, Text: "Вчерашний квест один."},
				{ID: 23, Text: "Вчерашний квест два."},
				{ID: 48, Text: "Вчерашний квест три."},
			},
		},
	}
	provider := &fakeMotivationProvider{
		dailyQuests: []dailyquest.DailyQuest{
			{ID: 11, Text: "Нарисуй обычный предмет 20 минут."},
			{ID: 30, Text: "Освежи базу первой помощи."},
			{ID: 44, Text: "Спроси родителей про молодость."},
		},
	}
	service := NewReceiptService(
		store,
		&fakePrinter{},
		fixedClock,
		WithMotivationProvider(provider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAtAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDailyQuests: true,
	}, scheduledAt)
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if provider.dailyQuestCalls != 1 {
		t.Fatalf("expected AI generation for new scheduled date, got %d", provider.dailyQuestCalls)
	}
	if store.motivationSettings.QuestCacheDate != motivation.CacheDate(scheduledAt) {
		t.Fatalf("expected cache date to use scheduled time, got %#v", store.motivationSettings)
	}
	if !lineTextsContain(lines, "1. Нарисуй обычный предмет 20") || !lineTextsContain(lines, "минут.") {
		t.Fatalf("expected generated scheduled daily quests, got %#v", lines)
	}
}

func TestReceiptServiceFallsBackWhenDailyQuestAIUnavailable(t *testing.T) {
	provider := &fakeMotivationProvider{dailyQuestErr: errors.New("ollama offline")}
	service := NewReceiptService(
		&fakeStore{},
		&fakePrinter{},
		fixedClock,
		WithMotivationProvider(provider),
	)

	lines, warnings, err := service.BuildDailyReceiptWithContentAndWarnings(context.Background(), receipt.ContentSettings{
		Configured:      true,
		ShowDailyQuests: true,
	})
	if err != nil {
		t.Fatalf("build receipt: %v", err)
	}
	if !containsWarning(warnings, "AI-квесты дня недоступны") {
		t.Fatalf("expected daily quest warning, got %#v", warnings)
	}
	if !lineTextsContain(lines, "Квест на день") ||
		!lineTextsContainSubstring(lines, "1. ") ||
		!lineTextsContainSubstring(lines, "2. ") ||
		!lineTextsContainSubstring(lines, "3. ") {
		t.Fatalf("expected fallback daily quests in receipt, got %#v", lines)
	}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 25, 9, 7, 0, 0, time.UTC)
}

type fakeStore struct {
	config                  printer.Config
	location                weather.Location
	portfolio               finance.TonPortfolio
	newsSettings            news.Settings
	denisTrends             denistrends.Settings
	receiptStyle            receipt.StyleSettings
	receiptContent          receipt.ContentSettings
	receiptContentLoadCalls int
	snapshotSettings        receiptsnapshot.Settings
	motivationSettings      motivation.Settings
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

func (s *fakeStore) LoadDenisTrends() (denistrends.Settings, error) {
	if len(s.denisTrends.Periods) == 0 && len(s.denisTrends.Sources) == 0 {
		return denistrends.DefaultSettings(), nil
	}
	return s.denisTrends.Normalized(), nil
}

func (s *fakeStore) LoadReceiptStyle() (receipt.StyleSettings, error) {
	if s.receiptStyle == (receipt.StyleSettings{}) {
		return receipt.DefaultStyleSettings(), nil
	}
	return s.receiptStyle.Normalized(), nil
}

func (s *fakeStore) LoadReceiptContent() (receipt.ContentSettings, error) {
	s.receiptContentLoadCalls++
	content := s.receiptContent.Normalized()
	if s.receiptContent == (receipt.ContentSettings{}) {
		content.ShowOilPrice = false
	}
	return content, nil
}

func (s *fakeStore) LoadReceiptSnapshotSettings() (receiptsnapshot.Settings, error) {
	return s.snapshotSettings.Normalized(), nil
}

func (s *fakeStore) LoadMotivation() (motivation.Settings, error) {
	if reflect.DeepEqual(s.motivationSettings, motivation.Settings{}) {
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
	printErr      error
}

func (p *fakePrinter) PrintReceipt(_ context.Context, config printer.Config, lines []receipt.Line) error {
	p.printedConfig = config
	p.printedLines = append([]receipt.Line(nil), lines...)
	if p.printErr != nil {
		return p.printErr
	}
	return nil
}

type fakeReceiptSnapshotStore struct {
	id                  string
	err                 error
	finalizeErr         error
	createdItems        []receiptsnapshot.NewsItem
	createdLines        []receiptsnapshot.ReceiptLine
	finalizedID         string
	finalizedLines      []receiptsnapshot.ReceiptLine
	finalizedPaperChars int
	publishedID         string
	failedID            string
	failedErr           error
}

func (s *fakeReceiptSnapshotStore) Create(_ context.Context, input receiptsnapshot.CreateInput) (receiptsnapshot.Snapshot, error) {
	if s.err != nil {
		return receiptsnapshot.Snapshot{}, s.err
	}
	s.createdItems = append([]receiptsnapshot.NewsItem(nil), input.NewsItems...)
	s.createdLines = append([]receiptsnapshot.ReceiptLine(nil), input.ReceiptLines...)
	id := s.id
	if id == "" {
		id = "snapshot-1"
	}
	return receiptsnapshot.Snapshot{ID: id, Status: receiptsnapshot.StatusPending, NewsItems: input.NewsItems, ReceiptLines: input.ReceiptLines}, nil
}

func (s *fakeReceiptSnapshotStore) FinalizeReceiptLines(_ context.Context, id string, lines []receiptsnapshot.ReceiptLine, paperChars int) error {
	if s.finalizeErr != nil {
		return s.finalizeErr
	}
	s.finalizedID = id
	s.finalizedLines = append([]receiptsnapshot.ReceiptLine(nil), lines...)
	s.finalizedPaperChars = paperChars
	return nil
}

func (s *fakeReceiptSnapshotStore) Publish(_ context.Context, id string) error {
	s.publishedID = id
	return nil
}

func (s *fakeReceiptSnapshotStore) Fail(_ context.Context, id string, err error) error {
	s.failedID = id
	s.failedErr = err
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

type fakeOilProvider struct {
	price      finance.OilPrice
	chart      finance.OilMarketChart
	priceErr   error
	chartErr   error
	priceCalls int
	chartCalls int
}

func (p *fakeOilProvider) CurrentPrice(context.Context) (finance.OilPrice, error) {
	p.priceCalls++
	if p.priceErr != nil {
		return finance.OilPrice{}, p.priceErr
	}
	return p.price, nil
}

func (p *fakeOilProvider) MarketChart(context.Context) (finance.OilMarketChart, error) {
	p.chartCalls++
	if p.chartErr != nil {
		return finance.OilMarketChart{}, p.chartErr
	}
	return p.chart, nil
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
	items    []news.Item
	settings news.Settings
}

func (p *fakeNewsProvider) Current(_ context.Context, settings news.Settings) ([]news.Item, error) {
	p.settings = settings
	return p.items, nil
}

type fakeDenisTrendsProvider struct {
	sections []denistrends.Section
	now      time.Time
	settings denistrends.Settings
}

func (p *fakeDenisTrendsProvider) Current(_ context.Context, settings denistrends.Settings, now time.Time) ([]denistrends.Section, error) {
	p.settings = settings
	p.now = now
	return p.sections, nil
}

type fakeHistoryProvider struct {
	events []history.Event
	err    error
	calls  int
	date   time.Time
}

func (p *fakeHistoryProvider) Current(_ context.Context, date time.Time) ([]history.Event, error) {
	p.calls++
	p.date = date
	if p.err != nil {
		return nil, p.err
	}
	return append([]history.Event(nil), p.events...), nil
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
	historyFacts        []motivation.HistoryFact
	dailyQuests         []dailyquest.DailyQuest
	translations        []motivation.NewsTranslation
	err                 error
	calendarAdviceErr   error
	historyFactsErr     error
	dailyQuestErr       error
	translationErr      error
	weatherContext      motivation.WeatherContext
	calendarContext     motivation.CalendarContext
	historyEvents       []motivation.HistoryEvent
	dailyQuestInput     []dailyquest.Quest
	translatedTitles    []motivation.NewsTitle
	quoteCalls          int
	adviceCalls         int
	calendarAdviceCalls int
	historyFactCalls    int
	dailyQuestCalls     int
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

func (p *fakeMotivationProvider) GenerateHistoryFacts(_ context.Context, _ motivation.Settings, events []motivation.HistoryEvent) ([]motivation.HistoryFact, error) {
	p.historyFactCalls++
	p.historyEvents = append([]motivation.HistoryEvent(nil), events...)
	if p.historyFactsErr != nil {
		return nil, p.historyFactsErr
	}
	return append([]motivation.HistoryFact(nil), p.historyFacts...), p.err
}

func (p *fakeMotivationProvider) GenerateDailyQuests(_ context.Context, _ motivation.Settings, quests []dailyquest.Quest) ([]dailyquest.DailyQuest, error) {
	p.dailyQuestCalls++
	p.dailyQuestInput = append([]dailyquest.Quest(nil), quests...)
	if p.dailyQuestErr != nil {
		return nil, p.dailyQuestErr
	}
	if len(p.dailyQuests) == 0 {
		return dailyquest.Fallback(quests), p.err
	}
	result := append([]dailyquest.DailyQuest(nil), p.dailyQuests...)
	for index := range result {
		if index >= len(quests) {
			break
		}
		result[index].ID = quests[index].ID
	}
	return result, p.err
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

func receiptLinkForText(lines []receipt.Line, text string) string {
	for _, line := range lines {
		if line.Text == text {
			return line.Link
		}
	}
	return ""
}

func lineTextsContainSubstring(lines []receipt.Line, want string) bool {
	for _, line := range lines {
		if strings.Contains(line.Text, want) {
			return true
		}
	}
	return false
}

func indexOfLineText(lines []receipt.Line, want string) int {
	for index, line := range lines {
		if line.Text == want {
			return index
		}
	}
	return -1
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

func receiptLinesContainQRCode(lines []receipt.Line, value string) bool {
	for _, line := range lines {
		if value == "" {
			if line.QRCode != "" {
				return true
			}
			continue
		}
		if line.QRCode == value {
			return true
		}
	}
	return false
}

func requireQRCodeSeparatedFromReceiptContent(t *testing.T, lines []receipt.Line, value string) {
	t.Helper()
	index := receiptQRCodeIndex(lines, value)
	if index < 3 {
		t.Fatalf("expected QR code to have blank line, separator, and blank line before it, got %#v", lines)
	}
	if strings.TrimSpace(lines[index-3].Text) != "" {
		t.Fatalf("expected blank line before QR separator, got %#v in %#v", lines[index-3], lines)
	}
	if lines[index-2].Text != strings.Repeat("-", 16) || lines[index-2].Alignment != receipt.AlignmentCenter {
		t.Fatalf("expected centered separator before QR, got %#v in %#v", lines[index-2], lines)
	}
	if strings.TrimSpace(lines[index-1].Text) != "" {
		t.Fatalf("expected blank line after QR separator, got %#v in %#v", lines[index-1], lines)
	}
}

func receiptQRCodeIndex(lines []receipt.Line, value string) int {
	for index, line := range lines {
		if line.QRCode == value {
			return index
		}
	}
	return -1
}

func receiptLinesContainAnyQRCode(lines []receipt.Line) bool {
	for _, line := range lines {
		if strings.TrimSpace(line.QRCode) != "" {
			return true
		}
	}
	return false
}

func snapshotReceiptLinesContain(lines []receiptsnapshot.ReceiptLine, want string) bool {
	for _, line := range lines {
		if line.Text == want {
			return true
		}
	}
	return false
}

func snapshotReceiptLinesContainQRCode(lines []receiptsnapshot.ReceiptLine, value string) bool {
	for _, line := range lines {
		if line.QRCode == value {
			return true
		}
	}
	return false
}

func requireSnapshotQRCodeSeparatedFromReceiptContent(t *testing.T, lines []receiptsnapshot.ReceiptLine, value string) {
	t.Helper()
	index := snapshotReceiptQRCodeIndex(lines, value)
	if index < 3 {
		t.Fatalf("expected snapshot QR code to have blank line, separator, and blank line before it, got %#v", lines)
	}
	if strings.TrimSpace(lines[index-3].Text) != "" {
		t.Fatalf("expected blank snapshot line before QR separator, got %#v in %#v", lines[index-3], lines)
	}
	if lines[index-2].Text != strings.Repeat("-", 16) || lines[index-2].Alignment != string(receipt.AlignmentCenter) {
		t.Fatalf("expected centered snapshot separator before QR, got %#v in %#v", lines[index-2], lines)
	}
	if strings.TrimSpace(lines[index-1].Text) != "" {
		t.Fatalf("expected blank snapshot line after QR separator, got %#v in %#v", lines[index-1], lines)
	}
}

func snapshotReceiptQRCodeIndex(lines []receiptsnapshot.ReceiptLine, value string) int {
	for index, line := range lines {
		if line.QRCode == value {
			return index
		}
	}
	return -1
}

func snapshotReceiptLinesContainLinkedText(lines []receiptsnapshot.ReceiptLine, textPart string, link string) bool {
	for _, line := range lines {
		if strings.Contains(line.Text, textPart) && line.Link == link {
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
