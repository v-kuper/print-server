package telegramfax

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"atol-server/internal/printer"
	"atol-server/internal/receipt"
)

var businessAllowedUpdates = []string{
	"business_connection",
	"business_message",
	"edited_business_message",
	"deleted_business_messages",
}

type PrinterConfigStore interface {
	LoadPrinter() (printer.Config, error)
}

type PrintJobStore interface {
	StartPrintJob(kind string, request any) (string, error)
	FinishPrintJob(id string, printErr error) error
}

type Printer interface {
	PrintReceipt(context.Context, printer.Config, []receipt.Line) error
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
	clock            func() time.Time
	location         *time.Location
	retryDelay       time.Duration
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
		location:         time.Local,
		retryDelay:       5 * time.Second,
		connectionOwners: make(map[string]int64),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) Start(ctx context.Context) {
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

func (s *Service) PollOnce(ctx context.Context) error {
	state, err := s.stateStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("load telegram fax state: %w", err)
	}
	updates, err := s.client.GetUpdates(ctx, GetUpdatesRequest{
		Offset:         state.NextUpdateOffset,
		Timeout:        int(s.config.PollTimeout / time.Second),
		AllowedUpdates: append([]string(nil), businessAllowedUpdates...),
	})
	if err != nil {
		return fmt.Errorf("get Telegram updates: %w", err)
	}
	for _, update := range updates {
		if err := s.processUpdate(ctx, update); err != nil {
			return err
		}
		if update.UpdateID >= state.NextUpdateOffset {
			state.NextUpdateOffset = update.UpdateID + 1
		}
		if err := s.stateStore.Save(ctx, state); err != nil {
			return fmt.Errorf("save telegram fax state: %w", err)
		}
	}
	return nil
}

func (s *Service) processUpdate(ctx context.Context, update Update) error {
	if update.BusinessConnection != nil {
		s.rememberConnection(*update.BusinessConnection)
		return nil
	}
	if update.BusinessMessage == nil {
		return nil
	}
	message := update.BusinessMessage
	if message.From == nil || message.From.IsBot {
		return nil
	}
	photo, hasPhoto := bestPhotoSize(message.Photo)
	if strings.TrimSpace(message.Text) == "" && !hasPhoto {
		return nil
	}
	if len(s.config.AllowedSenderIDs) > 0 && !s.config.AllowedSenderIDs.Contains(message.From.ID) {
		return nil
	}
	ownerID, ok, err := s.ownerIDForMessage(ctx, *message)
	if err != nil {
		return err
	}
	if !ok || !s.config.OwnerIDs.Contains(ownerID) {
		return nil
	}
	if message.From.ID == ownerID {
		return nil
	}
	if hasPhoto {
		if err := s.printPhotoMessage(ctx, *message, photo, ownerID); err != nil {
			s.logf("telegram fax photo print failed: %v", err)
		}
		return nil
	}
	if err := s.printMessage(ctx, *message, ownerID); err != nil {
		s.logf("telegram fax print failed: %v", err)
	}
	return nil
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
