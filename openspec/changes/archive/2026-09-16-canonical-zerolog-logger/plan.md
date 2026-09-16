# canonical-zerolog-logger Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `log/slog` with `zerolog` as the single canonical logger in prod and tests, with strict level semantics and no alert false-positives.

**Architecture:** One factory (`internal/logger.New`) builds every logger; each package receives a `zerolog.Logger` value by injection (no globals, no wrappers); a temporary unexported `slogBridge` in `cmd/wzap` keeps not-yet-migrated constructors compiling until Task 4.1 deletes it under a grep gate.

**Tech Stack:** Go 1.26, `github.com/rs/zerolog v1.35.1` (direct), stdlib only otherwise.

**Spec:** `openspec/changes/canonical-zerolog-logger/specs/wzap-operations/spec.md` — the plan argues from the spec; executors read both. Design: `openspec/changes/canonical-zerolog-logger/design.md`.

## Global Constraints

- Go 1.26 pinned in `go.mod`; `CGO_ENABLED=0` must keep building.
- Internal imports rooted at module `wzap`.
- REST envelopes (`{"data": …}` / `{"error": {"code","message"}}` + `X-Request-Id`) and the versioned event contract (`event_version: 1`, stable `event_id` as `Nats-Msg-Id`) are UNTOUCHED.
- `WZAP_LOG_LEVEL` accepts exactly `debug, info, warn, error` (anything else fails boot naming the variable); `WZAP_LOG_FORMAT` accepts `json` (default), `console`, and legacy alias `text` (= console + exactly one deprecation Warn).
- QR / token / apikey / cookie / password / vcard bytes NEVER appear in any log at any level (only `qr_present`, `expires_at` booleans).
- `Warn/Error` carry only opaque IDs (`instance_id`, `conversation_id`, `message_id`, `event_id`); `phone/jid` live in `Debug` at most.
- Never commit secrets or `.env` files; conventional commits (`feat(logging): …`, `fix(logging): …`).
- Gates per task: `gofmt -l` empty on touched files, `go vet` on touched packages, focused `go test`, full `go test ./... -count=1` once before each commit.

## Scope Check

Single subsystem (logging). No decomposition needed — one plan, tasks ordered by dependency (factory → boot/bridges → packages → test unification → docs → gates).

## File Structure

- Create: `internal/logger/logger.go` — `New(level, format string) (zerolog.Logger, error)` + `NewTestLogger() (*bytes.Buffer, zerolog.Logger)` + `AssertNoSecret(t testing.TB, buf *bytes.Buffer, secrets ...string)`. DONE in Task 1.
- Create: `internal/logger/logger_test.go` — factory + helper tests. DONE in Task 1.
- Modify per task: `cmd/wzap/{main,seed}.go` + tests (DONE 2.1, incl. temporary unexported `slogBridge`); `internal/session/whatsmeow/{store,manager,version}.go`; `internal/httpapi/{middleware,server,connection,chatwoot}.go`; `internal/instance/service.go`; `internal/app/{runtime,editdelete}.go`; `internal/events/relay.go`; `internal/message/{outbox,jidresolver}.go`; `internal/media/cleaner.go`; `internal/webhook/worker.go`; `internal/chatwoot/{contacts/contacts,conversations/conversations,mirror/worker,inbound/webhook,import/scheduler}.go`; all co-located `*_test.go` (minimal adaptation; full unification in 4.1).
- Modify: `README.md` env table (`WZAP_LOG_FORMAT` row).
- Delete in 4.1: `slogBridge` in `cmd/wzap/main.go`; every `log/slog` import in prod code.

## Universal Translation Pattern (applies to every migration task)

```go
// BEFORE (slog, key-value pairs)
slog.Warn("connect instance failed", "instance_id", id, "op", "connect", "branch", "new-pairing", "error", err)
r.log.ErrorContext(ctx, "record connection change", "instance_id", instanceID, "status", status, "error", err)
log := w.log.With("instance_id", instanceID, "event_id", eventID)

// AFTER (zerolog, chained fields; ctx dropped — it stays for cancel/DB only)
s.log.Warn().Str("instance_id", id.String()).Str("op", "connect").Str("branch", "new-pairing").Err(err).Msg("connect instance failed")
r.log.Error().Str("instance_id", instanceID.String()).Str("status", string(status)).Err(err).Msg("record connection change")
log := w.log.With().Str("instance_id", instanceID.String()).Str("event_id", eventID.String()).Logger()
```

