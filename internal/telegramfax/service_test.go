package telegramfax

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"atol-server/internal/printer"
	"atol-server/internal/receipt"
)

func TestPollOncePrintsAllowedBusinessMessageAndAdvancesOffset(t *testing.T) {
	state := &fakeStateStore{state: State{NextUpdateOffset: 7}}
	client := &fakeTelegramClient{
		updates: []Update{
			{
				UpdateID: 10,
				BusinessConnection: &BusinessConnection{
					ID:   "bc-1",
					User: User{ID: 1001},
				},
			},
			{
				UpdateID: 11,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					MessageID:            22,
					Date:                 time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC).Unix(),
					From:                 &User{ID: 2001, FirstName: "Vitali"},
					Text:                 "Hello fax",
				},
			},
		},
	}
	store := &fakePrinterConfigStore{config: printer.Config{Host: "192.168.0.118", Port: 5555}}
	jobs := &fakePrintJobStore{}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(testConfig(), client, state, store, jobs, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if len(client.requests) != 1 {
		t.Fatalf("expected one getUpdates request, got %#v", client.requests)
	}
	if client.requests[0].Offset != 7 {
		t.Fatalf("expected offset 7, got %d", client.requests[0].Offset)
	}
	if !reflect.DeepEqual(client.requests[0].AllowedUpdates, businessAllowedUpdates) {
		t.Fatalf("expected business allowed updates, got %#v", client.requests[0].AllowedUpdates)
	}
	if state.state.NextUpdateOffset != 12 {
		t.Fatalf("expected next offset 12, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) == 0 {
		t.Fatalf("expected fax to be printed")
	}
	if gateway.printedLines[0].Text != "INCOMING FAX" || gateway.printedLines[4].Text != "Hello fax" {
		t.Fatalf("unexpected printed lines: %#v", gateway.printedLines)
	}
	if jobs.startedKind != "telegram_fax" || jobs.finishedID != "job-1" || jobs.finishedErr != "" {
		t.Fatalf("unexpected print job state: %#v", jobs)
	}
}

func TestPollOnceSkipsDisallowedUpdatesAndStillAdvancesOffset(t *testing.T) {
	for _, test := range []struct {
		name   string
		update Update
	}{
		{
			name: "normal message",
			update: Update{
				UpdateID: 20,
				Message:  &Message{From: &User{ID: 2001}, Text: "ordinary bot chat"},
			},
		},
		{
			name: "wrong business owner",
			update: Update{
				UpdateID: 21,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-wrong-owner",
					From:                 &User{ID: 2001},
					Text:                 "private",
				},
			},
		},
		{
			name: "empty text",
			update: Update{
				UpdateID: 23,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					From:                 &User{ID: 2001},
					Text:                 "   \n\t ",
				},
			},
		},
		{
			name: "bot sender",
			update: Update{
				UpdateID: 24,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					From:                 &User{ID: 2001, IsBot: true},
					Text:                 "private",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &fakeStateStore{}
			client := &fakeTelegramClient{
				updates: []Update{
					{UpdateID: 19, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
					test.update,
				},
				connections: map[string]BusinessConnection{
					"bc-wrong-owner": {ID: "bc-wrong-owner", User: User{ID: 9999}},
				},
			}
			gateway := &fakeTelegramFaxPrinter{}
			service := NewService(testConfig(), client, state, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

			if err := service.PollOnce(context.Background()); err != nil {
				t.Fatalf("poll once: %v", err)
			}
			if len(gateway.printedLines) != 0 {
				t.Fatalf("expected no print, got %#v", gateway.printedLines)
			}
			if state.state.NextUpdateOffset != test.update.UpdateID+1 {
				t.Fatalf("expected offset %d, got %d", test.update.UpdateID+1, state.state.NextUpdateOffset)
			}
		})
	}
}

func TestPollOncePrintsAnySenderWhenSenderAllowlistIsEmpty(t *testing.T) {
	config := testConfig()
	config.AllowedSenderIDs = nil
	client := &fakeTelegramClient{
		updates: []Update{
			{UpdateID: 50, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
			{
				UpdateID: 51,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					From:                 &User{ID: 9999, FirstName: "Any"},
					Text:                 "Telegram Business allowed this chat",
				},
			},
		},
	}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(config, client, &fakeStateStore{}, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if len(gateway.printedLines) == 0 {
		t.Fatalf("expected message from any sender to print")
	}
	if gateway.printedLines[4].Text != "Telegram Business allowed this chat" {
		t.Fatalf("unexpected printed lines: %#v", gateway.printedLines)
	}
}

func TestPollOnceStillFiltersSendersWhenSenderAllowlistIsConfigured(t *testing.T) {
	client := &fakeTelegramClient{
		updates: []Update{
			{UpdateID: 60, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
			{
				UpdateID: 61,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					From:                 &User{ID: 9999},
					Text:                 "private",
				},
			},
		},
	}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(testConfig(), client, &fakeStateStore{}, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if len(gateway.printedLines) != 0 {
		t.Fatalf("expected configured sender allowlist to skip sender, got %#v", gateway.printedLines)
	}
}

func TestPollOnceResolvesUnknownBusinessConnection(t *testing.T) {
	client := &fakeTelegramClient{
		updates: []Update{
			{
				UpdateID: 30,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-resolved",
					From:                 &User{ID: 2001},
					Text:                 "Resolved owner",
				},
			},
		},
		connections: map[string]BusinessConnection{
			"bc-resolved": {ID: "bc-resolved", User: User{ID: 1001}},
		},
	}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(testConfig(), client, &fakeStateStore{}, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if client.resolvedConnectionID != "bc-resolved" {
		t.Fatalf("expected business connection lookup, got %q", client.resolvedConnectionID)
	}
	if len(gateway.printedLines) == 0 {
		t.Fatalf("expected message to print after resolving owner")
	}
}

func TestPollOnceRecordsFailedPrintJobAndAdvancesOffset(t *testing.T) {
	state := &fakeStateStore{}
	client := &fakeTelegramClient{
		updates: []Update{
			{UpdateID: 40, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
			{UpdateID: 41, BusinessMessage: &Message{BusinessConnectionID: "bc-1", From: &User{ID: 2001}, Text: "print me"}},
		},
	}
	jobs := &fakePrintJobStore{}
	gateway := &fakeTelegramFaxPrinter{printErr: errors.New("printer offline")}
	service := NewService(testConfig(), client, state, &fakePrinterConfigStore{}, jobs, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once should not fail permanently on printer errors: %v", err)
	}

	if state.state.NextUpdateOffset != 42 {
		t.Fatalf("expected offset 42, got %d", state.state.NextUpdateOffset)
	}
	if jobs.finishedErr != "printer offline" {
		t.Fatalf("expected failed print job, got %#v", jobs)
	}
}

func testConfig() Config {
	return Config{
		Token:            "123:abc",
		APIBaseURL:       defaultAPIBaseURL,
		PollTimeout:      25 * time.Second,
		OwnerIDs:         NewIDSet(1001),
		AllowedSenderIDs: NewIDSet(2001),
	}
}

func fixedFaxClock() time.Time {
	return time.Date(2026, 6, 1, 15, 10, 0, 0, time.UTC)
}

type fakeStateStore struct {
	state State
}

func (s *fakeStateStore) Load(_ context.Context) (State, error) {
	return s.state, nil
}

func (s *fakeStateStore) Save(_ context.Context, state State) error {
	s.state = state
	return nil
}

type fakeTelegramClient struct {
	requests             []GetUpdatesRequest
	updates              []Update
	connections          map[string]BusinessConnection
	resolvedConnectionID string
}

func (c *fakeTelegramClient) GetUpdates(_ context.Context, request GetUpdatesRequest) ([]Update, error) {
	c.requests = append(c.requests, request)
	return append([]Update(nil), c.updates...), nil
}

func (c *fakeTelegramClient) GetBusinessConnection(_ context.Context, id string) (BusinessConnection, error) {
	c.resolvedConnectionID = id
	connection, ok := c.connections[id]
	if !ok {
		return BusinessConnection{}, ErrBusinessConnectionNotFound
	}
	return connection, nil
}

type fakePrinterConfigStore struct {
	config printer.Config
}

func (s *fakePrinterConfigStore) LoadPrinter() (printer.Config, error) {
	if s.config == (printer.Config{}) {
		return printer.Config{Host: "192.168.0.118", Port: 5555}, nil
	}
	return s.config, nil
}

type fakePrintJobStore struct {
	startedKind string
	finishedID  string
	finishedErr string
}

func (s *fakePrintJobStore) StartPrintJob(kind string, _ any) (string, error) {
	s.startedKind = kind
	return "job-1", nil
}

func (s *fakePrintJobStore) FinishPrintJob(id string, printErr error) error {
	s.finishedID = id
	if printErr != nil {
		s.finishedErr = printErr.Error()
	}
	return nil
}

type fakeTelegramFaxPrinter struct {
	printedLines []receipt.Line
	printErr     error
}

func (p *fakeTelegramFaxPrinter) PrintReceipt(_ context.Context, _ printer.Config, lines []receipt.Line) error {
	p.printedLines = append([]receipt.Line(nil), lines...)
	return p.printErr
}
