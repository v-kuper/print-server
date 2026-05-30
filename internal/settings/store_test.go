package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"atol-server/internal/finance"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/schedule"
	"atol-server/internal/weather"
)

func TestStoreLoadReturnsDefaultWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	config, err := store.LoadPrinter()
	if err != nil {
		t.Fatalf("load default settings: %v", err)
	}

	if config != printer.DefaultConfig() {
		t.Fatalf("expected default config %#v, got %#v", printer.DefaultConfig(), config)
	}
}

func TestStoreLoadReturnsDefaultWeatherWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	location, err := store.LoadWeather()
	if err != nil {
		t.Fatalf("load default weather settings: %v", err)
	}

	if location != weather.DefaultLocation() {
		t.Fatalf("expected default weather location %#v, got %#v", weather.DefaultLocation(), location)
	}
}

func TestStoreLoadReturnsDefaultFinanceWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	portfolio, err := store.LoadFinance()
	if err != nil {
		t.Fatalf("load default finance settings: %v", err)
	}

	if portfolio != finance.DefaultTonPortfolio() {
		t.Fatalf("expected default portfolio %#v, got %#v", finance.DefaultTonPortfolio(), portfolio)
	}
}

func TestStoreLoadReturnsDefaultNewsWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	settings, err := store.LoadNews()
	if err != nil {
		t.Fatalf("load default news settings: %v", err)
	}

	if len(settings.Sources) != len(news.Presets()) {
		t.Fatalf("expected all presets, got %#v", settings.Sources)
	}
	if !settings.Sources[0].Enabled || settings.Sources[0].MaxItems != news.DefaultMaxItems {
		t.Fatalf("unexpected first news source defaults: %#v", settings.Sources[0])
	}
}

func TestStoreLoadReturnsDefaultReceiptStyleWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	style, err := store.LoadReceiptStyle()
	if err != nil {
		t.Fatalf("load default receipt style: %v", err)
	}

	if style != receipt.DefaultStyleSettings() {
		t.Fatalf("expected default receipt style %#v, got %#v", receipt.DefaultStyleSettings(), style)
	}
}

func TestStoreLoadReturnsDefaultReceiptContentWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	content, err := store.LoadReceiptContent()
	if err != nil {
		t.Fatalf("load default receipt content: %v", err)
	}

	want := receipt.DefaultContentSettings()
	if content != want {
		t.Fatalf("expected default receipt content %#v, got %#v", want, content)
	}
	if content.ShowMail {
		t.Fatalf("mail must be disabled by default, got %#v", content)
	}
	if !content.ShowCalendar || !content.ShowHistory || !content.ShowWeather || !content.ShowUsdBynRate || !content.ShowBankRates {
		t.Fatalf("expected main receipt sections enabled by default, got %#v", content)
	}
}

func TestStoreMigratesOldContentSettingsToShowHistoryByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	oldSettings := `{
		"receiptContent": {
			"configured": true,
			"showWeather": true,
			"showWeatherAdvice": false,
			"showMotivationQuote": false,
			"showTonPortfolio": false,
			"showUsdBynRate": true,
			"showBankRates": false,
			"showMail": false,
			"showCalendar": true,
			"showNews": false
		},
		"schedule": {
			"enabled": true,
			"mode": "daily_times",
			"intervalMinutes": 15,
			"timezone": "Europe/Minsk",
			"times": ["07:00"],
			"intervalContent": {
				"configured": true,
				"showWeather": true,
				"showNews": false
			},
			"runs": [{
				"time": "07:00",
				"profile": "custom",
				"content": {
					"configured": true,
					"showWeather": true,
					"showNews": false
				}
			}]
		}
	}`
	if err := os.WriteFile(path, []byte(oldSettings), 0o644); err != nil {
		t.Fatalf("write old settings: %v", err)
	}

	store := NewStore(path)
	content, err := store.LoadReceiptContent()
	if err != nil {
		t.Fatalf("load receipt content: %v", err)
	}
	if !content.ShowHistory || content.ShowNews {
		t.Fatalf("expected migrated history=true and preserved news=false, got %#v", content)
	}

	scheduleSettings, err := store.LoadSchedule()
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if scheduleSettings.IntervalContent == nil || !scheduleSettings.IntervalContent.ShowHistory || scheduleSettings.IntervalContent.ShowNews {
		t.Fatalf("expected migrated interval content, got %#v", scheduleSettings.IntervalContent)
	}
	if len(scheduleSettings.Runs) != 1 || scheduleSettings.Runs[0].Content == nil || !scheduleSettings.Runs[0].Content.ShowHistory || scheduleSettings.Runs[0].Content.ShowNews {
		t.Fatalf("expected migrated run content, got %#v", scheduleSettings.Runs)
	}
}

