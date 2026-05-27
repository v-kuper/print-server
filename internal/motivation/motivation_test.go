package motivation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaProviderParsesChatResponse(t *testing.T) {
	var requestPath string
	var requestPayload ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"  **Делай важное спокойно.**\n\n"}}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.Client())
	quote, err := provider.Generate(context.Background(), Settings{
		Enabled:     true,
		BaseURL:     server.URL,
		Model:       "gemma4:31b-cloud",
		CachedQuote: "Маленькие шаги ведут к большим переменам.",
	})
	if err != nil {
		t.Fatalf("generate quote: %v", err)
	}

	if requestPath != "/api/chat" {
		t.Fatalf("expected /api/chat path, got %q", requestPath)
	}
	if requestPayload.Model != "gemma4:31b-cloud" {
		t.Fatalf("expected model in request, got %#v", requestPayload)
	}
	if requestPayload.Stream {
		t.Fatal("expected non-streaming Ollama request")
	}
	if len(requestPayload.Messages) == 0 || !strings.Contains(requestPayload.Messages[0].Content, "русском") {
		t.Fatalf("expected Russian quote prompt, got %#v", requestPayload.Messages)
	}
	if !strings.Contains(requestPayload.Messages[0].Content, "Не повторяй предыдущую цитату") ||
		!strings.Contains(requestPayload.Messages[0].Content, "Маленькие шаги ведут к большим переменам") {
		t.Fatalf("expected anti-repeat quote prompt, got %#v", requestPayload.Messages)
	}
	if requestPayload.Options.Temperature < 0.9 || requestPayload.Options.RepeatPenalty <= 1 {
		t.Fatalf("expected creative quote sampling options, got %#v", requestPayload.Options)
	}
	if quote.Text != "Делай важное спокойно." {
		t.Fatalf("expected sanitized quote, got %q", quote.Text)
	}
}

func TestNewOllamaProviderUsesCloudFriendlyTimeout(t *testing.T) {
	provider := NewOllamaProvider(nil)

	if provider.Client.Timeout < 45*time.Second {
		t.Fatalf("expected Ollama timeout to allow slower cloud translation responses, got %s", provider.Client.Timeout)
	}
}

func TestOllamaProviderBuildsWeatherAdvicePrompt(t *testing.T) {
	var requestPayload ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"Спокойная прогулка подойдет, оденься по погоде."}}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.Client())
	advice, err := provider.GenerateWeatherAdvice(context.Background(), Settings{
		Enabled: true,
		BaseURL: server.URL,
		Model:   "gemma4:31b-cloud",
	}, WeatherContext{
		LocationName:                   "Гомель",
		ObservedAt:                     time.Date(2026, 5, 25, 19, 20, 0, 0, time.FixedZone("MSK", 3*60*60)),
		Condition:                      "Небольшой дождь",
		TemperatureC:                   12.4,
		ApparentTemperatureC:           ptrFloat(11.7),
		DayTemperatureC:                ptrFloat(14.2),
		NightTemperatureC:              ptrFloat(6.1),
		WindSpeedMs:                    ptrFloat(8.8),
		WindGustsMs:                    ptrFloat(14.2),
		WindDirectionDeg:               ptrFloat(315),
		RelativeHumidityPct:            ptrFloat(64),
		UVIndexMax:                     ptrFloat(6.2),
		PrecipitationProbabilityMaxPct: ptrFloat(83),
		VisibilityM:                    ptrFloat(12000),
		DewPointC:                      ptrFloat(6.2),
		PrecipitationMm:                ptrFloat(1.8),
		SurfacePressureHpa:             ptrFloat(1011.4),
		Forecast: []WeatherForecastPoint{
			{
				ObservedAt:                  time.Date(2026, 5, 25, 20, 0, 0, 0, time.FixedZone("MSK", 3*60*60)),
				TemperatureC:                ptrFloat(11.8),
				ApparentTemperatureC:        ptrFloat(9.9),
				PrecipitationProbabilityPct: ptrFloat(60),
				PrecipitationMm:             ptrFloat(0.3),
				WindSpeedMs:                 ptrFloat(8.9),
				WindGustsMs:                 ptrFloat(14.0),
				WeatherCode:                 ptrInt(61),
			},
		},
	})
	if err != nil {
		t.Fatalf("generate weather advice: %v", err)
	}

	if advice.Text != "Спокойная прогулка подойдет, оденься по погоде." {
		t.Fatalf("expected sanitized weather advice, got %q", advice.Text)
	}
	if len(requestPayload.Messages) == 0 {
		t.Fatal("expected weather advice prompt")
	}
	prompt := requestPayload.Messages[0].Content
	for _, want := range []string{
		"Гомель",
		"25.05.2026 19:20",
		"Небольшой дождь",
		"12 C",
		"Ощущается как: 12 C",
		"Северо-западный ветер 9 м/с",
		"Порывы до 14 м/с",
		"Влажность 64%",
		"UV сегодня 6.2 высокий",
		"Вероятность осадков 83%",
		"Осадки 1.8 мм",
		"Видимость 12 км",
		"Точка росы 6 C",
		"Ближайшие часы",
		"20:00",
		"осадки 60%",
		"практичный совет",
		"Опирайся только на эти данные",
		"Не выдумывай",
		"не противоречит погоде",
		"милый вердикт",
		"не повторяй цифры",
		"не начинай с состояния погоды",
		"собака-девочка породы джек-рассел",
		"Бонни",
		"сейчас они идут гулять",
		"общий совет или мягкое напоминание",
		"не упоминай имя Бонни",
		"не упоминай породу",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected weather prompt to contain %q, got %q", want, prompt)
		}
	}
	for _, unwanted := range []string{
		"взять зонт",
		"полотенце",
		"вытереть лапы",
		"короткий маршрут",
	} {
		if strings.Contains(prompt, unwanted) {
			t.Fatalf("expected weather prompt not to prescribe %q, got %q", unwanted, prompt)
		}
	}
	if requestPayload.Options.Temperature >= requestPayload.Options.TopP {
		t.Fatalf("expected grounded weather advice options, got %#v", requestPayload.Options)
	}
}

