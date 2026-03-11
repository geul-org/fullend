package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"
)

// SessionModel provides key-value + TTL storage for user-bound state (login, cart, etc.).
type SessionModel interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// --- PostgreSQL implementation ---

type postgresSession struct {
	db *sql.DB
}

// NewPostgresSession creates a SessionModel backed by PostgreSQL.
// It auto-creates the fullend_sessions table if not exists.
func NewPostgresSession(ctx context.Context, db *sql.DB) (SessionModel, error) {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS fullend_sessions (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		)`)
	if err != nil {
		return nil, err
	}
	return &postgresSession{db: db}, nil
}

func (s *postgresSession) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(ttl)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO fullend_sessions (key, value, expires_at) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = $2, expires_at = $3`,
		key, string(data), expiresAt)
	return err
}

func (s *postgresSession) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
		SELECT value FROM fullend_sessions WHERE key = $1 AND expires_at > NOW()`,
		key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *postgresSession) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM fullend_sessions WHERE key = $1`, key)
	return err
}

// --- Memory implementation ---

type memoryEntry struct {
	value     string
	expiresAt time.Time
}

type memorySession struct {
	mu    sync.RWMutex
	store map[string]memoryEntry
}

// NewMemorySession creates an in-memory SessionModel. Data is lost on restart.
func NewMemorySession() SessionModel {
	return &memorySession{store: make(map[string]memoryEntry)}
}

func (s *memorySession) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.store[key] = memoryEntry{value: string(data), expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	return nil
}

func (s *memorySession) Get(_ context.Context, key string) (string, error) {
	s.mu.RLock()
	entry, ok := s.store[key]
	s.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return "", nil
	}
	return entry.value, nil
}

func (s *memorySession) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.store, key)
	s.mu.Unlock()
	return nil
}
