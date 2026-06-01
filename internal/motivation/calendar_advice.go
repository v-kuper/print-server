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
		"\n\nДай короткую конструктивную рекомендацию по календарю на русском языке для печати на чековой ленте. " +
		"Тон: спокойный и деловой, без слащавой поддержки, без гипербол, лозунгов, бодрящих междометий и драматизации. " +
		"Оцени загрузку по фактам: количество событий, плотность встреч, ближайшее событие, режим утро/вечер, пустые секции и переход на завтра. " +
		"Если день перегружен или встречи идут плотно, предложи одно конкретное действие: укоротить или перенести низкоприоритетную встречу, заранее подготовить вопросы, оставить буфер между встречами или зафиксировать итог после ключевой встречи. " +
		"Если загрузка умеренная или низкая, не советуй отдых ради отдыха; предложи сфокусироваться на ближайшем важном событии, закрыть один хвост или подготовить завтрашний слот. " +
		"Если событий нет, скажи, что ограничений по календарю нет, и предложи выбрать 1-2 приоритета. " +
		"Поддержка допустима только сдержанная и по делу. Не используй универсальные фразы про отдых, паузы и настроение без конкретной привязки к данным. " +
		"Опирайся только на эти события. Не выдумывай встречи, дедлайны, поездки, свободные окна, перегрузку или отдых, которых нет в данных. " +
		"Не пересказывай календарь и не повторяй все названия событий; можно упомянуть одно событие или время только если это делает совет конкретным. " +
		"без markdown, без кавычек, 1-2 короткие строки: оценка нагрузки + действие."
}
