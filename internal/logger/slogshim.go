package logger

import (
	"log/slog"

	"github.com/rs/zerolog"
)

// SlogShim returns a *slog.Logger that forwards every record into l.
//
// Temporary bridge for constructors that still take *slog.Logger (cmd/wzap
// serve() wiring) until later tasks migrate them to zerolog.Logger. Records
// keep the canonical level gate, format and destination of l. Delete this
// file once no *slog.Logger constructor remains.
func SlogShim(l zerolog.Logger) *slog.Logger {
	return slog.New(zerolog.NewSlogHandler(l))
}
