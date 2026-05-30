package news

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultMaxItems = 10
	MinItems        = 1
	MaxItems        = 100
)

const deprecatedEconomistWorldThisWeekFeed = "https://www.economist.com/the-world-this-week/rss.xml"

type Preset string

const (
	PresetReuters             Preset = "reuters"
	PresetEconomist           Preset = "economist"
	PresetHackerNews          Preset = "hacker_news"
	PresetBloombergMarkets    Preset = "bloomberg_markets"
	PresetBloombergTechnology Preset = "bloomberg_technology"
	PresetBloombergPolitics   Preset = "bloomberg_politics"
	PresetBloombergBusiness   Preset = "bloomberg_business"
	PresetBloombergEconomics  Preset = "bloomberg_economics"
	PresetBloombergWealth     Preset = "bloomberg_wealth"
	PresetBloombergCrypto     Preset = "bloomberg_crypto"
)

type PresetInfo struct {
	Preset      Preset `json:"preset"`
	DisplayName string `json:"displayName"`
	FeedURL     string `json:"feedUrl"`
}

type SourceSettings struct {
	Preset   Preset `json:"preset"`
	Enabled  bool   `json:"enabled"`
	FeedURL  string `json:"feedUrl"`
	MaxItems int    `json:"maxItems"`
}

type Settings struct {
	Sources         []SourceSettings `json:"sources"`
	TranslateTitles *bool            `json:"translateTitles,omitempty"`
}

type Item struct {
	Title         string
	OriginalTitle string
	SourceName    string
	Link          string
}

type Provider struct {
	Client *http.Client
}

func Presets() []PresetInfo {
	return []PresetInfo{
		{
			Preset:      PresetReuters,
			DisplayName: "Reuters",
			FeedURL:     "https://news.google.com/rss/search?q=site%3Areuters.com&hl=en-US&gl=US&ceid=US%3Aen",
		},
		{
			Preset:      PresetEconomist,
			DisplayName: "Economist",
			FeedURL:     "https://www.economist.com/latest/rss.xml",
		},
		{
			Preset:      PresetHackerNews,
			DisplayName: "Hacker News",
			FeedURL:     "https://news.ycombinator.com/rss",
		},
		{
			Preset:      PresetBloombergMarkets,
			DisplayName: "Bloomberg Markets",
			FeedURL:     "https://feeds.bloomberg.com/markets/news.rss",
		},
		{
			Preset:      PresetBloombergTechnology,
			DisplayName: "Bloomberg Technology",
			FeedURL:     "https://feeds.bloomberg.com/technology/news.rss",
		},
		{
			Preset:      PresetBloombergPolitics,
			DisplayName: "Bloomberg Politics",
			FeedURL:     "https://feeds.bloomberg.com/politics/news.rss",
		},
		{
			Preset:      PresetBloombergBusiness,
			DisplayName: "Bloomberg Business",
			FeedURL:     "https://feeds.bloomberg.com/business/news.rss",
		},
		{
			Preset:      PresetBloombergEconomics,
			DisplayName: "Bloomberg Economics",
			FeedURL:     "https://feeds.bloomberg.com/economics/news.rss",
		},
		{
			Preset:      PresetBloombergWealth,
			DisplayName: "Bloomberg Wealth",
			FeedURL:     "https://feeds.bloomberg.com/wealth/news.rss",
		},
		{
			Preset:      PresetBloombergCrypto,
			DisplayName: "Bloomberg Crypto",
			FeedURL:     "https://feeds.bloomberg.com/crypto/news.rss",
		},
	}
}

func DefaultSettings() Settings {
	sources := make([]SourceSettings, 0, len(Presets()))
	for _, preset := range Presets() {
		sources = append(sources, SourceSettings{
			Preset:   preset.Preset,
			Enabled:  defaultEnabled(preset.Preset),
			FeedURL:  preset.FeedURL,
			MaxItems: DefaultMaxItems,
		})
	}
	translateTitles := true
	return Settings{Sources: sources, TranslateTitles: &translateTitles}
}

