package whatsmeow

import (
	"context"
	"errors"

	"wzap/internal/session"
)

// privateSessionError keeps the upstream cause available for errors.Is/As
// without allowing its device identifiers into logs or connection reasons.
type privateSessionError struct {
	message string
	cause   error
}

func (e *privateSessionError) Error() string { return e.message }
func (e *privateSessionError) Unwrap() error { return e.cause }

func safeSessionError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	detail := "failed"
	for _, category := range []error{
		context.Canceled, context.DeadlineExceeded, session.ErrDeviceJIDTaken,
		session.ErrNoDevice, session.ErrNotConnected, session.ErrTransient,
	} {
		if errors.Is(cause, category) {
			detail = category.Error()
			break
		}
	}
	return &privateSessionError{message: operation + ": " + detail, cause: cause}
}
