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

func calendarAdvicePrompt(calendar CalendarContext, previousAdvice string) string {
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

	prompt := "Вот календарь пользователя:\n" +
		strings.Join(parts, "\n") +
		"\n\nНапиши короткий живой комментарий по календарю на русском языке для печати на чековой ленте. " +
		"Тон разговорный и естественный: как близкий человек помогает быстро сориентироваться в дне — без офисного канцелярита, слащавости, лозунгов и драматизации. " +
		"Сначала молча выбери один самый полезный ракурс: ближайший приоритет, подготовка к конкретному событию, плотный стык, разумный буфер, переход на завтра или свободная ёмкость дня. Не перечисляй ракурсы в ответе. " +
		"Не начинай каждый ответ с оценки загруженности и не используй обязательную формулу из двух частей. Меняй структуру, начало и ритм: это может быть наблюдение, одно точное действие или короткая связка между двумя событиями. " +
		"Если событий нет, естественно предложи один-два приоритета, но не объявляй день автоматически отдыхом. " +
		"Избегай универсальных фраз вроде 'день плотный', 'держи паузы', 'сосредоточься на главном', 'не забудь отдохнуть' и 'продуктивного дня', если они не дают конкретики. " +
		"Опирайся только на эти события. Не выдумывай встречи, дедлайны, поездки, свободные окна, перегрузку или отдых, которых нет в данных. " +
		"Не пересказывай календарь и не повторяй все названия событий; можно упомянуть одно событие или время только если это делает совет конкретным. " +
		"Без markdown, без кавычек, 1-2 короткие строки.\n" +
		generationVariantInstruction()

	previousAdvice = sanitizeQuote(previousAdvice)
	if previousAdvice != "" {
		prompt += "\nПредыдущий календарный комментарий: " + previousAdvice + ". Не повторяй его и не перефразируй близко; выбери другой ракурс и начало."
	}
	return strings.TrimSpace(prompt)
}
