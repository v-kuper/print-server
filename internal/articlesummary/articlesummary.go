package articlesummary

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"atol-server/internal/motivation"
)

const (
	MaxFetchBytes       = 1 << 20
	MaxPromptTextRunes  = 12000
	MinArticleTextRunes = 80
)

const defaultHTTPTimeout = 55 * time.Second

type Article struct {
	URL   string
	Title string
	Text  string
}

type Result struct {
	URL         string    `json:"url"`
	Title       string    `json:"title,omitempty"`
	Summary     string    `json:"summary"`
	Bullets     []string  `json:"bullets"`
	GeneratedAt time.Time `json:"generatedAt"`
}

type Provider struct {
	Client *http.Client
	Now    func() time.Time
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

type modelContent struct {
	Summary string   `json:"summary"`
	Bullets []string `json:"bullets"`
}

var articleSummaryOptions = ollamaOptions{
	Temperature:   0.2,
	TopP:          0.75,
	RepeatPenalty: 1.04,
	RepeatLastN:   128,
}

var (
	titlePattern      = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	h1Pattern         = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	dropBlockPattern  = regexp.MustCompile(`(?is)<(script|style|noscript|svg|nav|header|footer|form|aside)[^>]*>.*?</\s*(script|style|noscript|svg|nav|header|footer|form|aside)\s*>`)
	commentPattern    = regexp.MustCompile(`(?is)<!--.*?-->`)
	blockTagPattern   = regexp.MustCompile(`(?is)</?(article|main|section|div|p|br|li|ul|ol|h[1-6]|blockquote|tr|td|th)[^>]*>`)
	remainingTagRegex = regexp.MustCompile(`(?is)<[^>]+>`)
	blankLinePattern  = regexp.MustCompile(`\n{3,}`)
)

func NewProvider(client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Provider{Client: client}
}

func (p *Provider) Summarize(ctx context.Context, settings motivation.Settings, rawURL string) (Result, error) {
	articleURL, err := ValidateArticleURL(rawURL)
	if err != nil {
		return Result{}, err
	}
	article, err := p.fetchArticle(ctx, articleURL)
	if err != nil {
		return Result{}, err
	}
	if utf8.RuneCountInString(article.Text) < MinArticleTextRunes {
		return Result{}, fmt.Errorf("article text is too short to summarize")
	}

	content, err := p.generate(ctx, settings, article)
	if err != nil {
		return Result{}, err
	}
	generatedAt := time.Now().UTC()
	if p.Now != nil {
		generatedAt = p.Now().UTC()
	}
	return Result{
		URL:         article.URL,
		Title:       article.Title,
		Summary:     content.Summary,
		Bullets:     content.Bullets,
		GeneratedAt: generatedAt,
	}, nil
}

func ValidateArticleURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("article URL is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", errors.New("article URL is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("article URL scheme must be http or https")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if blockedHostname(host) {
		return "", errors.New("article URL host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return "", errors.New("article URL IP is not allowed")
	}
	return parsed.String(), nil
}

func ExtractReadableText(body []byte) Article {
	source := string(body)
	title := cleanText(extractFirstMatch(titlePattern, source))
	if title == "" {
		title = cleanText(stripTags(extractFirstMatch(h1Pattern, source)))
	}

	source = commentPattern.ReplaceAllString(source, " ")
	source = dropBlockPattern.ReplaceAllString(source, " ")
	source = blockTagPattern.ReplaceAllString(source, "\n")
	source = stripTags(source)
	source = html.UnescapeString(source)
	source = normalizeReadableText(source)

	return Article{Title: title, Text: source}
}

func BuildPrompt(article Article) string {
	title := cleanText(article.Title)
	sourceText := clipRunes(article.Text, MaxPromptTextRunes)
	return strings.TrimSpace(fmt.Sprintf(`Ты делаешь краткое factual summary источника для мобильного слепка чека.
Ответь по-русски. Используй только факты из текста ниже, без мнений, без советов и без выводов от себя; не додумывай отсутствующие детали.
Верни строгий JSON без markdown и без пояснений:
{"summary":"одна короткая строка, о чем материал","bullets":["3-6 коротких тезисов по источнику"]}

URL: %s
Заголовок: %s
Текст источника:
%s`, article.URL, title, sourceText))
}

func ParseModelResponse(value string) (modelContent, error) {
	raw := extractJSONObject(strings.TrimSpace(value))
	if raw == "" {
		return modelContent{}, errors.New("model did not return JSON object")
	}
	var content modelContent
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return modelContent{}, fmt.Errorf("decode model summary: %w", err)
	}
	content.Summary = cleanText(content.Summary)
	content.Bullets = cleanBullets(content.Bullets)
	if content.Summary == "" {
		return modelContent{}, errors.New("model returned empty summary")
	}
	if len(content.Bullets) == 0 {
		return modelContent{}, errors.New("model returned no bullets")
	}
	return content, nil
}

func (p *Provider) fetchArticle(ctx context.Context, articleURL string) (Article, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return Article{}, err
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8")
	request.Header.Set("User-Agent", "atol-go-server/1.0 article-summary")

	response, err := p.client().Do(request)
	if err != nil {
		return Article{}, fmt.Errorf("request article: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Article{}, fmt.Errorf("article returned HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, MaxFetchBytes+1))
	if err != nil {
		return Article{}, fmt.Errorf("read article: %w", err)
	}
	if len(body) > MaxFetchBytes {
		body = body[:MaxFetchBytes]
	}
	article := ExtractReadableText(body)
	article.URL = articleURL
	return article, nil
}

func (p *Provider) generate(ctx context.Context, settings motivation.Settings, article Article) (modelContent, error) {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return modelContent{}, err
	}
	body, err := json.Marshal(ollamaChatRequest{
		Model: normalized.Model,
		Messages: []ollamaMessage{
			{Role: "user", Content: BuildPrompt(article)},
		},
		Stream:  false,
		Options: articleSummaryOptions,
	})
	if err != nil {
		return modelContent{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(normalized.BaseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return modelContent{}, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")

	response, err := p.client().Do(request)
	if err != nil {
		return modelContent{}, fmt.Errorf("request Ollama: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return modelContent{}, fmt.Errorf("Ollama returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		Message ollamaMessage `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return modelContent{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	return ParseModelResponse(payload.Message.Content)
}

func (p *Provider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func blockedHostname(host string) bool {
	if host == "" {
		return true
	}
	if host == "localhost" || host == "host.docker.internal" {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".lan", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast()
}

func extractFirstMatch(pattern *regexp.Regexp, value string) string {
	matches := pattern.FindStringSubmatch(value)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func stripTags(value string) string {
	return remainingTagRegex.ReplaceAllString(value, " ")
}

func normalizeReadableText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = cleanText(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(blankLinePattern.ReplaceAllString(strings.Join(cleaned, "\n"), "\n\n"))
}

func cleanText(value string) string {
	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, `"'«»“”`)
	value = strings.TrimSpace(value)
	return value
}

func clipRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func extractJSONObject(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)
	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start < 0 || end < start {
		return ""
	}
	return value[start : end+1]
}

func cleanBullets(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = cleanText(value)
		value = strings.TrimLeft(value, "-•*0123456789. ")
		value = cleanText(value)
		if value == "" {
			continue
		}
		result = append(result, value)
		if len(result) == 6 {
			break
		}
	}
	return result
}
