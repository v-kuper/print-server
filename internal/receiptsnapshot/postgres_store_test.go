package receiptsnapshot

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"atol-server/internal/storage"
)

func TestPostgresStoreCreatesPublishesFailsAndLoadsSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := openSnapshotTestPool(t, ctx)
	resetSnapshotTestDatabase(t, ctx, pool)
	workspaceID := ensureSnapshotTestWorkspace(t, ctx, pool)
	store := NewPostgresStore(pool, workspaceID)

	items := []NewsItem{
		{SourceName: "BBC Russian", Title: "Заголовок", OriginalTitle: "Title", Link: "https://example.com/1"},
	}
	lines := []ReceiptLine{
		{Text: "25 Мая", Alignment: "center", Role: "calendar"},
		{Text: "Коротко о мире:", Alignment: "center"},
	}
	created, err := store.Create(ctx, CreateInput{NewsItems: items, ReceiptLines: lines})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if created.ID == "" || created.Status != StatusPending {
		t.Fatalf("expected pending snapshot with id, got %#v", created)
	}
	if !reflect.DeepEqual(created.NewsItems, items) {
		t.Fatalf("expected saved news items %#v, got %#v", items, created.NewsItems)
	}
	if !reflect.DeepEqual(created.ReceiptLines, lines) {
		t.Fatalf("expected saved receipt lines %#v, got %#v", lines, created.ReceiptLines)
	}

	if err := store.Publish(ctx, created.ID); err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}
	published, err := store.Load(ctx, created.ID)
	if err != nil {
		t.Fatalf("load published snapshot: %v", err)
	}
	if published.Status != StatusPublished || published.PublishedAt == nil {
		t.Fatalf("expected published snapshot, got %#v", published)
	}

	failed, err := store.Create(ctx, CreateInput{NewsItems: items, ReceiptLines: lines})
	if err != nil {
		t.Fatalf("create failed snapshot: %v", err)
	}
	if err := store.Fail(ctx, failed.ID, errors.New("paper empty")); err != nil {
		t.Fatalf("fail snapshot: %v", err)
	}
	loadedFailed, err := store.Load(ctx, failed.ID)
	if err != nil {
		t.Fatalf("load failed snapshot: %v", err)
	}
	if loadedFailed.Status != StatusFailed || loadedFailed.FailedAt == nil || loadedFailed.Error != "paper empty" {
		t.Fatalf("expected failed snapshot with error, got %#v", loadedFailed)
	}
}

func openSnapshotTestPool(t *testing.T, ctx context.Context) storage.Pool {
	t.Helper()
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
	return pool
}

func resetSnapshotTestDatabase(t *testing.T, ctx context.Context, pool storage.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `TRUNCATE
		receipt_snapshots,
		legacy_imports,
		audit_events,
		print_jobs,
		google_tokens,
		image_editor_state,
		scheduler_state,
		workspace_settings,
		printers,
		workspace_memberships,
		users,
		workspaces
	RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset test database: %v", err)
	}
}

func ensureSnapshotTestWorkspace(t *testing.T, ctx context.Context, pool storage.Pool) string {
	t.Helper()
	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspaces (slug, name)
		VALUES ('default', 'Default')
		RETURNING id::text`).Scan(&workspaceID); err != nil {
		t.Fatalf("ensure test workspace: %v", err)
	}
	return workspaceID
}
