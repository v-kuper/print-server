package receipt

import (
	"strings"
	"testing"
	"time"

	"atol-server/internal/bankrates"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/weather"
)

func TestWeatherReceiptFormatsCalendarAndWeatherDetails(t *testing.T) {
	weatherCode := 61
	precipitation := 0.2
	wind := 11.3
	pressure := 1012.4
	day := 24.1
	night := 12.6
	minsk, err := time.LoadLocation("Europe/Minsk")
	if err != nil {
		t.Fatalf("load Minsk location: %v", err)
	}

	lines := WeatherReceipt(weather.Snapshot{
		Timezone:           "Europe/Minsk",
		ObservedAt:         time.Date(2026, 5, 24, 9, 15, 0, 0, minsk),
		TemperatureC:       21.4,
		WeatherCode:        &weatherCode,
		PrecipitationMm:    &precipitation,
		WindSpeedMs:        &wind,
		SurfacePressureHpa: &pressure,
		DayTemperatureC:    &day,
		NightTemperatureC:  &night,
	})

	got := texts(lines)
	want := []string{
		" ",
		"24 Мая",
		"Воскресенье",
		" ",
		"",
		"Небольшой дождь",
		"+21 C",
		" ",
		"Днем +24 C",
		"Ночью +13 C",
		"Ветер 11 м/с",
		"Давление 1012 гПа",
		"Осадки 0.2 мм",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d: expected %q, got %q", i, want[i], got[i])
		}
	}
	if !lines[1].DoubleWidth || !lines[1].DoubleHeight {
		t.Fatalf("expected calendar title to be large: %#v", lines[1])
	}
	if lines[4].ImageKey != "rain" {
		t.Fatalf("expected printable rain icon line, got %#v", lines[4])
	}
	if !lines[6].DoubleWidth || !lines[6].DoubleHeight {
		t.Fatalf("expected temperature to be large: %#v", lines[6])
	}
}

func TestWeatherReceiptDoesNotPrintTechnicalTitle(t *testing.T) {
	lines := WeatherReceipt(weather.Snapshot{
		ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
		TemperatureC: 21.4,
	})

	for _, line := range lines {
		if line.Text == "Погода сейчас" {
			t.Fatal("weather receipt must not include the old technical title")
		}
	}
}

func TestDailyReceiptAppendsFinanceAndNewsBlocks(t *testing.T) {
	weatherCode := 0
	lines := DailyReceipt(DailyReceiptData{
		Weather: weather.Snapshot{
			ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
			TemperatureC: 21.4,
			WeatherCode:  &weatherCode,
		},
		TonPortfolio: &finance.TonPortfolioSummary{
			AmountTon:       1230.591,
			Price:           finance.TonPrice{USD: 1.7435687405482407},
			CurrentValueUSD: 2145.62,
			ProfitLossUSD:   168.04,
		},
		TonChartImage: &Image{
			Path:   "/tmp/ton-24h.png",
			URL:    "/assets/generated/ton-24h.png",
			Width:  384,
			Height: 96,
		},
		USDBYNRate: &finance.FiatRate{
			BaseCode:  "USD",
			QuoteCode: "BYN",
			Scale:     1,
			Rate:      3.1234,
		},
		USDBYNChartImage: &Image{
			Path:   "/tmp/usd-byn-7d.png",
			URL:    "/assets/generated/usd-byn-7d.png",
			Width:  384,
			Height: 96,
		},
		BankRates: &bankrates.Summary{
			Source: "TheMoney.by",
			SellUSD: &bankrates.Offer{
				Rate:      3.255,
				BankNames: []string{"Банк Б", "Банк В"},
			},
			BuyUSD: &bankrates.Offer{
				Rate:      3.279,
				BankNames: []string{"Банк Д"},
			},
		},
		WeatherAdvice:   &motivation.WeatherAdvice{Text: "Возьми зонт."},
		MotivationQuote: &motivation.Quote{Text: "Сегодня достаточно одного честного шага."},
		NewsItems: []news.Item{
			{Title: "Первый заголовок", SourceName: "BBC Russian"},
			{Title: "Второй заголовок", SourceName: "BBC Russian"},
			{Title: "Atom заголовок", SourceName: "Hacker News"},
		},
	})

	got := texts(lines)
	requireContains(t, got, "TON")
	requireContains(t, got, "TON $1.744 x1230.591")
	requireContains(t, got, "$2145.62 P/L +$168.04")
	requireContainsImage(t, lines, "/tmp/ton-24h.png", "/assets/generated/ton-24h.png")
	requireContains(t, got, "Курс доллара")
	requireContains(t, got, "USD/BYN 3.1234")
	requireContainsImage(t, lines, "/tmp/usd-byn-7d.png", "/assets/generated/usd-byn-7d.png")
	requireContains(t, got, "В банках")
	requireContains(t, got, "Продать $ 3.2550")
	requireContains(t, got, "Банк Б / Банк В")
	requireContains(t, got, "Купить $ 3.2790")
	requireContains(t, got, "Банк Д")
	requireContains(t, got, "Коротко о мире:")
	requireContains(t, got, "BBC Russian")
	requireContains(t, got, "- Первый заголовок")
	requireContains(t, got, "Hacker News")

	adviceIndex := indexOfText(got, "Возьми зонт.")
	quoteIndex := indexOfTextContaining(got, "Сегодня достаточно")
	tonIndex := indexOfText(got, "TON")
	if adviceIndex < 0 || quoteIndex < 0 || tonIndex < 0 || adviceIndex > quoteIndex || quoteIndex > tonIndex {
		t.Fatalf("expected weather advice then quote before TON, got %#v", got)
	}
}

