package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"
)

// CacheModel provides key-value + TTL storage for data efficiency (caching).
type CacheModel interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// --- PostgreSQL implementation ---

type postgresCache struct {
	db *sql.DB
}

// NewPostgresCache creates a CacheModel backed by PostgreSQL.
// It auto-creates the fullend_cache table if not exists.
func NewPostgresCache(ctx context.Context, db *sql.DB) (CacheModel, error) {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS fullend_cache (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		)`)
	if err != nil {
		return nil, err
	}
	return &postgresCache{db: db}, nil
}

func (c *postgresCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(ttl)
	_, err = c.db.ExecContext(ctx, `
		INSERT INTO fullend_cache (key, value, expires_at) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = $2, expires_at = $3`,
		key, string(data), expiresAt)
	return err
}

func (c *postgresCache) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := c.db.QueryRowContext(ctx, `
		SELECT value FROM fullend_cache WHERE key = $1 AND expires_at > NOW()`,
		key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (c *postgresCache) Delete(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, `DELETE FROM fullend_cache WHERE key = $1`, key)
	return err
}

// --- Memory implementation ---

type memoryEntry struct {
	value     string
	expiresAt time.Time
}

type memoryCache struct {
	mu    sync.RWMutex
	store map[string]memoryEntry
}

// NewMemoryCache creates an in-memory CacheModel. Data is lost on restart.
func NewMemoryCache() CacheModel {
	return &memoryCache{store: make(map[string]memoryEntry)}
}

func (c *memoryCache) Set(_ context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.store[key] = memoryEntry{value: string(data), expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
	return nil
}

func (c *memoryCache) Get(_ context.Context, key string) (string, error) {
	c.mu.RLock()
	entry, ok := c.store[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return "", nil
	}
	return entry.value, nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.store, key)
	c.mu.Unlock()
	return nil
}
