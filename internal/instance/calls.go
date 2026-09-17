package instance

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrUnsupported reports that the upstream protocol does not support the
// operation on this session (for example rejecting a call the companion
// library cannot reject, or setting a profile field it exposes no setter
// for). The handler maps it to 501 with a stable code.
var ErrUnsupported = errors.New("operation not supported")

// RejectCall rejects the active call callID from fromJID through the instance
// session. It is synchronous and direct: no outbox, no retry. The service
// never initiates calls. A disconnected session is ErrNotConnected, a
// malformed caller is ErrInvalidInput and an upstream that cannot reject is
// ErrUnsupported; anything else is returned unchanged for a 500.
func (s *Service) RejectCall(ctx context.Context, id uuid.UUID, fromJID, callID string) error {
	instance, err := s.repo.Get(ctx, id)
	if err != nil {
		return mapError("reject call", err)
	}

	sess, err := s.sessionFor(ctx, instance)
	if err != nil {
		return fmt.Errorf("reject call: create session: %w", err)
	}
	if err := sess.RejectCall(ctx, fromJID, callID); err != nil {
		return mapCallError("reject call", err)
	}
	return nil
}

// mapCallError translates a call session failure into the service sentinel
// the HTTP layer maps to a status code, preserving the operation context for
// the logs. Unknown failures pass through unchanged for a 500 without leaking
// their cause (the handler never echoes them).
func mapCallError(op string, err error) error {
	return mapSessionError(op, err)
}
