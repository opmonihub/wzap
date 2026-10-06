package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"wzap/internal/model"
	"wzap/internal/storage"
)

const outboxEventColumns = `id, subject, envelope, attempt_count, ` +
	`COALESCE(last_error_message, '') AS last_error_message, created_at`

// EventOutboxRepository is the pgx-backed storage.EventOutboxRepository. The
// remodeled table holds pending events only: a confirmed publication deletes
// the row instead of stamping published_at.
type EventOutboxRepository struct {
	pool *pgxpool.Pool
}

var _ storage.EventOutboxRepository = (*EventOutboxRepository)(nil)

// NewEventOutboxRepository returns an event outbox repository backed by pool.
func NewEventOutboxRepository(pool *pgxpool.Pool) *EventOutboxRepository {
	return &EventOutboxRepository{pool: pool}
}

// Enqueue appends an event to the outbox.
func (r *EventOutboxRepository) Enqueue(ctx context.Context, id uuid.UUID, subject string, envelope []byte) error {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO event_outbox (id, subject, envelope) VALUES ($1, $2, $3)`,
		id, subject, envelope); err != nil {
		return fmt.Errorf("enqueue event: %w", err)
	}
	return nil
}

// ClaimPending returns up to limit pending events, oldest first. The rows are
// read with FOR UPDATE SKIP LOCKED so the single relay worker never blocks on
// concurrent inserts or a competing claim. Claiming does not change the rows:
// the relay publishes them and calls MarkPublished or MarkAttempt.
func (r *EventOutboxRepository) ClaimPending(ctx context.Context, limit int) ([]model.OutboxEvent, error) {
	if limit <= 0 {
		return []model.OutboxEvent{}, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("claim pending events: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT `+outboxEventColumns+` FROM event_outbox `+
		`ORDER BY created_at, id FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending events: select: %w", err)
	}

	events := []model.OutboxEvent{}
	for rows.Next() {
		var event model.OutboxEvent
		if err := scanOutboxEventRow(rows, &event); err != nil {
			rows.Close()
			return nil, fmt.Errorf("claim pending events: %w", err)
		}
		events = append(events, event)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim pending events: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("claim pending events: commit: %w", err)
	}
	return events, nil
}

// MarkPublished confirms the event was published: the pending row is deleted
// (the table stores pendings only). It returns storage.ErrNotFound when the
// event does not exist.
func (r *EventOutboxRepository) MarkPublished(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM event_outbox WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark event published: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark event published: %w", storage.ErrNotFound)
	}
	return nil
}

// MarkAttempt records a failed publish attempt, keeping the event pending.
func (r *EventOutboxRepository) MarkAttempt(ctx context.Context, id uuid.UUID, errMsg string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE event_outbox
		SET attempt_count = attempt_count + 1,
		    last_error_code = CASE WHEN NULLIF($2, '') IS NULL THEN NULL ELSE 'publish_failed' END,
		    last_error_message = NULLIF($2, ''),
		    last_error_at = CASE WHEN NULLIF($2, '') IS NULL THEN NULL ELSE now() END,
		    updated_at = now()
		WHERE id = $1`, id, errMsg)
	if err != nil {
		return fmt.Errorf("mark event attempt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark event attempt: %w", storage.ErrNotFound)
	}
	return nil
}

// DeletePublishedBefore is a no-op under the pending-only model: published
// events are deleted by MarkPublished, so there is nothing left to sweep.
// The method stays on the interface until the relay rework (task 4.1).
func (r *EventOutboxRepository) DeletePublishedBefore(ctx context.Context, t time.Time) (int64, error) {
	return 0, nil
}

func scanOutboxEventRow(scanner rowScanner, event *model.OutboxEvent) error {
	return scanner.Scan(
		&event.ID, &event.Subject, &event.Envelope, &event.Attempts,
		&event.LastError, &event.CreatedAt,
	)
}
