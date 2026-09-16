package whatsmeow

import (
	"context"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the pure-Go pgx database/sql driver
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// postgresDialect is the database/sql driver exposed by pgx's stdlib adapter.
// sqlstore maps it to its Postgres dialect; the driver is pure Go, so the
// service still builds with CGO_ENABLED=0.
const postgresDialect = "pgx"

// openDeviceStore opens the whatsmeow session tables in the Postgres database
// addressed by dsn, creating or upgrading them as needed.
func openDeviceStore(ctx context.Context, dsn string, log zerolog.Logger) (*sqlstore.Container, error) {
	container, err := sqlstore.New(ctx, postgresDialect, dsn, newWALogger(log))
	if err != nil {
		return nil, fmt.Errorf("open whatsmeow device store: %w", err)
	}
	return container, nil
}

// waLogger adapts a zerolog.Logger to the logger interface expected by
// whatsmeow, keeping the library's logging in the service's structured
// output.
type waLogger struct {
	log    zerolog.Logger
	module string
}

var _ waLog.Logger = (*waLogger)(nil)

// newWALogger returns the whatsmeow logger writing through zerolog.
func newWALogger(log zerolog.Logger) waLog.Logger {
	return &waLogger{log: log, module: "whatsmeow"}
}

func (l *waLogger) Debugf(msg string, args ...any) {
	l.log.Debug().Str("module", l.module).Msg(fmt.Sprintf(msg, args...))
}

func (l *waLogger) Infof(msg string, args ...any) {
	l.log.Info().Str("module", l.module).Msg(fmt.Sprintf(msg, args...))
}

func (l *waLogger) Warnf(msg string, args ...any) {
	l.log.Warn().Str("module", l.module).Msg(fmt.Sprintf(msg, args...))
}

func (l *waLogger) Errorf(msg string, args ...any) {
	l.log.Error().Str("module", l.module).Msg(fmt.Sprintf(msg, args...))
}

func (l *waLogger) Sub(module string) waLog.Logger {
	return &waLogger{log: l.log, module: l.module + "." + module}
}
