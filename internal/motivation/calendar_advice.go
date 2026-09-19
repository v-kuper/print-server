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
		"\n\nДай короткую рекомендацию по календарю на русском языке для печати на чековой ленте. Работай в роли строгого персонального планировщика: конкретно, спокойно и по делу, без домашнего или семейного тона. " +
		"Цель — помочь управлять фактической нагрузкой и сохранять work-life balance. Сначала молча оцени количество и плотность событий, ближайшее обязательство, переход на завтра и свободную ёмкость дня. " +
		"Затем выбери один самый полезный ракурс: обозначить приоритет, подготовить вопросы или материалы, заметить плотный стык, оставить разумный буфер, перенести необязательную задачу либо запланировать восстановление. Не перечисляй ракурсы в ответе. " +
		"Предлагай паузу или советуй отдых только тогда, когда это логично следует из плотности календаря или свободной части дня; не выдумывай свободные окна. " +
		"Не начинай каждый ответ с оценки загруженности и не используй обязательную формулу из двух частей. Меняй структуру, начало и ритм: это может быть наблюдение, одно точное действие или короткая связка между двумя событиями. " +
		"Если событий нет, естественно предложи один-два приоритета, но не объявляй день автоматически отдыхом. " +
		"Избегай универсальных фраз вроде 'день плотный', 'держи паузы', 'сосредоточься на главном', 'не забудь отдохнуть' и 'продуктивного дня', если они не дают конкретики. " +
		"Не упоминай семью, супругу, детей, домашних животных, погоду или прогулки и не переноси сюда контекст из других блоков чека. Личные события в календаре оценивай только как задачи, не додумывая отношения пользователя. " +
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