func TestStoreLoadReturnsDefaultScheduleWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	settings, err := store.LoadSchedule()
	if err != nil {
		t.Fatalf("load default schedule: %v", err)
	}

	if !reflectScheduleSettingsEqual(settings, schedule.DefaultSettings()) {
		t.Fatalf("expected default schedule %#v, got %#v", schedule.DefaultSettings(), settings)
	}
}

func TestStoreLoadReturnsDefaultMotivationWhenFileDoesNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))

	settings, err := store.LoadMotivation()
	if err != nil {
		t.Fatalf("load default motivation settings: %v", err)
	}

	if settings != motivation.DefaultSettings() {
		t.Fatalf("expected default motivation settings %#v, got %#v", motivation.DefaultSettings(), settings)
	}
}

func TestStoreSavesAndLoadsPrinterConfig(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := printer.Config{Host: " 192.168.0.118 ", Port: 5555}

	if err := store.SavePrinter(want); err != nil {
		t.Fatalf("save printer config: %v", err)
	}

	got, err := store.LoadPrinter()
	if err != nil {
		t.Fatalf("load printer config: %v", err)
	}

	if got != want.Normalized() {
		t.Fatalf("expected config %#v, got %#v", want.Normalized(), got)
	}
}

func TestStoreRejectsInvalidPrinterConfig(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	config := printer.Config{Host: "bad", Port: 5555}

	if err := store.SavePrinter(config); err == nil {
		t.Fatal("expected invalid printer config to be rejected")
	}
}

func TestStoreSavesAndLoadsWeatherLocation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := weather.Location{Name: " Гомель ", Latitude: 52.4345, Longitude: 30.9754}

	if err := store.SaveWeather(want); err != nil {
		t.Fatalf("save weather location: %v", err)
	}

	got, err := store.LoadWeather()
	if err != nil {
		t.Fatalf("load weather location: %v", err)
	}

	if got != want.Normalized() {
		t.Fatalf("expected location %#v, got %#v", want.Normalized(), got)
	}
}

func TestStoreRejectsInvalidWeatherLocation(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	location := weather.Location{Name: "Гомель", Latitude: 120, Longitude: 30.9754}

	if err := store.SaveWeather(location); err == nil {
		t.Fatal("expected invalid weather location to be rejected")
	}
}

func TestStoreSavesAndLoadsFinanceSettings(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := finance.TonPortfolio{AmountTon: 10.5, InvestedUSD: 20.25}

	if err := store.SaveFinance(want); err != nil {
		t.Fatalf("save finance settings: %v", err)
	}

	got, err := store.LoadFinance()
	if err != nil {
		t.Fatalf("load finance settings: %v", err)
	}
	if got != want {
		t.Fatalf("expected portfolio %#v, got %#v", want, got)
	}
}

func TestStoreSavesAndLoadsNewsSettings(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := news.Settings{
		Sources: []news.SourceSettings{
			{Preset: news.PresetReuters, Enabled: false, FeedURL: "https://example.com/reuters.xml", MaxItems: 200},
			{Preset: news.PresetEconomist, Enabled: false, FeedURL: "https://example.com/economist.xml", MaxItems: 0},
			{Preset: news.PresetHackerNews, Enabled: true, FeedURL: "https://example.com/hn.xml", MaxItems: 1},
		},
	}

	if err := store.SaveNews(want); err != nil {
		t.Fatalf("save news settings: %v", err)
	}

	got, err := store.LoadNews()
	if err != nil {
		t.Fatalf("load news settings: %v", err)
	}

	normalized := want.Normalized()
	if len(got.Sources) != len(normalized.Sources) {
		t.Fatalf("expected %d news sources, got %d", len(normalized.Sources), len(got.Sources))
	}
	if got.Sources[0].MaxItems != news.MaxItems {
		t.Fatalf("expected max items to be clamped to %d, got %d", news.MaxItems, got.Sources[0].MaxItems)
	}
	if got.Sources[1].MaxItems != news.MinItems {
		t.Fatalf("expected min items to be clamped to %d, got %d", news.MinItems, got.Sources[1].MaxItems)
	}
}

func TestStoreRejectsEnabledNewsSourceWithoutCount(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	settings := news.Settings{
		Sources: []news.SourceSettings{
			{Preset: news.PresetReuters, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 0},
		},
	}

	if err := store.SaveNews(settings); err == nil {
		t.Fatal("expected enabled news source without count to be rejected")
	}
}

