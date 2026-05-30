package motivation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"atol-server/internal/dailyquest"
)

var dailyQuestOptions = ollamaOptions{
	Temperature:   0.72,
	TopP:          0.86,
	RepeatPenalty: 1.08,
	RepeatLastN:   128,
}

func (p *OllamaProvider) GenerateDailyQuests(ctx context.Context, settings Settings, quests []dailyquest.Quest) ([]dailyquest.DailyQuest, error) {
	if len(quests) == 0 {
		return nil, nil
	}
	text, err := p.chat(ctx, settings, dailyQuestPrompt(quests), dailyQuestOptions)
	if err != nil {
		return nil, err
	}
	result, err := parseDailyQuests(text)
	if err != nil {
		return nil, err
	}
	if !dailyquest.IsValidGenerated(quests, result) {
		return nil, fmt.Errorf("invalid daily quest response")
	}
	return result, nil
}

func dailyQuestPrompt(quests []dailyquest.Quest) string {
	type promptQuest struct {
		ID       int    `json:"id"`
		Category string `json:"category"`
		Text     string `json:"text"`
	}
	payload := make([]promptQuest, 0, len(quests))
	for _, quest := range quests {
		payload = append(payload, promptQuest{
			ID:       quest.ID,
			Category: string(quest.Category),
			Text:     dailyquest.SafeText(quest),
		})
	}
	body, _ := json.Marshal(payload)
	return strings.TrimSpace(`Квест на день.
Переформулируй выбранные квесты коротко, живо и по-русски для печати на кассовой ленте.
Сохрани смысл и ограничения каждого ID.
Constraints: free, solo-friendly, doable today, safe/respectful.
Не добавляй траты, покупки, опасные задания, давление на незнакомых людей или моральные оценки.
Ответ строго JSON-массивом без markdown: [{"id":7,"text":"..."},{"id":21,"text":"..."},{"id":48,"text":"..."}]
Выбранные квесты: ` + string(body))
}

func parseDailyQuests(value string) ([]dailyquest.DailyQuest, error) {
	jsonText := extractJSONArray(value)
	if jsonText == "" {
		return nil, fmt.Errorf("daily quest response is not JSON array")
	}
	var raw []dailyquest.DailyQuest
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return nil, fmt.Errorf("decode daily quests: %w", err)
	}
	result := make([]dailyquest.DailyQuest, 0, len(raw))
	for _, quest := range raw {
		text := sanitizeQuestText(quest.Text)
		if quest.ID == 0 || text == "" {
			continue
		}
		result = append(result, dailyquest.DailyQuest{ID: quest.ID, Text: text})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("daily quest response is empty")
	}
	return result, nil
}

func sanitizeQuestText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'«»“”`)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.TrimPrefix(value, "- ")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'«»“”`)
	return strings.TrimSpace(value)
}
