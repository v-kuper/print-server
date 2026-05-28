package imageeditor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"atol-server/internal/printer"
	"atol-server/internal/storage"

	"github.com/jackc/pgx/v5"
)

type PostgresStore struct {
	pool        storage.Pool
	workspaceID string
	clock       func() time.Time
}

func NewPostgresStore(pool storage.Pool, workspaceID string, clock func() time.Time) *PostgresStore {
	if clock == nil {
		clock = time.Now
	}
	return &PostgresStore{
		pool:        pool,
		workspaceID: workspaceID,
		clock:       clock,
	}
}

func (s *PostgresStore) State() (State, error) {
	var (
		width     int
		height    int
		settings  []byte
		updatedAt time.Time
	)
	err := s.pool.QueryRow(context.Background(), `
		SELECT width, height, settings, updated_at
		FROM image_editor_state
		WHERE workspace_id = $1`, s.workspaceID).Scan(&width, &height, &settings, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{Settings: map[string]any{}}, nil
	}
	if err != nil {
		return State{}, err
	}
	decodedSettings := map[string]any{}
	if err := json.Unmarshal(settings, &decodedSettings); err != nil {
		return State{}, fmt.Errorf("decode image editor settings: %w", err)
	}
	return State{
		Available:  true,
		Width:      width,
		Height:     height,
		PreviewURL: "/api/image-editor/preview",
		UpdatedAt:  &updatedAt,
		Settings:   decodedSettings,
	}, nil
}

func (s *PostgresStore) Save(input SaveInput) (State, error) {
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
	settings := cloneSettings(input.Settings)
	if settings == nil {
		settings = map[string]any{}
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return State{}, err
	}
	updatedAt := s.clock().UTC()
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO image_editor_state (workspace_id, width, height, pixels, preview_png, settings, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
		ON CONFLICT (workspace_id)
		DO UPDATE SET
			width = EXCLUDED.width,
			height = EXCLUDED.height,
			pixels = EXCLUDED.pixels,
			preview_png = EXCLUDED.preview_png,
			settings = EXCLUDED.settings,
			updated_at = EXCLUDED.updated_at`,
		s.workspaceID, input.Width, input.Height, input.Pixels, input.PreviewPNG, string(settingsJSON), updatedAt)
	if err != nil {
		return State{}, err
	}
	if err := s.audit("image_editor.save", map[string]any{"width": input.Width, "height": input.Height}); err != nil {
		return State{}, err
	}
	return State{
		Available:  true,
		Width:      input.Width,
		Height:     input.Height,
		PreviewURL: "/api/image-editor/preview",
		UpdatedAt:  &updatedAt,
		Settings:   cloneSettings(settings),
	}, nil
}

func (s *PostgresStore) LoadBuffer() (printer.PixelBuffer, error) {
	var (
		width  int
		height int
		pixels []byte
	)
	err := s.pool.QueryRow(context.Background(), `
		SELECT width, height, pixels
		FROM image_editor_state
		WHERE workspace_id = $1`, s.workspaceID).Scan(&width, &height, &pixels)
	if errors.Is(err, pgx.ErrNoRows) {
		return printer.PixelBuffer{}, fmt.Errorf("image editor buffer is not saved: %w", os.ErrNotExist)
	}
	if err != nil {
		return printer.PixelBuffer{}, err
	}
	buffer := printer.PixelBuffer{Width: width, Height: height, Pixels: pixels}
	if err := buffer.Validate(); err != nil {
		return printer.PixelBuffer{}, err
	}
	return buffer.Normalized(), nil
}

func (s *PostgresStore) LoadPreviewPNG() ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT preview_png
		FROM image_editor_state
		WHERE workspace_id = $1`, s.workspaceID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("image editor preview is not saved: %w", os.ErrNotExist)
	}
	if err != nil {
		return nil, err
	}
	if err := validatePreviewPNG(data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *PostgresStore) Clear() error {
	if _, err := s.pool.Exec(context.Background(), `DELETE FROM image_editor_state WHERE workspace_id = $1`, s.workspaceID); err != nil {
		return err
	}
	return s.audit("image_editor.clear", map[string]any{})
}

func (s *PostgresStore) ImportLegacy(ctx context.Context, dir string) error {
	imported, err := s.legacyImported(ctx, "image-editor")
	if err != nil || imported {
		return err
	}
	if strings.TrimSpace(dir) == "" {
		return s.markLegacyImported(ctx, "image-editor")
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM image_editor_state WHERE workspace_id = $1`, s.workspaceID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return s.markLegacyImported(ctx, "image-editor")
	}
	fileStore := NewStore(dir, s.clock)
	state, err := fileStore.State()
	if err != nil {
		return err
	}
	if !state.Available {
		return s.markLegacyImported(ctx, "image-editor")
	}
	buffer, err := fileStore.LoadBuffer()
	if err != nil {
		return err
	}
	preview, err := fileStore.LoadPreviewPNG()
	if err != nil {
		return err
	}
	if _, err := s.Save(SaveInput{
		Width:      buffer.Width,
		Height:     buffer.Height,
		Pixels:     buffer.Pixels,
		PreviewPNG: preview,
		Settings:   state.Settings,
	}); err != nil {
		return err
	}
	if err := s.audit("legacy.import", map[string]any{"source": "image-editor", "dir": dir}); err != nil {
		return err
	}
	return s.markLegacyImported(ctx, "image-editor")
}

func (s *PostgresStore) legacyImported(ctx context.Context, source string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM legacy_imports WHERE source = $1)`, source).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PostgresStore) markLegacyImported(ctx context.Context, source string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO legacy_imports (source)
		VALUES ($1)
		ON CONFLICT (source) DO NOTHING`, source)
	return err
}

func (s *PostgresStore) audit(action string, metadata any) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO audit_events (workspace_id, action, subject, metadata)
		VALUES ($1, $2, 'image_editor', $3::jsonb)`, s.workspaceID, action, string(data))
	return err
}