func requireContainsImage(t *testing.T, lines []Line, path string, url string) {
	t.Helper()
	for _, line := range lines {
		if line.ImagePath == path && line.ImageURL == url {
			if line.ImageWidth != 384 || line.ImageHeight != 96 {
				t.Fatalf("expected chart image dimensions 384x96, got %#v", line)
			}
			return
		}
	}
	t.Fatalf("expected image %q/%q in lines, got %#v", path, url, lines)
}

func TestDailyReceiptPrintsTranslatedNewsWithOriginalInSmallerFont(t *testing.T) {
	weatherCode := 0
	lines := DailyReceiptWithStyle(DailyReceiptData{
		Weather: weather.Snapshot{
			ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
			TemperatureC: 21.4,
			WeatherCode:  &weatherCode,
		},
		NewsItems: []news.Item{
			{
				Title:         "Reuters готовит новый обзор рынков",
				OriginalTitle: "Reuters prepares a new market wrap",
				SourceName:    "Reuters",
			},
		},
	}, StyleSettings{
		Configured: true,
		NormalFont: 0,
	})

	got := texts(lines)
	translatedIndex := indexOfText(got, "- Reuters готовит новый обзор")
	if translatedIndex < 0 {
		translatedIndex = indexOfTextContaining(got, "Reuters готовит")
	}
	originalIndex := indexOfText(got, "Reuters prepares a new market")
	if originalIndex < 0 {
		originalIndex = indexOfTextContaining(got, "Reuters prepares")
	}
	if translatedIndex < 0 || originalIndex < 0 || originalIndex <= translatedIndex {
		t.Fatalf("expected translated title before original title, got %#v", got)
	}
	if lines[originalIndex].Role != RoleOriginal {
		t.Fatalf("expected original title to use original role, got %#v", lines[originalIndex])
	}
	if lines[originalIndex].Font <= lines[translatedIndex].Font {
		t.Fatalf("expected original title to use smaller receipt font, got translated=%#v original=%#v", lines[translatedIndex], lines[originalIndex])
	}
}

