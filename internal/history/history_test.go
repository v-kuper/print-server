package history

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProviderFetchesSelectedEventsByDate(t *testing.T) {
	var gotPath string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotPath = request.URL.Path
		if got := request.Header.Get("User-Agent"); got == "" {
			t.Fatal("expected User-Agent header")
		}
		return jsonResponse(http.StatusOK, `{"selected":[{"year":1961,"text":"Venera 1 became the first spacecraft to fly by Venus.","pages":[{"content_urls":{"desktop":{"page":"https://en.example/venera"}}}]}]}`), nil
	})}

	provider := NewProvider(client)
	provider.BaseURL = "https://history.test/en/selected"

	events, err := provider.Current(context.Background(), time.Date(2026, time.May, 28, 9, 0, 0, 0, time.FixedZone("MSK", 3*60*60)))
	if err != nil {
		t.Fatalf("expected events, got error: %v", err)
	}
	if gotPath != "/en/selected/05/28" {
		t.Fatalf("expected date path /en/selected/05/28, got %q", gotPath)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %#v", events)
	}
	if events[0].Year != 1961 || events[0].Text != "Venera 1 became the first spacecraft to fly by Venus." || events[0].Link != "https://en.example/venera" {
		t.Fatalf("unexpected event: %#v", events[0])
	}
}

func TestProviderUsesPassedDateLocation(t *testing.T) {
	var gotPath string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotPath = request.URL.Path
		return jsonResponse(http.StatusOK, `{"selected":[{"year":2026,"text":"Local date event."}]}`), nil
	})}

	provider := NewProvider(client)
	provider.BaseURL = "https://history.test/en/selected"

	_, err := provider.Current(context.Background(), time.Date(2026, time.May, 28, 0, 30, 0, 0, time.FixedZone("MSK", 3*60*60)))
	if err != nil {
		t.Fatalf("expected events, got error: %v", err)
	}
	if gotPath != "/en/selected/05/28" {
		t.Fatalf("expected local date path /en/selected/05/28, got %q", gotPath)
	}
}

func TestProviderReturnsErrorOnBadStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadGateway, `nope`), nil
	})}

	provider := NewProvider(client)
	provider.BaseURL = "https://history.test/en/selected"

	_, err := provider.Current(context.Background(), time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProviderRejectsMalformedJSON(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"selected":`), nil
	})}

	provider := NewProvider(client)
	provider.BaseURL = "https://history.test/en/selected"

	_, err := provider.Current(context.Background(), time.Date(2026, time.May, 28, 9, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
