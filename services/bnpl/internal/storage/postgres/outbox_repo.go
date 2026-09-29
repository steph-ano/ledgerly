package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OutboxEvent struct {
	ID             uuid.UUID       `json:"id"`
	EventType      string          `json:"event_type"`
	AggregateID    uuid.UUID       `json:"aggregate_id"`
	DestinationURL string          `json:"destination_url"`
	Payload        json.RawMessage `json:"payload"`
	Status         string          `json:"status"`
	AttemptCount   int             `json:"attempt_count"`
	NextRetryAt    *time.Time      `json:"next_retry_at,omitempty"`
	PublishedAt    *time.Time      `json:"published_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type QueryExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// SaveEvent inserts an outbox event. Accepts a transaction or db pool.
func (r *OutboxRepository) SaveEvent(
	ctx context.Context,
	exec QueryExecutor,
	eventType string,
	aggregateID uuid.UUID,
	destURL string,
	payload any,
) error {
	if destURL == "" {
		return nil // No webhook destination registered
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal outbox payload: %w", err)
	}

	query := `
		INSERT INTO outbox_events (id, event_type, aggregate_id, destination_url, payload, status, attempt_count, created_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', 0, NOW())
	`
	if exec == nil {
		exec = r.db
	}

	_, err = exec.ExecContext(ctx, query,
		uuid.New(),
		eventType,
		aggregateID,
		destURL,
		payloadBytes,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}
	return nil
}

// LeasePendingEvents retrieves and locks pending outbox events using SKIP LOCKED.
func (r *OutboxRepository) LeasePendingEvents(ctx context.Context, limit int, now time.Time) ([]OutboxEvent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin outbox lease transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT id, event_type, aggregate_id, destination_url, payload, status, attempt_count, next_retry_at, published_at, created_at
		FROM outbox_events
		WHERE status = 'pending'
		  AND (next_retry_at IS NULL OR next_retry_at <= $1)
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED;
	`
	rows, err := tx.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to select pending outbox events: %w", err)
	}
	defer rows.Close()

	var events []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(
			&e.ID,
			&e.EventType,
			&e.AggregateID,
			&e.DestinationURL,
			&e.Payload,
			&e.Status,
			&e.AttemptCount,
			&e.NextRetryAt,
			&e.PublishedAt,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan outbox event: %w", err)
		}
		events = append(events, e)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit outbox read: %w", err)
	}

	return events, nil
}

// MarkDelivered marks an outbox event as delivered.
func (r *OutboxRepository) MarkDelivered(ctx context.Context, eventID uuid.UUID, now time.Time) error {
	query := `
		UPDATE outbox_events
		SET status = 'delivered', published_at = $1
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, now, eventID)
	return err
}

// MarkFailed records a failed attempt and schedules a retry or marks failed if attempts >= 5.
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID uuid.UUID, now time.Time, attempts int) error {
	status := "pending"
	var nextRetry *time.Time

	if attempts >= 5 {
		status = "failed"
	} else {
		// Exponential backoff: 1m, 2m, 4m, 8m, 16m
		backoff := time.Duration(1<<attempts) * time.Minute
		t := now.Add(backoff)
		nextRetry = &t
	}

	query := `
		UPDATE outbox_events
		SET status = $1, attempt_count = $2, next_retry_at = $3
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, status, attempts, nextRetry, eventID)
	return err
}