func TestDailyReceiptAppendsMailAndCalendarBeforeNews(t *testing.T) {
	weatherCode := 0
	lines := DailyReceipt(DailyReceiptData{
		Weather: weather.Snapshot{
			ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
			TemperatureC: 21.4,
			WeatherCode:  &weatherCode,
		},
		MailMessages: []googleintegration.MailMessage{
			{From: "Alice", Subject: "Morning update"},
			{From: "Bob", Subject: "Invoice"},
		},
		CalendarEvents: []googleintegration.CalendarEvent{
			{TimeLabel: "18:30", Title: "Ветеринар"},
			{TimeLabel: "Весь день", Title: "День без встреч"},
		},
		NewsItems: []news.Item{{Title: "Новость", SourceName: "BBC Russian"}},
	})

	got := texts(lines)
	requireContains(t, got, "Почта")
	requireContains(t, got, "Alice")
	requireContains(t, got, "Morning update")
	requireContains(t, got, "Календарь")
	requireContains(t, got, "18:30      Ветеринар")
	requireContains(t, got, "Весь день  День без встреч")

	mailIndex := indexOfText(got, "Почта")
	calendarIndex := indexOfText(got, "Календарь")
	newsIndex := indexOfText(got, "Коротко о мире:")
	if mailIndex < 0 || calendarIndex < 0 || newsIndex < 0 || mailIndex > calendarIndex || calendarIndex > newsIndex {
		t.Fatalf("expected mail then calendar before news, got %#v", got)
	}
}

func TestDailyReceiptPrintsCalendarEventsInTimeAndTitleColumns(t *testing.T) {
	weatherCode := 0
	lines := DailyReceipt(DailyReceiptData{
		Weather: weather.Snapshot{
			ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
			TemperatureC: 21.4,
			WeatherCode:  &weatherCode,
		},
		CalendarEvents: []googleintegration.CalendarEvent{
			{TimeLabel: "09:00", Title: "Длинная встреча с командой по продукту"},
		},
	})

	got := texts(lines)
	firstLine := indexOfText(got, "09:00      Длинная встреча с")
	if firstLine < 0 {
		t.Fatalf("expected calendar time and title columns, got %#v", got)
	}
	if firstLine+1 >= len(got) || got[firstLine+1] != "           командой по продукту" {
		t.Fatalf("expected wrapped title to stay in title column, got %#v", got)
	}
	if lines[firstLine].Alignment != AlignmentLeft || lines[firstLine+1].Alignment != AlignmentLeft {
		t.Fatalf("expected calendar event lines to be left aligned, got %#v and %#v", lines[firstLine], lines[firstLine+1])
	}
}

func TestDailyReceiptSeparatesMajorSectionsWithBlankLines(t *testing.T) {
	weatherCode := 0
	lines := DailyReceipt(DailyReceiptData{
		Weather: weather.Snapshot{
			ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
			TemperatureC: 21.4,
			WeatherCode:  &weatherCode,
		},
		TonPortfolio: &finance.TonPortfolioSummary{
			AmountTon:       1230.591,
			Price:           finance.TonPrice{USD: 1.7435687405482407},
			CurrentValueUSD: 2145.62,
			ProfitLossUSD:   168.04,
		},
		USDBYNRate: &finance.FiatRate{
			BaseCode:  "USD",
			QuoteCode: "BYN",
			Scale:     1,
			Rate:      3.1234,
		},
		BankRates: &bankrates.Summary{
			SellUSD: &bankrates.Offer{Rate: 3.255, BankNames: []string{"Банк Б"}},
		},
		MailMessages: []googleintegration.MailMessage{
			{From: "Alice", Subject: "Morning update"},
		},
		CalendarEvents: []googleintegration.CalendarEvent{
			{TimeLabel: "18:30", Title: "Ветеринар"},
		},
		NewsItems: []news.Item{{Title: "Новость", SourceName: "BBC Russian"}},
	})

	got := texts(lines)
	for _, title := range []string{
		"TON",
		"Курс доллара",
		"В банках",
		"Почта",
		"Календарь",
		"Коротко о мире:",
	} {
		requireSectionStartsAfterBlankLine(t, got, title)
	}

	newsIndex := indexOfText(got, "Коротко о мире:")
	if newsIndex < 0 || newsIndex+1 >= len(got) || strings.TrimSpace(got[newsIndex+1]) != "" {
		t.Fatalf("expected blank line between news title and first source, got %#v", got)
	}
}

