package settings

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"atol-server/internal/finance"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/receiptsnapshot"
	"atol-server/internal/schedule"
	"atol-server/internal/storage"
	"atol-server/internal/weather"
)

func TestPostgresStoreSavesLoadsSettingsAndRecordsAudit(t *testing.T) {
	ctx := context.Background()
	pool := openPostgresStoreTestPool(t, ctx)
	resetPostgresStoreTestDatabase(t, ctx, pool)

	store, err := NewPostgresStore(ctx, pool)
	if err != nil {
		t.Fatalf("create postgres store: %v", err)
	}

	if got, err := store.LoadWeather(); err != nil || got != weather.DefaultLocation() {
		t.Fatalf("expected default weather location, got %#v, err=%v", got, err)
	}

	printerConfig := printer.Config{Host: " 192.168.0.118 ", Port: 5555}
	if err := store.SavePrinter(printerConfig); err != nil {
		t.Fatalf("save printer: %v", err)
	}
	if got, err := store.LoadPrinter(); err != nil || got != printerConfig.Normalized() {
		t.Fatalf("expected printer %#v, got %#v, err=%v", printerConfig.Normalized(), got, err)
	}

	weatherLocation := weather.Location{Name: " Гомель ", Latitude: 52.4345, Longitude: 30.9754}
	if err := store.SaveWeather(weatherLocation); err != nil {
		t.Fatalf("save weather: %v", err)
	}
	if got, err := store.LoadWeather(); err != nil || got != weatherLocation.Normalized() {
		t.Fatalf("expected weather %#v, got %#v, err=%v", weatherLocation.Normalized(), got, err)
	}

	portfolio := finance.TonPortfolio{AmountTon: 12.5, InvestedUSD: 100}
	if err := store.SaveFinance(portfolio); err != nil {
		t.Fatalf("save finance: %v", err)
	}
	if got, err := store.LoadFinance(); err != nil || got != portfolio {
		t.Fatalf("expected finance %#v, got %#v, err=%v", portfolio, got, err)
	}

	newsSettings := news.DefaultSettings()
	newsSettings.Sources[0].Enabled = false
	translateTitles := true
	newsSettings.TranslateTitles = &translateTitles
	if err := store.SaveNews(newsSettings); err != nil {
		t.Fatalf("save news: %v", err)
	}
	if got, err := store.LoadNews(); err != nil || !reflect.DeepEqual(got, newsSettings.Normalized()) {
		t.Fatalf("expected news %#v, got %#v, err=%v", newsSettings.Normalized(), got, err)
	}

	motivationSettings := motivation.DefaultSettings()
	motivationSettings.Model = "llama3.2"
	if err := store.SaveMotivation(motivationSettings); err != nil {
		t.Fatalf("save motivation: %v", err)
	}
	if got, err := store.LoadMotivation(); err != nil || got != motivationSettings.Normalized() {
		t.Fatalf("expected motivation %#v, got %#v, err=%v", motivationSettings.Normalized(), got, err)
	}

	style := receipt.DefaultStyleSettings()
	style.NormalFont = 2
	if err := store.SaveReceiptStyle(style); err != nil {
		t.Fatalf("save receipt style: %v", err)
	}
	if got, err := store.LoadReceiptStyle(); err != nil || got != style.Normalized() {
		t.Fatalf("expected receipt style %#v, got %#v, err=%v", style.Normalized(), got, err)
	}

	content := receipt.DefaultContentSettings()
	content.ShowNews = true
	content.ShowMail = true
	if err := store.SaveReceiptContent(content); err != nil {
		t.Fatalf("save receipt content: %v", err)
	}
	if got, err := store.LoadReceiptContent(); err != nil || got != content.Normalized() {
		t.Fatalf("expected receipt content %#v, got %#v, err=%v", content.Normalized(), got, err)
	}

	scheduleSettings := schedule.DefaultSettings()
	scheduleSettings.Enabled = true
	scheduleSettings.IntervalMinutes = 30
	if err := store.SaveSchedule(scheduleSettings); err != nil {
		t.Fatalf("save schedule: %v", err)
	}
	if got, err := store.LoadSchedule(); err != nil || !reflect.DeepEqual(got, scheduleSettings.Normalized()) {
		t.Fatalf("expected schedule %#v, got %#v, err=%v", scheduleSettings.Normalized(), got, err)
	}

	state := schedule.State{
		LastAttemptAt: time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC),
		LastSuccessAt: time.Date(2026, 5, 28, 8, 1, 0, 0, time.UTC),
		NextRunAt:     time.Date(2026, 5, 28, 9, 0, 0, 0, time.UTC),
	}
	if err := store.SaveScheduleState(state); err != nil {
		t.Fatalf("save schedule state: %v", err)
	}
	if got, err := store.LoadScheduleState(); err != nil || !reflect.DeepEqual(got, state) {
		t.Fatalf("expected schedule state %#v, got %#v, err=%v", state, got, err)
	}

	snapshotSettings := receiptsnapshot.Settings{BaseURL: "http://192.168.0.25:8080/"}
	if err := store.SaveReceiptSnapshotSettings(snapshotSettings); err != nil {
		t.Fatalf("save receipt snapshot settings: %v", err)
	}
	if got, err := store.LoadReceiptSnapshotSettings(); err != nil || got.BaseURL != "http://192.168.0.25:8080" {
		t.Fatalf("expected normalized receipt snapshot settings, got %#v, err=%v", got, err)
	}

	jobID, err := store.StartPrintJob("text", map[string]any{"blocks": 1})
	if err != nil {
		t.Fatalf("start print job: %v", err)
	}
	if err := store.FinishPrintJob(jobID, errors.New("paper empty")); err != nil {
		t.Fatalf("finish print job: %v", err)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE workspace_id = $1`, store.WorkspaceID()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit events: %v", err)
	}
	if auditCount < 8 {
		t.Fatalf("expected audit events for saved settings and print job, got %d", auditCount)
	}
}

func TestPostgresStoreImportsLegacySettingsOnlyOnce(t *testing.T) {
	ctx := context.Background()
	pool := openPostgresStoreTestPool(t, ctx)
	resetPostgresStoreTestDatabase(t, ctx, pool)

	store, err := NewPostgresStore(ctx, pool)
	if err != nil {
		t.Fatalf("create postgres store: %v", err)
	}

	legacyPath := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(legacyPath, []byte(`{
		"printer":{"host":"192.168.0.118","port":5555},
		"weather":{"name":"Минск","latitude":53.9,"longitude":27.5667},
		"finance":{"amountTon":7,"investedUSD":14},
		"receiptContent":{"configured":true,"showWeather":true,"showHistory":true},
		"schedule":{"enabled":true,"mode":"interval","intervalMinutes":30,"timezone":"Europe/Minsk"},
		"scheduleState":{"lastError":"legacy"}
	}`), 0o644); err != nil {
		t.Fatalf("write legacy settings: %v", err)
	}

	if err := store.ImportLegacySettings(ctx, legacyPath); err != nil {
		t.Fatalf("import legacy settings: %v", err)
	}
	if got, err := store.LoadPrinter(); err != nil || got != (printer.Config{Host: "192.168.0.118", Port: 5555}) {
		t.Fatalf("expected imported printer, got %#v, err=%v", got, err)
	}

	updated := printer.Config{Host: "192.168.0.119", Port: 5556}
	if err := store.SavePrinter(updated); err != nil {
		t.Fatalf("save updated printer: %v", err)
	}
	if err := store.ImportLegacySettings(ctx, legacyPath); err != nil {
		t.Fatalf("re-import legacy settings: %v", err)
	}
	if got, err := store.LoadPrinter(); err != nil || got != updated {
		t.Fatalf("legacy import must not overwrite db data, got %#v, err=%v", got, err)
	}
}

func openPostgresStoreTestPool(t *testing.T, ctx context.Context) storage.Pool {
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

func resetPostgresStoreTestDatabase(t *testing.T, ctx context.Context, pool storage.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `TRUNCATE
		receipt_snapshot_summaries,
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
