package news

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseFeedParsesRssTitlesAndDeduplicates(t *testing.T) {
	xml := `
		<rss><channel>
			<item><title>Первый заголовок</title><link>https://example.com/1</link></item>
			<item><title>Первый заголовок</title><link>https://example.com/dup</link></item>
			<item><title>Второй &amp; важный</title><link>https://example.com/2</link></item>
		</channel></rss>
	`

	items, err := ParseFeed(xml, "BBC", 3)
	if err != nil {
		t.Fatalf("parse RSS: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %#v", len(items), items)
	}
	if items[0].Title != "Первый заголовок" || items[0].SourceName != "BBC" || items[0].Link != "https://example.com/1" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Title != "Второй & важный" {
		t.Fatalf("unexpected second title: %q", items[1].Title)
	}
}

func TestParseFeedParsesAtomLinks(t *testing.T) {
	xml := `
		<feed xmlns="http://www.w3.org/2005/Atom">
			<entry>
				<title>Atom заголовок</title>
				<link href="https://example.com/a" />
			</entry>
		</feed>
	`

	items, err := ParseFeed(xml, "Atom", 10)
	if err != nil {
		t.Fatalf("parse Atom: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Link != "https://example.com/a" {
		t.Fatalf("expected Atom href, got %q", items[0].Link)
	}
}

func TestParseFeedStripsGoogleNewsSourceSuffix(t *testing.T) {
	xml := `
		<rss><channel>
			<item>
				<title>Netanyahu admits difficulty influencing Trump decisions on Iran, sources say - Reuters</title>
				<link>https://news.google.com/rss/articles/1</link>
				<source url="https://www.reuters.com">Reuters</source>
			</item>
			<item>
				<title>France’s Gen Z has fallen for a 74-year-old radical socialist - The Economist</title>
				<link>https://news.google.com/rss/articles/2</link>
				<source url="https://www.economist.com">The Economist</source>
			</item>
		</channel></rss>
	`

	reutersItems, err := ParseFeed(xml, "Reuters", 1)
	if err != nil {
		t.Fatalf("parse Reuters Google News RSS: %v", err)
	}
	if got, want := reutersItems[0].Title, "Netanyahu admits difficulty influencing Trump decisions on Iran, sources say"; got != want {
		t.Fatalf("expected Reuters suffix to be stripped, got %q", got)
	}

	economistItems, err := ParseFeed(xml, "Economist", 2)
	if err != nil {
		t.Fatalf("parse Economist Google News RSS: %v", err)
	}
	if got, want := economistItems[1].Title, "France’s Gen Z has fallen for a 74-year-old radical socialist"; got != want {
		t.Fatalf("expected Economist suffix to be stripped, got %q", got)
	}
}

func TestParseFeedKeepsTextInsideNestedTitleTags(t *testing.T) {
	xml := `
		<rss><channel>
			<item><title>AI <b>super</b> apps are remaking China’s internet</title></item>
		</channel></rss>
	`

	items, err := ParseFeed(xml, "Economist", 1)
	if err != nil {
		t.Fatalf("parse nested RSS title: %v", err)
	}
	if got, want := items[0].Title, "AI super apps are remaking China’s internet"; got != want {
		t.Fatalf("expected nested title text to be preserved, got %q", got)
	}
}

func TestEconomistPresetUsesLatestFeedAndMigratesOldDefault(t *testing.T) {
	info := presetInfo(PresetEconomist)
	if got, want := info.FeedURL, "https://www.economist.com/latest/rss.xml"; got != want {
		t.Fatalf("unexpected Economist preset URL: got %q want %q", got, want)
	}

	settings := SourceSettings{
		Preset:   PresetEconomist,
		Enabled:  true,
		FeedURL:  deprecatedEconomistWorldThisWeekFeed,
		MaxItems: 10,
	}.Normalized()
	if got, want := settings.FeedURL, info.FeedURL; got != want {
		t.Fatalf("expected deprecated Economist feed to migrate, got %q want %q", got, want)
	}
}

func TestSettingsDefaultsTranslateTitlesAndPreservesDisabledTranslation(t *testing.T) {
	if !DefaultSettings().TranslateTitlesEnabled() {
		t.Fatal("expected news title translation to be enabled by default")
	}

	migrated := Settings{}.Normalized()
	if !migrated.TranslateTitlesEnabled() {
		t.Fatalf("expected missing translation setting to migrate to enabled, got %#v", migrated)
	}

	disabled := false
	settings := Settings{TranslateTitles: &disabled}.Normalized()
	if settings.TranslateTitlesEnabled() {
		t.Fatalf("expected explicit disabled translation setting to be preserved, got %#v", settings)
	}
}

func TestProviderFetchesEnabledSources(t *testing.T) {
	client := &http.Client{Transport: newsRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/rss+xml"}},
			Body: io.NopCloser(strings.NewReader(`
				<rss><channel>
					<item><title>Новость 1</title></item>
					<item><title>Новость 2</title></item>
				</channel></rss>
			`)),
		}, nil
	})}
	provider := NewProvider(client)
	settings := Settings{
		Sources: []SourceSettings{
			{Preset: PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 1},
			{Preset: PresetReuters, Enabled: false, FeedURL: "https://example.com/reuters", MaxItems: 10},
			{Preset: PresetEconomist, Enabled: false, FeedURL: "https://example.com/economist", MaxItems: 10},
			{Preset: PresetHackerNews, Enabled: false, FeedURL: "https://example.com/hn", MaxItems: 10},
		},
	}

	items, err := provider.Current(context.Background(), settings)
	if err != nil {
		t.Fatalf("load news: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected one limited item, got %d: %#v", len(items), items)
	}
	if items[0].SourceName != "BBC Russian" {
		t.Fatalf("expected preset source name, got %q", items[0].SourceName)
	}
}

func TestSettingsValidateRejectsEnabledSourceWithoutCount(t *testing.T) {
	settings := Settings{Sources: []SourceSettings{
		{Preset: PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 0},
	}}

	if err := settings.Validate(); err == nil {
		t.Fatal("expected enabled source without count to be rejected")
	}
}

func TestSettingsValidateAllowsDisabledSourceWithoutCount(t *testing.T) {
	settings := Settings{Sources: []SourceSettings{
		{Preset: PresetBBCRussian, Enabled: false, FeedURL: "https://example.com/rss", MaxItems: 0},
	}}

	if err := settings.Validate(); err != nil {
		t.Fatalf("expected disabled source without count to be ignored, got %v", err)
	}
}

type newsRoundTripFunc func(*http.Request) (*http.Response, error)

func (f newsRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