func TestDailyReceiptWithStyleAppliesFontSettingsAndRoles(t *testing.T) {
	weatherCode := 0
	lines := DailyReceiptWithStyle(DailyReceiptData{
		Weather: weather.Snapshot{
			ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
			TemperatureC: 21.4,
			WeatherCode:  &weatherCode,
		},
	}, StyleSettings{
		Configured:              true,
		NormalFont:              2,
		EmphasisFont:            3,
		CalendarDoubleWidth:     false,
		CalendarDoubleHeight:    true,
		TemperatureDoubleWidth:  false,
		TemperatureDoubleHeight: false,
	})

	if lines[1].Text != "24 Мая" || lines[1].Role != RoleCalendar {
		t.Fatalf("expected calendar line with role, got %#v", lines[1])
	}
	if lines[1].Font != 3 || lines[1].DoubleWidth || !lines[1].DoubleHeight {
		t.Fatalf("expected calendar font/settings from style, got %#v", lines[1])
	}
	if lines[4].ImageKey != "clear" {
		t.Fatalf("expected weather icon line, got %#v", lines[4])
	}
	if lines[5].Role != RoleNormal || lines[5].Font != 2 {
		t.Fatalf("expected condition to use normal style, got %#v", lines[5])
	}
	if lines[6].Text != "+21 C" || lines[6].Role != RoleTemperature {
		t.Fatalf("expected temperature line with role, got %#v", lines[6])
	}
	if lines[6].Font != 3 || lines[6].DoubleWidth || lines[6].DoubleHeight {
		t.Fatalf("expected temperature font/settings from style, got %#v", lines[6])
	}
}

func TestDailyReceiptWithStyleCanUseDifferentCalendarAndTemperatureFonts(t *testing.T) {
	lines := WeatherReceiptWithStyle(weather.Snapshot{
		ObservedAt:   time.Date(2026, 5, 24, 9, 15, 0, 0, time.UTC),
		TemperatureC: 21.4,
	}, StyleSettings{
		Configured:              true,
		CalendarFont:            1,
		TemperatureFont:         4,
		CalendarDoubleWidth:     true,
		CalendarDoubleHeight:    false,
		TemperatureDoubleWidth:  false,
		TemperatureDoubleHeight: true,
	})

	if lines[1].Text != "24 Мая" || lines[1].Font != 1 || !lines[1].DoubleWidth || lines[1].DoubleHeight {
		t.Fatalf("expected calendar-specific style, got %#v", lines[1])
	}
	if lines[6].Text != "+21 C" || lines[6].Font != 4 || lines[6].DoubleWidth || !lines[6].DoubleHeight {
		t.Fatalf("expected temperature-specific style, got %#v", lines[6])
	}
}

func requireContains(t *testing.T, lines []string, want string) {
	t.Helper()
	for _, line := range lines {
		if line == want {
			return
		}
	}
	t.Fatalf("expected lines to contain %q, got %#v", want, lines)
}

func requireSectionStartsAfterBlankLine(t *testing.T, lines []string, title string) {
	t.Helper()
	titleIndex := indexOfText(lines, title)
	if titleIndex < 2 {
		t.Fatalf("expected section title %q with blank line and separator before it, got %#v", title, lines)
	}
	if lines[titleIndex-1] != strings.Repeat("-", weatherMaxLineLength/2) {
		t.Fatalf("expected separator before %q, got %#v", title, lines[titleIndex-1])
	}
	if strings.TrimSpace(lines[titleIndex-2]) != "" {
		t.Fatalf("expected blank line before %q separator, got %#v in %#v", title, lines[titleIndex-2], lines)
	}
}

func indexOfText(lines []string, want string) int {
	for index, line := range lines {
		if line == want {
			return index
		}
	}
	return -1
}

func indexOfTextContaining(lines []string, want string) int {
	for index, line := range lines {
		if strings.Contains(line, want) {
			return index
		}
	}
	return -1
}
