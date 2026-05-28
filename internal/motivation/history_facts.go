package motivation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type HistoryEvent struct {
	Year int    `json:"year"`
	Text string `json:"text"`
	Link string `json:"link,omitempty"`
}

type HistoryFact struct {
	Year int    `json:"year"`
	Text string `json:"text"`
}

func historyFactsPrompt(events []HistoryEvent) string {
	var builder strings.Builder
	builder.WriteString("Выбери исторические факты о том, что произошло в этот день в прошлые годы.\n")
	builder.WriteString("Используй только входные события, не выдумывай факты и даты. Выбери до 3 наиболее важных или интересных событий.\n")
	builder.WriteString("Переведи на естественный русский и сожми каждое событие до одной короткой строки для чековой ленты.\n")
	builder.WriteString("Верни только JSON-массив объектов вида {\"year\": number, \"text\": \"короткий факт\"}. Без markdown и пояснений.\n")
	builder.WriteString("События:\n")
	for _, event := range events {
		text := strings.TrimSpace(event.Text)
		if text == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("- year=%d text=%s\n", event.Year, text))
	}
	return strings.TrimSpace(builder.String())
}

func parseHistoryFacts(value string) ([]HistoryFact, error) {
	raw := extractJSONArray(value)
	if raw == "" {
		return nil, fmt.Errorf("Ollama returned no JSON history facts")
	}

	var decoded []HistoryFact
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("decode history facts: %w", err)
	}

	result := make([]HistoryFact, 0, len(decoded))
	for _, item := range decoded {
		item.Text = sanitizeQuote(item.Text)
		if item.Text == "" {
			continue
		}
		result = append(result, item)
		if len(result) == 3 {
			break
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Ollama returned empty history facts")
	}
	return result, nil
}
