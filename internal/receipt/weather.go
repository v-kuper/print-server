package receipt

import (
	"fmt"
	"math"
	"strings"
	"time"

	"atol-server/internal/bankrates"
	"atol-server/internal/finance"
	"atol-server/internal/googleintegration"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/weather"
)

const weatherMaxLineLength = 32
const calendarTimeColumnWidth = 9
const calendarColumnGap = 2

var russianMonths = []string{
	"Января",
	"Февраля",
	"Марта",
	"Апреля",
	"Мая",
	"Июня",
	"Июля",
	"Августа",
	"Сентября",
	"Октября",
	"Ноября",
	"Декабря",
}

var russianWeekdays = []string{
	"Воскресенье",
	"Понедельник",
	"Вторник",
	"Среда",
	"Четверг",
	"Пятница",
	"Суббота",
}

type DailyReceiptData struct {
	Weather          weather.Snapshot
	HideWeather      bool
	WeatherAdvice    *motivation.WeatherAdvice
	MotivationQuote  *motivation.Quote
	TonPortfolio     *finance.TonPortfolioSummary
	TonChartImage    *Image
	USDBYNRate       *finance.FiatRate
	USDBYNChartImage *Image
	BankRates        *bankrates.Summary
	MailMessages     []googleintegration.MailMessage
	CalendarEvents   []googleintegration.CalendarEvent
	NewsItems        []news.Item
}

func WeatherReceipt(snapshot weather.Snapshot) []Line {
	return WeatherReceiptWithStyle(snapshot, DefaultStyleSettings())
}

func WeatherReceiptWithStyle(snapshot weather.Snapshot, styleSettings StyleSettings) []Line {
	styles := styleSettings.Normalized()
	normalStyle := styles.normalLineStyle()
	calendarStyle := styles.calendarLineStyle()
	temperatureStyle := styles.temperatureLineStyle()
	observedAt := snapshot.ObservedAt.In(snapshot.TimeLocation())
	result := make([]Line, 0, 12)

	result = append(result, blankLine(normalStyle))
	result = append(result, center(calendarTitle(observedAt), calendarStyle))
	result = append(result, center(weekdayName(observedAt.Weekday()), normalStyle))
	result = append(result, blankLine(normalStyle))
	if iconKey := weather.ConditionIconKey(snapshot); iconKey != "" {
		result = append(result, weatherIcon(iconKey, normalStyle))
	}
	result = append(result, center(weather.ConditionLabel(snapshot), normalStyle))
	result = append(result, center(formatTemperature(displayTemperature(snapshot)), temperatureStyle))
	result = append(result, blankLine(normalStyle))

	if snapshot.DayTemperatureC != nil {
		result = append(result, wrappedCenter("Днем "+formatTemperature(*snapshot.DayTemperatureC), normalStyle)...)
	}
	if snapshot.NightTemperatureC != nil {
		result = append(result, wrappedCenter("Ночью "+formatTemperature(*snapshot.NightTemperatureC), normalStyle)...)
	}
	if snapshot.WindSpeedMs != nil {
		result = append(result, wrappedCenter(formatWindLine(snapshot.WindSpeedMs, snapshot.WindDirectionDeg), normalStyle)...)
	}
	if snapshot.WindGustsMs != nil {
		result = append(result, wrappedCenter(fmt.Sprintf("Порывы до %d м/с", round(*snapshot.WindGustsMs)), normalStyle)...)
	}
	if snapshot.RelativeHumidityPct != nil {
		result = append(result, wrappedCenter(fmt.Sprintf("Влажность %d%%", round(*snapshot.RelativeHumidityPct)), normalStyle)...)
	}
	if uvLine := formatUVLine(snapshot.UVIndexMax, snapshot.UVIndex); uvLine != "" {
		result = append(result, wrappedCenter(uvLine, normalStyle)...)
	}
	if snapshot.PrecipitationProbabilityMaxPct != nil {
		result = append(result, wrappedCenter(fmt.Sprintf("Вероятность осадков %d%%", round(*snapshot.PrecipitationProbabilityMaxPct)), normalStyle)...)
	}
	if snapshot.SurfacePressureHpa != nil {
		result = append(result, wrappedCenter(fmt.Sprintf("Давление %d гПа", round(*snapshot.SurfacePressureHpa)), normalStyle)...)
	}
	if snapshot.PrecipitationMm != nil {
		result = append(result, wrappedCenter(fmt.Sprintf("Осадки %s мм", formatDecimal(*snapshot.PrecipitationMm)), normalStyle)...)
	}

	return result
}