Field-type rule: `uuid.UUID` → `.String()` + `Str`; `error` → `Err(err)` (never `Any("error", err)`); counts → `Int`/`Int64`; bools → `Bool`; times → `Time`.

## Status: DONE — Task 1 (1.1+1.2): factory + tests (commits d80b939..44019e6)

Delivered `internal/logger` with `New`, `NewTestLogger`, `AssertNoSecret`, level allow-list, gate-proof `text` deprecation. No further steps.

## Status: DONE — Task 2.1: cmd/wzap wiring (commits 44019e6..23e0a7d)

`newLogger` delegates to `logger.New`; `stopComponents`/`seedAdmin` take `zerolog.Logger`; temporary unexported `slogBridge(log zerolog.Logger) *slog.Logger` in `cmd/wzap/main.go` feeds the 14 not-yet-migrated downstream ctors. Deletion gate: Tasks 2.2–3.5 remove uses; Task 4.1 deletes the function + adds grep gate.

---

### Task 2.2: waLogger on zerolog

**Files:**
- Modify: `internal/session/whatsmeow/store.go`
- Modify: `internal/session/whatsmeow/manager.go` (`Manager.log`, `instanceSession.log`, `NewManager`, `newSession` signatures)
- Modify: `internal/session/whatsmeow/version.go` (`RefreshWAVersion`, `refreshWAVersion` signatures)
- Modify (minimal adaptation only): `internal/session/whatsmeow/*_test.go` (`slog.Default()` → `zerolog.Nop()`; keep in-memory-handler tests compiling via `logger.NewTestLogger`)

**Interfaces:**
- Consumes: `logger.New`, `logger.NewTestLogger` from Task 1.
- Produces: `waLog.Logger` over zerolog for `whatsmeow.NewClient` + `sqlstore.New`; `*slog.Logger` params become `zerolog.Logger`.

- [ ] **Step 1: Rewrite waLogger over zerolog**

In `internal/session/whatsmeow/store.go`, replace the struct and methods:

```go
type waLogger struct {
	log    zerolog.Logger
	module string
}

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
```

Delete the `log/slog` import; add `github.com/rs/zerolog`. Delete the `if log == nil { log = slog.Default() }` fallback in `newWALogger`/`openDeviceStore` (callers always pass a value now).

- [ ] **Step 2: Change Manager/session/version signatures**

```go
// manager.go
log           zerolog.Logger
func NewManager(ctx context.Context, databaseURL string, instances storage.InstanceRepository, log zerolog.Logger, sink session.EventSink, maxMediaBytes int64) (*Manager, error)
func newSession(instanceID uuid.UUID, device *store.Device, log zerolog.Logger, sink session.EventSink, maxMediaBytes int64) (*instanceSession, error)
// version.go
func RefreshWAVersion(ctx context.Context, log zerolog.Logger)
```

Replace `m.log.Warn("restore session failed", "instance_id", instance.ID, "jid", …)` with `.Str("instance_id", …)` and move the raw JID to `Debug` (spec: PII minimizada). Keep message strings byte-identical.

- [ ] **Step 3: Minimal test adaptation**

In `manager_test.go`, `restore_test.go`, `reconnect_test.go`: `slog.Default()` → `zerolog.Nop()`. In `pairing_logging_test.go` and `version_test.go`: keep tests compiling with `logger.NewTestLogger()` (full helper unification is Task 4.1 — do not rewrite the in-memory handler here).

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/session/whatsmeow/ -count=1`
Expected: PASS.

- [ ] **Step 5: Run full suite + vet + fmt**

Run: `go vet ./internal/session/whatsmeow/ && test -z "$(gofmt -l internal/session/whatsmeow/)" && go test ./... -count=1`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add internal/session/whatsmeow/
git commit -m "feat(logging): migrate whatsmeow session to zerolog"
```

### Task 2.3: middleware Logging/Recover + probe skips

**Files:**
- Modify: `internal/httpapi/middleware.go` (`Logging`, `Recover` signatures + bodies)
- Modify: `internal/httpapi/server.go` (pass-through unchanged — signatures already `*slog.Logger`, change to `zerolog.Logger`)
- Modify (minimal): `internal/httpapi/middleware_test.go` (`discardLogger`, JSON-buffer test)

