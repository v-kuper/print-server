package imageeditor

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"testing"

	"atol-server/internal/storage"
)

func TestPostgresStoreSavesLoadsPreviewAndClearsState(t *testing.T) {
	ctx := context.Background()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	if err := storage.Migrate(ctx, databaseURL); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	pool, err := storage.OpenPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE legacy_imports, audit_events, image_editor_state, workspaces RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("reset image editor tables: %v", err)
	}
	workspaceID := "00000000-0000-0000-0000-000000000001"
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, slug, name) VALUES ($1, 'default', 'Default')`, workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	previewPNG, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lJypqAAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("decode preview png: %v", err)
	}
	pixels := make([]byte, ReceiptWidth)
	for index := range pixels {
		if index%2 == 1 {
			pixels[index] = 255
		}
	}

	store := NewPostgresStore(pool, workspaceID, nil)
	state, err := store.Save(SaveInput{
		Width:      ReceiptWidth,
		Height:     1,
		Pixels:     pixels,
		PreviewPNG: previewPNG,
		Settings:   map[string]any{"threshold": float64(128)},
	})
	if err != nil {
		t.Fatalf("save image editor state: %v", err)
	}
	if !state.Available || state.Width != ReceiptWidth || state.Height != 1 {
		t.Fatalf("unexpected saved state: %#v", state)
	}

	loadedPreview, err := store.LoadPreviewPNG()
	if err != nil {
		t.Fatalf("load preview png: %v", err)
	}
	if !bytes.Equal(loadedPreview, previewPNG) {
		t.Fatalf("expected saved preview png")
	}
	buffer, err := store.LoadBuffer()
	if err != nil {
		t.Fatalf("load pixel buffer: %v", err)
	}
	if buffer.Width != ReceiptWidth || buffer.Height != 1 || !bytes.Equal(buffer.Pixels, pixels) {
		t.Fatalf("unexpected pixel buffer: %#v", buffer)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear image editor state: %v", err)
	}
	state, err = store.State()
	if err != nil {
		t.Fatalf("load cleared state: %v", err)
	}
	if state.Available || len(state.Settings) != 0 {
		t.Fatalf("expected empty state after clear, got %#v", state)
	}
}
