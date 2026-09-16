package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewJSONLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		var buf bytes.Buffer
		l, err := newWithWriter(level, "json", &buf)
		if err != nil {
			t.Fatalf("newWithWriter(%q, json): unexpected error: %v", level, err)
		}
		lvl, err := zerolog.ParseLevel(level)
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", level, err)
		}
		l.WithLevel(lvl).Str("k", "v").Msg("hello")
		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("level %q: output is not JSON: %v (%q)", level, err, buf.String())
		}
		if entry["message"] != "hello" {
			t.Errorf("level %q: message = %v, want hello", level, entry["message"])
		}
		if _, ok := entry["time"]; !ok {
			t.Errorf("level %q: entry has no timestamp field", level)
		}
	}
}

func TestNewLevelGating(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("info", "json", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(info, json): unexpected error: %v", err)
	}
	l.Debug().Msg("suppressed")
	if buf.Len() != 0 {
		t.Fatalf("debug event passed info gate: %q", buf.String())
	}
	l.Info().Msg("emitted")
	if !strings.Contains(buf.String(), "emitted") {
		t.Errorf("info event dropped by info gate: %q", buf.String())
	}
}

func TestNewConsoleFormat(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("debug", "console", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(debug, console): unexpected error: %v", err)
	}
	l.Info().Msg("readable")
	if !strings.Contains(buf.String(), "readable") {
		t.Errorf("console output missing message: %q", buf.String())
	}
}

func TestNewTextAliasEmitsDeprecation(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("info", "text", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(info, text): unexpected error: %v", err)
	}
	lowered := strings.ToLower(buf.String())
	if !strings.Contains(lowered, "deprecat") {
		t.Fatalf("text alias emitted no deprecation warning: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "console") {
		t.Errorf("deprecation warning does not point at console: %q", buf.String())
	}
	buf.Reset()
	l.Info().Msg("still-readable")
	if !strings.Contains(buf.String(), "still-readable") {
		t.Errorf("text alias logger unusable: %q", buf.String())
	}
}

func TestNewTextAliasDeprecationVisibleAtErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("error", "text", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(error, text): unexpected error: %v", err)
	}
	if lowered := strings.ToLower(buf.String()); !strings.Contains(lowered, "deprecat") {
		t.Fatalf("error+text emitted no visible deprecation warning: %q", buf.String())
	}
	if got := l.GetLevel(); got != zerolog.ErrorLevel {
		t.Errorf("GetLevel() = %v, want error (gate must be kept)", got)
	}
}

func TestNewInvalidLevel(t *testing.T) {
	for _, level := range []string{"verbose", "", "trace", "fatal", "panic", "disabled"} {
		if _, err := New(level, "json"); err == nil {
			t.Errorf("level %q: expected error, got nil", level)
		} else if !strings.Contains(err.Error(), "WZAP_LOG_LEVEL") {
			t.Errorf("level %q: error does not name WZAP_LOG_LEVEL: %v", level, err)
		}
	}
}

func TestNewInvalidFormat(t *testing.T) {
	if _, err := New("info", "xml"); err == nil {
		t.Fatal("expected error for invalid format, got nil")
	} else if !strings.Contains(err.Error(), "WZAP_LOG_FORMAT") {
		t.Errorf("error does not name WZAP_LOG_FORMAT: %v", err)
	}
}

func TestNewTestLogger(t *testing.T) {
	buf, l := NewTestLogger()
	if l.GetLevel() != zerolog.DebugLevel {
		t.Errorf("GetLevel() = %v, want debug", l.GetLevel())
	}
	l.Debug().Str("instance_id", "inst-1").Msg("dbg")
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("test logger output is not JSON: %v (%q)", err, buf.String())
	}
	if entry["level"] != zerolog.DebugLevel.String() {
		t.Errorf("level = %v, want debug", entry["level"])
	}
}

func TestAssertNoSecret(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(`{"level":"info","message":"ok"}`)
	AssertNoSecret(t, &buf, "s3cr3t-token", "55 11 99999-0000")

	if got := findSecret(buf.String(), "s3cr3t-token", "missing"); got != "" {
		t.Errorf("findSecret on clean buffer = %q, want empty", got)
	}
	if got := findSecret(`{"token":"s3cr3t-token"}`, "s3cr3t-token"); got != "s3cr3t-token" {
		t.Errorf("findSecret = %q, want the leaked secret", got)
	}
}
