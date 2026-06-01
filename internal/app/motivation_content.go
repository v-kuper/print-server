package app

import (
	"context"
	"strings"
	"time"
	"unicode"

	"atol-server/internal/dailyquest"
	"atol-server/internal/denistrends"
	"atol-server/internal/history"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/receipt"
	"atol-server/internal/weather"
)

func (s *ReceiptService) resolveMotivationContent(ctx context.Context, snapshot weather.Snapshot, includeAdvice bool, includeQuote bool, includeDailyQuests bool, effectiveTime time.Time) (*motivation.WeatherAdvice, *motivation.Quote, []dailyquest.DailyQuest, string, error) {
	if !includeAdvice && !includeQuote && !includeDailyQuests {
		return nil, nil, nil, "", nil
	}
	settings, err := s.store.LoadMotivation()
	if err != nil {
		return nil, nil, nil, "", err
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

	var dailyQuests []dailyquest.DailyQuest
	if includeDailyQuests {
		if effectiveTime.IsZero() {
			effectiveTime = s.clock()
		}
		var questWarning string
		updated, dailyQuests, questWarning = s.resolveDailyQuests(ctx, updated, effectiveTime)
		if questWarning != "" {
			warnings = append(warnings, questWarning)
		}
	}
	if len(warnings) > 0 {
		updated.LastError = strings.Join(warnings, "; ")
	} else {
		updated.LastError = ""
	}
	if saveErr := s.store.SaveMotivation(updated); saveErr != nil {
		return nil, nil, nil, "", saveErr
	}
	return weatherAdvice, quote, dailyQuests, strings.Join(warnings, "\n"), nil
}

func (s *ReceiptService) resolveDailyQuests(ctx context.Context, settings motivation.Settings, effectiveTime time.Time) (motivation.Settings, []dailyquest.DailyQuest, string) {
	cacheDate := motivation.CacheDate(effectiveTime)
	if settings.QuestCacheDate == cacheDate && validCachedDailyQuests(settings.CachedDailyQuests) {
		return settings, append([]dailyquest.DailyQuest(nil), settings.CachedDailyQuests...), ""
	}

	selected := dailyquest.Select(effectiveTime)
	if len(selected) == 0 {
		return settings, nil, ""
	}

	quests, err := s.motivationProvider.GenerateDailyQuests(ctx, settings, selected)
	warning := ""
	if err != nil || !dailyquest.IsValidGenerated(selected, quests) {
		if err != nil {
			warning = "AI-квесты дня недоступны: " + err.Error()
		} else {
			warning = "AI-квесты дня недоступны: пустой или некорректный ответ"
		}
		quests = dailyquest.Fallback(selected)
	}

	settings.QuestCacheDate = cacheDate
	settings.CachedDailyQuests = append([]dailyquest.DailyQuest(nil), quests...)
	return settings, quests, warning
}

func validCachedDailyQuests(quests []dailyquest.DailyQuest) bool {
	if len(quests) != 3 {
		return false
	}
	for _, quest := range quests {
		if quest.ID == 0 || strings.TrimSpace(quest.Text) == "" {
			return false
		}
	}
	return true
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
		strings.Contains(sourceName, "hacker news") ||
		strings.Contains(sourceName, "bloomberg")
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
