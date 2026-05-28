package receiptsnapshot

import (
	"strings"
	"testing"
	"time"
)

func TestRenderSnapshotHTMLGroupsNewsAndEscapesContent(t *testing.T) {
	snapshot := Snapshot{
		ID:         "snapshot-1",
		Status:     StatusPublished,
		CreatedAt:  time.Date(2026, 5, 28, 9, 10, 0, 0, time.UTC),
		PaperChars: 42,
		ReceiptLines: []ReceiptLine{
			{Text: "25 Мая", Alignment: "center", Role: "calendar", Font: 2, DoubleHeight: true, LineSize: 13.33},
			{ImageKey: "clear", ImageWidth: 96, ImageHeight: 96, ImageScalePercent: 100, Alignment: "center"},
			{Text: "Коротко о мире:", Alignment: "center", LineSize: 15},
			{Text: "- Первый <script>alert(\"x\")</script>", Link: "https://example.com/first", Alignment: "center", LineSize: 15},
			{Text: "First <World>", Alignment: "left", Role: "original", LineSize: 12},
			{Text: "- Unsafe", Link: "javascript:alert(1)", Alignment: "center", LineSize: 15},
			{QRCode: "http://192.168.0.25:8080/snapshots/snapshot-1", Alignment: "center"},
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
		`--paper-chars: 42;`,
		`class="receipt-line align-center role-calendar"`,
		`--line-size: 13.33px`,
		`class="receipt-line-text"`,
		`class="receipt-line receipt-image-line align-center role-normal"`,
		`class="receipt-image"`,
		`src="/assets/weather-icons/print/clear.png"`,
		`class="receipt-line receipt-qr-line align-center role-normal"`,
		`class="receipt-qr"`,
		"25 Мая",
		"Коротко о мире",
		"Первый &lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;",
		"First &lt;World&gt;",
		`href="https://example.com/first"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected rendered HTML to contain %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, `class="receipt"`) || strings.Contains(body, `class="line `) || strings.Contains(body, "snapshot-news-line") {
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
