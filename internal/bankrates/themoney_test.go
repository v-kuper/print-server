package bankrates

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTheMoneySummaryFindsBestUsdRates(t *testing.T) {
	html := `<!doctype html><script>self.__next_f.push([1,"5d:[\"$\",\"$L\",null,{\"buyData\":{\"currency\":{\"code\":\"usd\"},\"rates\":[{\"bank\":{\"title\":\"Банк А\"},\"rate\":3.241,\"collectedAt\":\"2026-05-25T10:00:00Z\"},{\"bank\":{\"title\":\"Банк Б\"},\"rate\":3.255,\"collectedAt\":\"2026-05-25T10:02:00Z\"},{\"bank\":{\"title\":\"Банк В\"},\"rate\":3.255,\"collectedAt\":\"2026-05-25T10:04:00Z\"}]},\"sellData\":{\"currency\":{\"code\":\"usd\"},\"rates\":[{\"bank\":{\"title\":\"Банк Г\"},\"rate\":3.281,\"collectedAt\":\"2026-05-25T10:01:00Z\"},{\"bank\":{\"title\":\"Банк Д\"},\"rate\":3.279,\"collectedAt\":\"2026-05-25T10:03:00Z\"}]},\"channel\":\"cash\"}]]"]);</script>`

	summary, err := ParseTheMoneySummary([]byte(html))
	if err != nil {
		t.Fatalf("parse summary: %v", err)
	}

	if summary.Source != "TheMoney.by" {
		t.Fatalf("expected source TheMoney.by, got %q", summary.Source)
	}
	if summary.SellUSD == nil || summary.SellUSD.Rate != 3.255 {
		t.Fatalf("expected best user sell rate 3.255, got %#v", summary.SellUSD)
	}
	if got := summary.SellUSD.BankNames; len(got) != 2 || got[0] != "Банк Б" || got[1] != "Банк В" {
		t.Fatalf("expected tied banks for selling USD, got %#v", got)
	}
	if summary.BuyUSD == nil || summary.BuyUSD.Rate != 3.279 {
		t.Fatalf("expected best user buy rate 3.279, got %#v", summary.BuyUSD)
	}
	if got := summary.BuyUSD.BankNames; len(got) != 1 || got[0] != "Банк Д" {
		t.Fatalf("expected best bank for buying USD, got %#v", got)
	}
	if summary.UpdatedAt.IsZero() {
		t.Fatalf("expected update time from collectedAt values, got %#v", summary)
	}
}

func TestTheMoneyProviderRequestsConfiguredURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "text/html,application/xhtml+xml" {
			t.Fatalf("unexpected accept header: %q", r.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte(`{"buyData":{"rates":[{"bank":{"title":"Банк А"},"rate":3.25,"collectedAt":"2026-05-25T10:00:00Z"}]},"sellData":{"rates":[{"bank":{"title":"Банк Б"},"rate":3.28,"collectedAt":"2026-05-25T10:00:00Z"}]}}`))
	}))
	defer server.Close()

	provider := NewTheMoneyProvider(server.Client())
	provider.URL = server.URL

	summary, err := provider.Current(context.Background())
	if err != nil {
		t.Fatalf("current rates: %v", err)
	}
	if summary.SellUSD == nil || summary.SellUSD.BankNames[0] != "Банк А" {
		t.Fatalf("expected provider to parse response, got %#v", summary)
	}
}
