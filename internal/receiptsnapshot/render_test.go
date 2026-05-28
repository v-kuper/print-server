package receiptsnapshot

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSnapshotHTMLGroupsNewsAndEscapesContent(t *testing.T) {
	snapshot := Snapshot{
		ID:        "snapshot-1",
		Status:    StatusPublished,
		CreatedAt: time.Date(2026, 5, 28, 9, 10, 0, 0, time.UTC),
		ReceiptLines: []ReceiptLine{
			{Text: "25 Мая", Alignment: "center", Role: "calendar"},
			{Text: "Погода", Alignment: "center"},
			{Text: "Коротко о мире:", Alignment: "center"},
			{Text: "- Первый <script>alert(\"x\")</script>", Alignment: "center"},
		},
		NewsItems: []NewsItem{
			{
				SourceName:    "BBC <Russian>",
				Title:         `Первый <script>alert("x")</script>`,
				OriginalTitle: "First <World>",
				Link:          "https://example.com/first",
			},
			{
				SourceName: "BBC <Russian>",
				Title:      "Второй",
				Link:       "javascript:alert(1)",
			},
			{
				SourceName: "Reuters",
				Title:      "Third",
				Link:       "https://example.com/third",
			},
		},
	}

	html, err := RenderSnapshotHTML(snapshot)
	if err != nil {
		t.Fatalf("render snapshot: %v", err)
	}
	body := string(html)

	for _, want := range []string{
		`class="receipt-preview"`,
		`class="receipt-paper"`,
		`class="receipt-line align-center role-calendar"`,
		`class="receipt-line-text"`,
		"25 Мая",
		"Погода",
		"Коротко о мире",
		"BBC &lt;Russian&gt;",
		"Первый &lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;",
		"First &lt;World&gt;",
		`href="https://example.com/first"`,
		"Reuters",
		`href="https://example.com/third"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected rendered HTML to contain %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `class="receipt"`) || strings.Contains(body, `class="line `) {
		t.Fatalf("snapshot must use receipt preview structure:\n%s", body)
	}
	if strings.Contains(body, "javascript:alert") {
		t.Fatalf("unsafe javascript link must not be rendered:\n%s", body)
	}
}

func TestRenderSnapshotHTMLShowsPendingNotice(t *testing.T) {
	html, err := RenderSnapshotHTML(Snapshot{
		ID:        "snapshot-1",
		Status:    StatusPending,
		CreatedAt: time.Date(2026, 5, 28, 9, 10, 0, 0, time.UTC),
		NewsItems: []NewsItem{{SourceName: "BBC", Title: "Заголовок"}},
	})
	if err != nil {
		t.Fatalf("render snapshot: %v", err)
	}
	if !strings.Contains(string(html), "печать еще не подтверждена") {
		t.Fatalf("expected pending notice in HTML:\n%s", string(html))
	}
}
