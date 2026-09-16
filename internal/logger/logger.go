// Package logger owns the canonical zerolog factory for the wzap service.
//
// New parses WZAP_LOG_LEVEL / WZAP_LOG_FORMAT values into a zerolog.Logger
// that always writes to os.Stdout with Timestamp enabled. Callers inject the
// returned logger by value; an absent logger is zerolog.Nop().
package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// New builds the canonical service logger for the given level
// (debug, info, warn, error) and format (json, console, text).
// The legacy text format is an alias of console and emits one deprecation
// Warn on the created logger.
func New(level, format string) (zerolog.Logger, error) {
	return newWithWriter(level, format, os.Stdout)
}

func newWithWriter(level, format string, out io.Writer) (zerolog.Logger, error) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.Nop(), fmt.Errorf("invalid WZAP_LOG_LEVEL %q: must be one of debug, info, warn, error: %w", level, err)
	}
	var l zerolog.Logger
	switch format {
	case "json":
		l = zerolog.New(out).With().Timestamp().Logger().Level(lvl)
	case "console":
		l = zerolog.New(zerolog.ConsoleWriter{Out: out}).With().Timestamp().Logger().Level(lvl)
	case "text":
		l = zerolog.New(zerolog.ConsoleWriter{Out: out}).With().Timestamp().Logger().Level(lvl)
		l.Warn().Msg(`WZAP_LOG_FORMAT "text" is deprecated, use "console"`)
	default:
		return zerolog.Nop(), fmt.Errorf("invalid WZAP_LOG_FORMAT %q: must be one of json, console, text", format)
	}
	return l, nil
}

// NewTestLogger returns a JSON logger at debug level writing into the
// returned buffer, for content assertions in tests.
func NewTestLogger() (*bytes.Buffer, zerolog.Logger) {
	var buf bytes.Buffer
	l := zerolog.New(&buf).With().Timestamp().Logger().Level(zerolog.DebugLevel)
	return &buf, l
}

// AssertNoSecret fails the test when any of the given secrets (tokens,
// phone numbers, JIDs, ...) shows up in the logged output.
func AssertNoSecret(t *testing.T, buf *bytes.Buffer, secrets ...string) {
	t.Helper()
	if found := findSecret(buf.String(), secrets...); found != "" {
		t.Errorf("log output leaks secret %q: %q", found, buf.String())
	}
}

func findSecret(output string, secrets ...string) string {
	for _, s := range secrets {
		if s != "" && strings.Contains(output, s) {
			return s
		}
	}
	return ""
}