func TestStoreSavesAndLoadsReceiptStyle(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := receipt.StyleSettings{
		Configured:              true,
		NormalFont:              1,
		EmphasisFont:            2,
		CalendarDoubleWidth:     false,
		CalendarDoubleHeight:    true,
		TemperatureDoubleWidth:  true,
		TemperatureDoubleHeight: false,
	}

	if err := store.SaveReceiptStyle(want); err != nil {
		t.Fatalf("save receipt style: %v", err)
	}

	got, err := store.LoadReceiptStyle()
	if err != nil {
		t.Fatalf("load receipt style: %v", err)
	}
	if got != want.Normalized() {
		t.Fatalf("expected receipt style %#v, got %#v", want.Normalized(), got)
	}
}

func TestStoreSavesAndLoadsReceiptContent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := receipt.ContentSettings{
		Configured:          true,
		ShowWeather:         false,
		ShowWeatherAdvice:   false,
		ShowMotivationQuote: true,
		ShowTonPortfolio:    false,
		ShowUsdBynRate:      true,
		ShowBankRates:       false,
		ShowMail:            true,
		ShowCalendar:        false,
		ShowHistory:         true,
		ShowNews:            true,
	}

	if err := store.SaveReceiptContent(want); err != nil {
		t.Fatalf("save receipt content: %v", err)
	}

	got, err := store.LoadReceiptContent()
	if err != nil {
		t.Fatalf("load receipt content: %v", err)
	}
	if got != want.Normalized() {
		t.Fatalf("expected receipt content %#v, got %#v", want.Normalized(), got)
	}
}

func TestStoreSavesAndLoadsScheduleSettings(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := schedule.Settings{
		Enabled:         true,
		Mode:            schedule.ModeDailyTimes,
		IntervalMinutes: 0,
		Times:           []string{"09:00", "07:00", "09:00"},
		Timezone:        "",
	}

	if err := store.SaveSchedule(want); err != nil {
		t.Fatalf("save schedule: %v", err)
	}

	got, err := store.LoadSchedule()
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if !reflectScheduleSettingsEqual(got, want.Normalized()) {
		t.Fatalf("expected schedule %#v, got %#v", want.Normalized(), got)
	}
}

func TestStoreRejectsInvalidScheduleSettings(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := schedule.DefaultSettings()
	want.Enabled = true
	want.Mode = schedule.ModeDailyTimes
	want.Times = []string{"25:00"}

	if err := store.SaveSchedule(want); err == nil {
		t.Fatal("expected invalid schedule to be rejected")
	}
}

func TestStoreSavesAndLoadsScheduleState(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	nextRun := time.Date(2026, 5, 25, 7, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	want := schedule.State{
		LastAttemptAt: time.Date(2026, 5, 24, 7, 0, 0, 0, time.UTC),
		LastSuccessAt: time.Date(2026, 5, 24, 7, 0, 4, 0, time.UTC),
		LastError:     "printer offline",
		NextRunAt:     nextRun,
	}

	if err := store.SaveScheduleState(want); err != nil {
		t.Fatalf("save schedule state: %v", err)
	}

	got, err := store.LoadScheduleState()
	if err != nil {
		t.Fatalf("load schedule state: %v", err)
	}
	if !got.LastAttemptAt.Equal(want.LastAttemptAt) ||
		!got.LastSuccessAt.Equal(want.LastSuccessAt) ||
		got.LastError != want.LastError ||
		!got.NextRunAt.Equal(want.NextRunAt) {
		t.Fatalf("expected schedule state %#v, got %#v", want, got)
	}
}

func TestStoreSavesAndLoadsMotivationSettings(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	want := motivation.Settings{
		Enabled:     true,
		BaseURL:     " http://localhost:11434 ",
		Model:       " gemma4:31b-cloud ",
		CacheDate:   "2026-05-25",
		CachedQuote: "Делай важное спокойно.",
		LastError:   "old error",
	}

	if err := store.SaveMotivation(want); err != nil {
		t.Fatalf("save motivation settings: %v", err)
	}

	got, err := store.LoadMotivation()
	if err != nil {
		t.Fatalf("load motivation settings: %v", err)
	}
	if got != want.Normalized() {
		t.Fatalf("expected motivation settings %#v, got %#v", want.Normalized(), got)
	}
}

func reflectScheduleSettingsEqual(left schedule.Settings, right schedule.Settings) bool {
	if left.Enabled != right.Enabled ||
		left.Mode != right.Mode ||
		left.IntervalMinutes != right.IntervalMinutes ||
		left.Timezone != right.Timezone ||
		len(left.Times) != len(right.Times) {
		return false
	}
	for index := range left.Times {
		if left.Times[index] != right.Times[index] {
			return false
		}
	}
	return true
}
