package receiptsnapshot

import (
	"regexp"
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
		`class="summary-button"`,
		`data-summary-line-index="3"`,
		`data-summary-modal`,
		`/api/snapshots/`,
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
	if strings.Contains(body, `data-summary-line-index="5"`) {
		t.Fatalf("unsafe link must not get summary button:\n%s", body)
	}
	assertReceiptPaperHasNoWhitespaceNodes(t, body)
}

func TestRenderSnapshotHTMLShowsOneLeadingSummaryButtonPerLinkedBlock(t *testing.T) {
	snapshot := Snapshot{
		ID:         "snapshot-1",
		Status:     StatusPublished,
		CreatedAt:  time.Date(2026, 5, 28, 9, 10, 0, 0, time.UTC),
		PaperChars: 32,
		ReceiptLines: []ReceiptLine{
			{Text: "Hacker News: Very long title", Link: "https://example.com/one", Alignment: "center", LineSize: 15},
			{Text: "continued wrapped title", Link: "https://example.com/one", Alignment: "center", LineSize: 15},
			{Text: "GitHub: Another linked title", Link: "https://example.com/two", Alignment: "center", LineSize: 15},
			{Text: "continued second title", Link: "https://example.com/two", Alignment: "center", LineSize: 15},
			{Text: "Unsafe title", Link: "javascript:alert(1)", Alignment: "center", LineSize: 15},
		},
	}

	html, err := RenderSnapshotHTML(snapshot)
	if err != nil {
		t.Fatalf("render snapshot: %v", err)
	}
	body := string(html)

	if got := strings.Count(body, `class="summary-button"`); got != 2 {
		t.Fatalf("expected one summary button per linked block, got %d:\n%s", got, body)
	}
	for _, want := range []string{`data-summary-line-index="0"`, `data-summary-line-index="2"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected summary button %s:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{`data-summary-line-index="1"`, `data-summary-line-index="3"`, `data-summary-line-index="4"`} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("did not expect summary button %s:\n%s", unwanted, body)
		}
	}
	row := linkedRowForSummaryIndex(t, body, "0")
	if buttonIndex := strings.Index(row, `class="summary-button"`); buttonIndex < 0 {
		t.Fatalf("summary button not found in row:\n%s", row)
	} else if textIndex := strings.Index(row, `class="receipt-line-text"`); textIndex < 0 || buttonIndex > textIndex {
		t.Fatalf("summary button must be rendered before linked text:\n%s", row)
	}
}

func TestRenderSnapshotHTMLDoesNotClipLongReceiptText(t *testing.T) {
	html, err := RenderSnapshotHTML(Snapshot{
		ID:     "snapshot-1",
		Status: StatusPublished,
		ReceiptLines: []ReceiptLine{{
			Text:      "Рейсы в аэропорту Мюнхена временно приостановлены из-за возможного появления дрона",
			Link:      "https://example.com/munich",
			Alignment: "center",
			LineSize:  15,
		}},
	})
	if err != nil {
		t.Fatalf("render snapshot: %v", err)
	}
	body := string(html)

	if !strings.Contains(body, "Рейсы в аэропорту Мюнхена временно приостановлены") {
		t.Fatalf("expected full long text in snapshot HTML:\n%s", body)
	}
	lineTextCSS := cssRuleBody(t, body, ".receipt-line-text")
	for _, clippedRule := range []string{"white-space: nowrap", "overflow: hidden"} {
		if strings.Contains(lineTextCSS, clippedRule) {
			t.Fatalf("snapshot receipt text must not be clipped by %q:\n%s", clippedRule, lineTextCSS)
		}
	}
	if !strings.Contains(lineTextCSS, "overflow-wrap: anywhere") && !strings.Contains(lineTextCSS, "overflow-wrap: break-word") {
		t.Fatalf("expected snapshot receipt text to allow wrapping long content:\n%s", lineTextCSS)
	}
}

func TestRenderSnapshotHTMLShowsPendingNotice(t *testing.T) {
	html, err := RenderSnapshotHTML(Snapshot{
		ID:        "snapshot-1",
		Status:    StatusPending,
		CreatedAt: time.Date(2026, 5, 28, 9, 10, 0, 0, time.UTC),
		NewsItems: []NewsItem{{SourceName: "Example News", Title: "Заголовок"}},
	})
	if err != nil {
		t.Fatalf("render snapshot: %v", err)
	}
	if !strings.Contains(string(html), "печать еще не подтверждена") {
		t.Fatalf("expected pending notice in HTML:\n%s", string(html))
	}
}

func linkedRowForSummaryIndex(t *testing.T, body string, lineIndex string) string {
	t.Helper()
	marker := `data-summary-line-index="` + lineIndex + `"`
	markerIndex := strings.Index(body, marker)
	if markerIndex < 0 {
		t.Fatalf("summary marker %q not found:\n%s", marker, body)
	}
	rowStart := strings.LastIndex(body[:markerIndex], `<span class="receipt-link-row"`)
	if rowStart < 0 {
		t.Fatalf("linked row start not found before %q:\n%s", marker, body)
	}
	rowEnd := strings.Index(body[rowStart:], `</span></div>`)
	if rowEnd < 0 {
		t.Fatalf("linked row end not found after %q:\n%s", marker, body)
	}
	return body[rowStart : rowStart+rowEnd+len(`</span>`)]
}

func cssRuleBody(t *testing.T, body string, selector string) string {
	t.Helper()
	start := strings.Index(body, selector+" {")
	if start < 0 {
		t.Fatalf("CSS selector %q not found:\n%s", selector, body)
	}
	open := strings.Index(body[start:], "{")
	if open < 0 {
		t.Fatalf("CSS selector %q has no body:\n%s", selector, body[start:])
	}
	bodyStart := start + open + 1
	end := strings.Index(body[bodyStart:], "}")
	if end < 0 {
		t.Fatalf("CSS selector %q has no closing brace:\n%s", selector, body[bodyStart:])
	}
	return body[bodyStart : bodyStart+end]
}

func assertReceiptPaperHasNoWhitespaceNodes(t *testing.T, body string) {
	t.Helper()
	start := strings.Index(body, `<article class="receipt-paper"`)
	if start < 0 {
		t.Fatalf("receipt paper not found:\n%s", body)
	}
	start = strings.Index(body[start:], ">") + start + 1
	end := strings.Index(body[start:], "</article>")
	if end < 0 {
		t.Fatalf("receipt paper closing tag not found:\n%s", body)
	}
	articleBody := body[start : start+end]
	if regexp.MustCompile(`>\s+<`).MatchString(articleBody) {
		t.Fatalf("receipt paper must not contain whitespace text nodes between line elements:\n%s", articleBody)
	}
}