func (s Settings) Normalized() Settings {
	byPreset := make(map[Preset]SourceSettings)
	for _, source := range s.Sources {
		byPreset[source.Preset] = source
	}

	result := make([]SourceSettings, 0, len(Presets()))
	for _, preset := range Presets() {
		source, ok := byPreset[preset.Preset]
		if !ok {
			source = SourceSettings{
				Preset:   preset.Preset,
				Enabled:  defaultEnabled(preset.Preset),
				FeedURL:  preset.FeedURL,
				MaxItems: DefaultMaxItems,
			}
		}
		result = append(result, source.Normalized())
	}
	translateTitles := true
	if s.TranslateTitles != nil {
		translateTitles = *s.TranslateTitles
	}
	return Settings{Sources: result, TranslateTitles: &translateTitles}
}

func (s Settings) TranslateTitlesEnabled() bool {
	normalized := s.Normalized()
	return normalized.TranslateTitles != nil && *normalized.TranslateTitles
}

func (s Settings) EnabledSources() []SourceSettings {
	normalized := s.Normalized()
	var result []SourceSettings
	for _, source := range normalized.Sources {
		if source.Enabled {
			result = append(result, source)
		}
	}
	return result
}

func (s Settings) Validate() error {
	for _, source := range s.Sources {
		if err := source.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s SourceSettings) Normalized() SourceSettings {
	info := presetInfo(s.Preset)
	if info.Preset == "" {
		info = PresetInfo{Preset: s.Preset, DisplayName: string(s.Preset), FeedURL: s.FeedURL}
	}
	if isKnownPreset(s.Preset) || isDeprecatedPresetFeed(s.Preset, s.FeedURL) {
		s.FeedURL = info.FeedURL
	} else {
		s.FeedURL = strings.TrimSpace(s.FeedURL)
	}
	if s.FeedURL == "" {
		s.FeedURL = info.FeedURL
	}
	if s.MaxItems < MinItems {
		s.MaxItems = MinItems
	}
	if s.MaxItems > MaxItems {
		s.MaxItems = MaxItems
	}
	return s
}

func (s SourceSettings) DisplayName() string {
	info := presetInfo(s.Preset)
	if info.DisplayName != "" {
		return info.DisplayName
	}
	return strings.TrimSpace(string(s.Preset))
}

func (s SourceSettings) Validate() error {
	if !s.Enabled {
		return nil
	}
	if s.MaxItems < MinItems || s.MaxItems > MaxItems {
		return fmt.Errorf("%s max items must be between %d and %d", s.DisplayName(), MinItems, MaxItems)
	}
	normalized := s.Normalized()
	if _, err := url.ParseRequestURI(normalized.FeedURL); err != nil {
		return fmt.Errorf("%s feed URL is invalid", normalized.DisplayName())
	}
	return nil
}

func NewProvider(client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Provider{Client: client}
}

func (p *Provider) Current(ctx context.Context, settings Settings) ([]Item, error) {
	var result []Item
	var failures []error
	for _, source := range settings.EnabledSources() {
		items, err := p.fetch(ctx, source)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		result = append(result, items...)
	}
	if len(result) == 0 && len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	return result, nil
}

func (p *Provider) fetch(ctx context.Context, source SourceSettings) ([]Item, error) {
	normalized := source.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized.FeedURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")
	request.Header.Set("User-Agent", "ATOL-Go-Server/1.0")

	response, err := p.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("%s: request RSS: %w", normalized.DisplayName(), err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, fmt.Errorf("%s: RSS returned HTTP %d", normalized.DisplayName(), response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: read RSS: %w", normalized.DisplayName(), err)
	}
	return ParseFeed(string(body), normalized.DisplayName(), normalized.MaxItems)
}

func (p *Provider) client() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func ParseFeed(value string, sourceName string, limit int) ([]Item, error) {
	if limit <= 0 {
		return nil, nil
	}

	decoder := xml.NewDecoder(strings.NewReader(value))
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" {
		sourceName = "RSS"
	}

	seen := make(map[string]struct{})
	var result []Item
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}

		start, ok := token.(xml.StartElement)
		if !ok || (start.Name.Local != "item" && start.Name.Local != "entry") {
			continue
		}

		item, err := parseItem(decoder, start, sourceName)
		if err != nil {
			return nil, err
		}
		if item.Title == "" {
			continue
		}
		dedupeKey := strings.ToLower(item.Title)
		if _, exists := seen[dedupeKey]; exists {
			continue
		}
		seen[dedupeKey] = struct{}{}
		result = append(result, item)
		if len(result) >= limit {
			return result, nil
		}
	}
}

