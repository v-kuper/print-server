package receiptsnapshot

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

type NewsGroup struct {
	SourceName string
	Items      []NewsItem
}

type snapshotPageData struct {
	ID            string
	PendingNotice bool
	PaperChars    int
	Lines         []snapshotReceiptLine
}

type snapshotReceiptLine struct {
	LineIndex          int
	Text               string
	Link               string
	ShowSummaryButton  bool
	QRCode             string
	QRDataURL          template.URL
	ImageSrc           string
	ImagePreviewWidth  int
	ImagePreviewHeight int
	ImageLineHeight    int
	Alignment          string
	Role               string
	LineSize           string
	ScaleX             int
	ScaleY             int
}

func RenderSnapshotHTML(snapshot Snapshot) ([]byte, error) {
	data := snapshotPageData{
		ID:            snapshot.ID,
		PendingNotice: snapshot.Status == StatusPending,
		PaperChars:    normalizePaperChars(snapshot.PaperChars),
		Lines:         prepareReceiptLines(snapshot),
	}
	var buffer bytes.Buffer
	if err := snapshotTemplate.Execute(&buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func prepareReceiptLines(snapshot Snapshot) []snapshotReceiptLine {
	lines := ReceiptLinesForSnapshot(snapshot)
	result := make([]snapshotReceiptLine, 0, len(lines))
	previousLink := ""
	for index, line := range lines {
		prepared := prepareReceiptLine(index, line)
		if prepared.Link != "" {
			prepared.ShowSummaryButton = prepared.Link != previousLink
			previousLink = prepared.Link
		} else {
			previousLink = ""
		}
		result = append(result, prepared)
	}
	return result
}

func ReceiptLinesForSnapshot(snapshot Snapshot) []ReceiptLine {
	if len(snapshot.ReceiptLines) > 0 {
		return snapshot.ReceiptLines
	}
	return fallbackNewsReceiptLines(snapshot.NewsItems)
}

func prepareReceiptLine(index int, line ReceiptLine) snapshotReceiptLine {
	line.Text = strings.TrimRight(line.Text, "\r\n")
	line = normalizeReceiptLine(line)
	result := snapshotReceiptLine{
		LineIndex: index,
		Text:      line.Text,
		Link:      safeHTTPURL(line.Link),
		QRCode:    strings.TrimSpace(line.QRCode),
		ImageSrc:  snapshotImageSrc(line),
		Alignment: line.Alignment,
		Role:      line.Role,
		LineSize:  fmt.Sprintf("%.2f", normalizeLineSize(line.LineSize)),
		ScaleX:    1,
		ScaleY:    1,
	}
	if line.DoubleWidth {
		result.ScaleX = 2
	}
	if line.DoubleHeight {
		result.ScaleY = 2
	}
	if result.QRCode != "" {
		result.QRDataURL = qrCodeDataURL(result.QRCode)
	}
	if result.ImageSrc != "" {
		width := line.ImageWidth
		if width <= 0 {
			width = 96
		}
		height := line.ImageHeight
		if height <= 0 {
			height = width
		}
		previewWidth := clampInt(int(float64(width)*0.8), 48, 320)
		previewHeight := int(float64(previewWidth) * float64(height) / float64(width))
		if previewHeight < 32 {
			previewHeight = 32
		}
		result.ImagePreviewWidth = previewWidth
		result.ImagePreviewHeight = previewHeight
		result.ImageLineHeight = previewHeight + 8
	}
	return result
}

func fallbackNewsReceiptLines(items []NewsItem) []ReceiptLine {
	groups := GroupNews(items)
	if len(groups) == 0 {
		return nil
	}
	var lines []ReceiptLine
	lines = append(lines,
		ReceiptLine{Text: "Коротко о мире:", Alignment: "left", Role: "normal", LineSize: 15},
		ReceiptLine{Text: " ", Alignment: "left", Role: "normal", LineSize: 15},
	)
	for sourceIndex, group := range groups {
		lines = append(lines, ReceiptLine{Text: group.SourceName, Alignment: "left", Role: "normal", LineSize: 15})
		for itemIndex, item := range group.Items {
			lines = append(lines, ReceiptLine{Text: "- " + item.Title, Link: item.Link, Alignment: "left", Role: "normal", LineSize: 15})
			if strings.TrimSpace(item.OriginalTitle) != "" {
				lines = append(lines, ReceiptLine{Text: item.OriginalTitle, Alignment: "left", Role: "original", LineSize: 13})
			}
			if itemIndex < len(group.Items)-1 {
				lines = append(lines, ReceiptLine{Text: " ", Alignment: "left", Role: "normal", LineSize: 15})
			}
		}
		if sourceIndex < len(groups)-1 {
			lines = append(lines, ReceiptLine{Text: " ", Alignment: "left", Role: "normal", LineSize: 15})
		}
	}
	return lines
}

func normalizeReceiptLine(line ReceiptLine) ReceiptLine {
	switch line.Alignment {
	case "left", "center", "right":
	default:
		line.Alignment = "center"
	}
	switch line.Role {
	case "normal", "calendar", "temperature", "original":
	default:
		line.Role = "normal"
	}
	return line
}

func normalizeLineSize(value float64) float64 {
	if value <= 0 {
		return 15
	}
	if value < 10 {
		return 10
	}
	if value > 32 {
		return 32
	}
	return value
}

func snapshotImageSrc(line ReceiptLine) string {
	if value := safeImageURL(line.ImageURL); value != "" {
		return value
	}
	key := strings.TrimSpace(line.ImageKey)
	if key == "" {
		return ""
	}
	return "/assets/weather-icons/print/" + url.PathEscape(key) + ".png"
}

func safeImageURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return value
	}
	return safeHTTPURL(value)
}

