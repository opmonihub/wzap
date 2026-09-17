package instance

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"wzap/internal/session"
)

// ErrStatusNotFound reports that the requested own status does not exist: it
// was never published here or it already expired upstream. The handler maps
// it to 404.
var ErrStatusNotFound = errors.New("status not found")

// PublishStatus publishes an own status (story) through the instance session
// and returns its upstream id. It is synchronous and direct: no outbox, no
// retry — the REST boundary answers 202 with the id and the same idempotency
// semantics as the message sends. A disconnected session is ErrNotConnected
// and a malformed input is ErrInvalidInput; anything else is returned
// unchanged for a 500.
func (s *Service) PublishStatus(ctx context.Context, id uuid.UUID, input session.StatusInput) (string, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", mapError("publish status", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return "", fmt.Errorf("publish status: create session: %w", err)
	}
	statusID, err := sess.PublishStatus(ctx, input)
	if err != nil {
		return "", mapStatusError("publish status", err)
	}
	return statusID, nil
}

// ListStatuses returns the own statuses published through the instance
// session. Error mapping follows PublishStatus.
func (s *Service) ListStatuses(ctx context.Context, id uuid.UUID) ([]session.StatusInfo, error) {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, mapError("list statuses", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("list statuses: create session: %w", err)
	}
	statuses, err := sess.ListStatuses(ctx)
	if err != nil {
		return nil, mapStatusError("list statuses", err)
	}
	return statuses, nil
}

// DeleteStatus removes the own statusID published through the instance
// session. An unknown id is ErrStatusNotFound. Other error mapping follows
// PublishStatus.
func (s *Service) DeleteStatus(ctx context.Context, id uuid.UUID, statusID string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("delete status", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("delete status: create session: %w", err)
	}
	if err := sess.DeleteStatus(ctx, statusID); err != nil {
		return mapStatusError("delete status", err)
	}
	return nil
}

// mapStatusError translates a status session call failure into the service
// sentinel the HTTP layer maps to a status code, preserving the operation
// context for the logs. Unknown failures pass through unchanged for a 500
// without leaking their cause (the handler never echoes them).
func mapStatusError(op string, err error) error {
	switch {
	case errors.Is(err, session.ErrStatusNotFound):
		return fmt.Errorf("%s: %w", op, ErrStatusNotFound)
	default:
		return mapSessionError(op, err)
	}
}