**Interfaces:**
- Consumes: `zerolog.Logger` value.
- Produces: same middleware shapes `func Logging(log zerolog.Logger) func(http.Handler) http.Handler`.

- [ ] **Step 1: Rewrite Logging with skip list**

```go
// Logging emits one structured log line per request, skipping probes and docs.
func Logging(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipAccessLog(r) {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info().
				Str("request_id", RequestIDFromContext(r.Context())).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rec.status).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("http request")
		})
	}
}

func skipAccessLog(r *http.Request) bool {
	if (r.Method == http.MethodGet) && (r.URL.Path == "/healthz" || r.URL.Path == "/readyz") {
		return true
	}
	return strings.HasPrefix(r.URL.Path, "/swagger/")
}
```

Add `strings` import; drop `log/slog`.

- [ ] **Step 2: Rewrite Recover**

```go
func Recover(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				log.Error().
					Str("request_id", RequestIDFromContext(r.Context())).
					Any("panic", recovered).
					Str("stack", string(debug.Stack())).
					Msg("panic recovered")
				Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}()
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 3: Update server.go + minimal test adaptation**

Change `func New(cfg config.Config, log *slog.Logger, deps Deps)` to `log zerolog.Logger`. In `middleware_test.go`: `discardLogger()` returns `zerolog.Nop()`; JSON-buffer test builds via `logger.NewTestLogger()` and asserts `request_id/method/path/status/duration_ms` fields (keep field names identical).

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/httpapi/ -run 'TestLogging|TestRecover|TestRequestID' -count=1`
Expected: PASS.

- [ ] **Step 5: Full suite + vet + fmt, then commit**

Run: `go vet ./internal/httpapi/ && test -z "$(gofmt -l internal/httpapi/)" && go test ./... -count=1`
Expected: green.

```bash
git add internal/httpapi/
git commit -m "feat(logging): zerolog middleware with probe skips"
```

### Task 3.1: instance + httpapi/connection (inject logger, branch constants)

**Files:**
- Modify: `internal/instance/service.go` (add `log zerolog.Logger` field, `NewService` param, 16 call sites)
- Modify: `internal/httpapi/connection.go` (add `log zerolog.Logger` param to `handleConnectInstance`, `handleQRInstance`, `handleInstanceStatus`, `handleDisconnectInstance`; 15 call sites)
- Modify: `internal/httpapi/server.go` (pass log at the 4 registration sites), `cmd/wzap/main.go` (`instance.NewService(..., log)` + remove one `slogBridge` use)
- Modify (minimal): `internal/instance/*_test.go`, `internal/httpapi/connection_logging_test.go` (construct with `zerolog.Nop()` / `logger.NewTestLogger`)

**Interfaces:**
- Consumes: `zerolog.Logger`.
- Produces: `instance.NewService(repo, sessions, media, users, keys, log zerolog.Logger) *Service`; handler ctors take trailing `log zerolog.Logger`.

Branch messages become constants (tests assert exact strings):

```go
const (
	msgConnectBranch = "connect instance branch"
	msgConnectFailed = "connect instance failed"
	msgQRBranch      = "qr instance branch"
	msgQRFailed      = "qr instance failed"
)
```

- [ ] **Step 1: instance/service.go — add field + param, convert 16 sites**

```go
type Service struct {
	// ... existing fields
	log zerolog.Logger
}

func NewService(repo storage.InstanceRepository, sessions session.Manager, media MediaRemover, users storage.UserRepository, keys storage.APIKeyRepository, log zerolog.Logger) *Service {
```

Convert each site, e.g.:

```go
// BEFORE
slog.Debug("connect instance branch", "instance_id", id, "op", "connect",
	"branch", "already-connected", "status", string(session.StatusConnected))
// AFTER
s.log.Debug().Str("instance_id", id.String()).Str("op", "connect").
	Str("branch", "already-connected").Str("status", string(session.StatusConnected)).
	Msg(msgConnectBranch)
```

Failure sites use `.Err(err)` + `Msg(msgConnectFailed)`. Keep `pairingLogAttrs` but change it to return `[]func(*zerolog.Event)`-style appenders or inline the three fields (`status`, `qr_present`, `expires_at`) at each of the 2 `append` sites — pick inline (2 sites, no new abstraction). Never log the QR string itself (guard stays).

