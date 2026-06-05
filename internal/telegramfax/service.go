package telegramfax

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"atol-server/internal/printcoord"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
)

var telegramFaxAllowedUpdates = []string{
	"message",
	"business_connection",
	"business_message",
	"edited_business_message",
	"deleted_business_messages",
}

const (
	faxDeliveredNotification = "Факс доставлен."
	faxAcceptedNotification  = "Факс принят. Принтер сейчас недоступен или занят; распечатаем и уведомим вас."
	faxFailedNotification    = "Факс не доставлен. Проверьте принтер и отправьте факс ещё раз."
)

type FlushReport struct {
	Printed []QueueItem
	Failed  []QueueItem
}

type enqueueResult struct {
	Item     QueueItem
	Inserted bool
}

type PrinterConfigStore interface {
	LoadPrinter() (printer.Config, error)
}

type PrintJobStore interface {
	StartPrintJob(kind string, request any) (string, error)
	FinishPrintJob(id string, printErr error) error
}

type Printer interface {
	CheckConnection(context.Context, printer.Config) (string, error)
	PrintReceipt(context.Context, printer.Config, []receipt.Line) error
}

type PrintCoordinator interface {
	TryRunFax(context.Context, func(context.Context) error) (bool, error)
}

type Logger interface {
	Printf(string, ...any)
}

type Service struct {
	config           Config
	client           Client
	stateStore       StateStore
	printerConfig    PrinterConfigStore
	printJobs        PrintJobStore
	printer          Printer
	queueStore       QueueStore
	printCoordinator PrintCoordinator
	clock            func() time.Time
	location         *time.Location
	retryDelay       time.Duration
	queueRetryDelay  time.Duration
	logger           Logger
	connectionOwners map[string]int64
}

type Option func(*Service)

func WithLocation(location *time.Location) Option {
	return func(s *Service) {
		if location != nil {
			s.location = location
		}
	}
}

