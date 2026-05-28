package receiptsnapshot

import (
	"bytes"
	"html/template"
	"net/url"
	"strings"
)

type NewsGroup struct {
	SourceName string
	Items      []NewsItem
}

type snapshotPageData struct {
	ID            string
	PendingNotice bool
	ReceiptLines  []ReceiptLine
	Groups        []NewsGroup
}

func RenderSnapshotHTML(snapshot Snapshot) ([]byte, error) {
	data := snapshotPageData{
		ID:            snapshot.ID,
		PendingNotice: snapshot.Status == StatusPending,
		ReceiptLines:  receiptLinesBeforeNews(snapshot.ReceiptLines),
		Groups:        GroupNews(snapshot.NewsItems),
	}
	var buffer bytes.Buffer
	if err := snapshotTemplate.Execute(&buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func receiptLinesBeforeNews(lines []ReceiptLine) []ReceiptLine {
	if len(lines) == 0 {
		return nil
	}
	result := make([]ReceiptLine, 0, len(lines))
	for _, line := range lines {
		line.Text = strings.TrimRight(line.Text, "\r\n")
		if strings.TrimSpace(line.Text) == "Коротко о мире:" {
			break
		}
		result = append(result, normalizeReceiptLine(line))
	}
	return result
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
    .align-left   { text-align: left;   justify-content: flex-start; }
    .align-center { text-align: center; justify-content: center;     }
    .align-right  { text-align: right;  justify-content: flex-end;   }
    .align-left  .receipt-line-text { transform-origin: left center;  }
    .align-right .receipt-line-text { transform-origin: right center; }
    .double-width { --line-scale-x: 2; }
    .double-height { --line-scale-y: 2; }
    .role-calendar .receipt-line-text,
    .role-temperature .receipt-line-text {
      font-weight: 700;
    }
    .role-original .receipt-line-text {
      color: #6f6658;
      font-size: 13px;
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
    a {
      color: #0b5cad;
      text-decoration: underline;
      text-underline-offset: 2px;
    }
    .snapshot-news-line {
      width: 100%;
      min-height: auto;
      padding: 1px 0;
    }
    .snapshot-news-line .receipt-line-text {
      max-width: 100%;
      white-space: normal;
      overflow: visible;
      overflow-wrap: anywhere;
      transform: none;
    }
    .snapshot-source-line {
      margin-top: 6px;
      font-weight: 700;
    }
    .snapshot-source-line:first-of-type {
      margin-top: 0;
    }
    .snapshot-news-title-line {
      margin-top: 8px;
      padding-top: 8px;
      border-top: 1px dashed #d5cec0;
    }
  </style>
</head>
<body>
  <main>
    <section class="receipt-preview">
      <article class="receipt-paper" style="--paper-chars: 32;">
        {{if .ReceiptLines}}
        {{range .ReceiptLines}}
          <div class="receipt-line align-{{.Alignment}} role-{{.Role}}{{if .DoubleWidth}} double-width{{end}}{{if .DoubleHeight}} double-height{{end}}">
            <span class="receipt-line-text">{{if .Text}}{{.Text}}{{else}}&nbsp;{{end}}</span>
          </div>
        {{end}}
        {{end}}
        {{if .PendingNotice}}<p class="notice">Этот слепок создан, но печать еще не подтверждена.</p>{{end}}
        <div class="receipt-line align-center role-normal snapshot-news-title-line"><span class="receipt-line-text">Коротко о мире:</span></div>
        <div class="receipt-line align-center role-normal"><span class="receipt-line-text">&nbsp;</span></div>
        {{range .Groups}}
          <div class="receipt-line align-center role-normal snapshot-news-line snapshot-source-line"><span class="receipt-line-text">{{.SourceName}}</span></div>
          {{range .Items}}
          <div class="receipt-line align-center role-normal snapshot-news-line">
            {{if .Link}}<a class="receipt-line-text" href="{{.Link}}" rel="noopener noreferrer">- {{.Title}}</a>{{else}}<span class="receipt-line-text">- {{.Title}}</span>{{end}}
          </div>
          {{if .OriginalTitle}}
          <div class="receipt-line align-left role-original snapshot-news-line"><span class="receipt-line-text">{{.OriginalTitle}}</span></div>
          {{end}}
          {{end}}
          <div class="receipt-line align-center role-normal"><span class="receipt-line-text">&nbsp;</span></div>
        {{end}}
      </article>
    </section>
  </main>
</body>
</html>`))
