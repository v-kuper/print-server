package articlesummary

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"atol-server/internal/motivation"
)

func TestExtractReadableTextRemovesPageChromeAndKeepsTitle(t *testing.T) {
	html := []byte(`<!doctype html>
		<html>
			<head><title>Article Title</title><style>.hidden{display:none}</style></head>
			<body>
				<nav>Navigation noise</nav>
				<script>alert("ignore")</script>
				<article>
					<h1>Article Heading</h1>
					<p>First paragraph &amp; important context.</p>
					<p>Second paragraph with details.</p>
				</article>
				<footer>Footer noise</footer>
			</body>
		</html>`)

	article := ExtractReadableText(html)

	if article.Title != "Article Title" {
		t.Fatalf("expected title, got %q", article.Title)
	}
	for _, unwanted := range []string{"Navigation noise", "alert", "Footer noise", "display:none"} {
		if strings.Contains(article.Text, unwanted) {
			t.Fatalf("expected extracted text to omit %q, got %q", unwanted, article.Text)
		}
	}
	for _, want := range []string{"Article Heading", "First paragraph & important context.", "Second paragraph with details."} {
		if !strings.Contains(article.Text, want) {
			t.Fatalf("expected extracted text to contain %q, got %q", want, article.Text)
		}
	}
}

func TestBuildPromptCapsArticleTextAndAsksForFactualRussianJSON(t *testing.T) {
	article := Article{
		URL:   "https://example.com/news",
		Title: "Long Article",
		Text:  strings.Repeat("0123456789", MaxPromptTextRunes/10+100),
	}

	prompt := BuildPrompt(article)

	if len([]rune(prompt)) > MaxPromptTextRunes+2500 {
		t.Fatalf("expected prompt to cap source text, length=%d", len([]rune(prompt)))
	}
	for _, want := range []string{"строгий JSON", "без мнений", "не додумывай", "3-6"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q:\n%s", want, prompt)
		}
	}
}

func TestParseModelResponseExtractsSummaryAndBullets(t *testing.T) {
	content, err := ParseModelResponse("```json\n{\"summary\":\"Материал описывает запуск функции.\",\"bullets\":[\"Первый тезис\",\"Второй тезис\"]}\n```")
	if err != nil {
		t.Fatalf("parse model response: %v", err)
	}

	if content.Summary != "Материал описывает запуск функции." {
		t.Fatalf("unexpected summary: %#v", content)
	}
	if len(content.Bullets) != 2 || content.Bullets[0] != "Первый тезис" || content.Bullets[1] != "Второй тезис" {
		t.Fatalf("unexpected bullets: %#v", content.Bullets)
	}
}

func TestValidateArticleURLBlocksLocalAndPrivateTargets(t *testing.T) {
	blocked := []string{
		"http://localhost/article",
		"http://printer.local/article",
		"http://127.0.0.1/article",
		"http://10.0.0.4/article",
		"http://172.16.0.4/article",
		"http://192.168.0.4/article",
		"http://169.254.1.10/article",
		"javascript:alert(1)",
	}
	for _, rawURL := range blocked {
		if _, err := ValidateArticleURL(rawURL); err == nil {
			t.Fatalf("expected %q to be blocked", rawURL)
		}
	}

	normalized, err := ValidateArticleURL(" https://example.com/news ")
	if err != nil {
		t.Fatalf("expected public URL to be allowed: %v", err)
	}
	if normalized != "https://example.com/news" {
		t.Fatalf("expected normalized URL, got %q", normalized)
	}
}

func TestProviderFetchesArticleAndSummarizesWithOllama(t *testing.T) {
	var articleRequested bool
	var ollamaRequested bool
	provider := NewProvider(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host {
		case "example.com":
			articleRequested = true
			return htmlResponse(200, `<html><head><title>Launch</title></head><body><article><p>The feature launched for all users today with a gradual rollout.</p><p>Maintainers say it reduces manual review work.</p></article></body></html>`), nil
		case "ollama.local":
			ollamaRequested = true
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read Ollama request: %v", err)
			}
			if strings.Contains(string(body), "<script>") || !strings.Contains(string(body), "The feature launched") {
				t.Fatalf("expected cleaned article text in prompt, got %s", string(body))
			}
			return jsonResponse(200, `{"message":{"role":"assistant","content":"{\"summary\":\"Запуск функции стал доступен пользователям.\",\"bullets\":[\"Функция запущена для всех пользователей\",\"Развертывание будет постепенным\"]}"}}`), nil
		default:
			t.Fatalf("unexpected host %q", request.URL.Host)
			return nil, nil
		}
	})})

	result, err := provider.Summarize(context.Background(), motivation.Settings{
		Configured: true,
		Enabled:    true,
		BaseURL:    "http://ollama.local",
		Model:      "summary-model",
	}, "https://example.com/news")
	if err != nil {
		t.Fatalf("summarize article: %v", err)
	}

	if !articleRequested || !ollamaRequested {
		t.Fatalf("expected article and Ollama requests, article=%v ollama=%v", articleRequested, ollamaRequested)
	}
	if result.URL != "https://example.com/news" || result.Title != "Launch" || result.Summary == "" || len(result.Bullets) != 2 {
		t.Fatalf("unexpected summary result: %#v", result)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func htmlResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