func qrCodeDataURL(value string) template.URL {
	png, err := qrcode.Encode(value, qrcode.Medium, 156)
	if err != nil {
		return ""
	}
	return template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func GroupNews(items []NewsItem) []NewsGroup {
	indexBySource := make(map[string]int)
	groups := make([]NewsGroup, 0)
	for _, item := range items {
		item.Title = strings.TrimSpace(item.Title)
		if item.Title == "" {
			continue
		}
		item.OriginalTitle = strings.TrimSpace(item.OriginalTitle)
		item.SourceName = strings.TrimSpace(item.SourceName)
		if item.SourceName == "" {
			item.SourceName = "RSS"
		}
		item.Link = safeHTTPURL(item.Link)
		index, exists := indexBySource[item.SourceName]
		if !exists {
			index = len(groups)
			indexBySource[item.SourceName] = index
			groups = append(groups, NewsGroup{SourceName: item.SourceName})
		}
		groups[index].Items = append(groups[index].Items, item)
	}
	return groups
}

func safeHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

var snapshotTemplate = template.Must(template.New("receipt-snapshot").Parse(`<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Коротко о мире</title>
  <style>
    :root { color-scheme: light; }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background: #f6f2eb;
      color: #111;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      line-height: 1.45;
    }
    main {
      width: min(520px, 100%);
      margin: 0 auto;
      padding: 18px 12px 32px;
    }
    .receipt-preview {
      overflow: visible;
      background: #FEFCF9;
      border: 1px solid #DDD5C4;
      border-radius: 7px;
      padding: 16px 14px;
    }
    .receipt-paper {
      width: 100%;
      margin: 0;
      color: #111;
      font-family: "Courier New", ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 15px;
      line-height: 1.5;
      letter-spacing: 0;
    }
    /* Text lines: render inline so 32-char printer-split lines flow together */
    .receipt-line {
      display: inline;
    }
    /* After each text line add a space so joined words don't merge */
    .receipt-line::after {
      content: " ";
    }
    /* Image and QR lines: keep as blocks with centering */
    .receipt-image-line,
    .receipt-qr-line {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 100%;
    }
    .receipt-image-line::after,
    .receipt-qr-line::after {
      content: none;
    }
    .receipt-line-text {
      display: inline;
      font-size: var(--line-size);
      line-height: 1.5;
      white-space: normal;
      overflow-wrap: break-word;
      word-break: break-word;
    }
    a.receipt-line-text {
      color: inherit;
      text-decoration: none;
    }
    a.receipt-line-text:hover,
    a.receipt-line-text:focus {
      text-decoration: underline;
      text-underline-offset: 2px;
    }
    .receipt-link-row {
      display: inline;
    }
    .receipt-link-row .receipt-line-text {
      display: inline;
    }
    .summary-button {
      appearance: none;
      border: 1px solid #cfc5b2;
      background: #fffaf1;
      color: #302719;
      width: 22px;
      height: 22px;
      border-radius: 50%;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      flex: 0 0 22px;
      margin-top: 1px;
      padding: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-size: 12px;
      line-height: 1;
      cursor: pointer;
      align-self: flex-start;
    }
    .summary-button:hover,
    .summary-button:focus {
      border-color: #8f7a55;
      background: #fff1d2;
      outline: none;
    }
    .summary-modal[hidden] { display: none; }
    .summary-modal {
      position: fixed;
      inset: 0;
      z-index: 20;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 18px;
    }
    .summary-backdrop {
      position: absolute;
      inset: 0;
      background: rgba(27, 22, 16, 0.36);
    }
    .summary-dialog {
      position: relative;
      width: min(460px, 100%);
      max-height: min(78vh, 680px);
      overflow: auto;
      border: 1px solid #d8cdb8;
      border-radius: 8px;
      background: #fffdf8;
      box-shadow: 0 18px 45px rgba(30, 24, 14, 0.24);
      padding: 18px 18px 16px;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      white-space: normal;
    }
    .summary-close {
      appearance: none;
      position: absolute;
      top: 10px;
      right: 10px;
      width: 30px;
      height: 30px;
      border: 1px solid transparent;
      border-radius: 50%;
      background: transparent;
      color: #41382b;
      font-size: 20px;
      line-height: 1;
      cursor: pointer;
    }
    .summary-close:hover,
    .summary-close:focus {
      border-color: #d8cdb8;
      background: #f8f0df;
      outline: none;
    }
    .summary-source {
      margin: 0 34px 4px 0;
      color: #6d6251;
      font-size: 12px;
      overflow-wrap: anywhere;
    }
    .summary-title {
      margin: 0 34px 10px 0;
      color: #17130d;
      font-size: 18px;
      line-height: 1.25;
      letter-spacing: 0;
    }
    .summary-status {
      margin: 0 0 10px;
      color: #5a4c37;
      font-size: 14px;
    }
    .summary-status.error { color: #8a241f; }
    .summary-text {
      margin: 0 0 10px;
      color: #211b12;
      font-size: 15px;
      line-height: 1.45;
    }
    .summary-bullets {
      margin: 0 0 14px 18px;
      padding: 0;
      color: #211b12;
      font-size: 15px;
      line-height: 1.45;
    }
    .summary-original {
      color: #5d4619;
      font-size: 14px;
      text-decoration: underline;
      text-underline-offset: 2px;
    }
    .align-left   { text-align: left;   justify-content: flex-start; }
    .align-center { text-align: center; justify-content: center;     }
    .align-right  { text-align: right;  justify-content: flex-end;   }
    .align-center .receipt-line-text { transform-origin: center center; }
    .align-right  .receipt-line-text { transform-origin: right center;  }
    .receipt-image-line {
      width: 100%;
      min-height: var(--image-line-height, 84px);
      padding: 4px 0;
    }
    .receipt-image {
      display: block;
      width: 76px;
      height: 76px;
      object-fit: contain;
      image-rendering: auto;
    }
    .receipt-qr-line {
      width: 100%;
      min-height: 168px;
      padding: 8px 0 10px;
    }
    .receipt-qr {
      display: block;
      width: 156px;
      height: 156px;
      background: #fff;
      image-rendering: pixelated;
    }
    .notice {
      margin: 12px 0;
      padding: 10px 12px;
      border: 1px solid #dec16a;
      background: #fff7d7;
      color: #4f3d00;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-size: 14px;
    }
  </style>
</head>
<body>
  <main data-snapshot-id="{{.ID}}">
    <section class="receipt-preview">
      <article class="receipt-paper" style="--paper-chars: {{.PaperChars}};">{{- if .PendingNotice -}}<p class="notice">Этот слепок создан, но печать еще не подтверждена.</p>{{- end -}}{{- range .Lines -}}{{- if .QRCode -}}<div class="receipt-line receipt-qr-line align-{{.Alignment}} role-{{.Role}}">{{- if .QRDataURL -}}<img class="receipt-qr" src="{{.QRDataURL}}" alt="{{.QRCode}}">{{- else -}}<span class="receipt-line-text">{{.QRCode}}</span>{{- end -}}</div>{{- else if .ImageSrc -}}<div class="receipt-line receipt-image-line align-{{.Alignment}} role-{{.Role}}" style="--image-line-height: {{.ImageLineHeight}}px;"><img class="receipt-image" src="{{.ImageSrc}}" alt="" style="width: {{.ImagePreviewWidth}}px; height: {{.ImagePreviewHeight}}px;"></div>{{- else -}}<div class="receipt-line align-{{.Alignment}} role-{{.Role}}" style="--line-size: {{.LineSize}}px; --line-scale-x: {{.ScaleX}}; --line-scale-y: {{.ScaleY}};">{{- if .Link -}}<span class="receipt-link-row">{{- if .ShowSummaryButton -}}<button class="summary-button" type="button" data-summary-button data-summary-line-index="{{.LineIndex}}" title="Сделать summary" aria-label="Сделать summary"><span aria-hidden="true">✦</span></button>{{- end -}}<a class="receipt-line-text" href="{{.Link}}" target="_blank" rel="noopener noreferrer">{{if .Text}}{{.Text}}{{else}}&nbsp;{{end}}</a></span>{{- else -}}<span class="receipt-line-text">{{if .Text}}{{.Text}}{{else}}&nbsp;{{end}}</span>{{- end -}}</div>{{- end -}}{{- end -}}</article>
    </section>
    <div class="summary-modal" data-summary-modal hidden>
      <div class="summary-backdrop" data-summary-close></div>
      <section class="summary-dialog" role="dialog" aria-modal="true" aria-labelledby="summary-title">
        <button class="summary-close" type="button" data-summary-close aria-label="Закрыть">×</button>
        <p class="summary-source" data-summary-source></p>
        <h2 class="summary-title" id="summary-title" data-summary-title>Кратко</h2>
        <p class="summary-status" data-summary-status></p>
        <p class="summary-text" data-summary-text></p>
        <ul class="summary-bullets" data-summary-bullets></ul>
        <a class="summary-original" data-summary-original target="_blank" rel="noopener noreferrer">Открыть оригинал</a>
      </section>
    </div>
  </main>
  <script>
    (() => {
      const root = document.querySelector("[data-snapshot-id]");
      const modal = document.querySelector("[data-summary-modal]");
      if (!root || !modal) return;
      const snapshotID = root.dataset.snapshotId || "";
      const status = modal.querySelector("[data-summary-status]");
      const source = modal.querySelector("[data-summary-source]");
      const title = modal.querySelector("[data-summary-title]");
      const text = modal.querySelector("[data-summary-text]");
      const bullets = modal.querySelector("[data-summary-bullets]");
      const original = modal.querySelector("[data-summary-original]");

      function setOpen(open) {
        modal.hidden = !open;
        document.body.style.overflow = open ? "hidden" : "";
      }

      function resetModal(lineText) {
        title.textContent = lineText || "Кратко";
        source.textContent = "";
        status.textContent = "Читаю источник...";
        status.classList.remove("error");
        text.textContent = "";
        bullets.replaceChildren();
        original.hidden = true;
        original.removeAttribute("href");
      }

      function renderSummary(payload) {
        title.textContent = payload.title || "Кратко";
        source.textContent = payload.cached ? "Сохраненное summary" : "Новое summary";
        status.textContent = "";
        text.textContent = payload.summary || "";
        bullets.replaceChildren();
        for (const item of payload.bullets || []) {
          const li = document.createElement("li");
          li.textContent = item;
          bullets.appendChild(li);
        }
        if (payload.url) {
          original.href = payload.url;
          original.hidden = false;
        }
      }

      function renderError(message, href) {
        status.textContent = message || "Не удалось собрать summary. Открой оригинал.";
        status.classList.add("error");
        text.textContent = "";
        bullets.replaceChildren();
        if (href) {
          original.href = href;
          original.hidden = false;
        }
      }

      async function loadSummary(button) {
        const lineIndex = button.dataset.summaryLineIndex;
        const line = button.closest(".receipt-line");
        const link = line ? line.querySelector("a.receipt-line-text") : null;
        resetModal(link ? link.textContent.trim() : "");
        if (link && link.href) {
          original.href = link.href;
          original.hidden = false;
        }
        setOpen(true);
        try {
          const response = await fetch("/api/snapshots/" + encodeURIComponent(snapshotID) + "/lines/" + encodeURIComponent(lineIndex) + "/summary", { method: "POST" });
          const payload = await response.json();
          if (!response.ok || !payload.ok) {
            throw new Error(payload.error || "Не удалось собрать summary.");
          }
          renderSummary(payload);
        } catch (error) {
          renderError(error && error.message ? error.message : "Не удалось собрать summary.", link ? link.href : "");
        }
      }

      document.querySelectorAll("[data-summary-button]").forEach(button => {
        button.addEventListener("click", event => {
          event.preventDefault();
          event.stopPropagation();
          loadSummary(button);
        });
      });
      modal.querySelectorAll("[data-summary-close]").forEach(button => {
        button.addEventListener("click", () => setOpen(false));
      });
      document.addEventListener("keydown", event => {
        if (event.key === "Escape") setOpen(false);
      });
    })();
  </script>
</body>
</html>`))
