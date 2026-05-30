package receipt

import (
	"testing"
	"time"

	"atol-server/internal/denistrends"
	"atol-server/internal/weather"
)

func TestDailyReceiptRendersDenisTrendsAsSeparateSection(t *testing.T) {
	lines := DailyReceipt(DailyReceiptData{
		HideWeather: true,
		Weather: weather.Snapshot{
			ObservedAt: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		},
		DenisTrendSections: []denistrends.Section{
			{
				Period: denistrends.PeriodDay,
				Items: []denistrends.Item{
					{Title: "HN title", SourceName: "Hacker News", Link: "https://example.com/hn"},
					{Title: "Repo title", SourceName: "GitHub", Link: "https://github.com/acme/repo"},
				},
			},
			{
				Period: denistrends.PeriodWeek,
				Items: []denistrends.Item{
					{Title: "Model title", SourceName: "Hype Replicate"},
				},
			},
		},
	})

	got := texts(lines)
	requireContains(t, got, "Denis Trends")
	requireContains(t, got, "Top day")
	requireContains(t, got, "Hacker News: HN title")
	requireContains(t, got, "GitHub: Repo title")
	requireContains(t, got, "Top week")
	requireContains(t, got, "Hype Replicate: Model title")

	if link := linkForText(lines, "Hacker News: HN title"); link != "https://example.com/hn" {
		t.Fatalf("expected link to be retained on trend title line, got %q", link)
	}
}

func linkForText(lines []Line, text string) string {
	for _, line := range lines {
		if line.Text == text {
			return line.Link
		}
	}
	return ""
}