func DailyReceipt(data DailyReceiptData) []Line {
	return DailyReceiptWithStyle(data, DefaultStyleSettings())
}

func DailyReceiptWithStyle(data DailyReceiptData, styleSettings StyleSettings) []Line {
	normalStyle := styleSettings.Normalized().normalLineStyle()
	originalStyle := styleSettings.Normalized().originalLineStyle()
	result := make([]Line, 0, 32)
	if !data.HideWeather {
		result = append(result, WeatherReceiptWithStyle(data.Weather, styleSettings)...)
	}
	if data.WeatherAdvice != nil && strings.TrimSpace(data.WeatherAdvice.Text) != "" {
		result = append(result, blankLine(normalStyle))
		result = append(result, wrappedCenter(data.WeatherAdvice.Text, normalStyle)...)
	}
	if data.MotivationQuote != nil && strings.TrimSpace(data.MotivationQuote.Text) != "" {
		result = append(result, blankLine(normalStyle))
		result = append(result, wrappedCenter(data.MotivationQuote.Text, normalStyle)...)
	}
	if data.TonPortfolio != nil {
		result = appendSectionHeader(result, "TON", normalStyle)
		result = append(result, wrappedCenter(formatTonPortfolioLine(*data.TonPortfolio), normalStyle)...)
		result = append(result, wrappedCenter(formatTonValueLine(*data.TonPortfolio), normalStyle)...)
		if data.TonChartImage != nil {
			result = append(result, imageLine(*data.TonChartImage, normalStyle))
		}
	}
	if data.USDBYNRate != nil {
		result = appendSectionHeader(result, "Курс доллара", normalStyle)
		result = append(result, wrappedCenter(formatFiatRate(*data.USDBYNRate), normalStyle)...)
		if data.USDBYNChartImage != nil {
			result = append(result, imageLine(*data.USDBYNChartImage, normalStyle))
		}
	}
	if data.BankRates != nil && (data.BankRates.SellUSD != nil || data.BankRates.BuyUSD != nil) {
		result = appendSectionHeader(result, "В банках", normalStyle)
		if data.BankRates.SellUSD != nil {
			result = append(result, wrappedCenter(formatBankRateLine("Продать $", *data.BankRates.SellUSD), normalStyle)...)
			result = append(result, wrappedCenter(formatBankNames(data.BankRates.SellUSD.BankNames), normalStyle)...)
		}
		if data.BankRates.SellUSD != nil && data.BankRates.BuyUSD != nil {
			result = append(result, blankLine(normalStyle))
		}
		if data.BankRates.BuyUSD != nil {
			result = append(result, wrappedCenter(formatBankRateLine("Купить $", *data.BankRates.BuyUSD), normalStyle)...)
			result = append(result, wrappedCenter(formatBankNames(data.BankRates.BuyUSD.BankNames), normalStyle)...)
		}
		if !data.BankRates.UpdatedAt.IsZero() {
			result = append(result, wrappedCenter(formatBankRatesUpdate(data.BankRates.UpdatedAt, data.Weather.TimeLocation()), normalStyle)...)
		}
	}
	if len(data.MailMessages) > 0 {
		result = appendSectionHeader(result, "Почта", normalStyle)
		for itemIndex, message := range data.MailMessages {
			if strings.TrimSpace(message.From) != "" {
				result = append(result, wrappedCenter(message.From, normalStyle)...)
			}
			if strings.TrimSpace(message.Subject) != "" {
				result = append(result, wrappedCenter(message.Subject, normalStyle)...)
			}
			if itemIndex < len(data.MailMessages)-1 {
				result = append(result, blankLine(normalStyle))
			}
		}
	}
	if len(data.CalendarEvents) > 0 {
		result = appendSectionHeader(result, "Календарь", normalStyle)
		for _, event := range data.CalendarEvents {
			result = append(result, calendarEventLines(event, normalStyle)...)
		}
	}
	if len(data.NewsItems) > 0 {
		result = appendSectionHeader(result, "Коротко о мире:", normalStyle)
		result = append(result, blankLine(normalStyle))

		grouped := groupNews(data.NewsItems)
		for sourceIndex, group := range grouped {
			result = append(result, wrappedCenter(group.SourceName, normalStyle)...)
			for itemIndex, item := range group.Items {
				result = append(result, wrappedCenter("- "+item.Title, normalStyle)...)
				if strings.TrimSpace(item.OriginalTitle) != "" {
					result = append(result, wrappedCenter(item.OriginalTitle, originalStyle)...)
				}
				if itemIndex < len(group.Items)-1 {
					result = append(result, blankLine(normalStyle))
				}
			}
			if sourceIndex < len(grouped)-1 {
				result = append(result, blankLine(normalStyle))
			}
		}
	}
	return result
}