- [ ] **Step 2: connection.go — add log param, convert 15 sites**

```go
func handleConnectInstance(instances InstanceService, log zerolog.Logger) http.HandlerFunc {
```

Same pattern: `slog.Debug("connect instance request", …)` → `log.Debug().Str("instance_id", …).Str("op", "connect").Msg("connect instance request")`; failures → `log.Warn()…Err(err).Msg("connect instance failed")`. Message strings byte-identical. Same for `chatwoot.go:203,212` only if that handler already takes log — otherwise leave `chatwoot.go` for Task 3.5 and do NOT touch it here.

- [ ] **Step 3: Update callers**

`server.go`: `handleConnectInstance(deps.Instances, log)` (4 sites). `main.go`: `instance.NewService(instances, sessions, mediaStorage, users, keys, log)`; delete the corresponding `slogBridge` wrapper use.

- [ ] **Step 4: Minimal test adaptation**

All `NewService(repo, sessions, …)` in tests gain trailing `zerolog.Nop()` (or `logger.NewTestLogger()` where the test captures output). Handler tests pass a logger likewise. Do NOT rewrite the in-memory handlers (Task 4.1).

- [ ] **Step 5: Focused tests**

Run: `go test ./internal/instance/ ./internal/httpapi/ -count=1`
Expected: PASS.

- [ ] **Step 6: Full suite + vet + fmt, then commit**

```bash
git add internal/instance/ internal/httpapi/ cmd/wzap/
git commit -m "feat(logging): inject zerolog into instance service and connection handlers"
```

### Task 3.2: app (drop *Context)

**Files:**
- Modify: `internal/app/runtime.go` (8 sites), `internal/app/editdelete.go` (4 sites)
- Modify (minimal): `internal/app/*_test.go`

- [ ] **Step 1: Convert runtime.go sites**

`r.log.ErrorContext(ctx, "handle inbound message", …)` → `r.log.Error().Str("instance_id", …).Str("message_id", …).Err(err).Msg("handle inbound message")`. `DebugContext("connection change received")` keeps `jid_present` bool (never the JID). `WarnContext("skip connection event for unknown instance")` stays `Warn` (genuinely unexpected). Drop the `ctx` param where it becomes unused (check `applyConnection(ctx, log, …)` — keep `ctx` for the DB calls inside, only the log line drops it).

- [ ] **Step 2: Convert editdelete.go (4 ErrorContext → Error)**

Message strings identical; `.Str("instance_id", …).Str("message_id", …).Err(err)`.

- [ ] **Step 3: Minimal test adaptation** (`NewRuntime(…, log)` already takes a logger — pass `logger.NewTestLogger()` / `Nop`).

- [ ] **Step 4: Focused tests**

Run: `go test ./internal/app/ -count=1`
Expected: PASS.

- [ ] **Step 5: Full suite + vet + fmt, then commit**

```bash
git add internal/app/
git commit -m "feat(logging): migrate app runtime to zerolog"
```

### Task 3.3: events/message/media + loop throttle

**Files:**
- Modify: `internal/events/relay.go` (6 sites + throttle)
- Modify: `internal/message/outbox.go` (9 sites + throttle on claim loop), `internal/message/jidresolver.go` (2 sites; `phone` digits → `Debug`)
- Modify: `internal/media/cleaner.go` (2 sites)
- Modify (minimal): co-located `*_test.go`

**Interfaces:** constructors take `zerolog.Logger` (value) instead of `*slog.Logger`; delete nil→`slog.Default()` fallbacks.

- [ ] **Step 1: relay.go — convert + throttle**

Convert the 6 lines per the universal pattern. Then add throttle state:

```go
type Relay struct {
	// ... existing fields
	mu          sync.Mutex
	lastWarn    map[string]time.Time
	warnEvery   time.Duration // default time.Minute, test-overridable field
}

func (r *Relay) warnThrottled(key, msg string, fields func(*zerolog.Event) *zerolog.Event) {
	r.mu.Lock()
	now := time.Now()
	last, ok := r.lastWarn[key]
	if ok && now.Sub(last) < r.warnEvery {
		r.mu.Unlock()
		return
	}
	r.lastWarn[key] = now
	r.mu.Unlock()
	fields(r.log.Warn()).Msg(msg)
}
```

