# Task 2 report: manager development HTTP proxy

Status: implemented and focused verification passed. OpenSpec task 1.2 only; no task checkbox or planning artifact changes.

## Changes and evidence

- `manager/devproxy.go:11`: new `DevHandler(*url.URL)` using stdlib `httputil.ReverseProxy` with `Rewrite`.
- `manager/devproxy.go:14`: sets only upstream scheme/host, retaining the complete request Path and RawPath, including `/manager/`; explicitly restores original RawQuery because the stdlib cleans unparsable query parameters before Rewrite.
- `manager/devproxy.go:20`: preserves public request Host and reconstructs forwarding headers with `SetXForwarded`; incoming spoofed forwarding headers are removed by Rewrite's standard processing.
- `manager/devproxy.go:23`: generic plain-text 503 ErrorHandler, with no transport-error logging or static fallback; errors cannot reproduce internal address, query, cookie or apikey.
- `manager/devproxy.go:29`: canonical `/manager` redirect uses 301 and preserves query, including the empty query marker.
- `internal/httpapi/server.go:111`: constructs exactly one manager handler during router composition, parsing the already-validated configured origin once; absent configuration uses the existing static `manager.Handler()` without changing it. Both manager mounts share the constructed handler. Parse failure from a manually constructed Config panics with a generic variable-name-only error.
- `internal/httpapi/manager_test.go:96`: real HTTP entry + real httptest upstream exercise root page, nested page, asset query (including semicolon), escaped path/RawPath, public Host, request-id middleware, cookies/apikey, and forwarding-header reconstruction.
- `internal/httpapi/manager_test.go:147`: real HTTP canonical redirect preserves query without contacting the upstream.
- `internal/httpapi/manager_test.go:161`: real refused connection while a usable disk bundle exists returns 503 for page/deep link/asset, checks safe response/logs, then rebinds the same upstream address and proves recovery through the unchanged backend handler.
- `internal/httpapi/manager_test.go:206`: root `/auth`, `/auth/me`, `/instances`, `/users`, `/media`, probes and Swagger keep backend status/auth/middleware behavior and produce zero upstream requests.
- `manager/devproxy_test.go:14`: real TLS entry forwards POST body and upstream status/response header, keeps public Host/URI, reconstructs HTTPS forwarded headers and strips a client-specified hop-by-hop header.
- `manager/devproxy_test.go:73`: real HTTP redirect preserves an explicit empty query even with an unreachable target.

## TDD and debugging evidence

Before implementation, added composition behavior tests and ran:

```
PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0 go test ./internal/httpapi -run TestManagerDev -count=1
```

RED (exit 1):

- `TestManagerDevMountForwardsHTTP`: all four cases returned the old static handler's `503 manager console is not built into this binary` rather than the upstream 200.
- `TestManagerDevRedirectPreservesQuery`: returned old static 503, with no Location, instead of 301 `/manager/?next=instances`.
- `TestManagerDevFailureHasNoStaticFallbackAndRecovers`: page/deep link returned 200 with `stale manager bundle`, asset returned 404, and recovered upstream still returned stale static content.
- `TestManagerDevKeepsBackendRoutes` passed in RED, establishing existing backend routing behavior independently of manager forwarding.

Investigation traced these observed failures to `server.go` mounting `manager.Handler()` twice regardless of ManagerDevURL. The current static Handler's disk selection explains the stale-bundle result; no networking failure was involved. The single hypothesis was that selecting a dev handler once at composition would route all manager requests correctly while leaving backend mounts intact. Inspected local stdlib Rewrite/SetXForwarded documentation to confirm path retention, forwarded-header sanitization and query cleanup before implementing the branch and proxy.

GREEN: reran the same command after implementation; exit 0, `ok wzap/internal/httpapi 0.031s`. Added direct TLS/body/response/hop-header and empty-query contract coverage, closed the temporary httptest listener before rebinding the recovery fixture, and formatted the owned files.

## Final verification

```
PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0 gofmt -w manager/devproxy.go manager/devproxy_test.go internal/httpapi/server.go internal/httpapi/manager_test.go
PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0 go test ./manager ./internal/config ./internal/httpapi/... -count=1
git diff --check -- manager/devproxy.go manager/devproxy_test.go internal/httpapi/server.go internal/httpapi/manager_test.go
```

All exited 0. Manager/config/httpapi/authsession/core/representation packages passed; remaining resource packages reported no test files. Existing static manager tests retained. Embedded-build tests may skip on a placeholder-only checkout, as before; usable disk behavior is tested in existing manager package tests. No database or NATS environment was set or integration-test claim made. No Swagger annotation edits, dependencies, service startup, Docker mutations, AGENTS edits, UI changes or WebSocket tests in this task. Task 3 owns WebSocket verification. Full Go baseline was already run by the parent; this task ran the specified focused suite and did not repeat unrelated broader checks.

## Concerns and handoff

No known HTTP concerns. ReverseProxy's standard WebSocket behavior is retained but not yet verified by this task. Runtime Nuxt/HMR/browser validation remains for later tasks. Parent-owned AGENTS.md changes and untracked manager/AGENTS.md were left untouched and excluded from the implementation commit.