func parseItem(decoder *xml.Decoder, start xml.StartElement, sourceName string) (Item, error) {
	item := Item{SourceName: sourceName}
	feedSourceName := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return Item{}, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "title":
				text, err := readElementText(decoder, typed)
				if err != nil {
					return Item{}, err
				}
				item.Title = cleanText(text)
			case "link":
				item.Link = cleanText(attribute(typed, "href"))
				text, err := readElementText(decoder, typed)
				if err != nil {
					return Item{}, err
				}
				if item.Link == "" {
					item.Link = cleanText(text)
				}
			case "source":
				text, err := readElementText(decoder, typed)
				if err != nil {
					return Item{}, err
				}
				feedSourceName = cleanText(text)
			default:
				if err := decoder.Skip(); err != nil {
					return Item{}, err
				}
			}
		case xml.EndElement:
			if typed.Name.Local == start.Name.Local {
				item.Title = normalizeTitle(item.Title, sourceName, feedSourceName)
				return item, nil
			}
		}
	}
}

func readElementText(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var builder strings.Builder
	depth := 1
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch typed := token.(type) {
		case xml.CharData:
			builder.Write([]byte(typed))
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && typed.Name.Local == start.Name.Local {
				return builder.String(), nil
			}
		}
	}
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func cleanText(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func attribute(element xml.StartElement, name string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func normalizeTitle(title string, sourceName string, feedSourceName string) string {
	title = cleanText(title)
	for _, alias := range sourceAliases(sourceName, feedSourceName) {
		for _, separator := range []string{" - ", " – ", " — ", " | "} {
			var stripped bool
			title, stripped = stripTitleSuffix(title, separator+alias)
			if stripped {
				return cleanText(title)
			}
		}
	}
	return title
}

func sourceAliases(sourceName string, feedSourceName string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, value := range []string{feedSourceName, sourceName} {
		value = cleanText(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}

	if strings.EqualFold(sourceName, "Economist") {
		result = appendMissingAlias(result, seen, "The Economist")
	}
	if strings.EqualFold(sourceName, "Reuters") {
		result = appendMissingAlias(result, seen, "Reuters.com")
	}
	return result
}

func appendMissingAlias(values []string, seen map[string]struct{}, value string) []string {
	key := strings.ToLower(value)
	if _, exists := seen[key]; exists {
		return values
	}
	seen[key] = struct{}{}
	return append(values, value)
}

func stripTitleSuffix(title string, suffix string) (string, bool) {
	if len(title) < len(suffix) {
		return title, false
	}
	tail := title[len(title)-len(suffix):]
	if !strings.EqualFold(tail, suffix) {
		return title, false
	}
	return strings.TrimSpace(title[:len(title)-len(suffix)]), true
}

func isDeprecatedPresetFeed(preset Preset, feedURL string) bool {
	return preset == PresetEconomist && strings.EqualFold(feedURL, deprecatedEconomistWorldThisWeekFeed)
}

func isKnownPreset(preset Preset) bool {
	return presetInfo(preset).Preset != ""
}

func defaultEnabled(preset Preset) bool {
	return !strings.HasPrefix(string(preset), "bloomberg_")
}

func presetInfo(preset Preset) PresetInfo {
	for _, info := range Presets() {
		if info.Preset == preset {
			return info
		}
	}
	return PresetInfo{}
}
