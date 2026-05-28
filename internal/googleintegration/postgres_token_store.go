package googleintegration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"atol-server/internal/storage"

	"github.com/jackc/pgx/v5"
)

type PostgresTokenStore struct {
	pool        storage.Pool
	workspaceID string
}

func NewPostgresTokenStore(pool storage.Pool, workspaceID string) *PostgresTokenStore {
	return &PostgresTokenStore{pool: pool, workspaceID: workspaceID}
}

func (s *PostgresTokenStore) LoadToken(ctx context.Context) (Token, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `
		SELECT value
		FROM google_tokens
		WHERE workspace_id = $1`, s.workspaceID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return Token{}, ErrNotAuthorized
	}
	if err != nil {
		return Token{}, err
	}
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return Token{}, fmt.Errorf("decode Google token: %w", err)
	}
	if token.AccessToken == "" && token.RefreshToken == "" {
		return Token{}, ErrNotAuthorized
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return token, nil
}

func (s *PostgresTokenStore) SaveToken(ctx context.Context, token Token) error {
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO google_tokens (workspace_id, value)
		VALUES ($1, $2::jsonb)
		ON CONFLICT (workspace_id)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, s.workspaceID, string(data))
	if err != nil {
		return err
	}
	return s.audit(ctx, "google.token.save", map[string]any{"authorized": true})
}

func (s *PostgresTokenStore) DeleteToken(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM google_tokens WHERE workspace_id = $1`, s.workspaceID); err != nil {
		return err
	}
	return s.audit(ctx, "google.token.delete", map[string]any{})
}

func (s *PostgresTokenStore) ImportLegacyToken(ctx context.Context, path string) error {
	imported, err := s.legacyImported(ctx, "google-token")
	if err != nil || imported {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return s.markLegacyImported(ctx, "google-token")
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM google_tokens WHERE workspace_id = $1`, s.workspaceID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return s.markLegacyImported(ctx, "google-token")
	}
	token, err := loadTokenFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrNotAuthorized) {
			return s.markLegacyImported(ctx, "google-token")
		}
		return err
	}
	if err := s.SaveToken(ctx, token); err != nil {
		return err
	}
	if err := s.audit(ctx, "legacy.import", map[string]any{"source": "google-token", "path": path}); err != nil {
		return err
	}
	return s.markLegacyImported(ctx, "google-token")
}

func loadTokenFile(path string) (Token, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Token{}, err
	}
	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return Token{}, fmt.Errorf("decode Google token: %w", err)
	}
	if token.AccessToken == "" && token.RefreshToken == "" {
		return Token{}, ErrNotAuthorized
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return token, nil
}

func (s *PostgresTokenStore) legacyImported(ctx context.Context, source string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM legacy_imports WHERE source = $1)`, source).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *PostgresTokenStore) markLegacyImported(ctx context.Context, source string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO legacy_imports (source)
		VALUES ($1)
		ON CONFLICT (source) DO NOTHING`, source)
	return err
}

func (s *PostgresTokenStore) audit(ctx context.Context, action string, metadata any) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_events (workspace_id, action, subject, metadata)
		VALUES ($1, $2, 'google_token', $3::jsonb)`, s.workspaceID, action, string(data))
	return err
}
