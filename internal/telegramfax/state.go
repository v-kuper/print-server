package telegramfax

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"atol-server/internal/storage"

	"github.com/jackc/pgx/v5"
)

const stateSettingKey = "telegram_fax_state"

type State struct {
	NextUpdateOffset int64 `json:"nextUpdateOffset"`
}

type StateStore interface {
	Load(context.Context) (State, error)
	Save(context.Context, State) error
}

type PostgresStateStore struct {
	pool        storage.Pool
	workspaceID string
}

func NewPostgresStateStore(pool storage.Pool, workspaceID string) *PostgresStateStore {
	return &PostgresStateStore{pool: pool, workspaceID: workspaceID}
}

func (s *PostgresStateStore) Load(ctx context.Context) (State, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `
		SELECT value
		FROM workspace_settings
		WHERE workspace_id = $1 AND key = $2`, s.workspaceID, stateSettingKey).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode telegram fax state: %w", err)
	}
	return state, nil
}

func (s *PostgresStateStore) Save(ctx context.Context, state State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO workspace_settings (workspace_id, key, value)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (workspace_id, key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		s.workspaceID, stateSettingKey, data)
	return err
}
