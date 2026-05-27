package motivation

import (
	"fmt"
	"strings"
	"time"
)

type CalendarAdvice struct {
	Text string `json:"text"`
}

type CalendarContext struct {
	GeneratedAt time.Time
	Mode        string
	Sections    []CalendarSectionContext
}

type CalendarSectionContext struct {
	Title  string
	Date   time.Time
	Events []CalendarEventContext
}

type CalendarEventContext struct {
	TimeLabel string
	Title     string
	Start     time.Time
	End       time.Time
	AllDay    bool
}

func calendarAdvicePrompt(calendar CalendarContext) string {
	var parts []string
	add := func(format string, args ...any) {
		parts = append(parts, fmt.Sprintf(format, args...))
	}

	if !calendar.GeneratedAt.IsZero() {
		add("Время печати: %s", calendar.GeneratedAt.Format("02.01.2006 15:04"))
	}
	if strings.TrimSpace(calendar.Mode) != "" {
		add("Режим: %s", strings.TrimSpace(calendar.Mode))
	}
	totalEvents := 0
	for _, section := range calendar.Sections {
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = "Календарь"
		}
		if !section.Date.IsZero() {
			add("%s (%s):", title, section.Date.Format("02.01.2006"))
		} else {
			add("%s:", title)
		}
		if len(section.Events) == 0 {
			add("- событий нет")
			continue
		}
		totalEvents += len(section.Events)
		for _, event := range section.Events {
			line := strings.TrimSpace(event.Title)
			if line == "" {
				line = "Без названия"
			}
			timeLabel := strings.TrimSpace(event.TimeLabel)
			if timeLabel == "" {
				timeLabel = "без времени"
			}
			add("- %s: %s", timeLabel, line)
		}
	}
	add("Всего событий в напечатанных секциях: %d", totalEvents)

	return "Вот календарь пользователя:\n" +
		strings.Join(parts, "\n") +
		"\n\nДай милый, бодрый и практичный совет на день по загруженности календаря на русском языке для печати на чековой ленте. " +
		"Оцени загруженность календаря, плотность встреч и необходимость пауз. " +
		"Опирайся только на эти события. Не выдумывай встречи, дедлайны, поездки или свободные окна, которых нет в данных. " +
		"Не пересказывай календарь и не повторяй все названия событий; сделай короткий человеческий вывод. " +
		"без markdown, без кавычек, 1-2 короткие строки."
}
