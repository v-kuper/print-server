package telegramfax

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"strconv"
	"testing"
	"time"

	"atol-server/internal/printcoord"
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
	if !reflect.DeepEqual(client.requests[0].AllowedUpdates, telegramFaxAllowedUpdates) {
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

func TestPollOnceSkipsBusinessOwnerMessagesWhenSenderAllowlistIsEmpty(t *testing.T) {
	config := testConfig()
	config.AllowedSenderIDs = nil
	client := &fakeTelegramClient{
		updates: []Update{
			{UpdateID: 55, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
			{
				UpdateID: 56,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					From:                 &User{ID: 1001, FirstName: "Owner"},
					Text:                 "My outgoing reply",
				},
			},
		},
	}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(config, client, &fakeStateStore{}, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if len(gateway.printedLines) != 0 {
		t.Fatalf("expected owner message to be skipped, got %#v", gateway.printedLines)
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

func TestPollOncePrintsAllowedDirectMessageAndAdvancesOffset(t *testing.T) {
	state := &fakeStateStore{state: State{NextUpdateOffset: 80}}
	client := &fakeTelegramClient{
		updates: []Update{
			{
				UpdateID: 80,
				Message: &Message{
					MessageID: 88,
					Date:      time.Date(2026, 6, 2, 10, 20, 0, 0, time.UTC).Unix(),
					From:      &User{ID: 2001, FirstName: "Direct", Username: "direct_user"},
					Chat:      &Chat{ID: 2001, Type: "private"},
					Text:      "Print this from bot DM",
				},
			},
		},
	}
	jobs := &fakePrintJobStore{}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(testConfig(), client, state, &fakePrinterConfigStore{}, jobs, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if !reflect.DeepEqual(client.requests[0].AllowedUpdates, telegramFaxAllowedUpdates) {
		t.Fatalf("unexpected allowed updates: %#v", client.requests[0].AllowedUpdates)
	}
	if state.state.NextUpdateOffset != 81 {
		t.Fatalf("expected next offset 81, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) == 0 || gateway.printedLines[4].Text != "Print this from bot DM" {
		t.Fatalf("expected direct message to print, got %#v", gateway.printedLines)
	}
	if jobs.startedKind != "telegram_fax" || jobs.finishedID != "job-1" || jobs.finishedErr != "" {
		t.Fatalf("unexpected direct print job state: %#v", jobs)
	}
	request, ok := jobs.startedRequest.(map[string]any)
	if !ok || request["source"] != "telegram_bot_direct" || request["contentType"] != "text" || request["senderId"] != int64(2001) {
		t.Fatalf("unexpected direct print job request: %#v", jobs.startedRequest)
	}
}

func TestPollOncePrintsAnyDirectMessageWhenSenderAllowlistIsEmpty(t *testing.T) {
	config := testConfig()
	config.AllowedSenderIDs = nil
	client := &fakeTelegramClient{
		updates: []Update{
			{
				UpdateID: 82,
				Message: &Message{
					From: &User{ID: 9999, FirstName: "Unknown"},
					Chat: &Chat{ID: 9999, Type: "private"},
					Text: "Public direct print",
				},
			},
		},
	}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(config, client, &fakeStateStore{}, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if len(gateway.printedLines) == 0 || gateway.printedLines[4].Text != "Public direct print" {
		t.Fatalf("expected any direct message to print, got %#v", gateway.printedLines)
	}
}

func TestPollOnceSkipsUnsafeDirectMessages(t *testing.T) {
	config := testConfig()
	config.AllowedSenderIDs = nil
	for _, test := range []struct {
		name   string
		update Update
	}{
		{
			name: "group chat",
			update: Update{
				UpdateID: 91,
				Message: &Message{
					From: &User{ID: 1001, FirstName: "Owner"},
					Chat: &Chat{ID: -100, Type: "group"},
					Text: "group print",
				},
			},
		},
		{
			name: "bot command",
			update: Update{
				UpdateID: 92,
				Message: &Message{
					From: &User{ID: 1001, FirstName: "Owner"},
					Chat: &Chat{ID: 1001, Type: "private"},
					Text: "/start",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := &fakeStateStore{}
			gateway := &fakeTelegramFaxPrinter{}
			service := NewService(config, &fakeTelegramClient{updates: []Update{test.update}}, state, &fakePrinterConfigStore{}, &fakePrintJobStore{}, gateway, fixedFaxClock, WithLocation(time.UTC))

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

func TestPollOncePrintsBusinessPhotoMessage(t *testing.T) {
	imageData := testTelegramPhotoPNG(t, 4, 2)
	client := &fakeTelegramClient{
		updates: []Update{
			{UpdateID: 70, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
			{
				UpdateID: 71,
				BusinessMessage: &Message{
					BusinessConnectionID: "bc-1",
					MessageID:            77,
					Date:                 time.Date(2026, 6, 1, 16, 30, 0, 0, time.UTC).Unix(),
					From:                 &User{ID: 2001, FirstName: "Photo", Username: "photo_user"},
					Caption:              "Look\nat this",
					Photo: []PhotoSize{
						{FileID: "small", Width: 2, Height: 1, FileSize: 10},
						{FileID: "big", Width: 4, Height: 2, FileSize: int64(len(imageData))},
					},
				},
			},
		},
		files: map[string]File{
			"big": {FileID: "big", FilePath: "photos/big.png", FileSize: int64(len(imageData))},
		},
		downloads: map[string][]byte{
			"photos/big.png": imageData,
		},
	}
	jobs := &fakePrintJobStore{}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(testConfig(), client, &fakeStateStore{}, &fakePrinterConfigStore{}, jobs, gateway, fixedFaxClock, WithLocation(time.UTC))

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if client.requestedFileID != "big" {
		t.Fatalf("expected largest photo file_id to be requested, got %q", client.requestedFileID)
	}
	if client.downloadedFilePath != "photos/big.png" {
		t.Fatalf("expected photo file to be downloaded, got %q", client.downloadedFilePath)
	}
	if len(gateway.printedLines) == 0 {
		t.Fatalf("expected photo fax to print")
	}
	if gateway.printedLines[0].Text != "INCOMING PHOTO FAX" {
		t.Fatalf("unexpected header line: %#v", gateway.printedLines[0])
	}
	if gateway.printedLines[4].Text != "Look" || gateway.printedLines[5].Text != "at this" {
		t.Fatalf("expected multiline caption before photo, got %#v", gateway.printedLines)
	}
	imageLine := receipt.Line{}
	for _, line := range gateway.printedLines {
		if len(line.ImagePixelBuffer) > 0 {
			imageLine = line
			break
		}
	}
	if imageLine.ImageWidth != 384 || imageLine.ImageHeight != 192 {
		t.Fatalf("expected photo to fit receipt width at 384x192, got %dx%d", imageLine.ImageWidth, imageLine.ImageHeight)
	}
	if len(imageLine.ImagePixelBuffer) != 384*192 {
		t.Fatalf("unexpected photo pixel buffer length %d", len(imageLine.ImagePixelBuffer))
	}
	if gateway.printedLines[len(gateway.printedLines)-1].Text != faxBottomSeparator {
		t.Fatalf("expected bottom separator, got %#v", gateway.printedLines[len(gateway.printedLines)-1])
	}
	if jobs.startedKind != "telegram_fax" || jobs.finishedID != "job-1" || jobs.finishedErr != "" {
		t.Fatalf("unexpected photo print job state: %#v", jobs)
	}
	request, ok := jobs.startedRequest.(map[string]any)
	if !ok || request["contentType"] != "photo" || request["telegramFileId"] != "big" {
		t.Fatalf("unexpected photo print job request: %#v", jobs.startedRequest)
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

func TestPollOnceQueuesFaxWhenPrinterConnectionFailsAndFlushesLater(t *testing.T) {
	state := &fakeStateStore{}
	client := &fakeTelegramClient{
		updates: []Update{
			{UpdateID: 40, BusinessConnection: &BusinessConnection{ID: "bc-1", User: User{ID: 1001}}},
			{UpdateID: 41, BusinessMessage: &Message{BusinessConnectionID: "bc-1", MessageID: 42, From: &User{ID: 2001}, Text: "print me later"}},
		},
	}
	queue := newFakeQueueStore()
	jobs := &fakePrintJobStore{}
	gateway := &fakeTelegramFaxPrinter{checkErr: errors.New("printer offline")}
	service := NewService(
		testConfig(),
		client,
		state,
		&fakePrinterConfigStore{},
		jobs,
		gateway,
		fixedFaxClock,
		WithLocation(time.UTC),
		WithQueueStore(queue),
		WithPrintCoordinator(printcoord.New()),
	)

	if err := service.PollOnce(context.Background()); err != nil {
		t.Fatalf("poll once: %v", err)
	}

	if state.state.NextUpdateOffset != 42 {
		t.Fatalf("expected offset 42 after queueing update, got %d", state.state.NextUpdateOffset)
	}
	if len(gateway.printedLines) != 0 {
		t.Fatalf("printer is offline, fax must remain queued; got printed lines %#v", gateway.printedLines)
	}
	if jobs.startedKind != "" {
		t.Fatalf("print job must not start before printer is reachable, got %#v", jobs)
	}
	pending := queue.pendingItems()
	if len(pending) != 1 {
		t.Fatalf("expected one pending fax, got %#v", pending)
	}
	if pending[0].DedupeKey != "business:bc-1:42" || pending[0].ContentType != "text" {
		t.Fatalf("unexpected queued fax: %#v", pending[0])
	}

	gateway.checkErr = nil
	if err := service.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush pending: %v", err)
	}

	if len(gateway.printedLines) == 0 || gateway.printedLines[4].Text != "print me later" {
		t.Fatalf("expected queued fax to print after reconnect, got %#v", gateway.printedLines)
	}
	if jobs.startedKind != "telegram_fax" || jobs.finishedID != "job-1" || jobs.finishedErr != "" {
		t.Fatalf("unexpected print job state after flush: %#v", jobs)
	}
	if len(queue.pendingItems()) != 0 {
		t.Fatalf("expected queue to be empty after successful print, got %#v", queue.pendingItems())
	}
}

func TestFlushPendingStopsWhenFaxCoordinatorIsBusy(t *testing.T) {
	queue := newFakeQueueStore()
	_, inserted, err := queue.Enqueue(context.Background(), QueueItem{
		DedupeKey:   "direct:2001:7",
		Source:      "telegram_bot_direct",
		ContentType: "text",
		Message: Message{
			MessageID: 7,
			Date:      time.Date(2026, 6, 2, 10, 20, 0, 0, time.UTC).Unix(),
			From:      &User{ID: 2001, FirstName: "Direct"},
			Chat:      &Chat{ID: 2001, Type: "private"},
			Text:      "wait behind user print",
		},
	})
	if err != nil || !inserted {
		t.Fatalf("enqueue fake fax: inserted=%v err=%v", inserted, err)
	}
	coordinator := &fakeFaxCoordinator{run: false}
	gateway := &fakeTelegramFaxPrinter{}
	service := NewService(
		testConfig(),
		&fakeTelegramClient{},
		&fakeStateStore{},
		&fakePrinterConfigStore{},
		&fakePrintJobStore{},
		gateway,
		fixedFaxClock,
		WithLocation(time.UTC),
		WithQueueStore(queue),
		WithPrintCoordinator(coordinator),
	)

	if err := service.FlushPending(context.Background()); err != nil {
		t.Fatalf("flush pending: %v", err)
	}

	if coordinator.calls != 1 {
		t.Fatalf("expected coordinator to be consulted once, got %d", coordinator.calls)
	}
	if len(gateway.printedLines) != 0 {
		t.Fatalf("fax must not print while coordinator refuses it, got %#v", gateway.printedLines)
	}
	if len(queue.pendingItems()) != 1 {
		t.Fatalf("fax must remain pending, got %#v", queue.pendingItems())
	}
}

func TestMemoryQueueStoreDeduplicatesAndReturnsPendingFIFO(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store := newMemoryQueueStore(func() time.Time { return now })

	first, inserted, err := store.Enqueue(ctx, QueueItem{
		DedupeKey:   "direct:1:1",
		Source:      "telegram_bot_direct",
		ContentType: "text",
		Message:     Message{MessageID: 1, From: &User{ID: 1}, Chat: &Chat{ID: 1, Type: "private"}, Text: "first"},
	})
	if err != nil || !inserted {
		t.Fatalf("enqueue first: inserted=%v err=%v", inserted, err)
	}
	duplicate, inserted, err := store.Enqueue(ctx, QueueItem{
		DedupeKey:   "direct:1:1",
		Source:      "telegram_bot_direct",
		ContentType: "text",
		Message:     Message{MessageID: 1, From: &User{ID: 1}, Chat: &Chat{ID: 1, Type: "private"}, Text: "duplicate"},
	})
	if err != nil || inserted {
		t.Fatalf("enqueue duplicate: inserted=%v err=%v", inserted, err)
	}
	if duplicate.ID != first.ID {
		t.Fatalf("expected duplicate enqueue to return first item, got %#v vs %#v", duplicate, first)
	}
	if _, _, err := store.Enqueue(ctx, QueueItem{
		DedupeKey:   "direct:1:2",
		Source:      "telegram_bot_direct",
		ContentType: "text",
		Message:     Message{MessageID: 2, From: &User{ID: 1}, Chat: &Chat{ID: 1, Type: "private"}, Text: "second"},
	}); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	next, ok, err := store.NextPending(ctx, now)
	if err != nil || !ok {
		t.Fatalf("next pending first: ok=%v err=%v", ok, err)
	}
	if next.ID != first.ID {
		t.Fatalf("expected FIFO first item, got %#v", next)
	}
	if err := store.MarkPrinted(ctx, next.ID); err != nil {
		t.Fatalf("mark printed: %v", err)
	}
	next, ok, err = store.NextPending(ctx, now)
	if err != nil || !ok {
		t.Fatalf("next pending second: ok=%v err=%v", ok, err)
	}
	if next.Message.Text != "second" {
		t.Fatalf("expected second item, got %#v", next)
	}
	retryAt := now.Add(time.Minute)
	if err := store.MarkFailed(ctx, next.ID, errors.New("printer offline"), retryAt); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if _, ok, err := store.NextPending(ctx, now); err != nil || ok {
		t.Fatalf("failed item must wait until next attempt, ok=%v err=%v", ok, err)
	}
	next, ok, err = store.NextPending(ctx, retryAt)
	if err != nil || !ok {
		t.Fatalf("failed item should be retryable, ok=%v err=%v", ok, err)
	}
	if next.Attempts != 1 || next.LastError != "printer offline" {
		t.Fatalf("expected failed attempt metadata, got %#v", next)
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
	files                map[string]File
	downloads            map[string][]byte
	resolvedConnectionID string
	requestedFileID      string
	downloadedFilePath   string
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

func (c *fakeTelegramClient) GetFile(_ context.Context, fileID string) (File, error) {
	c.requestedFileID = fileID
	file, ok := c.files[fileID]
	if !ok {
		return File{}, errors.New("telegram file not found")
	}
	return file, nil
}

func (c *fakeTelegramClient) DownloadFile(_ context.Context, filePath string) ([]byte, error) {
	c.downloadedFilePath = filePath
	data, ok := c.downloads[filePath]
	if !ok {
		return nil, errors.New("telegram file download not found")
	}
	return append([]byte(nil), data...), nil
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
	startedKind    string
	startedRequest any
	finishedID     string
	finishedErr    string
}

func (s *fakePrintJobStore) StartPrintJob(kind string, request any) (string, error) {
	s.startedKind = kind
	s.startedRequest = request
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
	checkErr     error
	checks       int
}

func (p *fakeTelegramFaxPrinter) CheckConnection(context.Context, printer.Config) (string, error) {
	p.checks++
	if p.checkErr != nil {
		return "", p.checkErr
	}
	return "Подключено", nil
}

func (p *fakeTelegramFaxPrinter) PrintReceipt(_ context.Context, _ printer.Config, lines []receipt.Line) error {
	p.printedLines = append([]receipt.Line(nil), lines...)
	return p.printErr
}

type fakeFaxCoordinator struct {
	run   bool
	calls int
}

func (c *fakeFaxCoordinator) TryRunFax(ctx context.Context, fn func(context.Context) error) (bool, error) {
	c.calls++
	if !c.run {
		return false, nil
	}
	return true, fn(ctx)
}

type fakeQueueStore struct {
	nextID int
	items  []QueueItem
}

func newFakeQueueStore() *fakeQueueStore {
	return &fakeQueueStore{}
}

func (s *fakeQueueStore) Enqueue(_ context.Context, item QueueItem) (QueueItem, bool, error) {
	for _, existing := range s.items {
		if existing.DedupeKey == item.DedupeKey {
			return existing, false, nil
		}
	}
	s.nextID++
	item.ID = "queue-" + strconv.Itoa(s.nextID)
	item.Status = QueueStatusPending
	s.items = append(s.items, item)
	return item, true, nil
}

func (s *fakeQueueStore) NextPending(_ context.Context, now time.Time) (QueueItem, bool, error) {
	for _, item := range s.items {
		if item.Status == QueueStatusPending && !item.NextAttemptAt.After(now) {
			return item, true, nil
		}
	}
	return QueueItem{}, false, nil
}

func (s *fakeQueueStore) MarkPrinted(_ context.Context, id string) error {
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Status = QueueStatusPrinted
			return nil
		}
	}
	return errors.New("queue item not found")
}

func (s *fakeQueueStore) MarkFailed(_ context.Context, id string, err error, nextAttemptAt time.Time) error {
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Attempts++
			s.items[i].LastError = err.Error()
			s.items[i].NextAttemptAt = nextAttemptAt
			return nil
		}
	}
	return errors.New("queue item not found")
}

func (s *fakeQueueStore) pendingItems() []QueueItem {
	var pending []QueueItem
	for _, item := range s.items {
		if item.Status == QueueStatusPending {
			pending = append(pending, item)
		}
	}
	return pending
}

func testTelegramPhotoPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.White)
			} else {
				img.Set(x, y, color.Black)
			}
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return buffer.Bytes()
}
