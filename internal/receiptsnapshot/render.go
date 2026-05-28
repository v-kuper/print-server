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
	Text               string
	Link               string
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
	lines := snapshot.ReceiptLines
	if len(lines) == 0 {
		lines = fallbackNewsReceiptLines(snapshot.NewsItems)
	}
	result := make([]snapshotReceiptLine, 0, len(lines))
	for _, line := range lines {
		result = append(result, prepareReceiptLine(line))
	}
	return result
}

func prepareReceiptLine(line ReceiptLine) snapshotReceiptLine {
	line.Text = strings.TrimRight(line.Text, "\r\n")
	line = normalizeReceiptLine(line)
	result := snapshotReceiptLine{
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
		ReceiptLine{Text: "Коротко о мире:", Alignment: "center", Role: "normal", LineSize: 15},
		ReceiptLine{Text: " ", Alignment: "center", Role: "normal", LineSize: 15},
	)
	for sourceIndex, group := range groups {
		lines = append(lines, ReceiptLine{Text: group.SourceName, Alignment: "center", Role: "normal", LineSize: 15})
		for itemIndex, item := range group.Items {
			lines = append(lines, ReceiptLine{Text: "- " + item.Title, Link: item.Link, Alignment: "center", Role: "normal", LineSize: 15})
			if strings.TrimSpace(item.OriginalTitle) != "" {
				lines = append(lines, ReceiptLine{Text: item.OriginalTitle, Alignment: "left", Role: "original", LineSize: 13})
			}
			if itemIndex < len(group.Items)-1 {
				lines = append(lines, ReceiptLine{Text: " ", Alignment: "center", Role: "normal", LineSize: 15})
			}
		}
		if sourceIndex < len(groups)-1 {
			lines = append(lines, ReceiptLine{Text: " ", Alignment: "center", Role: "normal", LineSize: 15})
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
      width: min(100%, calc(var(--paper-chars, 32) * 1ch));
      margin: 0 auto;
      color: #111;
      font-family: "Courier New", ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 16px;
      line-height: 1.22;
      letter-spacing: 0;
      white-space: pre-wrap;
    }
    .receipt-line {
      --line-size: 15px;
      --line-scale-x: 1;
      --line-scale-y: 1;
      display: flex;
      align-items: center;
      justify-content: flex-start;
      width: calc(100% / var(--line-scale-x));
      margin: 0 auto;
      min-height: calc(var(--line-size) * 1.22 * var(--line-scale-y));
      line-height: 1.22;
      overflow: visible;
    }
    .receipt-line-text {
      display: inline-block;
      max-width: calc(100% / var(--line-scale-x));
      font-size: var(--line-size);
      line-height: 1.22;
      white-space: nowrap;
      overflow: hidden;
      transform: scale(var(--line-scale-x), var(--line-scale-y));
      transform-origin: center center;
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
    .align-left   { text-align: left;   justify-content: flex-start; }
    .align-center { text-align: center; justify-content: center;     }
    .align-right  { text-align: right;  justify-content: flex-end;   }
    .align-left  .receipt-line-text { transform-origin: left center;  }
    .align-right .receipt-line-text { transform-origin: right center; }
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
  <main>
    <section class="receipt-preview">
      <article class="receipt-paper" style="--paper-chars: {{.PaperChars}};">
        {{if .PendingNotice}}<p class="notice">Этот слепок создан, но печать еще не подтверждена.</p>{{end}}
        {{range .Lines}}
          {{if .QRCode}}
          <div class="receipt-line receipt-qr-line align-{{.Alignment}} role-{{.Role}}">
            {{if .QRDataURL}}<img class="receipt-qr" src="{{.QRDataURL}}" alt="{{.QRCode}}">{{else}}<span class="receipt-line-text">{{.QRCode}}</span>{{end}}
          </div>
          {{else if .ImageSrc}}
          <div class="receipt-line receipt-image-line align-{{.Alignment}} role-{{.Role}}" style="--image-line-height: {{.ImageLineHeight}}px;">
            <img class="receipt-image" src="{{.ImageSrc}}" alt="" style="width: {{.ImagePreviewWidth}}px; height: {{.ImagePreviewHeight}}px;">
          </div>
          {{else}}
          <div class="receipt-line align-{{.Alignment}} role-{{.Role}}" style="--line-size: {{.LineSize}}px; --line-scale-x: {{.ScaleX}}; --line-scale-y: {{.ScaleY}};">
            {{if .Link}}<a class="receipt-line-text" href="{{.Link}}" target="_blank" rel="noopener noreferrer">{{if .Text}}{{.Text}}{{else}}&nbsp;{{end}}</a>{{else}}<span class="receipt-line-text">{{if .Text}}{{.Text}}{{else}}&nbsp;{{end}}</span>{{end}}
          </div>
          {{end}}
        {{end}}
      </article>
    </section>
  </main>
</body>
</html>`))
