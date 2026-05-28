package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"atol-server/internal/finance"
	"atol-server/internal/motivation"
	"atol-server/internal/news"
	"atol-server/internal/printer"
	"atol-server/internal/receipt"
	"atol-server/internal/receiptsnapshot"
	"atol-server/internal/schedule"
	"atol-server/internal/storage"
	"atol-server/internal/weather"

	"github.com/jackc/pgx/v5"
)

const (
	defaultWorkspaceSlug = "default"
	defaultWorkspaceName = "Default"
	defaultAdminEmail    = "admin@local.atol"
	defaultAdminName     = "Local Admin"

	settingWeather         = "weather"
	settingFinance         = "finance"
	settingNews            = "news"
	settingMotivation      = "motivation"
	settingReceiptStyle    = "receipt_style"
	settingReceiptContent  = "receipt_content"
	settingReceiptSnapshot = "receipt_snapshot"
	settingSchedule        = "schedule"
)

type PostgresStore struct {
	pool        storage.Pool
	workspaceID string
}

func NewPostgresStore(ctx context.Context, pool storage.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, errors.New("postgres pool is required")
	}
	store := &PostgresStore{pool: pool}
	if err := store.ensureFoundation(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) WorkspaceID() string {
	return s.workspaceID
}

func (s *PostgresStore) LoadPrinter() (printer.Config, error) {
	var data []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT config
		FROM printers
		WHERE workspace_id = $1 AND is_default
		ORDER BY created_at
		LIMIT 1`, s.workspaceID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return printer.DefaultConfig(), nil
	}
	if err != nil {
		return printer.Config{}, err
	}
	var config printer.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return printer.Config{}, fmt.Errorf("decode printer config: %w", err)
	}
	return config.Normalized(), nil
}

func (s *PostgresStore) SavePrinter(config printer.Config) error {
	normalized := config.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	data, err := marshalJSON(normalized)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO printers (workspace_id, name, config, is_default)
		VALUES ($1, 'default', $2::jsonb, true)
		ON CONFLICT (workspace_id, name)
		DO UPDATE SET config = EXCLUDED.config, is_default = true, updated_at = now()`,
		s.workspaceID, data)
	if err != nil {
		return err
	}
	return s.audit("settings.save", "printer", normalized)
}

