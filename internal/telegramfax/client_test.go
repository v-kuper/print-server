package telegramfax

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestPollOnceWithHTTPClientFakeTelegramAPI(t *testing.T) {
	var getUpdatesRequest GetUpdatesRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/bot123:abc/getUpdates" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&getUpdatesRequest); err != nil {
			t.Fatalf("decode getUpdates request: %v", err)
		}
		writeTelegramTestJSON(w, map[string]any{
			"ok": true,
			"result": []any{
				map[string]any{
					"update_id": 5,
					"business_connection": map[string]any{
						"id": "bc-1",
						"user": map[string]any{
							"id":         1001,
							"first_name": "Owner",
						},
					},
				},
				map[string]any{
					"update_id": 6,
					"business_message": map[string]any{
						"business_connection_id": "bc-1",
						"message_id":             99,
						"date":                   time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC).Unix(),
						"from": map[string]any{
							"id":         2001,
							"first_name": "Sender",
						},
						"text": "HTTP client fax",
					},
				},
			},
		})
	}))
	defer api.Close()

	state := &fakeStateStore{state: State{NextUpdateOffset: 5}}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(
		testConfig(),
		NewHTTPClient("123:abc", api.URL, api.Client()),
		state,
		&fakePrinterConfigStore{},
		&fakePrintJobStore{},
		gateway,
		fixedFaxClock,
		WithLocation(time.UTC),
	)

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if getUpdatesRequest.Offset != 5 {
		t.Fatalf("expected offset 5, got %d", getUpdatesRequest.Offset)
	}
	if !reflect.DeepEqual(getUpdatesRequest.AllowedUpdates, businessAllowedUpdates) {
		t.Fatalf("unexpected allowed updates: %#v", getUpdatesRequest.AllowedUpdates)
	}
	if state.state.NextUpdateOffset != 7 {
		t.Fatalf("expected next offset 7, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) == 0 || gateway.printedLines[4].Text != "HTTP client fax" {
		t.Fatalf("expected fake Telegram message to print, got %#v", gateway.printedLines)
	}
}

func TestHTTPClientMapsMissingBusinessConnection(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTelegramTestJSON(w, map[string]any{
			"ok":          false,
			"error_code":  400,
			"description": "Bad Request: business connection not found",
		})
	}))
	defer api.Close()

	_, err := NewHTTPClient("123:abc", api.URL, api.Client()).GetBusinessConnection(context.Background(), "missing")
	if !errors.Is(err, ErrBusinessConnectionNotFound) {
		t.Fatalf("expected ErrBusinessConnectionNotFound, got %v", err)
	}
}

func writeTelegramTestJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		panic(err)
	}
}
