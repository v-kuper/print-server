package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSaveDenisTrendsEndpointPersistsSettings(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	body := bytes.NewBufferString(`{
		"periods": {
			"now": {"enabled": true, "maxItems": 15},
			"day": {"enabled": true, "maxItems": 20},
			"week": {"enabled": true, "maxItems": 10},
			"month": {"enabled": false, "maxItems": 5}
		},
		"sources": {"hackerNews": true, "github": true, "hypeReplicate": false}
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/settings/denis-trends", body)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if !store.denisTrends.Periods["day"].Enabled || store.denisTrends.Periods["week"].MaxItems != 10 {
		t.Fatalf("expected settings to be persisted, got %#v", store.denisTrends)
	}
	if store.denisTrends.Periods["now"].MaxItems != 15 {
		t.Fatalf("expected now period to be persisted, got %#v", store.denisTrends)
	}
}

func TestBootstrapIncludesDenisTrendsSettings(t *testing.T) {
	store := &fakeStore{}
	server := NewServer(store, &fakePrinter{}, fixedClock)

	request := httptest.NewRequest(http.MethodGet, "/api/bootstrap", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			DenisTrends json.RawMessage `json:"denisTrends"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode bootstrap: %v", err)
	}
	if len(payload.Data.DenisTrends) == 0 || string(payload.Data.DenisTrends) == "null" {
		t.Fatalf("expected denisTrends in bootstrap payload: %s", response.Body.String())
	}
	if bytes.Contains(payload.Data.DenisTrends, []byte(`"sources"`)) {
		t.Fatalf("denisTrends bootstrap must not expose source filters: %s", payload.Data.DenisTrends)
	}
	if !bytes.Contains(payload.Data.DenisTrends, []byte(`"now"`)) {
		t.Fatalf("denisTrends bootstrap must expose now period: %s", payload.Data.DenisTrends)
	}
}

func TestStaticClientRendersDenisTrendsPeriodsWithoutSources(t *testing.T) {
	server := NewServer(&fakeStore{}, &fakePrinter{}, fixedClock)
	request := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{`{ key: "now", label: "Top now" }`, `dataset.denisTrendsMode`, `content-denis-trends-mode`, `Top now`, `Top day`} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("expected app.js to contain %q", want)
		}
	}
	for _, unwanted := range []string{`denisTrendSources`, `data-denis-trend-source`, `Источники`} {
		if bytes.Contains([]byte(body), []byte(unwanted)) {
			t.Fatalf("expected app.js not to contain source UI %q", unwanted)
		}
	}
}
