# Task 3 report — real manager WebSocket regression

Status: complete. Commit: `38e4bbe` (`feat(api): cover manager development WebSocket forwarding`).

## Scope and implementation

Added only `internal/httpapi/manager_websocket_test.go`; no production code, Nuxt, dependency, Swagger, Docker, AGENTS, task or plan changes.

The regression exercises the existing `newManagerDevEntry` helper, which constructs `httpapi.New` with `Config.ManagerDevURL` and exposes the complete router/middleware chain through a real `httptest.Server`. A raw TCP client sends an RFC 6455 version-13 handshake; a separate real HTTP upstream hijacks its connection and exchanges short text frames. This covers `manager.DevHandler` through its public composition, rather than duplicating the same scenario on an isolated handler or using an in-memory recorder.

Evidence:

- `internal/httpapi/manager_websocket_test.go:22`: composed real-network WebSocket regression.
- `internal/httpapi/manager_websocket_test.go:40`: manager HMR request URI `/manager/_nuxt/hmr?token=a%2Fb&v=1`, public Host `public.example:8081`, WebSocket headers and `vite-hmr` subprotocol.
- `internal/httpapi/manager_websocket_test.go:60`: requires `101`, RFC example's literal accept value, upgrade headers, subprotocol and middleware's request ID.
- `internal/httpapi/manager_websocket_test.go:72`: upstream receives exact original path, query and public Host.
- `internal/httpapi/manager_websocket_test.go:80`: unsolicited `nuxt-ready` upstream frame is received before the client sends a masked `hmr-update` frame, followed by upstream `ack:hmr-update`.
- `internal/httpapi/manager_websocket_test.go:101`: real upstream handshake and socket implementation.
- `internal/httpapi/manager_websocket_test.go:142`: stdlib-only short text frames; client uses a random four-byte mask, upstream frames are unmasked, reads validate FIN/text/masking/short length.

Connections and channel waits have three-second deadlines; cleanup closes sockets and HTTP servers. Buffered result channels prevent handler goroutines blocking if the test fails early.

## RED → GREEN evidence

Named regression: replacing the development manager branch with the static handler loses WebSocket forwarding; a middleware wrapper that blocks hijacking also prevents this test from reaching `101` and exchanging frames.

Temporarily mutated exactly one existing condition in `internal/httpapi/server.go`:

```go
if cfg.ManagerDevURL != "" && false {
```

This precisely forces the pre-proxy static manager branch without removing otherwise-valid Go imports. Ran:

```text
PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0 go test ./internal/httpapi -run '^TestManagerDevMountForwardsWebSocket$' -count=1
--- FAIL: TestManagerDevMountForwardsWebSocket (0.00s)
    manager_websocket_test.go:60: WebSocket handshake status = 503, want 101
FAIL wzap/internal/httpapi 0.087s
```

Exit 1. The failure is the required missing upgrade, not a build error. Restored `server.go` from its saved content, verified its Git diff is empty, then reran the same targeted command: exit 0 (`ok wzap/internal/httpapi 0.020s`). The existing proxy/middleware already supported the upgrade, so no production fix was needed.

## Verification

All Go commands used `PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0`.

- `go test ./manager ./internal/httpapi/... -count=1`: exit 0. Final run: manager 0.207s, httpapi 17.702s, authsession 0.095s, core 0.714s, representation 0.209s; remaining resource packages have no test files.
- `go test -race ./internal/httpapi -run '^TestManagerDevMountForwardsWebSocket$' -count=1`: final exit 0, httpapi 1.800s, no race reports.
- `go vet ./manager ./internal/httpapi/...`: exit 0.
- `gofmt -l internal/httpapi/manager_websocket_test.go`: no output.
- `git diff --check`: exit 0.
- `git diff -- internal/httpapi/server.go`: empty after restoring the RED mutation.

One intermediate optional race build failed with `could not import crypto/rand (open : no such file or directory)` after I added that import while the compiler was already running. I kept the final source stable and reran the race check and required package suite; both exited 0. This was a verification sequencing mistake, with no runtime or production fix needed.

No Postgres or NATS integration URL was set for these commands, and no database-backed integration execution is claimed. No external services were started. The frame fixture intentionally supports only the minimal text-frame traffic needed to prove proxy forwarding, rather than implementing a full WebSocket library. No unresolved concerns within Task 3 scope.