func WithLogger(logger Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithRetryDelay(delay time.Duration) Option {
	return func(s *Service) {
		if delay > 0 {
			s.retryDelay = delay
		}
	}
}

func WithQueueRetryDelay(delay time.Duration) Option {
	return func(s *Service) {
		if delay > 0 {
			s.queueRetryDelay = delay
		}
	}
}

func WithQueueStore(store QueueStore) Option {
	return func(s *Service) {
		if store != nil {
			s.queueStore = store
		}
	}
}

func WithPrintCoordinator(coordinator PrintCoordinator) Option {
	return func(s *Service) {
		if coordinator != nil {
			s.printCoordinator = coordinator
		}
	}
}

func NewService(
	config Config,
	client Client,
	stateStore StateStore,
	printerConfig PrinterConfigStore,
	printJobs PrintJobStore,
	printerGateway Printer,
	clock func() time.Time,
	options ...Option,
) *Service {
	if clock == nil {
		clock = time.Now
	}
	service := &Service{
		config:           config,
		client:           client,
		stateStore:       stateStore,
		printerConfig:    printerConfig,
		printJobs:        printJobs,
		printer:          printerGateway,
		clock:            clock,
		queueStore:       newMemoryQueueStore(clock),
		printCoordinator: printcoord.New(),
		location:         time.Local,
		retryDelay:       5 * time.Second,
		queueRetryDelay:  time.Minute,
		connectionOwners: make(map[string]int64),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) Start(ctx context.Context) {
	go s.retryQueuedFaxes(ctx)
	for ctx.Err() == nil {
		if err := s.PollOnce(ctx); err != nil && ctx.Err() == nil {
			s.logf("telegram fax polling failed: %v", err)
			timer := time.NewTimer(s.retryDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (s *Service) retryQueuedFaxes(ctx context.Context) {
	ticker := time.NewTicker(s.queueRetryDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.FlushPending(ctx); err != nil && ctx.Err() == nil {
				s.logf("telegram fax queue flush failed: %v", err)
			}
		}
	}
}

func (s *Service) PollOnce(ctx context.Context) error {
	state, err := s.stateStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("load telegram fax state: %w", err)
	}
	updates, err := s.client.GetUpdates(ctx, GetUpdatesRequest{
		Offset:         state.NextUpdateOffset,
		Timeout:        int(s.config.PollTimeout / time.Second),
		AllowedUpdates: append([]string(nil), telegramFaxAllowedUpdates...),
	})
	if err != nil {
		return fmt.Errorf("get Telegram updates: %w", err)
	}
	var newItems []QueueItem
	for _, update := range updates {
		result, err := s.processUpdate(ctx, update)
		if err != nil {
			return err
		}
		if result.Inserted {
			newItems = append(newItems, result.Item)
		}
		if update.UpdateID >= state.NextUpdateOffset {
			state.NextUpdateOffset = update.UpdateID + 1
		}
		if err := s.stateStore.Save(ctx, state); err != nil {
			return fmt.Errorf("save telegram fax state: %w", err)
		}
	}
	if len(updates) > 0 {
		report, err := s.FlushPending(ctx)
		if err != nil {
			return fmt.Errorf("flush telegram fax queue: %w", err)
		}
		s.notifyAcceptedForPendingNewItems(ctx, newItems, report)
	}
	return nil
}

func (s *Service) processUpdate(ctx context.Context, update Update) (enqueueResult, error) {
	if update.BusinessConnection != nil {
		s.rememberConnection(*update.BusinessConnection)
		return enqueueResult{}, nil
	}
	if update.BusinessMessage != nil {
		return s.processBusinessMessage(ctx, *update.BusinessMessage)
	}
	if update.Message != nil {
		return s.processDirectMessage(ctx, *update.Message)
	}
	return enqueueResult{}, nil
}

func (s *Service) processBusinessMessage(ctx context.Context, message Message) (enqueueResult, error) {
	if message.From == nil || message.From.IsBot {
		return enqueueResult{}, nil
	}
	photo, hasPhoto := bestPhotoSize(message.Photo)
	if strings.TrimSpace(message.Text) == "" && !hasPhoto {
		return enqueueResult{}, nil
	}
	if len(s.config.AllowedSenderIDs) > 0 && !s.config.AllowedSenderIDs.Contains(message.From.ID) {
		return enqueueResult{}, nil
	}
	ownerID, ok, err := s.ownerIDForMessage(ctx, message)
	if err != nil {
		return enqueueResult{}, err
	}
	if !ok || !s.config.OwnerIDs.Contains(ownerID) {
		return enqueueResult{}, nil
	}
	if message.From.ID == ownerID {
		return enqueueResult{}, nil
	}
	if hasPhoto {
		return s.enqueueMessage(ctx, QueueItem{
			DedupeKey:       businessDedupeKey(message),
			Source:          "telegram_business",
			ContentType:     "photo",
			Message:         message,
			BusinessOwnerID: ownerID,
			Photo:           photo,
		})
	}
	return s.enqueueMessage(ctx, QueueItem{
		DedupeKey:       businessDedupeKey(message),
		Source:          "telegram_business",
		ContentType:     "text",
		Message:         message,
		BusinessOwnerID: ownerID,
	})
}

func (s *Service) processDirectMessage(ctx context.Context, message Message) (enqueueResult, error) {
	if message.From == nil || message.From.IsBot {
		return enqueueResult{}, nil
	}
	if message.Chat == nil || message.Chat.Type != "private" {
		return enqueueResult{}, nil
	}
	photo, hasPhoto := bestPhotoSize(message.Photo)
	text := strings.TrimSpace(message.Text)
	if text == "" && !hasPhoto {
		return enqueueResult{}, nil
	}
	if strings.HasPrefix(text, "/") {
		return enqueueResult{}, nil
	}
	if hasPhoto {
		return s.enqueueMessage(ctx, QueueItem{
			DedupeKey:   directDedupeKey(message),
			Source:      "telegram_bot_direct",
			ContentType: "photo",
			Message:     message,
			Photo:       photo,
		})
	}
	return s.enqueueMessage(ctx, QueueItem{
		DedupeKey:   directDedupeKey(message),
		Source:      "telegram_bot_direct",
		ContentType: "text",
		Message:     message,
	})
}

func (s *Service) enqueueMessage(ctx context.Context, item QueueItem) (enqueueResult, error) {
	if s.queueStore == nil {
		s.queueStore = newMemoryQueueStore(s.clock)
	}
	queued, inserted, err := s.queueStore.Enqueue(ctx, item)
	if err != nil {
		return enqueueResult{}, fmt.Errorf("enqueue telegram fax: %w", err)
	}
	return enqueueResult{Item: queued, Inserted: inserted}, nil
}

func (s *Service) rememberConnection(connection BusinessConnection) {
	if strings.TrimSpace(connection.ID) == "" || connection.User.ID <= 0 {
		return
	}
	s.connectionOwners[connection.ID] = connection.User.ID
}

func (s *Service) ownerIDForMessage(ctx context.Context, message Message) (int64, bool, error) {
	connectionID := strings.TrimSpace(message.BusinessConnectionID)
	if connectionID == "" {
		return 0, false, nil
	}
	if ownerID, ok := s.connectionOwners[connectionID]; ok {
		return ownerID, true, nil
	}
	connection, err := s.client.GetBusinessConnection(ctx, connectionID)
	if errors.Is(err, ErrBusinessConnectionNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("resolve Telegram business connection %s: %w", connectionID, err)
	}
	s.rememberConnection(connection)
	ownerID, ok := s.connectionOwners[connectionID]
	return ownerID, ok, nil
}

func (s *Service) FlushPending(ctx context.Context) (FlushReport, error) {
	report := FlushReport{}
	if s.queueStore == nil {
		return report, nil
	}
	item, ok, err := s.queueStore.NextPending(ctx, s.clock())
	if err != nil {
		return report, fmt.Errorf("load pending telegram fax: %w", err)
	}
	if !ok {
		return report, nil
	}
	config, err := s.printerConfig.LoadPrinter()
	if err != nil {
		return report, fmt.Errorf("load printer config: %w", err)
	}
	if _, err := s.printer.CheckConnection(ctx, config); err != nil {
		s.logf("telegram fax printer is unavailable, queued faxes retained: %v", err)
		return report, nil
	}
	for ctx.Err() == nil {
		ran, printErr := s.printCoordinator.TryRunFax(ctx, func(ctx context.Context) error {
			return s.printQueuedItem(ctx, config, item)
		})
		if printErr != nil {
			nextAttemptAt := s.nextAttemptAt(item)
			exhausted := item.Attempts+1 >= maxQueueAttempts
			if err := s.queueStore.MarkFailed(ctx, item.ID, printErr, nextAttemptAt); err != nil {
				return report, fmt.Errorf("mark telegram fax failed: %w", err)
			}
			s.logf("telegram queued fax print failed: %v", printErr)
			if exhausted {
				report.Failed = append(report.Failed, item)
				s.notifyQueueItem(ctx, item, faxFailedNotification)
			}
			item, ok, err = s.queueStore.NextPending(ctx, s.clock())
			if err != nil {
				return report, fmt.Errorf("load pending telegram fax: %w", err)
			}
			if !ok {
				return report, nil
			}
			continue
		}
		if !ran {
			return report, nil
		}
		if err := s.queueStore.MarkPrinted(ctx, item.ID); err != nil {
			return report, fmt.Errorf("mark telegram fax printed: %w", err)
		}
		report.Printed = append(report.Printed, item)
		s.notifyQueueItem(ctx, item, faxDeliveredNotification)
		item, ok, err = s.queueStore.NextPending(ctx, s.clock())
		if err != nil {
			return report, fmt.Errorf("load pending telegram fax: %w", err)
		}
		if !ok {
			return report, nil
		}
	}
	return report, ctx.Err()
}

func (s *Service) printQueuedItem(ctx context.Context, config printer.Config, item QueueItem) error {
	request := printJobRequestForQueueItem(item)
	jobID := ""
	if s.printJobs != nil {
		var err error
		jobID, err = s.printJobs.StartPrintJob("telegram_fax", request)
		if err != nil {
			return fmt.Errorf("start telegram fax print job: %w", err)
		}
	}
	var printErr error
	if item.ContentType == "photo" {
		printErr = s.downloadAndPrintPhoto(ctx, config, item.Message, item.Photo)
	} else {
		printErr = s.printer.PrintReceipt(ctx, config, FormatReceiptLines(item.Message, s.location))
	}
	if s.printJobs != nil && jobID != "" {
		if err := s.printJobs.FinishPrintJob(jobID, printErr); err != nil {
			s.logf("finish telegram fax print job failed: %v", err)
		}
	}
	return printErr
}

func (s *Service) nextAttemptAt(item QueueItem) time.Time {
	delay := time.Duration(item.Attempts+1) * time.Minute
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	return s.clock().Add(delay)
}

func (s *Service) notifyAcceptedForPendingNewItems(ctx context.Context, newItems []QueueItem, report FlushReport) {
	if len(newItems) == 0 {
		return
	}
	completed := reportedQueueItems(report)
	for _, item := range newItems {
		if _, ok := completed[queueItemReportKey(item)]; ok {
			continue
		}
		s.notifyQueueItem(ctx, item, faxAcceptedNotification)
	}
}

func reportedQueueItems(report FlushReport) map[string]struct{} {
	result := make(map[string]struct{}, len(report.Printed)+len(report.Failed))
	for _, item := range report.Printed {
		result[queueItemReportKey(item)] = struct{}{}
	}
	for _, item := range report.Failed {
		result[queueItemReportKey(item)] = struct{}{}
	}
	return result
}

func queueItemReportKey(item QueueItem) string {
	if strings.TrimSpace(item.ID) != "" {
		return "id:" + item.ID
	}
	return "dedupe:" + item.DedupeKey
}

func (s *Service) notifyQueueItem(ctx context.Context, item QueueItem, text string) {
	if s.client == nil {
		return
	}
	request, ok := sendMessageRequestForQueueItem(item, text)
	if !ok {
		s.logf("telegram fax notification skipped: missing chat id for queue item %s", item.DedupeKey)
		return
	}
	if err := s.client.SendMessage(ctx, request); err != nil {
		s.logf("telegram fax notification failed: %v", err)
	}
}

func sendMessageRequestForQueueItem(item QueueItem, text string) (SendMessageRequest, bool) {
	chatID := int64(0)
	if item.Message.Chat != nil {
		chatID = item.Message.Chat.ID
	} else if item.Message.From != nil {
		chatID = item.Message.From.ID
	}
	if chatID == 0 {
		return SendMessageRequest{}, false
	}
	request := SendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}
	if item.Source == "telegram_business" {
		request.BusinessConnectionID = strings.TrimSpace(item.Message.BusinessConnectionID)
	}
	return request, true
}

func businessDedupeKey(message Message) string {
	return fmt.Sprintf("business:%s:%d", strings.TrimSpace(message.BusinessConnectionID), message.MessageID)
}

func directDedupeKey(message Message) string {
	chatID := int64(0)
	if message.Chat != nil {
		chatID = message.Chat.ID
	}
	return fmt.Sprintf("direct:%d:%d", chatID, message.MessageID)
}

func printJobRequestForQueueItem(item QueueItem) map[string]any {
	request := map[string]any{
		"queueId":     item.ID,
		"dedupeKey":   item.DedupeKey,
		"attempt":     item.Attempts + 1,
		"source":      item.Source,
		"contentType": item.ContentType,
		"messageId":   item.Message.MessageID,
		"sender":      senderDisplayName(item.Message.From),
	}
	if item.Message.From != nil {
		request["senderId"] = item.Message.From.ID
	}
	if item.Source == "telegram_business" {
		request["businessConnectionId"] = item.Message.BusinessConnectionID
		request["businessOwnerId"] = item.BusinessOwnerID
	} else if item.Message.Chat != nil {
		request["chatId"] = item.Message.Chat.ID
	}
	if item.ContentType == "photo" {
		request["caption"] = item.Message.Caption
		request["telegramFileId"] = item.Photo.FileID
		request["telegramFileUniqueId"] = item.Photo.FileUniqueID
		request["telegramPhotoWidth"] = item.Photo.Width
		request["telegramPhotoHeight"] = item.Photo.Height
		request["telegramFileSize"] = item.Photo.FileSize
	} else {
		request["text"] = item.Message.Text
	}
	return request
}

func (s *Service) printMessage(ctx context.Context, message Message, ownerID int64) error {
	config, err := s.printerConfig.LoadPrinter()
	if err != nil {
		return fmt.Errorf("load printer config: %w", err)
	}
	request := map[string]any{
		"source":               "telegram_business",
		"businessConnectionId": message.BusinessConnectionID,
		"businessOwnerId":      ownerID,
		"messageId":            message.MessageID,
		"senderId":             message.From.ID,
		"sender":               senderDisplayName(message.From),
		"text":                 message.Text,
	}
	jobID := ""
	if s.printJobs != nil {
		var err error
		jobID, err = s.printJobs.StartPrintJob("telegram_fax", request)
		if err != nil {
			return fmt.Errorf("start telegram fax print job: %w", err)
		}
	}
	printErr := s.printer.PrintReceipt(ctx, config, FormatReceiptLines(message, s.location))
	if s.printJobs != nil && jobID != "" {
		if err := s.printJobs.FinishPrintJob(jobID, printErr); err != nil {
			return fmt.Errorf("finish telegram fax print job: %w", err)
		}
	}
	return printErr
}

func (s *Service) printDirectMessage(ctx context.Context, message Message) error {
	config, err := s.printerConfig.LoadPrinter()
	if err != nil {
		return fmt.Errorf("load printer config: %w", err)
	}
	request := map[string]any{
		"source":      "telegram_bot_direct",
		"contentType": "text",
		"messageId":   message.MessageID,
		"senderId":    message.From.ID,
		"sender":      senderDisplayName(message.From),
		"chatId":      message.Chat.ID,
		"text":        message.Text,
	}
	jobID := ""
	if s.printJobs != nil {
		var err error
		jobID, err = s.printJobs.StartPrintJob("telegram_fax", request)
		if err != nil {
			return fmt.Errorf("start telegram direct fax print job: %w", err)
		}
	}
	printErr := s.printer.PrintReceipt(ctx, config, FormatReceiptLines(message, s.location))
	if s.printJobs != nil && jobID != "" {
		if err := s.printJobs.FinishPrintJob(jobID, printErr); err != nil {
			return fmt.Errorf("finish telegram direct fax print job: %w", err)
		}
	}
	return printErr
}

func (s *Service) printPhotoMessage(ctx context.Context, message Message, photo PhotoSize, ownerID int64) error {
	config, err := s.printerConfig.LoadPrinter()
	if err != nil {
		return fmt.Errorf("load printer config: %w", err)
	}
	request := map[string]any{
		"source":               "telegram_business",
		"contentType":          "photo",
		"businessConnectionId": message.BusinessConnectionID,
		"businessOwnerId":      ownerID,
		"messageId":            message.MessageID,
		"senderId":             message.From.ID,
		"sender":               senderDisplayName(message.From),
		"caption":              message.Caption,
		"telegramFileId":       photo.FileID,
		"telegramFileUniqueId": photo.FileUniqueID,
		"telegramPhotoWidth":   photo.Width,
		"telegramPhotoHeight":  photo.Height,
		"telegramFileSize":     photo.FileSize,
	}
	jobID := ""
	if s.printJobs != nil {
		var err error
		jobID, err = s.printJobs.StartPrintJob("telegram_fax", request)
		if err != nil {
			return fmt.Errorf("start telegram fax photo print job: %w", err)
		}
	}

	printErr := s.downloadAndPrintPhoto(ctx, config, message, photo)
	if s.printJobs != nil && jobID != "" {
		if err := s.printJobs.FinishPrintJob(jobID, printErr); err != nil {
			return fmt.Errorf("finish telegram fax photo print job: %w", err)
		}
	}
	return printErr
}

func (s *Service) printDirectPhotoMessage(ctx context.Context, message Message, photo PhotoSize) error {
	config, err := s.printerConfig.LoadPrinter()
	if err != nil {
		return fmt.Errorf("load printer config: %w", err)
	}
	request := map[string]any{
		"source":               "telegram_bot_direct",
		"contentType":          "photo",
		"messageId":            message.MessageID,
		"senderId":             message.From.ID,
		"sender":               senderDisplayName(message.From),
		"chatId":               message.Chat.ID,
		"caption":              message.Caption,
		"telegramFileId":       photo.FileID,
		"telegramFileUniqueId": photo.FileUniqueID,
		"telegramPhotoWidth":   photo.Width,
		"telegramPhotoHeight":  photo.Height,
		"telegramFileSize":     photo.FileSize,
	}
	jobID := ""
	if s.printJobs != nil {
		var err error
		jobID, err = s.printJobs.StartPrintJob("telegram_fax", request)
		if err != nil {
			return fmt.Errorf("start telegram direct fax photo print job: %w", err)
		}
	}

	printErr := s.downloadAndPrintPhoto(ctx, config, message, photo)
	if s.printJobs != nil && jobID != "" {
		if err := s.printJobs.FinishPrintJob(jobID, printErr); err != nil {
			return fmt.Errorf("finish telegram direct fax photo print job: %w", err)
		}
	}
	return printErr
}

func (s *Service) downloadAndPrintPhoto(ctx context.Context, config printer.Config, message Message, photo PhotoSize) error {
	file, err := s.client.GetFile(ctx, photo.FileID)
	if err != nil {
		return fmt.Errorf("get Telegram photo file: %w", err)
	}
	if strings.TrimSpace(file.FilePath) == "" {
		return fmt.Errorf("Telegram photo file path is empty")
	}
	data, err := s.client.DownloadFile(ctx, file.FilePath)
	if err != nil {
		return fmt.Errorf("download Telegram photo file: %w", err)
	}
	buffer, err := PhotoBytesToPixelBuffer(data)
	if err != nil {
		return fmt.Errorf("render Telegram photo: %w", err)
	}
	return s.printer.PrintReceipt(ctx, config, FormatPhotoReceiptLines(message, buffer, s.location))
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}
