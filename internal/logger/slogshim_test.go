package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestSlogShimForwardsRecords pins the contract serve() relies on: records
// logged through the shim land in the canonical logger output as JSON with
// level, message, fields and a single timestamp.
func TestSlogShimForwardsRecords(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("info", "json", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(info, json): unexpected error: %v", err)
	}

	SlogShim(l).Info("hello", "k", "v")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("shim output is not JSON: %v (output %q)", err, buf.String())
	}
	if entry["level"] != "info" {
		t.Errorf("level = %v, want info", entry["level"])
	}
	if entry["message"] != "hello" {
		t.Errorf("message = %v, want hello", entry["message"])
	}
	if entry["k"] != "v" {
		t.Errorf("k = %v, want v", entry["k"])
	}
	if entry["time"] == nil {
		t.Error("time is absent, want Timestamp on every record")
	}
	if n := strings.Count(buf.String(), `"time":`); n != 1 {
		t.Errorf("found %d time fields, want exactly 1 (no duplicate timestamp)", n)
	}
}

// TestSlogShimRespectsLevelGate pins that the shim honors the canonical
// level gate instead of emitting everything.
func TestSlogShimRespectsLevelGate(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("warn", "json", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(warn, json): unexpected error: %v", err)
	}
	sl := SlogShim(l)

	if sl.Handler().Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled(Info) = true at warn gate, want false")
	}
	sl.Info("suppressed")
	if buf.Len() != 0 {
		t.Errorf("Info at warn gate emitted %q, want silence", buf.String())
	}
	sl.Error("boom", "error", errBoom())
	if buf.Len() == 0 {
		t.Fatal("Error at warn gate emitted nothing, want a record")
	}
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("shim output is not JSON: %v", err)
	}
	if entry["level"] != "error" || entry["message"] != "boom" {
		t.Errorf("entry = %v, want level=error message=boom", entry)
	}
}

// TestSlogShimKeepsAttrsAndGroups pins that With attrs and groups survive
// the bridge with their values. Group keys are flat and dot-qualified
// ("g.b"); no production caller uses slog groups, so nested stdlib shape is
// not required of this temporary bridge.
func TestSlogShimKeepsAttrsAndGroups(t *testing.T) {
	var buf bytes.Buffer
	l, err := newWithWriter("debug", "json", &buf)
	if err != nil {
		t.Fatalf("newWithWriter(debug, json): unexpected error: %v", err)
	}
	sl := SlogShim(l)

	sl.With("a", "1").Warn("w", "b", "2")
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("shim output is not JSON: %v", err)
	}
	if entry["a"] != "1" {
		t.Errorf("a = %v, want 1", entry["a"])
	}
	if entry["b"] != "2" {
		t.Errorf("b = %v, want 2", entry["b"])
	}

	buf.Reset()
	sl.WithGroup("g").Warn("w", "b", "2")
	entry = nil
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("shim output is not JSON: %v", err)
	}
	if entry["g.b"] != "2" {
		t.Errorf("g.b = %v, want 2 (group-qualified key)", entry["g.b"])
	}
}

func errBoom() error { return errShimBoom{} }

type errShimBoom struct{}

func (errShimBoom) Error() string { return "boom" }