func calendarTitle(value time.Time) string {
	month := ""
	monthIndex := int(value.Month()) - 1
	if monthIndex >= 0 && monthIndex < len(russianMonths) {
		month = russianMonths[monthIndex]
	}
	return strings.TrimSpace(fmt.Sprintf("%d %s", value.Day(), month))
}

func separator(style lineStyle) Line {
	return center(strings.Repeat("-", weatherMaxLineLength/2), style)
}

func appendSectionHeader(lines []Line, title string, style lineStyle) []Line {
	lines = append(lines, blankLine(style))
	lines = append(lines, separator(style))
	return append(lines, center(title, style))
}

func formatTonPortfolioLine(summary finance.TonPortfolioSummary) string {
	return fmt.Sprintf("TON $%.3f x%.3f", summary.Price.USD, summary.AmountTon)
}

func formatTonValueLine(summary finance.TonPortfolioSummary) string {
	return fmt.Sprintf("$%.2f P/L %s", summary.CurrentValueUSD, formatSignedCurrency(summary.ProfitLossUSD))
}

func formatFiatRate(rate finance.FiatRate) string {
	prefix := fmt.Sprintf("%s/%s", rate.BaseCode, rate.QuoteCode)
	if rate.Scale != 1 {
		prefix = fmt.Sprintf("%d %s", rate.Scale, rate.BaseCode)
	}
	return fmt.Sprintf("%s %.4f", prefix, rate.Rate)
}

func formatBankRateLine(label string, offer bankrates.Offer) string {
	return fmt.Sprintf("%s %.4f", label, offer.Rate)
}

func formatBankNames(names []string) string {
	value := strings.Join(names, " / ")
	if strings.TrimSpace(value) == "" {
		return "Банк не указан"
	}
	return value
}

func formatBankRatesUpdate(updatedAt time.Time, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	return "Обн. " + updatedAt.In(location).Format("15:04")
}

func calendarEventLines(event googleintegration.CalendarEvent, style lineStyle) []Line {
	timeLabel := strings.TrimSpace(event.TimeLabel)
	title := strings.TrimSpace(event.Title)
	if timeLabel == "" && title == "" {
		return nil
	}
	if title == "" {
		return []Line{left(timeLabel, style)}
	}
	if timeLabel == "" {
		return wrappedLeft(title, style, weatherMaxLineLength)
	}

	titleWidth := weatherMaxLineLength - calendarTimeColumnWidth - calendarColumnGap
	if style.DoubleWidth {
		titleWidth = weatherMaxLineLength/2 - calendarTimeColumnWidth - calendarColumnGap
	}
	if titleWidth < 8 {
		titleWidth = 8
	}

	titleParts := wrapWords(title, titleWidth)
	result := make([]Line, 0, len(titleParts))
	for index, part := range titleParts {
		prefix := strings.Repeat(" ", calendarTimeColumnWidth+calendarColumnGap)
		if index == 0 {
			prefix = padRight(timeLabel, calendarTimeColumnWidth) + strings.Repeat(" ", calendarColumnGap)
		}
		result = append(result, left(prefix+part, style))
	}
	return result
}

func formatSignedCurrency(value float64) string {
	sign := "+"
	if value < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s$%.2f", sign, math.Abs(value))
}

type newsGroup struct {
	SourceName string
	Items      []news.Item
}

func groupNews(items []news.Item) []newsGroup {
	indexBySource := make(map[string]int)
	var groups []newsGroup
	for _, item := range items {
		sourceName := strings.TrimSpace(item.SourceName)
		if sourceName == "" {
			sourceName = "RSS"
		}
		index, exists := indexBySource[sourceName]
		if !exists {
			index = len(groups)
			indexBySource[sourceName] = index
			groups = append(groups, newsGroup{SourceName: sourceName})
		}
		groups[index].Items = append(groups[index].Items, item)
	}
	return groups
}

func weekdayName(value time.Weekday) string {
	index := int(value)
	if index >= 0 && index < len(russianWeekdays) {
		return russianWeekdays[index]
	}
	return ""
}

