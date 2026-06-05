package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"atol-server/internal/app"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
)

type statusResponse struct {
	OK       bool            `json:"ok"`
	Message  string          `json:"message,omitempty"`
	Error    string          `json:"error,omitempty"`
	Config   *printer.Config `json:"config,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
}

type receiptPreviewResponse struct {
	OK       bool           `json:"ok"`
	Message  string         `json:"message,omitempty"`
	Error    string         `json:"error,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
	Lines    []receipt.Line `json:"lines,omitempty"`
}

type fontMetricsResponse struct {
	OK      bool                 `json:"ok"`
	Message string               `json:"message,omitempty"`
	Error   string               `json:"error,omitempty"`
	Fonts   []printer.FontMetric `json:"fonts,omitempty"`
}

type textPrintBlock struct {
	Text         string `json:"text"`
	Font         int    `json:"font"`
	Alignment    string `json:"alignment"`
	DoubleWidth  bool   `json:"doubleWidth"`
	DoubleHeight bool   `json:"doubleHeight"`
}

type textPrintRequest struct {
	Blocks []textPrintBlock `json:"blocks"`
}

func (s *Server) handlePrinterCheck(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	message, err := s.printer.CheckConnection(r.Context(), config)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: message,
	})
	s.flushTelegramFaxQueue()
}

func (s *Server) handlePrinterFonts(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, fontMetricsResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	fonts, err := s.printer.FontMetrics(r.Context(), config)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, fontMetricsResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, fontMetricsResponse{
		OK:      true,
		Message: "Метрики шрифтов ККТ загружены.",
		Fonts:   fonts,
	})
}

func (s *Server) handlePrintTest(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	jobID, err := s.startPrintJob("test", map[string]any{"receipt": "test"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.printCoordinator.RunUserPrint(r.Context(), func(ctx context.Context) error {
		return s.printer.PrintReceipt(ctx, config, receipt.TestReceipt(s.clock()))
	})
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Тестовый чек напечатан.",
	})
}

func (s *Server) handlePrintText(w http.ResponseWriter, r *http.Request) {
	var request textPrintRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: "invalid JSON: " + err.Error(),
		})
		return
	}
	lines, err := textPrintLines(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	jobID, err := s.startPrintJob("text", request)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.printCoordinator.RunUserPrint(r.Context(), func(ctx context.Context) error {
		return s.printer.PrintReceipt(ctx, config, lines)
	})
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, http.StatusBadGateway, statusResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		OK:      true,
		Message: "Текст напечатан.",
	})
}

func (s *Server) handleReceiptPreview(w http.ResponseWriter, r *http.Request) {
	var lines []receipt.Line
	var warnings []string
	var err error
	if service, ok := s.receiptService.(DailyReceiptPreviewWarningService); ok {
		lines, warnings, err = service.BuildDailyReceiptPreviewWithWarnings(r.Context())
	} else if service, ok := s.receiptService.(DailyReceiptWarningService); ok {
		lines, warnings, err = service.BuildDailyReceiptWithWarnings(r.Context())
	} else {
		lines, err = s.receiptService.BuildDailyReceipt(r.Context())
	}
	if err != nil {
		writeJSON(w, statusForBuildError(err), receiptPreviewResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	message := "Превью собрано."
	if len(warnings) > 0 {
		message += "\n" + strings.Join(warnings, "\n")
	}
	writeJSON(w, http.StatusOK, receiptPreviewResponse{
		OK:       true,
		Message:  message,
		Warnings: warnings,
		Lines:    lines,
	})
}

func (s *Server) handlePrintWeather(w http.ResponseWriter, r *http.Request) {
	jobID, err := s.startPrintJob("weather", map[string]any{"receipt": "daily"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	var warnings []string
	var printErr error
	if service, ok := s.receiptService.(DailyReceiptPrintWarningService); ok {
		warnings, printErr = service.PrintDailyReceiptWithWarnings(r.Context())
	} else {
		printErr = s.receiptService.PrintDailyReceipt(r.Context())
	}
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, statusForBuildError(printErr), statusResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, statusResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}

	message := "Чек напечатан."
	if len(warnings) > 0 {
		message += "\n" + strings.Join(warnings, "\n")
	}
	writeJSON(w, http.StatusOK, statusResponse{
		OK:       true,
		Message:  message,
		Warnings: warnings,
	})
}

func (s *Server) startPrintJob(kind string, request any) (string, error) {
	store, ok := s.store.(PrintJobStore)
	if !ok {
		return "", nil
	}
	return store.StartPrintJob(kind, request)
}

func (s *Server) finishPrintJob(id string, printErr error) error {
	if id == "" {
		return nil
	}
	store, ok := s.store.(PrintJobStore)
	if !ok {
		return nil
	}
	return store.FinishPrintJob(id, printErr)
}

func (s *Server) flushTelegramFaxQueue() {
	if s.telegramFaxFlusher == nil {
		return
	}
	go func() {
		_, _ = s.telegramFaxFlusher.FlushPending(context.Background())
	}()
}

func textPrintLines(request textPrintRequest) ([]receipt.Line, error) {
	if len(request.Blocks) == 0 {
		return nil, fmt.Errorf("blocks is required")
	}
	hasContent := false
	for _, b := range request.Blocks {
		if strings.TrimSpace(b.Text) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return nil, fmt.Errorf("at least one block must have text")
	}
	var lines []receipt.Line
	for _, block := range request.Blocks {
		font, err := validatedTextPrintFont(block.Font, "font")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(block.Alignment) == "" {
			block.Alignment = string(receipt.AlignmentLeft)
		}
		alignment, err := validatedTextPrintAlignment(block.Alignment, "alignment")
		if err != nil {
			return nil, err
		}
		text := strings.ReplaceAll(block.Text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		for _, lineText := range strings.Split(text, "\n") {
			lines = append(lines, receipt.Line{
				Text:         lineText,
				Alignment:    alignment,
				Role:         receipt.RoleNormal,
				Font:         font,
				DoubleWidth:  block.DoubleWidth,
				DoubleHeight: block.DoubleHeight,
			})
		}
	}
	return lines, nil
}

func validatedTextPrintFont(font int, field string) (int, error) {
	if font < 0 || font > maxTextPrintFont {
		return 0, fmt.Errorf("%s must be between 0 and %d", field, maxTextPrintFont)
	}
	return font, nil
}

func validatedTextPrintAlignment(value string, field string) (receipt.Alignment, error) {
	switch receipt.Alignment(strings.TrimSpace(value)) {
	case receipt.AlignmentLeft:
		return receipt.AlignmentLeft, nil
	case receipt.AlignmentCenter, "":
		return receipt.AlignmentCenter, nil
	case receipt.AlignmentRight:
		return receipt.AlignmentRight, nil
	default:
		return "", fmt.Errorf("%s must be left, center, or right", field)
	}
}

func statusForBuildError(err error) int {
	var buildErr app.BuildError
	if errors.As(err, &buildErr) {
		return buildErr.Status
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
