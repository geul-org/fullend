package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrNotInitialized = errors.New("queue: not initialized, call Init first")
	ErrUnknownBackend = errors.New("queue: unknown backend")
)

// publishConfig holds options for a single Publish call.
type publishConfig struct {
	delay    int    // seconds
	priority string // "high", "normal", "low"
}

// PublishOption configures a Publish call.
type PublishOption func(*publishConfig)

// WithDelay sets the delivery delay in seconds.
func WithDelay(seconds int) PublishOption {
	return func(c *publishConfig) { c.delay = seconds }
}

// WithPriority sets the message priority ("high", "normal", "low").
func WithPriority(p string) PublishOption {
	return func(c *publishConfig) { c.priority = p }
}

// singleton state
var (
	mu       sync.RWMutex
	handlers map[string][]func(ctx context.Context, msg []byte) error
	backend  string
	db       *sql.DB
	cancel   context.CancelFunc
	done     chan struct{}
	inited   bool
)

// Init initializes the queue with the given backend ("postgres" or "memory").
// For "postgres", db must be non-nil; the fullend_queue table is auto-created.
func Init(ctx context.Context, b string, d *sql.DB) error {
	mu.Lock()
	defer mu.Unlock()

	switch b {
	case "postgres":
		_, err := d.ExecContext(ctx, `
			CREATE TABLE IF NOT EXISTS fullend_queue (
				id           BIGSERIAL PRIMARY KEY,
				topic        TEXT NOT NULL,
				payload      JSONB NOT NULL,
				priority     TEXT NOT NULL DEFAULT 'normal',
				status       TEXT NOT NULL DEFAULT 'pending',
				created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deliver_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				processed_at TIMESTAMPTZ
			)`)
		if err != nil {
			return err
		}
		_, err = d.ExecContext(ctx, `
			CREATE INDEX IF NOT EXISTS idx_fullend_queue_pending
			ON fullend_queue (topic, status, deliver_at) WHERE status = 'pending'`)
		if err != nil {
			return err
		}
		db = d
	case "memory":
		// no setup needed
	default:
		return fmt.Errorf("%w: %s", ErrUnknownBackend, b)
	}

	backend = b
	handlers = make(map[string][]func(ctx context.Context, msg []byte) error)
	inited = true
	return nil
}

// Publish sends a message to the given topic.
func Publish(ctx context.Context, topic string, payload any, opts ...PublishOption) error {
	mu.RLock()
	if !inited {
		mu.RUnlock()
		return ErrNotInitialized
	}
	b := backend
	mu.RUnlock()

	cfg := publishConfig{priority: "normal"}
	for _, o := range opts {
		o(&cfg)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	switch b {
	case "postgres":
		deliverAt := time.Now()
		if cfg.delay > 0 {
			deliverAt = deliverAt.Add(time.Duration(cfg.delay) * time.Second)
		}
		_, err := db.ExecContext(ctx, `
			INSERT INTO fullend_queue (topic, payload, priority, deliver_at)
			VALUES ($1, $2, $3, $4)`,
			topic, data, cfg.priority, deliverAt)
		return err

	case "memory":
		mu.RLock()
		hs := handlers[topic]
		mu.RUnlock()
		for _, h := range hs {
			if err := h(ctx, data); err != nil {
				return err
			}
		}
		return nil
	}

	return nil
}

// Subscribe registers a handler for the given topic.
func Subscribe(topic string, handler func(ctx context.Context, msg []byte) error) {
	mu.Lock()
	defer mu.Unlock()
	handlers[topic] = append(handlers[topic], handler)
}

// Start begins processing queued messages. It blocks until the context is
// cancelled. For the memory backend this is a no-op that blocks until cancel.
func Start(ctx context.Context) error {
	mu.RLock()
	b := backend
	mu.RUnlock()

	innerCtx, c := context.WithCancel(ctx)
	mu.Lock()
	cancel = c
	done = make(chan struct{})
	mu.Unlock()

	defer close(done)

	if b == "memory" {
		<-innerCtx.Done()
		return nil
	}

	// postgres polling loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-innerCtx.Done():
			return nil
		case <-ticker.C:
			if err := pollOnce(innerCtx); err != nil {
				// log and continue; don't crash the loop
				_ = err
			}
		}
	}
}

// pollOnce processes one batch of pending messages from the database.
func pollOnce(ctx context.Context) error {
	mu.RLock()
	hs := handlers
	mu.RUnlock()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, topic, payload FROM fullend_queue
		WHERE status = 'pending' AND deliver_at <= NOW()
		ORDER BY
			CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,
			id
		FOR UPDATE SKIP LOCKED
		LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var topic string
		var payload []byte
		if err := rows.Scan(&id, &topic, &payload); err != nil {
			return err
		}

		status := "done"
		if topicHandlers, ok := hs[topic]; ok {
			for _, h := range topicHandlers {
				if err := h(ctx, payload); err != nil {
					status = "failed"
					break
				}
			}
		}

		_, err := tx.ExecContext(ctx, `
			UPDATE fullend_queue SET status = $1, processed_at = NOW() WHERE id = $2`,
			status, id)
		if err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	return tx.Commit()
}

// Close stops the polling loop and waits for it to finish.
func Close() error {
	mu.Lock()
	c := cancel
	d := done
	mu.Unlock()

	if c != nil {
		c()
	}
	if d != nil {
		<-d
	}

	mu.Lock()
	inited = false
	handlers = nil
	backend = ""
	db = nil
	cancel = nil
	done = nil
	mu.Unlock()

	return nil
}