func formatTemperature(value float64) string {
	rounded := round(value)
	if rounded > 0 {
		return fmt.Sprintf("+%d C", rounded)
	}
	return fmt.Sprintf("%d C", rounded)
}

func displayTemperature(snapshot weather.Snapshot) float64 {
	if snapshot.ApparentTemperatureC != nil {
		return *snapshot.ApparentTemperatureC
	}
	return snapshot.TemperatureC
}

func formatDecimal(value float64) string {
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%d", round(value))
	}
	return fmt.Sprintf("%.1f", value)
}

func formatWindLine(speedMs *float64, directionDeg *float64) string {
	if speedMs == nil {
		return ""
	}
	direction := weather.WindDirectionLabel(directionDeg)
	if direction == "" {
		return fmt.Sprintf("Ветер %d м/с", round(*speedMs))
	}
	return fmt.Sprintf("%s ветер %d м/с", direction, round(*speedMs))
}

func formatUVLine(maxUV *float64, currentUV *float64) string {
	if maxUV != nil {
		return fmt.Sprintf("UV сегодня %s %s", formatDecimal(*maxUV), weather.UVIndexLevel(*maxUV))
	}
	if currentUV != nil {
		return fmt.Sprintf("UV сейчас %s %s", formatDecimal(*currentUV), weather.UVIndexLevel(*currentUV))
	}
	return ""
}

func round(value float64) int {
	return int(math.Round(value))
}

func center(text string, style lineStyle) Line {
	return Line{
		Text:         text,
		Alignment:    AlignmentCenter,
		Role:         style.Role,
		Font:         style.Font,
		DoubleWidth:  style.DoubleWidth,
		DoubleHeight: style.DoubleHeight,
	}
}

func left(text string, style lineStyle) Line {
	return Line{
		Text:         text,
		Alignment:    AlignmentLeft,
		Role:         style.Role,
		Font:         style.Font,
		DoubleWidth:  style.DoubleWidth,
		DoubleHeight: style.DoubleHeight,
	}
}

func weatherIcon(key string, style lineStyle) Line {
	return Line{
		ImageKey:          key,
		ImageWidth:        96,
		ImageHeight:       96,
		ImageScalePercent: 100,
		Alignment:         AlignmentCenter,
		Role:              style.Role,
	}
}

func imageLine(image Image, style lineStyle) Line {
	scalePercent := image.ScalePercent
	if scalePercent <= 0 {
		scalePercent = 100
	}
	return Line{
		ImageKey:          image.Key,
		ImagePath:         image.Path,
		ImageURL:          image.URL,
		ImageWidth:        image.Width,
		ImageHeight:       image.Height,
		ImagePixelBuffer:  append([]byte(nil), image.PixelBuffer...),
		ImageScalePercent: scalePercent,
		Alignment:         AlignmentCenter,
		Role:              style.Role,
		Font:              style.Font,
		DoubleWidth:       style.DoubleWidth,
		DoubleHeight:      style.DoubleHeight,
	}
}

func wrappedCenter(text string, style lineStyle) []Line {
	lineLength := weatherMaxLineLength
	if style.DoubleWidth {
		lineLength /= 2
	}
	parts := wrapWords(text, lineLength)
	result := make([]Line, 0, len(parts))
	for _, part := range parts {
		result = append(result, center(part, style))
	}
	return result
}

func wrappedLeft(text string, style lineStyle, lineLength int) []Line {
	if style.DoubleWidth {
		lineLength /= 2
	}
	parts := wrapWords(text, lineLength)
	result := make([]Line, 0, len(parts))
	for _, part := range parts {
		result = append(result, left(part, style))
	}
	return result
}

func blankLine(style lineStyle) Line {
	return center(" ", style)
}

func padRight(value string, width int) string {
	if width <= 0 {
		return value
	}
	padding := width - runeLen(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func wrapWords(text string, limit int) []string {
	if runeLen(text) <= limit {
		return []string{text}
	}

	var result []string
	current := ""
	for _, word := range strings.Fields(text) {
		switch {
		case current == "":
			current = word
		case runeLen(current)+1+runeLen(word) <= limit:
			current += " " + word
		default:
			result = append(result, current)
			current = word
		}

		for runeLen(current) > limit {
			runes := []rune(current)
			result = append(result, string(runes[:limit]))
			current = string(runes[limit:])
		}
	}

	if current != "" {
		result = append(result, current)
	}
	return result
}

func runeLen(value string) int {
	return len([]rune(value))
}
