package denistrends

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultBaseURL  = "https://shir-man.com/api/rss"
	DefaultTimezone = "Europe/Minsk"
	DefaultMaxItems = 20
	MinItems        = 1
	MaxItems        = 100
)

type Period string

const (
	PeriodNow   Period = "now"
	PeriodDay   Period = "day"
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

type Source string

const (
	SourceHackerNews    Source = "hackerNews"
	SourceGitHub        Source = "github"
	SourceHypeReplicate Source = "hypeReplicate"
)

type PeriodSettings struct {
	Enabled  bool `json:"enabled"`
	MaxItems int  `json:"maxItems"`
}

type Settings struct {
	Periods map[Period]PeriodSettings `json:"periods"`
	Sources map[Source]bool           `json:"-"`
}

type Item struct {
	Title         string
	OriginalTitle string
	Source        Source
	SourceName    string
	Link          string
}

type Section struct {
	Period Period
	Title  string
	Items  []Item
}

type Provider struct {
	Client  *http.Client
	BaseURL string
}

func DefaultSettings() Settings {
	return Settings{
		Periods: map[Period]PeriodSettings{
			PeriodNow:   {Enabled: true, MaxItems: DefaultMaxItems},
			PeriodDay:   {Enabled: true, MaxItems: DefaultMaxItems},
			PeriodWeek:  {Enabled: true, MaxItems: DefaultMaxItems},
			PeriodMonth: {Enabled: true, MaxItems: DefaultMaxItems},
		},
	}
}

func (s Settings) Normalized() Settings {
	defaults := DefaultSettings()
	periods := make(map[Period]PeriodSettings, len(defaults.Periods))
	for _, period := range knownPeriods() {
		value, ok := s.Periods[period]
		if !ok {
			value = defaults.Periods[period]
		}
		if value.MaxItems == 0 {
			value.MaxItems = defaults.Periods[period].MaxItems
		}
		if value.MaxItems < MinItems {
			value.MaxItems = MinItems
		}
		if value.MaxItems > MaxItems {
			value.MaxItems = MaxItems
		}
		periods[period] = value
	}

	return Settings{Periods: periods}
}

func (s Settings) Validate() error {
	for _, period := range knownPeriods() {
		value, ok := s.Periods[period]
		if !ok {
			continue
		}
		if value.Enabled && (value.MaxItems < MinItems || value.MaxItems > MaxItems) {
			return fmt.Errorf("%s max items must be between %d and %d", period.DisplayName(), MinItems, MaxItems)
		}
	}
	return nil
}

func (s Settings) ActivePeriods(now time.Time) []Period {
	normalized := s.Normalized()
	var result []Period
	for _, period := range knownPeriods() {
		if normalized.Periods[period].Enabled {
			result = append(result, period)
		}
	}
	return result
}

func (p Period) DisplayName() string {
	switch p {
	case PeriodNow:
		return "Top now"
	case PeriodDay:
		return "Top day"
	case PeriodWeek:
		return "Top week"
	case PeriodMonth:
		return "Top month"
	default:
		return string(p)
	}
}

func (s Source) DisplayName() string {
	switch s {
	case SourceHackerNews:
		return "Hacker News"
	case SourceGitHub:
		return "GitHub"
	case SourceHypeReplicate:
		return "Hype Replicate"
	case "lessWrong":
		return "LessWrong"
	case "midjourney":
		return "Midjourney"
	case "lobsters":
		return "Lobsters"
	default:
		return string(s)
	}
}

func NewProvider(client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Provider{Client: client, BaseURL: DefaultBaseURL}
}

func (p *Provider) Current(ctx context.Context, settings Settings, now time.Time) ([]Section, error) {
	normalized := settings.Normalized()
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	var result []Section
	var failures []error
	for _, period := range normalized.ActivePeriods(now) {
		items, err := p.fetch(ctx, normalized, period)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if len(items) == 0 {
			continue
		}
		result = append(result, Section{
			Period: period,
			Title:  period.DisplayName(),
			Items:  items,
		})
	}
	if len(result) == 0 && len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	return result, nil
}

func (p *Provider) fetch(ctx context.Context, settings Settings, period Period) ([]Item, error) {
	endpoint, err := p.endpoint(period)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/rss+xml,application/xml,text/xml")
	request.Header.Set("User-Agent", "ATOL-Go-Server/1.0")

	response, err := p.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s: request Denis Trends: %w", period.DisplayName(), err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("%s: Denis Trends returned HTTP %d", period.DisplayName(), response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read Denis Trends: %w", period.DisplayName(), err)
	}
	return ParseFeed(body, settings, period)
}

func (p *Provider) endpoint(period Period) (string, error) {
	raw := strings.TrimSpace(p.BaseURL)
	if raw == "" {
		raw = DefaultBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	if period == PeriodNow {
		query.Del("sort")
	} else {
		query.Set("sort", string(period))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (p *Provider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func ParseFeed(body []byte, settings Settings, period Period) ([]Item, error) {
	normalized := settings.Normalized()
	var payload struct {
		Channel struct {
			Items []struct {
				Title  string `xml:"title"`
				Link   string `xml:"link"`
				Source Source `xml:"https://shir-man.com/rss/ns# source"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	limit := normalized.Periods[period].MaxItems
	if limit <= 0 {
		return nil, nil
	}

	var result []Item
	for _, value := range payload.Channel.Items {
		title := cleanText(value.Title)
		if title == "" {
			continue
		}
		result = append(result, Item{
			Title:      title,
			Source:     value.Source,
			SourceName: value.Source.DisplayName(),
			Link:       cleanText(value.Link),
		})
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func knownPeriods() []Period {
	return []Period{PeriodNow, PeriodDay, PeriodWeek, PeriodMonth}
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(html.UnescapeString(value)), " ")
}
