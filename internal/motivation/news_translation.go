package motivation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type NewsTitle struct {
	Index      int
	SourceName string
	Title      string
}

type NewsTranslation struct {
	Index int    `json:"index"`
	Title string `json:"title"`
}

func newsTranslationPrompt(titles []NewsTitle) string {
	var builder strings.Builder
	builder.WriteString("переведи заголовки новостей на естественный русский язык для печати на чековой ленте.\n")
	builder.WriteString("Сохраняй имена компаний, тикеры, названия продуктов и источников без выдумывания деталей. Не добавляй комментарии, markdown или пояснения.\n")
	builder.WriteString("Верни только JSON-массив объектов вида {\"index\": number, \"title\": \"перевод\"}. Индекс должен совпадать с входным.\n")
	builder.WriteString("Заголовки:\n")
	for _, title := range titles {
		builder.WriteString(fmt.Sprintf("- index=%d source=%s title=%s\n", title.Index, strings.TrimSpace(title.SourceName), strings.TrimSpace(title.Title)))
	}
	return strings.TrimSpace(builder.String())
}

func parseNewsTranslations(value string) ([]NewsTranslation, error) {
	raw := extractJSONArray(value)
	if raw == "" {
		return nil, fmt.Errorf("Ollama returned no JSON translations")
	}

	var decoded []NewsTranslation
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("decode news translations: %w", err)
	}

	result := make([]NewsTranslation, 0, len(decoded))
	for _, item := range decoded {
		item.Title = sanitizeQuote(item.Title)
		if item.Title == "" {
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Ollama returned empty news translations")
	}
	return result, nil
}

func extractJSONArray(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)

	start := strings.Index(value, "[")
	end := strings.LastIndex(value, "]")
	if start < 0 || end < start {
		return ""
	}
	return value[start : end+1]
}