Apply to `"event stream not ready"`, `"claim pending events"`, `"publish pending events"` (keys `"stream"`, `"claim"`, `"publish"`). Keep `"record event attempt"` as unthrottled `Error` (per-event data loss signal) and `"cleaned published events"` as `Info`.

- [ ] **Step 2: outbox.go — convert + throttle claim loop**

Same throttle shape (own struct fields, keys `"requeue"`, `"claim"`). Per-message `Error`s (`schedule message retry`, `mark message sent/failed`, `build message status event`) stay unthrottled. `"simulate send presence"` Warn keeps `message_id` + `Err`.

- [ ] **Step 3: jidresolver.go — convert + relevel PII**

`"jid cache read failed"` / `"jid cache write failed"` become `Debug` (cache miss path is routine) and keep `phone` digits ONLY at Debug:

```go
o.log.Debug().Str("phone", digits).Err(err).Msg("jid cache read failed")
```

- [ ] **Step 4: cleaner.go — convert 2 sites** (`Warn` on error, `Info` when `removed > 0`; no throttle — 1/min cadence is already bounded).

- [ ] **Step 5: Minimal test adaptation** (`discardLogger` → `zerolog.Nop()`; buffer tests → `logger.NewTestLogger()`).

- [ ] **Step 6: Focused tests**

Run: `go test ./internal/events/ ./internal/message/ ./internal/media/ -count=1`
Expected: PASS.

- [ ] **Step 7: Full suite + vet + fmt, then commit**

```bash
git add internal/events/ internal/message/ internal/media/
git commit -m "feat(logging): migrate events pipeline to zerolog with loop throttle"
```

### Task 3.4: webhook + delivery-failure throttle

**Files:**
- Modify: `internal/webhook/worker.go` (10 sites + per-job Warn cap)
- Modify (minimal): `internal/webhook/*_test.go`

- [ ] **Step 1: Convert 10 sites** per pattern; `"webhook worker stopped"` stays `Info` with `pending`; `"webhook dead letter"` stays `Error` (exactly 1 per exhausted job — keep that invariant, tests assert it).

- [ ] **Step 2: Cap per-job delivery Warns**

Delivery failures already back off (`1s..5m`) and stop after 8 attempts; keep that, but log attempts 2..7 at `Debug` and only attempts 1 and 8 at `Warn` (first signal + final state before dead-letter). Implement with the attempt count already available in `handle()`:

```go
ev := w.log.Debug()
if attempt == 1 || attempt == maxAttempts {
	ev = w.log.Warn()
}
ev.Str("instance_id", …).Str("event_id", …).Str("event_type", …).Err(err).Msg("webhook delivery failed")
```

`"webhook queue full, dropping event"` stays `Warn` (data loss). `"webhook drop: instance gone"` → `Debug` (routine unprovisioned-instance race, `done=true` no retry).

- [ ] **Step 3: Minimal test adaptation** — `instantWorker(…, log)` takes `zerolog.Logger`; buffer assertions keep working via `logger.NewTestLogger()`; dead-letter count assertions (`==1 ocorrência`) must still pass.

- [ ] **Step 4: Focused tests**

Run: `go test ./internal/webhook/ -count=1`
Expected: PASS.

- [ ] **Step 5: Full suite + vet + fmt, then commit**

```bash
git add internal/webhook/
git commit -m "feat(logging): migrate webhook worker with per-job warn cap"
```

### Task 3.5: chatwoot/* levels + PII

**Files:**
- Modify: `internal/chatwoot/contacts/contacts.go` (5), `internal/chatwoot/conversations/conversations.go` (3), `internal/chatwoot/mirror/worker.go` (~49), `internal/chatwoot/inbound/webhook.go` (13), `internal/chatwoot/import/scheduler.go` (5)
- Modify: `internal/httpapi/chatwoot.go:203,212` if not done in 3.1
- Modify (minimal): co-located tests

- [ ] **Step 1: contacts + conversations — relevel, keep phone/jid at Debug or drop**

`"contact create conflict recovered by identifier search"` stays `Info` but drops `phone` (keep nothing or `contact_id` if available). The three `* failed, skipping message mirror` become `Debug` with `reason` (`contact_create_failed`) — they are routine external-data outcomes; keep `Err(err)` for diagnosis:

```go
r.log.Debug().Str("reason", "contact_create_failed").Err(err).Msg("contact creation failed, skipping message mirror")
```

