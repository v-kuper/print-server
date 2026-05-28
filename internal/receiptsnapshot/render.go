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
	Groups        []NewsGroup
}

func RenderSnapshotHTML(snapshot Snapshot) ([]byte, error) {
	data := snapshotPageData{
		ID:            snapshot.ID,
		PendingNotice: snapshot.Status == StatusPending,
		Groups:        GroupNews(snapshot.NewsItems),
	}
	var buffer bytes.Buffer
	if err := snapshotTemplate.Execute(&buffer, data); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
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
      background: #f3f0e8;
      color: #151515;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
      line-height: 1.45;
    }
    main {
      width: min(480px, 100%);
      margin: 0 auto;
      padding: 20px 14px 32px;
    }
    .receipt {
      background: #fffdf6;
      border: 1px solid #d5cec0;
      box-shadow: 0 18px 40px rgba(36, 32, 24, 0.16);
      padding: 24px 18px;
    }
    h1 {
      margin: 0 0 18px;
      text-align: center;
      font-size: 22px;
      line-height: 1.2;
      letter-spacing: 0;
    }
    .notice {
      margin: 0 0 18px;
      padding: 10px 12px;
      border: 1px solid #dec16a;
      background: #fff7d7;
      color: #4f3d00;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-size: 14px;
    }
    .source {
      margin-top: 18px;
      padding-top: 14px;
      border-top: 1px dashed #b5ad9d;
    }
    .source:first-of-type {
      margin-top: 0;
    }
    h2 {
      margin: 0 0 10px;
      font-size: 16px;
      line-height: 1.3;
      letter-spacing: 0;
    }
    ul {
      margin: 0;
      padding-left: 18px;
    }
    li + li {
      margin-top: 12px;
    }
    a {
      color: #0b5cad;
      text-decoration: underline;
      text-underline-offset: 2px;
    }
    .original {
      display: block;
      margin-top: 3px;
      color: #6f6658;
      font-size: 13px;
    }
  </style>
</head>
<body>
  <main>
    <article class="receipt">
      <h1>Коротко о мире</h1>
      {{if .PendingNotice}}<p class="notice">Этот слепок создан, но печать еще не подтверждена.</p>{{end}}
      {{range .Groups}}
      <section class="source">
        <h2>{{.SourceName}}</h2>
        <ul>
          {{range .Items}}
          <li>
            {{if .Link}}<a href="{{.Link}}" rel="noopener noreferrer">{{.Title}}</a>{{else}}{{.Title}}{{end}}
            {{if .OriginalTitle}}<span class="original">{{.OriginalTitle}}</span>{{end}}
          </li>
          {{end}}
        </ul>
      </section>
      {{end}}
    </article>
  </main>
</body>
</html>`))
