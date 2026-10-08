# Task 1 implementation report

Scope: OpenSpec task 1.1; only `internal/config/config.go` and `internal/config/config_test.go` are application changes.

Commit: `4ba128cec61e3bf00b3eaac8c5f221f96cd16883` (`feat(api): configure optional manager dev URL`). Only the two owned application files were staged and committed; this report remains a local workflow artifact.

## Implementation and requirement evidence

- `internal/config/config.go:18`: exported `Config.ManagerDevURL string`.
- `internal/config/config.go:99`: loads `WZAP_MANAGER_DEV_URL` unchanged; absence and empty values retain the empty default.
- `internal/config/config.go:128`: validates HTTP/HTTPS, hostname presence, no userinfo, root or absent escaped path, no query (including a bare `?`), and no fragment (including a bare `#`). URL parser failures are reported without interpolating their potentially sensitive errors.
- `internal/config/config.go:134`: aggregated configuration error identifies the variable and validation constraint, without echoing its supplied value.
- `internal/config/config_test.go:12`: includes the variable in test environment cleanup.
- `internal/config/config_test.go:40`: 17 rejection cases exercise relative/scheme-relative/opaque URLs, unsupported scheme, missing host/hostname, credentials/username, query/empty query, fragment/empty fragment, path prefix/encoded slash, malformed escape/port and whitespace. Each checks the variable name and absence of the supplied URL and sensitive sentinel.
- `internal/config/config_test.go:83`: 8 acceptance/default cases cover absent, empty, HTTP/HTTPS with absent/root path, IPv4 and IPv6. Absent case actually unsets the variable and restores it through `t.Setenv` cleanup.

## TDD evidence

All Go commands use `PATH=/usr/local/go/bin:$PATH GOTOOLCHAIN=go1.26.0`.

Initial RED command: `go test ./internal/config -run TestLoadRejectsInvalidManagerDevURL -count=1` (exit 1).

```text
--- FAIL: TestLoadRejectsInvalidManagerDevURL (0.00s)
    --- FAIL: TestLoadRejectsInvalidManagerDevURL/relative (0.00s)
        config_test.go:70: Load() error = nil, want invalid manager dev URL rejected
```

The same expected assertion failed for all 17 cases: relative, scheme_relative, unsupported_scheme, missing_host, missing_hostname, opaque, credentials, username, query, empty_query, fragment, empty_fragment, path_prefix, encoded_path, invalid_escape, invalid_port, whitespace. This was runtime rejection failure, not a compilation failure.

Initial GREEN command after validation implementation: `go test ./internal/config -count=1` (exit 0).

```text
ok  wzap/internal/config 0.082s
```

Acceptance RED command: `go test ./internal/config -run '^TestLoadManagerDevURL$' -count=1` (exit 1). Temporarily removed the load assignment before writing the acceptance test to demonstrate the observable failure instead of relying on a missing-field compile error.

```text
--- FAIL: TestLoadManagerDevURL (0.00s)
    --- FAIL: TestLoadManagerDevURL/http (0.00s)
        config_test.go:114: Load().ManagerDevURL = "", want "http://manager-dev:3000"
```

The same expected assertion failed for all six nonempty valid origins (http, http_root, https, https_root, ipv4, ipv6). Absent and empty cases passed as expected. Restored environment loading and ran GREEN:

```text
$ go test ./internal/config -count=1
ok  wzap/internal/config 0.009s
```

## Verification

- Focused configuration tests: PASS, exit 0.
- `go vet ./internal/config`: PASS, exit 0.
- `gofmt -w internal/config/config.go internal/config/config_test.go`, followed by `gofmt -l` on those files: clean, exit 0 and no output.
- `git diff --check`: clean, exit 0.
- Full `go test ./... -count=1`: PASS, exit 0; complete output retained at `/tmp/wzap-task-1-all-tests.log`. All packages reported `ok` or `[no test files]`; no failing packages.
- Postgres/NATS integration variables were not set for this run; no integration execution claim.
- No application startup, Docker changes, dependencies, migrations, CI edits, Swagger changes or frontend changes. Supplied AGENTS files and task checkbox status were left unchanged.

## Concerns

None within the assigned configuration scope. Browser testing does not apply to this configuration-only task; controller owns integration and frontend verification.
