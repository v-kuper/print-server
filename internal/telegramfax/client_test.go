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
	var sendMessageRequest SendMessageRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot123:abc/getUpdates":
			if r.Method != http.MethodPost {
				t.Fatalf("expected getUpdates POST, got %s", r.Method)
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
		case "/bot123:abc/sendMessage":
			if r.Method != http.MethodPost {
				t.Fatalf("expected sendMessage POST, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&sendMessageRequest); err != nil {
				t.Fatalf("decode sendMessage request: %v", err)
			}
			writeTelegramTestJSON(w, map[string]any{
				"ok":     true,
				"result": map[string]any{"message_id": 77},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
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
	if !reflect.DeepEqual(getUpdatesRequest.AllowedUpdates, telegramFaxAllowedUpdates) {
		t.Fatalf("unexpected allowed updates: %#v", getUpdatesRequest.AllowedUpdates)
	}
	if state.state.NextUpdateOffset != 7 {
		t.Fatalf("expected next offset 7, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) == 0 || gateway.printedLines[4].Text != "HTTP client fax" {
		t.Fatalf("expected fake Telegram message to print, got %#v", gateway.printedLines)
	}
	if sendMessageRequest.BusinessConnectionID != "bc-1" || sendMessageRequest.ChatID != 2001 || sendMessageRequest.Text != "Факс доставлен." {
		t.Fatalf("unexpected sendMessage request: %#v", sendMessageRequest)
	}
}

func TestPollOncePrintsDirectMessageWithHTTPClientFakeTelegramAPI(t *testing.T) {
	var getUpdatesRequest GetUpdatesRequest
	var sendMessageRequest SendMessageRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot123:abc/getUpdates":
			if r.Method != http.MethodPost {
				t.Fatalf("expected getUpdates POST, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&getUpdatesRequest); err != nil {
				t.Fatalf("decode getUpdates request: %v", err)
			}
			writeTelegramTestJSON(w, map[string]any{
				"ok": true,
				"result": []any{
					map[string]any{
						"update_id": 7,
						"message": map[string]any{
							"message_id": 101,
							"date":       time.Date(2026, 6, 2, 10, 20, 0, 0, time.UTC).Unix(),
							"from": map[string]any{
								"id":         2001,
								"first_name": "Direct",
								"username":   "direct_user",
							},
							"chat": map[string]any{
								"id":         2001,
								"type":       "private",
								"first_name": "Direct",
								"username":   "direct_user",
							},
							"text": "HTTP direct fax",
						},
					},
				},
			})
		case "/bot123:abc/sendMessage":
			if r.Method != http.MethodPost {
				t.Fatalf("expected sendMessage POST, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&sendMessageRequest); err != nil {
				t.Fatalf("decode sendMessage request: %v", err)
			}
			writeTelegramTestJSON(w, map[string]any{
				"ok":     true,
				"result": map[string]any{"message_id": 77},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer api.Close()

	state := &fakeStateStore{state: State{NextUpdateOffset: 7}}
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

	if !reflect.DeepEqual(getUpdatesRequest.AllowedUpdates, telegramFaxAllowedUpdates) {
		t.Fatalf("unexpected allowed updates: %#v", getUpdatesRequest.AllowedUpdates)
	}
	if state.state.NextUpdateOffset != 8 {
		t.Fatalf("expected next offset 8, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) == 0 || gateway.printedLines[4].Text != "HTTP direct fax" {
		t.Fatalf("expected fake Telegram direct message to print, got %#v", gateway.printedLines)
	}
	if sendMessageRequest.BusinessConnectionID != "" || sendMessageRequest.ChatID != 2001 || sendMessageRequest.Text != "Факс доставлен." {
		t.Fatalf("unexpected sendMessage request: %#v", sendMessageRequest)
	}
}

func TestPollOnceRejectsPhotoWithHTTPClientFakeTelegramAPI(t *testing.T) {
	var getUpdatesRequest GetUpdatesRequest
	var sendMessageRequest SendMessageRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot123:abc/getUpdates":
			if r.Method != http.MethodPost {
				t.Fatalf("expected getUpdates POST, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&getUpdatesRequest); err != nil {
				t.Fatalf("decode getUpdates request: %v", err)
			}
			writeTelegramTestJSON(w, map[string]any{
				"ok": true,
				"result": []any{
					map[string]any{
						"update_id": 8,
						"business_connection": map[string]any{
							"id": "bc-photo",
							"user": map[string]any{
								"id": 1001,
							},
						},
					},
					map[string]any{
						"update_id": 9,
						"business_message": map[string]any{
							"business_connection_id": "bc-photo",
							"message_id":             100,
							"date":                   time.Date(2026, 6, 1, 16, 30, 0, 0, time.UTC).Unix(),
							"from": map[string]any{
								"id":         2001,
								"first_name": "Sender",
							},
							"caption": "HTTP photo",
							"photo": []any{
								map[string]any{
									"file_id":   "photo-file",
									"width":     4,
									"height":    2,
									"file_size": 123,
								},
							},
						},
					},
				},
			})
		case "/bot123:abc/sendMessage":
			if r.Method != http.MethodPost {
				t.Fatalf("expected sendMessage POST, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&sendMessageRequest); err != nil {
				t.Fatalf("decode sendMessage request: %v", err)
			}
			writeTelegramTestJSON(w, map[string]any{
				"ok":     true,
				"result": map[string]any{"message_id": 77},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer api.Close()

	state := &fakeStateStore{state: State{NextUpdateOffset: 8}}
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

	if getUpdatesRequest.Offset != 8 {
		t.Fatalf("expected offset 8, got %d", getUpdatesRequest.Offset)
	}
	if state.state.NextUpdateOffset != 10 {
		t.Fatalf("expected next offset 10, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) != 0 {
		t.Fatalf("photo must not print, got %#v", gateway.printedLines)
	}
	if sendMessageRequest.BusinessConnectionID != "bc-photo" || sendMessageRequest.ChatID != 2001 || sendMessageRequest.Text != "Поддерживаются только текстовые сообщения. Голосовые сообщения, фото, видео и другие медиафайлы не поддерживаются." {
		t.Fatalf("unexpected sendMessage request: %#v", sendMessageRequest)
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

func TestHTTPClientSendMessagePostsPayload(t *testing.T) {
	var request SendMessageRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/bot123:abc/sendMessage" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode sendMessage request: %v", err)
		}
		writeTelegramTestJSON(w, map[string]any{
			"ok":     true,
			"result": map[string]any{"message_id": 77},
		})
	}))
	defer api.Close()

	err := NewHTTPClient("123:abc", api.URL, api.Client()).SendMessage(context.Background(), SendMessageRequest{
		BusinessConnectionID: "bc-1",
		ChatID:               3001,
		Text:                 "Факс доставлен.",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}

	if request.BusinessConnectionID != "bc-1" || request.ChatID != 3001 || request.Text != "Факс доставлен." {
		t.Fatalf("unexpected sendMessage request: %#v", request)
	}
}

func writeTelegramTestJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		panic(err)
	}
}