`"contact merge failed"` / `"contact update failed"` stay `Warn` (state corruption signal) but keep only IDs. Same treatment in `conversations.go` (creation/validation/reopen failures stay `Warn`, IDs only — `remote_jid` moves to `Debug`).

- [ ] **Step 2: mirror/worker.go — the big relevel**

Apply this table mechanically (message strings identical):
- `Debug` (routine skip/flow): `"skipping filtered sender"` (already), `"throttling repeated connection notice"` (already), `"skipping message with unmappable type"` + add `Str("reason","unsupported_type")`, `"skipping edit/delete without mirrored original"` + `reason`, `"skipping mirror without inbox name"` / `"without provisioned inbox"` + `reason`, `"pairing qr lookup failed/empty/encoding"` (fallback path is by design), `"duplicate orphan removed after correlation race"`, `"mirroring attachment*"` (already).
- `Warn` (actionable): `"conversation resolution failed"`, `"chatwoot message/edit creation failed"`, `"correlation store failed"`, `"operational * failed"`, `"inbox listing failed"`, `"contact resolution failed"` (drop `jid` attr — keep `instance_id,event_id,wa_key` from the `With` base), `"dropping undecodable *"` (poison input, 1x each via Term path — keep), `"dropping event with unknown type"`.
- `Info`: `"chatwoot mirror disabled…"`, `"chatwoot mirror consuming"` (already).
- `With` bases: `w.log.With().Str("instance_id", …).Str("event_id", …).Str("wa_key", …).Logger()`.

- [ ] **Step 3: inbound/webhook.go — 13 Warns → split**

Routine external outcomes → `Debug` + `reason`: `"skipping reverse delete without correlation/session"`, `"skipping outgoing without recipient"`. Real failures stay `Warn` (IDs only): `"reverse delete failed"`, `"recipient resolution failed"`, `"attachment download/store failed"`, `"media/text enqueue failed"`, `"mark read failed"`, `"private note/operational confirm failed"`.

- [ ] **Step 4: scheduler.go — convert 5** (`Info` start, `Warn` skipped-cycle/failed/lookup — keep, low cadence 1/30m).

- [ ] **Step 5: Minimal test adaptation** per R2.

- [ ] **Step 6: Focused tests**

Run: `go test ./internal/chatwoot/... -count=1`
Expected: PASS.

- [ ] **Step 7: Full suite + vet + fmt, then commit**

```bash
git add internal/chatwoot/ internal/httpapi/chatwoot.go
git commit -m "feat(logging): relevel chatwoot mirror and minimize PII"
```

### Task 4.1: unify test helpers + delete bridge + grep gate

**Files:**
- Modify: every `*_test.go` helper from the pre-migration era (`serviceMemoryHandler`, `memoryHandler`, `discardLogger`, ad-hoc `NewTextHandler(&logs)`) → `logger.NewTestLogger()` / `zerolog.Nop()`
- Modify: `cmd/wzap/main.go` — DELETE `slogBridge` + its 14 uses become direct `zerolog.Logger` passes (all downstream ctors migrated by now — compile proves it)
- Create/modify: grep gate (simplest: `go vet`-adjacent script or a `TestNoSlogInProd` in `internal/logger` that walks prod dirs and fails on `log/slog` imports)

**Interfaces:** test helpers `logger.NewTestLogger`, `logger.AssertNoSecret` are the only sanctioned capture/assert tools.

- [ ] **Step 1: Replace in-memory slog handlers**

Files: `internal/instance/service_logging_test.go`, `internal/httpapi/connection_logging_test.go`, `internal/session/whatsmeow/pairing_logging_test.go`. Replace handler + `slog.SetDefault` dance with:

```go
logs, log := logger.NewTestLogger()
// ... run code with log ...
logger.AssertNoSecret(t, logs, qr)
```

Keep asserting the same branch constants + levels (now `zerolog.DebugLevel` etc.) + `qr_present`/`expires_at` presence. Keep the exact message strings (constants from 3.1).

- [ ] **Step 2: Replace discard/buffer loggers**

`internal/httpapi/middleware_test.go`, `internal/session/whatsmeow/version_test.go`, `internal/events/relay_test.go`, `internal/message/jidresolver_test.go`, `internal/media/cleaner_test.go`, `cmd/wzap/seed_test.go`, `internal/httpapi/health_test.go`, `internal/app/*_test.go`, `internal/message/outbox_test.go`, `internal/webhook/worker_test.go` (keep `syncBuffer` only if still needed, else use the returned `*bytes.Buffer`).

