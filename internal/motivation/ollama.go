package motivation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const quotePromptBase = "Сгенерируй короткую мотивационную цитату дня на русском языке для печати на кассовой ленте. Без markdown, без кавычек, без имени автора. 1-2 короткие строки, спокойно и по-человечески. Каждый ответ должен звучать свежо: избегай шаблонов, канцелярита и повторов вроде 'маленькие шаги ведут к большим переменам'."

const defaultOllamaTimeout = 45 * time.Second

type Provider interface {
	Generate(context.Context, Settings) (Quote, error)
	GenerateWeatherAdvice(context.Context, Settings, WeatherContext) (WeatherAdvice, error)
	GenerateCalendarAdvice(context.Context, Settings, CalendarContext) (CalendarAdvice, error)
	GenerateHistoryFacts(context.Context, Settings, []HistoryEvent) ([]HistoryFact, error)
	TranslateNewsTitles(context.Context, Settings, []NewsTitle) ([]NewsTranslation, error)
}

type OllamaProvider struct {
	Client *http.Client
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature   float64 `json:"temperature,omitempty"`
	TopP          float64 `json:"top_p,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
	RepeatLastN   int     `json:"repeat_last_n,omitempty"`
}

var quoteOptions = ollamaOptions{
	Temperature:   0.95,
	TopP:          0.92,
	RepeatPenalty: 1.18,
	RepeatLastN:   160,
}

var weatherAdviceOptions = ollamaOptions{
	Temperature:   0.68,
	TopP:          0.86,
	RepeatPenalty: 1.08,
	RepeatLastN:   96,
}

var calendarAdviceOptions = ollamaOptions{
	Temperature:   0.7,
	TopP:          0.86,
	RepeatPenalty: 1.08,
	RepeatLastN:   96,
}

var newsTranslationOptions = ollamaOptions{
	Temperature:   0.25,
	TopP:          0.8,
	RepeatPenalty: 1.03,
	RepeatLastN:   96,
}

var historyFactsOptions = ollamaOptions{
	Temperature:   0.25,
	TopP:          0.8,
	RepeatPenalty: 1.03,
	RepeatLastN:   96,
}

func NewOllamaProvider(client *http.Client) *OllamaProvider {
	if client == nil {
		client = &http.Client{Timeout: defaultOllamaTimeout}
	}
	return &OllamaProvider{Client: client}
}

func (p *OllamaProvider) Generate(ctx context.Context, settings Settings) (Quote, error) {
	text, err := p.generateWithPrompt(ctx, settings, quotePrompt(settings), quoteOptions)
	if err != nil {
		return Quote{}, err
	}
	return Quote{Text: text}, nil
}

func (p *OllamaProvider) GenerateWeatherAdvice(ctx context.Context, settings Settings, weather WeatherContext) (WeatherAdvice, error) {
	text, err := p.generateWithPrompt(ctx, settings, weatherAdvicePrompt(weather), weatherAdviceOptions)
	if err != nil {
		return WeatherAdvice{}, err
	}
	return WeatherAdvice{Text: text}, nil
}

func (p *OllamaProvider) GenerateCalendarAdvice(ctx context.Context, settings Settings, calendar CalendarContext) (CalendarAdvice, error) {
	text, err := p.generateWithPrompt(ctx, settings, calendarAdvicePrompt(calendar), calendarAdviceOptions)
	if err != nil {
		return CalendarAdvice{}, err
	}
	return CalendarAdvice{Text: text}, nil
}

func (p *OllamaProvider) GenerateHistoryFacts(ctx context.Context, settings Settings, events []HistoryEvent) ([]HistoryFact, error) {
	if len(events) == 0 {
		return nil, nil
	}
	text, err := p.chat(ctx, settings, historyFactsPrompt(events), historyFactsOptions)
	if err != nil {
		return nil, err
	}
	return parseHistoryFacts(text)
}

func (p *OllamaProvider) TranslateNewsTitles(ctx context.Context, settings Settings, titles []NewsTitle) ([]NewsTranslation, error) {
	if len(titles) == 0 {
		return nil, nil
	}
	text, err := p.chat(ctx, settings, newsTranslationPrompt(titles), newsTranslationOptions)
	if err != nil {
		return nil, err
	}
	return parseNewsTranslations(text)
}

func (p *OllamaProvider) generateWithPrompt(ctx context.Context, settings Settings, prompt string, options ollamaOptions) (string, error) {
	text, err := p.chat(ctx, settings, prompt, options)
	if err != nil {
		return "", err
	}
	text = sanitizeQuote(text)
	if text == "" {
		return "", fmt.Errorf("Ollama returned empty quote")
	}
	return text, nil
}

func (p *OllamaProvider) chat(ctx context.Context, settings Settings, prompt string, options ollamaOptions) (string, error) {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return "", err
	}

	body, err := json.Marshal(ollamaChatRequest{
		Model: normalized.Model,
		Messages: []ollamaMessage{
			{Role: "user", Content: prompt},
		},
		Stream:  false,
		Options: options,
	})
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, normalized.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := p.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("request Ollama: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("Ollama returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		Message ollamaMessage `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode Ollama response: %w", err)
	}

	text := strings.TrimSpace(payload.Message.Content)
	if text == "" {
		return "", fmt.Errorf("Ollama returned empty response")
	}
	return text, nil
}

func quotePrompt(settings Settings) string {
	prompt := quotePromptBase
	previous := sanitizeQuote(settings.CachedQuote)
	if previous != "" {
		prompt += "\nНе повторяй предыдущую цитату и не перефразируй ее близко: " + previous
	}
	prompt += "\nВариант генерации: " + time.Now().UTC().Format(time.RFC3339Nano) + ". Не печатай этот служебный маркер."
	return strings.TrimSpace(prompt)
}

func (p *OllamaProvider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: defaultOllamaTimeout}
}
