package receipt

import (
	"reflect"
	"testing"

	"atol-server/internal/denistrends"
	"atol-server/internal/news"
)

func TestContentSettingsNormalizePreservesEmbeddedScheduleSettings(t *testing.T) {
	translateTitles := false
	newsSettings := news.Settings{
		TranslateTitles: &translateTitles,
		Sources: []news.SourceSettings{
			{Preset: news.PresetBBCRussian, Enabled: true, FeedURL: "https://example.com/rss", MaxItems: 7},
		},
	}
	trendsSettings := denistrends.Settings{
		Periods: map[denistrends.Period]denistrends.PeriodSettings{
			denistrends.PeriodNow:   {Enabled: false, MaxItems: 4},
			denistrends.PeriodDay:   {Enabled: true, MaxItems: 9},
			denistrends.PeriodWeek:  {Enabled: true, MaxItems: 11},
			denistrends.PeriodMonth: {Enabled: true, MaxItems: 13},
		},
	}
	content := ContentSettings{
		Configured:          true,
		ShowNews:            true,
		ShowDenisTrends:     true,
		DenisTrendsMode:     DenisTrendsModeMonth,
		NewsSettings:        &newsSettings,
		DenisTrendsSettings: &trendsSettings,
	}

	normalized := content.Normalized()

	if normalized.NewsSettings == nil {
		t.Fatal("expected embedded news settings to be preserved")
	}
	if !reflect.DeepEqual(*normalized.NewsSettings, newsSettings.Normalized()) {
		t.Fatalf("expected normalized embedded news settings %#v, got %#v", newsSettings.Normalized(), *normalized.NewsSettings)
	}
	if normalized.DenisTrendsSettings == nil {
		t.Fatal("expected embedded Denis Trends settings to be preserved")
	}
	if !reflect.DeepEqual(*normalized.DenisTrendsSettings, trendsSettings.Normalized()) {
		t.Fatalf("expected normalized embedded Denis Trends settings %#v, got %#v", trendsSettings.Normalized(), *normalized.DenisTrendsSettings)
	}
	if normalized.DenisTrendsMode != DenisTrendsModeMonth {
		t.Fatalf("expected month mode to be preserved, got %q", normalized.DenisTrendsMode)
	}
}
