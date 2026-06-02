package telegramfax

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"atol-server/internal/storage"

	"github.com/jackc/pgx/v5"
)

type QueueStatus string

const (
	QueueStatusPending QueueStatus = "pending"
	QueueStatusPrinted QueueStatus = "printed"
	QueueStatusFailed  QueueStatus = "failed"

	maxQueueAttempts = 20
)

type QueueItem struct {
	ID              string
	DedupeKey       string
	Source          string
	ContentType     string
	Status          QueueStatus
	Message         Message
	BusinessOwnerID int64
	Photo           PhotoSize
	Attempts        int
	LastError       string
	NextAttemptAt   time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	PrintedAt       time.Time
}

type QueueStore interface {
	Enqueue(context.Context, QueueItem) (QueueItem, bool, error)
	NextPending(context.Context, time.Time) (QueueItem, bool, error)
	MarkPrinted(context.Context, string) error
	MarkFailed(context.Context, string, error, time.Time) error
}

type queuePayload struct {
	Message         Message   `json:"message"`
	BusinessOwnerID int64     `json:"businessOwnerId,omitempty"`
	Photo           PhotoSize `json:"photo,omitempty"`
}

type memoryQueueStore struct {
	mu     sync.Mutex
	clock  func() time.Time
	nextID int
	items  []QueueItem
}

func newMemoryQueueStore(clock func() time.Time) *memoryQueueStore {
	if clock == nil {
		clock = time.Now
	}
	return &memoryQueueStore{clock: clock}
}

func (s *memoryQueueStore) Enqueue(_ context.Context, item QueueItem) (QueueItem, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.items {
		if existing.DedupeKey == item.DedupeKey {
			return existing, false, nil
		}
	}
	s.nextID++
	now := s.clock()
	item.ID = fmt.Sprintf("memory-%d", s.nextID)
	item.Status = QueueStatusPending
	item.CreatedAt = now
	item.UpdatedAt = now
	s.items = append(s.items, item)
	return item, true, nil
}

func (s *memoryQueueStore) NextPending(_ context.Context, now time.Time) (QueueItem, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.Status == QueueStatusPending && !item.NextAttemptAt.After(now) {
			return item, true, nil
		}
	}
	return QueueItem{}, false, nil
}

func (s *memoryQueueStore) MarkPrinted(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Status = QueueStatusPrinted
			s.items[i].UpdatedAt = now
			s.items[i].PrintedAt = now
			return nil
		}
	}
	return errors.New("telegram fax queue item not found")
}

func (s *memoryQueueStore) MarkFailed(_ context.Context, id string, itemErr error, nextAttemptAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Attempts++
			if s.items[i].Attempts >= maxQueueAttempts {
				s.items[i].Status = QueueStatusFailed
			} else {
				s.items[i].Status = QueueStatusPending
			}
			s.items[i].LastError = errorString(itemErr)
			s.items[i].NextAttemptAt = nextAttemptAt
			s.items[i].UpdatedAt = now
			return nil
		}
	}
	return errors.New("telegram fax queue item not found")
}

type PostgresQueueStore struct {
	pool        storage.Pool
	workspaceID string
}

func NewPostgresQueueStore(pool storage.Pool, workspaceID string) *PostgresQueueStore {
	return &PostgresQueueStore{pool: pool, workspaceID: workspaceID}
}