func (s *PostgresStore) LoadWeather() (weather.Location, error) {
	value, err := loadSetting(s, settingWeather, weather.DefaultLocation())
	if err != nil {
		return weather.Location{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveWeather(location weather.Location) error {
	normalized := location.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	return s.saveSetting(settingWeather, normalized)
}

func (s *PostgresStore) LoadFinance() (finance.TonPortfolio, error) {
	value, err := loadSetting(s, settingFinance, finance.DefaultTonPortfolio())
	if err != nil {
		return finance.TonPortfolio{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveFinance(portfolio finance.TonPortfolio) error {
	normalized := portfolio.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	return s.saveSetting(settingFinance, normalized)
}

func (s *PostgresStore) LoadNews() (news.Settings, error) {
	value, err := loadSetting(s, settingNews, news.DefaultSettings())
	if err != nil {
		return news.Settings{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveNews(newsSettings news.Settings) error {
	if err := newsSettings.Validate(); err != nil {
		return err
	}
	normalized := newsSettings.Normalized()
	for _, source := range normalized.Sources {
		if err := source.Validate(); err != nil {
			return err
		}
	}
	return s.saveSetting(settingNews, normalized)
}

func (s *PostgresStore) LoadMotivation() (motivation.Settings, error) {
	value, err := loadSetting(s, settingMotivation, motivation.DefaultSettings())
	if err != nil {
		return motivation.Settings{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveMotivation(motivationSettings motivation.Settings) error {
	normalized := motivationSettings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	return s.saveSetting(settingMotivation, normalized)
}

func (s *PostgresStore) LoadReceiptStyle() (receipt.StyleSettings, error) {
	value, err := loadSetting(s, settingReceiptStyle, receipt.DefaultStyleSettings())
	if err != nil {
		return receipt.StyleSettings{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveReceiptStyle(style receipt.StyleSettings) error {
	return s.saveSetting(settingReceiptStyle, style.Normalized())
}

func (s *PostgresStore) LoadReceiptContent() (receipt.ContentSettings, error) {
	value, err := loadSetting(s, settingReceiptContent, receipt.DefaultContentSettings())
	if err != nil {
		return receipt.ContentSettings{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveReceiptContent(content receipt.ContentSettings) error {
	return s.saveSetting(settingReceiptContent, content.Normalized())
}

func (s *PostgresStore) LoadReceiptSnapshotSettings() (receiptsnapshot.Settings, error) {
	value, err := loadSetting(s, settingReceiptSnapshot, receiptsnapshot.DefaultSettings())
	if err != nil {
		return receiptsnapshot.Settings{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveReceiptSnapshotSettings(settings receiptsnapshot.Settings) error {
	normalized := settings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	return s.saveSetting(settingReceiptSnapshot, normalized)
}

func (s *PostgresStore) LoadSchedule() (schedule.Settings, error) {
	value, err := loadSetting(s, settingSchedule, schedule.DefaultSettings())
	if err != nil {
		return schedule.Settings{}, err
	}
	return value.Normalized(), nil
}

func (s *PostgresStore) SaveSchedule(scheduleSettings schedule.Settings) error {
	normalized := scheduleSettings.Normalized()
	if err := normalized.Validate(); err != nil {
		return err
	}
	return s.saveSetting(settingSchedule, normalized)
}

func (s *PostgresStore) LoadScheduleState() (schedule.State, error) {
	var data []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT value
		FROM scheduler_state
		WHERE workspace_id = $1`, s.workspaceID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return schedule.State{}, nil
	}
	if err != nil {
		return schedule.State{}, err
	}
	var state schedule.State
	if err := json.Unmarshal(data, &state); err != nil {
		return schedule.State{}, fmt.Errorf("decode schedule state: %w", err)
	}
	return state, nil
}

func (s *PostgresStore) SaveScheduleState(state schedule.State) error {
	data, err := marshalJSON(state)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO scheduler_state (workspace_id, value)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (workspace_id)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		s.workspaceID, data)
	if err != nil {
		return err
	}
	return s.audit("settings.save", "scheduler_state", state)
}

func (s *PostgresStore) StartPrintJob(kind string, request any) (string, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "unknown"
	}
	data, err := marshalJSON(request)
	if err != nil {
		return "", err
	}
	printerID := ""
	_ = s.pool.QueryRow(context.Background(), `
		SELECT id::text
		FROM printers
		WHERE workspace_id = $1 AND is_default
		ORDER BY created_at
		LIMIT 1`, s.workspaceID).Scan(&printerID)
	var id string
	if err := s.pool.QueryRow(context.Background(), `
		INSERT INTO print_jobs (workspace_id, printer_id, kind, status, request, started_at)
		VALUES ($1, NULLIF($2, '')::uuid, $3, 'running', $4::jsonb, now())
		RETURNING id::text`, s.workspaceID, printerID, kind, data).Scan(&id); err != nil {
		return "", err
	}
	if err := s.audit("print.start", kind, map[string]any{"jobId": id}); err != nil {
		return "", err
	}
	return id, nil
}

func (s *PostgresStore) FinishPrintJob(id string, printErr error) error {
	status := "succeeded"
	message := ""
	if printErr != nil {
		status = "failed"
		message = printErr.Error()
	}
	_, err := s.pool.Exec(context.Background(), `
		UPDATE print_jobs
		SET status = $2, error = $3, finished_at = now()
		WHERE id = $1::uuid`, id, status, message)
	if err != nil {
		return err
	}
	return s.audit("print."+status, "print_job", map[string]any{"jobId": id, "error": message})
}

func (s *PostgresStore) ImportLegacySettings(ctx context.Context, path string) error {
	imported, err := s.legacyImported(ctx, "settings-json")
	if err != nil || imported {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return s.markLegacyImported(ctx, "settings-json")
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.markLegacyImported(ctx, "settings-json")
		}
		return err
	}
	empty, err := s.settingsStateEmpty(ctx)
	if err != nil {
		return err
	}
	if !empty {
		return s.markLegacyImported(ctx, "settings-json")
	}

	legacy, err := NewStore(path).load()
	if err != nil {
		return err
	}
	if err := s.SavePrinter(legacy.Printer); err != nil {
		return err
	}
	if err := s.SaveWeather(legacy.Weather); err != nil {
		return err
	}
	if err := s.SaveFinance(legacy.Finance); err != nil {
		return err
	}
	if err := s.SaveNews(legacy.News); err != nil {
		return err
	}
	if err := s.SaveMotivation(legacy.Motivation); err != nil {
		return err
	}
	if err := s.SaveReceiptStyle(legacy.Receipt); err != nil {
		return err
	}
	if err := s.SaveReceiptContent(legacy.ReceiptContent); err != nil {
		return err
	}
	if err := s.SaveSchedule(legacy.Schedule); err != nil {
		return err
	}
	if err := s.SaveScheduleState(legacy.ScheduleState); err != nil {
		return err
	}
	if err := s.audit("legacy.import", "settings-json", map[string]any{"path": path}); err != nil {
		return err
	}
	return s.markLegacyImported(ctx, "settings-json")
}

func (s *PostgresStore) ensureFoundation(ctx context.Context) error {
	var workspaceID string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO workspaces (slug, name)
		VALUES ($1, $2)
		ON CONFLICT (slug)
		DO UPDATE SET name = EXCLUDED.name, updated_at = now()
		RETURNING id::text`, defaultWorkspaceSlug, defaultWorkspaceName).Scan(&workspaceID); err != nil {
		return fmt.Errorf("ensure default workspace: %w", err)
	}
	var userID string
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name)
		VALUES ($1, $2)
		ON CONFLICT (email)
		DO UPDATE SET display_name = EXCLUDED.display_name, updated_at = now()
		RETURNING id::text`, defaultAdminEmail, defaultAdminName).Scan(&userID); err != nil {
		return fmt.Errorf("ensure default admin user: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO workspace_memberships (workspace_id, user_id, role)
		VALUES ($1, $2, 'admin')
		ON CONFLICT (workspace_id, user_id)
		DO UPDATE SET role = EXCLUDED.role`, workspaceID, userID); err != nil {
		return fmt.Errorf("ensure default admin membership: %w", err)
	}
	s.workspaceID = workspaceID
	return nil
}

func loadSetting[T any](s *PostgresStore, key string, fallback T) (T, error) {
	var data []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT value
		FROM workspace_settings
		WHERE workspace_id = $1 AND key = $2`, s.workspaceID, key).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		var zero T
		return zero, err
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		var zero T
		return zero, fmt.Errorf("decode %s settings: %w", key, err)
	}
	return value, nil
}

func (s *PostgresStore) saveSetting(key string, value any) error {
	data, err := marshalJSON(value)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(context.Background(), `
		INSERT INTO workspace_settings (workspace_id, key, value)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (workspace_id, key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		s.workspaceID, key, data); err != nil {
		return err
	}
	return s.audit("settings.save", key, value)
}

func (s *PostgresStore) settingsStateEmpty(ctx context.Context) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM printers WHERE workspace_id = $1) +
			(SELECT count(*) FROM workspace_settings WHERE workspace_id = $1) +
			(SELECT count(*) FROM scheduler_state WHERE workspace_id = $1)`,
		s.workspaceID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
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

func (s *PostgresStore) audit(action string, subject string, metadata any) error {
	data, err := marshalJSON(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(context.Background(), `
		INSERT INTO audit_events (workspace_id, action, subject, metadata)
		VALUES ($1, $2, $3, $4::jsonb)`, s.workspaceID, action, subject, data)
	return err
}

func marshalJSON(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