- [ ] **Step 3: Delete slogBridge**

Remove the function + replace all 14 `slogBridge(log)` / `slogLog` uses with `log`. Remove the last `log/slog` import in prod code.

- [ ] **Step 4: Add grep gate test**

In `internal/logger/logger_test.go` (or a new `nogoslog_test.go` in package `logger_test` at repo root — prefer `internal/logger/slogban_test.go`):

```go
func TestNoSlogInProd(t *testing.T) {
	// walk ../.. skipping _test.go, vendor, .worktrees; fail on `"log/slog"` import lines
}
```

Keep it fast (<1s) and hermetic (relative path from the test file via `runtime.Caller`).

- [ ] **Step 5: Focused + full tests**

Run: `go test ./internal/logger/ ./internal/instance/ ./internal/httpapi/ ./internal/session/whatsmeow/ ./internal/app/ ./internal/webhook/ -count=1`
Expected: PASS. Then `go test ./... -count=1`.

- [ ] **Step 6: Commit**

```bash
git add internal/ cmd/wzap/
git commit -m "feat(logging): unify test helpers and remove slog bridge"
```

### Task 4.2: README env table

**Files:**
- Modify: `README.md` (`WZAP_LOG_LEVEL`, `WZAP_LOG_FORMAT` rows)

- [ ] **Step 1: Update rows**

```
| `WZAP_LOG_LEVEL` | não | `info` | Nível (`debug`, `info`, `warn`, `error`). |
| `WZAP_LOG_FORMAT` | não | `json` | Formato (`json` ou `console`; `text` é alias legado de `console`). |
```

- [ ] **Step 2: Verify + commit**

Run: `grep -n "WZAP_LOG_FORMAT" README.md`
Expected: updated row shown.

```bash
git add README.md
git commit -m "docs(logging): document json|console formats"
```

### Task 5.1: Gates finais

- [ ] **Step 1: Run the complete gate sequence**

Run: `go vet ./... && golangci-lint run && go test ./... -count=1 && go build ./... && test -z "$(gofmt -l .)"`
Expected: all green, `gofmt -l` prints nothing. (Postgres/NATS integration tests only if `WZAP_TEST_DATABASE_URL` / `WZAP_TEST_NATS_URL` are set per AGENTS.md; otherwise record as skipped, not executed.)

- [ ] **Step 2: Smoke both formats**

Run a boot-shape check for each format (same commands as Task 2.1 smoke) and confirm JSON vs human-readable output.

- [ ] **Step 3: Record evidence for verify.md**

Save command outputs to the SDD ledger; do NOT write `verify.md`/`retrospective.md` here (they are post-apply, pre-PR steps owned by the apply workflow tail).

## Self-Review

- Spec coverage: format/level scenarios → Tasks 1, 2.1, 4.2. Skip-sem-alerta → 3.4, 3.5. Retry-sem-flood → 2.3 (probe skip is access-log), 3.3, 3.4. Probes-fora-do-access-log → 2.3. PII/segredos → 3.1 (QR guard kept), 3.3, 3.5, 4.1 (guards). Rastreio por requisição → 2.3 (fields preserved). All covered.
- Placeholder scan: no TBD/TODO (the only TODO allowed is the pre-existing `slogBridge` deletion-gate comment, which Task 4.1 removes); every code step shows exact code; every test step shows exact commands.
- Type consistency: `logger.New(level, format string) (zerolog.Logger, error)`, `NewTestLogger() (*bytes.Buffer, zerolog.Logger)`, `AssertNoSecret(t testing.TB, buf, secrets...)` used identically across tasks; `slogBridge(log zerolog.Logger) *slog.Logger` unexported in `main` package only.

## Execution Handoff

Plan complete and saved to `openspec/changes/canonical-zerolog-logger/plan.md` (repo rule overrides the default `docs/superpowers/plans/` location: never write plan output to `docs/superpowers/`).

Execution is already underway via subagent-driven-development in worktree `.worktrees/sdd-canonical-zerolog-logger` (branch `sdd/canonical-zerolog-logger`): Tasks 1 and 2.1 complete and reviewed. Continue dispatching Tasks 2.2 → 5.1 in order with per-task review.
