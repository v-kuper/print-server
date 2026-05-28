package receiptsnapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"atol-server/internal/storage"

	"github.com/jackc/pgx/v5"
)

type PostgresStore struct {
	pool        storage.Pool
	workspaceID string
}

func NewPostgresStore(pool storage.Pool, workspaceID string) *PostgresStore {
	return &PostgresStore{pool: pool, workspaceID: strings.TrimSpace(workspaceID)}
}

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Snapshot, error) {
	newsData, err := json.Marshal(input.NewsItems)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode receipt snapshot news: %w", err)
	}
	lineData, err := json.Marshal(input.ReceiptLines)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode receipt snapshot lines: %w", err)
	}
	paperChars := normalizePaperChars(input.PaperChars)
	var snapshot Snapshot
	var rawItems []byte
	var rawLines []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO receipt_snapshots (workspace_id, status, news_items, receipt_lines, paper_chars)
		VALUES ($1::uuid, $2, $3::jsonb, $4::jsonb, $5)
		RETURNING id::text, workspace_id::text, status, news_items, receipt_lines, paper_chars, error, created_at, published_at, failed_at`,
		s.workspaceID, StatusPending, string(newsData), string(lineData), paperChars,
	).Scan(&snapshot.ID, &snapshot.WorkspaceID, &snapshot.Status, &rawItems, &rawLines, &snapshot.PaperChars, &snapshot.Error, &snapshot.CreatedAt, &snapshot.PublishedAt, &snapshot.FailedAt)
	if err != nil {
		return Snapshot{}, err
	}
	if err := json.Unmarshal(rawItems, &snapshot.NewsItems); err != nil {
		return Snapshot{}, fmt.Errorf("decode receipt snapshot news: %w", err)
	}
	if err := json.Unmarshal(rawLines, &snapshot.ReceiptLines); err != nil {
		return Snapshot{}, fmt.Errorf("decode receipt snapshot lines: %w", err)
	}
	return snapshot, nil
}

func (s *PostgresStore) Load(ctx context.Context, id string) (Snapshot, error) {
	snapshot, err := s.querySnapshot(ctx, `
		SELECT id::text, workspace_id::text, status, news_items, receipt_lines, paper_chars, error, created_at, published_at, failed_at
		FROM receipt_snapshots
		WHERE workspace_id = $1::uuid AND id = $2::uuid`, s.workspaceID, strings.TrimSpace(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, ErrNotFound
	}
	return snapshot, err
}

func (s *PostgresStore) FinalizeReceiptLines(ctx context.Context, id string, lines []ReceiptLine, paperChars int) error {
	lineData, err := json.Marshal(lines)
	if err != nil {
		return fmt.Errorf("encode receipt snapshot lines: %w", err)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE receipt_snapshots
		SET receipt_lines = $3::jsonb, paper_chars = $4
		WHERE workspace_id = $1::uuid AND id = $2::uuid`,
		s.workspaceID, strings.TrimSpace(id), string(lineData), normalizePaperChars(paperChars))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) Publish(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE receipt_snapshots
		SET status = $3, published_at = now()
		WHERE workspace_id = $1::uuid AND id = $2::uuid`,
		s.workspaceID, strings.TrimSpace(id), StatusPublished)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) Fail(ctx context.Context, id string, failure error) error {
	message := ""
	if failure != nil {
		message = failure.Error()
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE receipt_snapshots
		SET status = $3, error = $4, failed_at = now()
		WHERE workspace_id = $1::uuid AND id = $2::uuid`,
		s.workspaceID, strings.TrimSpace(id), StatusFailed, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) querySnapshot(ctx context.Context, query string, args ...any) (Snapshot, error) {
	var snapshot Snapshot
	var rawItems []byte
	var rawLines []byte
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&snapshot.ID,
		&snapshot.WorkspaceID,
		&snapshot.Status,
		&rawItems,
		&rawLines,
		&snapshot.PaperChars,
		&snapshot.Error,
		&snapshot.CreatedAt,
		&snapshot.PublishedAt,
		&snapshot.FailedAt,
	)
	if err != nil {
		return Snapshot{}, err
	}
	if err := json.Unmarshal(rawItems, &snapshot.NewsItems); err != nil {
		return Snapshot{}, fmt.Errorf("decode receipt snapshot news: %w", err)
	}
	if err := json.Unmarshal(rawLines, &snapshot.ReceiptLines); err != nil {
		return Snapshot{}, fmt.Errorf("decode receipt snapshot lines: %w", err)
	}
	snapshot.PaperChars = normalizePaperChars(snapshot.PaperChars)
	return snapshot, nil
}

func normalizePaperChars(value int) int {
	if value <= 0 {
		return 32
	}
	return value
}