func (s *PostgresQueueStore) Enqueue(ctx context.Context, item QueueItem) (QueueItem, bool, error) {
	payload, err := marshalQueuePayload(item)
	if err != nil {
		return QueueItem{}, false, err
	}
	var row queueRow
	err = s.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO telegram_fax_queue (workspace_id, dedupe_key, source, content_type, status, payload, next_attempt_at)
			VALUES ($1, $2, $3, $4, 'pending', $5::jsonb, now())
			ON CONFLICT (workspace_id, dedupe_key) DO NOTHING
			RETURNING id::text, dedupe_key, source, content_type, status, payload, attempts, last_error, next_attempt_at, created_at, updated_at, printed_at, true AS inserted
		)
		SELECT id, dedupe_key, source, content_type, status, payload, attempts, last_error, next_attempt_at, created_at, updated_at, printed_at, inserted
		FROM inserted
		UNION ALL
		SELECT id::text, dedupe_key, source, content_type, status, payload, attempts, last_error, next_attempt_at, created_at, updated_at, printed_at, false AS inserted
		FROM telegram_fax_queue
		WHERE workspace_id = $1 AND dedupe_key = $2
		LIMIT 1`,
		s.workspaceID, item.DedupeKey, item.Source, item.ContentType, payload,
	).Scan(&row.ID, &row.DedupeKey, &row.Source, &row.ContentType, &row.Status, &row.Payload, &row.Attempts, &row.LastError, &row.NextAttemptAt, &row.CreatedAt, &row.UpdatedAt, &row.PrintedAt, &row.Inserted)
	if err != nil {
		return QueueItem{}, false, err
	}
	queueItem, err := row.item()
	if err != nil {
		return QueueItem{}, false, err
	}
	return queueItem, row.Inserted, nil
}

func (s *PostgresQueueStore) NextPending(ctx context.Context, now time.Time) (QueueItem, bool, error) {
	var row queueRow
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, dedupe_key, source, content_type, status, payload, attempts, last_error, next_attempt_at, created_at, updated_at, printed_at, false AS inserted
		FROM telegram_fax_queue
		WHERE workspace_id = $1
			AND status = 'pending'
			AND next_attempt_at <= $2
		ORDER BY created_at ASC, id ASC
		LIMIT 1`, s.workspaceID, now).
		Scan(&row.ID, &row.DedupeKey, &row.Source, &row.ContentType, &row.Status, &row.Payload, &row.Attempts, &row.LastError, &row.NextAttemptAt, &row.CreatedAt, &row.UpdatedAt, &row.PrintedAt, &row.Inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return QueueItem{}, false, nil
	}
	if err != nil {
		return QueueItem{}, false, err
	}
	item, err := row.item()
	if err != nil {
		return QueueItem{}, false, err
	}
	return item, true, nil
}

func (s *PostgresQueueStore) MarkPrinted(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE telegram_fax_queue
		SET status = 'printed', last_error = '', updated_at = now(), printed_at = now()
		WHERE workspace_id = $1 AND id = $2::uuid`, s.workspaceID, id)
	return err
}

func (s *PostgresQueueStore) MarkFailed(ctx context.Context, id string, itemErr error, nextAttemptAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE telegram_fax_queue
		SET status = CASE WHEN attempts + 1 >= 20 THEN 'failed' ELSE 'pending' END,
			attempts = attempts + 1,
			last_error = $3,
			next_attempt_at = $4,
			updated_at = now()
		WHERE workspace_id = $1 AND id = $2::uuid`, s.workspaceID, id, errorString(itemErr), nextAttemptAt)
	return err
}

type queueRow struct {
	ID            string
	DedupeKey     string
	Source        string
	ContentType   string
	Status        string
	Payload       []byte
	Attempts      int
	LastError     string
	NextAttemptAt time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PrintedAt     sql.NullTime
	Inserted      bool
}

func (r queueRow) item() (QueueItem, error) {
	var payload queuePayload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return QueueItem{}, fmt.Errorf("decode telegram fax queue payload: %w", err)
	}
	item := QueueItem{
		ID:              r.ID,
		DedupeKey:       r.DedupeKey,
		Source:          r.Source,
		ContentType:     r.ContentType,
		Status:          QueueStatus(r.Status),
		Message:         payload.Message,
		BusinessOwnerID: payload.BusinessOwnerID,
		Photo:           payload.Photo,
		Attempts:        r.Attempts,
		LastError:       r.LastError,
		NextAttemptAt:   r.NextAttemptAt,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
	if r.PrintedAt.Valid {
		item.PrintedAt = r.PrintedAt.Time
	}
	return item, nil
}

func marshalQueuePayload(item QueueItem) (string, error) {
	data, err := json.Marshal(queuePayload{
		Message:         item.Message,
		BusinessOwnerID: item.BusinessOwnerID,
		Photo:           item.Photo,
	})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