func TestOllamaProviderTranslatesNewsTitles(t *testing.T) {
	var requestPayload ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"[{\"index\":0,\"title\":\"Reuters готовит новый обзор рынков\"},{\"index\":1,\"title\":\"Основатель стартапа рассказал о росте\"}]"}}`))
	}))
	defer server.Close()

	provider := NewOllamaProvider(server.Client())
	translations, err := provider.TranslateNewsTitles(context.Background(), Settings{
		Enabled: true,
		BaseURL: server.URL,
		Model:   "gemma4:31b-cloud",
	}, []NewsTitle{
		{Index: 0, SourceName: "Reuters", Title: "Reuters prepares a new market wrap"},
		{Index: 1, SourceName: "Hacker News", Title: "Founder mode and startup growth"},
	})
	if err != nil {
		t.Fatalf("translate news titles: %v", err)
	}

	if len(translations) != 2 {
		t.Fatalf("expected two translations, got %#v", translations)
	}
	if translations[0].Index != 0 || translations[0].Title != "Reuters готовит новый обзор рынков" {
		t.Fatalf("unexpected first translation: %#v", translations[0])
	}
	if translations[1].Index != 1 || translations[1].Title != "Основатель стартапа рассказал о росте" {
		t.Fatalf("unexpected second translation: %#v", translations[1])
	}
	prompt := requestPayload.Messages[0].Content
	for _, want := range []string{
		"переведи",
		"естественный русский",
		"JSON",
		"Reuters",
		"Reuters prepares a new market wrap",
		"Hacker News",
		"Founder mode and startup growth",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected translation prompt to contain %q, got %q", want, prompt)
		}
	}
	if requestPayload.Options.Temperature > 0.4 {
		t.Fatalf("expected stable translation sampling options, got %#v", requestPayload.Options)
	}
}

func TestResolveDailyQuoteRefreshesQuoteEvenForSameMinskDate(t *testing.T) {
	provider := &fakeProvider{quote: Quote{Text: "Новая цитата"}}
	settings := Settings{
		Enabled:     true,
		CacheDate:   "2026-05-25",
		CachedQuote: "Старая цитата",
	}
	now := time.Date(2026, 5, 25, 20, 0, 0, 0, time.UTC)

	updated, quote, err := ResolveDailyQuote(context.Background(), settings, now, provider)
	if err != nil {
		t.Fatalf("resolve quote: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("expected provider call even with saved quote, got %d calls", provider.calls)
	}
	if quote == nil || quote.Text != "Новая цитата" {
		t.Fatalf("expected refreshed quote, got %#v", quote)
	}
	if updated.CacheDate != "2026-05-25" || updated.CachedQuote != "Новая цитата" || updated.LastError != "" {
		t.Fatalf("expected refreshed quote to be stored for UI status, got %#v", updated)
	}
}

func TestResolveDailyQuoteStoresGeneratedQuoteAndRecordsFailures(t *testing.T) {
	now := time.Date(2026, 5, 25, 3, 0, 0, 0, time.UTC)
	provider := &fakeProvider{quote: Quote{Text: "Сегодня достаточно одного честного шага."}}

	updated, quote, err := ResolveDailyQuote(context.Background(), Settings{Enabled: true}, now, provider)
	if err != nil {
		t.Fatalf("resolve quote: %v", err)
	}
	if quote == nil || quote.Text != "Сегодня достаточно одного честного шага." {
		t.Fatalf("expected generated quote, got %#v", quote)
	}
	if updated.CacheDate != "2026-05-25" || updated.CachedQuote != quote.Text || updated.LastError != "" {
		t.Fatalf("expected cached generated quote, got %#v", updated)
	}

	provider.err = errTestFailure
	updated, quote, err = ResolveDailyQuote(context.Background(), Settings{Enabled: true}, now, provider)
	if err == nil {
		t.Fatal("expected provider error")
	}
	if quote != nil {
		t.Fatalf("expected no quote on failure, got %#v", quote)
	}
	if updated.LastError == "" {
		t.Fatalf("expected last error to be recorded, got %#v", updated)
	}
}

type fakeProvider struct {
	quote  Quote
	advice WeatherAdvice
	err    error
	calls  int
}

func (p *fakeProvider) Generate(context.Context, Settings) (Quote, error) {
	p.calls++
	return p.quote, p.err
}

func (p *fakeProvider) GenerateWeatherAdvice(context.Context, Settings, WeatherContext) (WeatherAdvice, error) {
	p.calls++
	return p.advice, p.err
}

func (p *fakeProvider) TranslateNewsTitles(context.Context, Settings, []NewsTitle) ([]NewsTranslation, error) {
	p.calls++
	return nil, p.err
}

func ptrFloat(value float64) *float64 {
	return &value
}

func ptrInt(value int) *int {
	return &value
}

var errTestFailure = &testFailure{"llama offline"}

type testFailure struct {
	message string
}

func (e *testFailure) Error() string {
	return e.message
}
