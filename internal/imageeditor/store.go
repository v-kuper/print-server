package imageeditor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"atol-server/internal/printer"
)

const ReceiptWidth = 384

type Store struct {
	dir   string
	clock func() time.Time
}

type SaveInput struct {
	Width      int
	Height     int
	Pixels     []byte
	PreviewPNG []byte
	Settings   map[string]any
}

type State struct {
	Available  bool           `json:"available"`
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	PreviewURL string         `json:"previewUrl,omitempty"`
	UpdatedAt  *time.Time     `json:"updatedAt,omitempty"`
	Settings   map[string]any `json:"settings"`
}

type metadata struct {
	Width     int            `json:"width"`
	Height    int            `json:"height"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Settings  map[string]any `json:"settings"`
}

func NewStore(dir string, clock func() time.Time) *Store {
	if clock == nil {
		clock = time.Now
	}
	return &Store{dir: dir, clock: clock}
}

func (s *Store) State() (State, error) {
	meta, err := s.loadMetadata()
	if os.IsNotExist(err) {
		return State{Settings: map[string]any{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	if _, err := os.Stat(s.bufferPath()); err != nil {
		if os.IsNotExist(err) {
			return State{Settings: map[string]any{}}, nil
		}
		return State{}, err
	}
	if _, err := os.Stat(s.previewPath()); err != nil {
		if os.IsNotExist(err) {
			return State{Settings: map[string]any{}}, nil
		}
		return State{}, err
	}
	return State{
		Available:  true,
		Width:      meta.Width,
		Height:     meta.Height,
		PreviewURL: "/api/image-editor/preview",
		UpdatedAt:  &meta.UpdatedAt,
		Settings:   cloneSettings(meta.Settings),
	}, nil
}

func (s *Store) Save(input SaveInput) (State, error) {
	if input.Width != ReceiptWidth {
		return State{}, fmt.Errorf("width must be %d", ReceiptWidth)
	}
	buffer := printer.PixelBuffer{
		Width:  input.Width,
		Height: input.Height,
		Pixels: append([]byte(nil), input.Pixels...),
	}
	if err := buffer.Validate(); err != nil {
		return State{}, err
	}
	if err := validatePreviewPNG(input.PreviewPNG); err != nil {
		return State{}, err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return State{}, err
	}

	meta := metadata{
		Width:     input.Width,
		Height:    input.Height,
		UpdatedAt: s.clock().UTC(),
		Settings:  cloneSettings(input.Settings),
	}
	if meta.Settings == nil {
		meta.Settings = map[string]any{}
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return State{}, err
	}
	metaData = append(metaData, '\n')

	if err := writeFileAtomic(s.bufferPath(), input.Pixels, 0o644); err != nil {
		return State{}, err
	}
	if err := writeFileAtomic(s.previewPath(), input.PreviewPNG, 0o644); err != nil {
		return State{}, err
	}
	if err := writeFileAtomic(s.metadataPath(), metaData, 0o644); err != nil {
		return State{}, err
	}
	return State{
		Available:  true,
		Width:      meta.Width,
		Height:     meta.Height,
		PreviewURL: "/api/image-editor/preview",
		UpdatedAt:  &meta.UpdatedAt,
		Settings:   cloneSettings(meta.Settings),
	}, nil
}

func (s *Store) LoadBuffer() (printer.PixelBuffer, error) {
	meta, err := s.loadMetadata()
	if err != nil {
		if os.IsNotExist(err) {
			return printer.PixelBuffer{}, fmt.Errorf("image editor buffer is not saved")
		}
		return printer.PixelBuffer{}, err
	}
	data, err := os.ReadFile(s.bufferPath())
	if err != nil {
		if os.IsNotExist(err) {
			return printer.PixelBuffer{}, fmt.Errorf("image editor buffer is not saved")
		}
		return printer.PixelBuffer{}, err
	}
	buffer := printer.PixelBuffer{Width: meta.Width, Height: meta.Height, Pixels: data}
	if err := buffer.Validate(); err != nil {
		return printer.PixelBuffer{}, err
	}
	return buffer.Normalized(), nil
}

func (s *Store) PreviewPath() string {
	return s.previewPath()
}

func (s *Store) Clear() error {
	for _, path := range []string{s.bufferPath(), s.metadataPath(), s.previewPath()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Store) loadMetadata() (metadata, error) {
	data, err := os.ReadFile(s.metadataPath())
	if err != nil {
		return metadata{}, err
	}
	var meta metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return metadata{}, err
	}
	if meta.Settings == nil {
		meta.Settings = map[string]any{}
	}
	return meta, nil
}

func (s *Store) bufferPath() string {
	return filepath.Join(s.dir, "current.bin")
}

func (s *Store) metadataPath() string {
	return filepath.Join(s.dir, "current.json")
}

func (s *Store) previewPath() string {
	return filepath.Join(s.dir, "preview.png")
}

func validatePreviewPNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("preview PNG is required")
	}
	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("preview PNG is invalid: %w", err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func cloneSettings(settings map[string]any) map[string]any {
	if settings == nil {
		return nil
	}
	clone := make(map[string]any, len(settings))
	for key, value := range settings {
		clone[key] = value
	}
	return clone
}
