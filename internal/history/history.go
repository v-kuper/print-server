package history

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL   = "https://en.wikipedia.org/api/rest_v1/feed/onthisday/selected"
	defaultUserAgent = "atol-go-server/1.0 (local receipt app)"
)

type Event struct {
	Year int
	Text string
	Link string
}

type Provider struct {
	Client    *http.Client
	BaseURL   string
	UserAgent string
}

func NewProvider(client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Provider{
		Client:    client,
		BaseURL:   defaultBaseURL,
		UserAgent: defaultUserAgent,
	}
}

func (p *Provider) Current(ctx context.Context, date time.Time) ([]Event, error) {
	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	url := strings.TrimRight(baseURL, "/") + date.Format("/01/02")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	userAgent := strings.TrimSpace(p.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	request.Header.Set("User-Agent", userAgent)

	response, err := p.Client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s returned HTTP %d", request.URL.Host, response.StatusCode)
	}

	var payload onThisDayResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode history response: %w", err)
	}

	events := make([]Event, 0, len(payload.Selected))
	for _, raw := range payload.Selected {
		text := strings.TrimSpace(raw.Text)
		if text == "" {
			continue
		}
		events = append(events, Event{
			Year: raw.Year,
			Text: text,
			Link: raw.firstLink(),
		})
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("history API returned no events")
	}
	return events, nil
}

type onThisDayResponse struct {
	Selected []onThisDayEvent `json:"selected"`
}

type onThisDayEvent struct {
	Year  int             `json:"year"`
	Text  string          `json:"text"`
	Pages []onThisDayPage `json:"pages"`
}

type onThisDayPage struct {
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

func (e onThisDayEvent) firstLink() string {
	for _, page := range e.Pages {
		link := strings.TrimSpace(page.ContentURLs.Desktop.Page)
		if link != "" {
			return link
		}
	}
	return ""
}
