package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"

	"atol-server/internal/imageeditor"
)

type imageEditorResponse struct {
	OK      bool               `json:"ok"`
	Message string             `json:"message,omitempty"`
	Error   string             `json:"error,omitempty"`
	Data    *imageeditor.State `json:"data,omitempty"`
}

type imageEditorSaveRequest struct {
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Pixels     []int          `json:"pixels"`
	Settings   map[string]any `json:"settings"`
	PreviewPNG string         `json:"previewPng"`
}

func (s *Server) handleImageEditorState(w http.ResponseWriter, r *http.Request) {
	state, err := s.imageEditorStore().State()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:   true,
		Data: &state,
	})
}

func (s *Server) handleImageEditorSave(w http.ResponseWriter, r *http.Request) {
	input, err := decodeImageEditorSaveRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	state, err := s.imageEditorStore().Save(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:      true,
		Message: "Изображение сохранено.",
		Data:    &state,
	})
}

func (s *Server) handleImageEditorPreview(w http.ResponseWriter, r *http.Request) {
	data, err := s.imageEditorStore().LoadPreviewPNG()
	if os.IsNotExist(err) {
		writeJSON(w, http.StatusNotFound, imageEditorResponse{
			OK:    false,
			Error: "image editor preview is not saved",
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handleImageEditorPrint(w http.ResponseWriter, r *http.Request) {
	config, err := s.store.LoadPrinter()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	buffer, err := s.imageEditorStore().LoadBuffer()
	if err != nil {
		writeJSON(w, http.StatusNotFound, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	jobID, err := s.startPrintJob("image", map[string]any{"source": "image_editor"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	printErr := s.printer.PrintPixelBuffer(r.Context(), config, buffer)
	finishErr := s.finishPrintJob(jobID, printErr)
	if printErr != nil {
		writeJSON(w, http.StatusBadGateway, imageEditorResponse{
			OK:    false,
			Error: printErr.Error(),
		})
		return
	}
	if finishErr != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: finishErr.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:      true,
		Message: "Изображение напечатано.",
	})
}

func (s *Server) handleImageEditorClear(w http.ResponseWriter, r *http.Request) {
	if err := s.imageEditorStore().Clear(); err != nil {
		writeJSON(w, http.StatusInternalServerError, imageEditorResponse{
			OK:    false,
			Error: err.Error(),
		})
		return
	}
	state := imageeditor.State{Settings: map[string]any{}}
	writeJSON(w, http.StatusOK, imageEditorResponse{
		OK:      true,
		Message: "Изображение удалено.",
		Data:    &state,
	})
}

func decodeImageEditorSaveRequest(r *http.Request) (imageeditor.SaveInput, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType == "multipart/form-data" {
		return decodeMultipartImageEditorSaveRequest(r)
	}

	var request imageEditorSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("invalid JSON: %w", err)
	}
	pixels, err := pixelsFromInts(request.Pixels)
	if err != nil {
		return imageeditor.SaveInput{}, err
	}
	preview, err := decodePreviewPNGField(request.PreviewPNG)
	if err != nil {
		return imageeditor.SaveInput{}, err
	}
	return imageeditor.SaveInput{
		Width:      request.Width,
		Height:     request.Height,
		Pixels:     pixels,
		PreviewPNG: preview,
		Settings:   request.Settings,
	}, nil
}

func decodeMultipartImageEditorSaveRequest(r *http.Request) (imageeditor.SaveInput, error) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("invalid multipart form: %w", err)
	}
	width, err := strconv.Atoi(strings.TrimSpace(r.FormValue("width")))
	if err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("width is required")
	}
	height, err := strconv.Atoi(strings.TrimSpace(r.FormValue("height")))
	if err != nil {
		return imageeditor.SaveInput{}, fmt.Errorf("height is required")
	}
	pixels, err := multipartFileBytes(r, "pixels")
	if err != nil {
		field := strings.TrimSpace(r.FormValue("pixels"))
		if field == "" {
			return imageeditor.SaveInput{}, err
		}
		var values []int
		if jsonErr := json.Unmarshal([]byte(field), &values); jsonErr != nil {
			return imageeditor.SaveInput{}, fmt.Errorf("pixels must be a binary file or JSON array: %w", jsonErr)
		}
		pixels, err = pixelsFromInts(values)
		if err != nil {
			return imageeditor.SaveInput{}, err
		}
	}
	preview, err := multipartFileBytes(r, "previewPng")
	if err != nil {
		preview, err = decodePreviewPNGField(r.FormValue("previewPng"))
		if err != nil {
			return imageeditor.SaveInput{}, err
		}
	}
	settings := map[string]any{}
	if rawSettings := strings.TrimSpace(r.FormValue("settings")); rawSettings != "" {
		if err := json.Unmarshal([]byte(rawSettings), &settings); err != nil {
			return imageeditor.SaveInput{}, fmt.Errorf("settings must be valid JSON: %w", err)
		}
	}
	return imageeditor.SaveInput{
		Width:      width,
		Height:     height,
		Pixels:     pixels,
		PreviewPNG: preview,
		Settings:   settings,
	}, nil
}

func multipartFileBytes(r *http.Request, field string) ([]byte, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("%s file is required", field)
	}
	defer file.Close()
	return io.ReadAll(file)
}

func pixelsFromInts(values []int) ([]byte, error) {
	pixels := make([]byte, len(values))
	for index, value := range values {
		if value != 0 && value != 255 {
			return nil, fmt.Errorf("pixel value at index %d must be 0 or 255, got %d", index, value)
		}
		pixels[index] = byte(value)
	}
	return pixels, nil
}

func decodePreviewPNGField(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("previewPng is required")
	}
	const prefix = "data:image/png;base64,"
	if strings.HasPrefix(value, prefix) {
		value = strings.TrimPrefix(value, prefix)
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("previewPng must be base64 PNG: %w", err)
	}
	return data, nil
}
