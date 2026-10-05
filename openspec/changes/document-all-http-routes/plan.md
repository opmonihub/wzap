# Document all HTTP routes — implementation plan

> Execution: one isolated implementation batch, then controller verification and review. The user explicitly requested immediate implementation; no additional approval checkpoint is needed.

**Goal:** Generated Swagger covers every registered HTTP operation and faithfully represents response bodies.

**Spec:** `specs/wzap-operations/spec.md` in this change directory.

**Architecture:** Keep Swaggo annotations with handlers, reuse the transport envelope, and validate the JSON served by the real router against the registered operation inventory. Preserve API behavior.

**Toolchain:** Go 1.26; Swaggo v1.16.6. Prefix Go commands with `PATH="/usr/local/go/bin:$PATH"`.

## Global Constraints

- Work only in `/home/obsidian/dev/wzap/.worktrees/document-all-http-routes`. This is a snapshot of the user's dirty tree; do not commit, stash, reset, discard or reformat unrelated work.
- Source write scope for the implementer: `internal/httpapi/` documentation, Swagger tests and only minimal reusable documentation DTOs if necessary; comments in `manager/manager.go`; generated `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`; Swagger instructions in `README.md`. No other edits. The controller owns OpenSpec files.
- Keep actual handlers, request validation, persistence and auth behavior unchanged. Prefer the existing `envelope` with inline schema composition. Do not add dependencies.
- No subagents from implementer; controller handles review. Do not run DB/NATS/WhatsApp/Chatwoot external integration tests.
- Every brief/report includes checked evidence and limitations. Task completion requires its verification.

### Task 1: Complete annotations, response schemas and regression coverage (tasks 1.1–1.3)

**Read first:** this task's scope and Global Constraints are copied into the brief. Read `design.md`, delta spec and relevant current handlers before editing.

1. Add meaningful tests of `/swagger/doc.json` served by `newTestServer`. Use Go AST to extract literal method/path registrations in `server.go`; require the corresponding documented operation, not just path. Avoid a duplicate list of all routes. Detect unsupported nonliteral patterns in the registrations you inspect. Check required path parameters, JSON success envelopes/array item schemas and real exceptions with a small set of independently chosen expectations. Observe RED before annotation edits.
2. Add annotations for GET/PUT `/instances/{id}/chatwoot`, POST `/instances/{id}/chatwoot/import`, POST `/instances/{id}/chatwoot/command`, POST `/chatwoot/webhook/{id}`, GET `/manager`, and optionally GET `/manager/` for the mounted public console. Describe public/private access, ownership restrictions, request schemas and actual status codes. Configuration Token is write-only: response is empty string. Import returns the count already imported with 202, not a job. Public webhook returns raw `{"content":""}` with 200 and no apikey security; operational commands are authenticated separately. Manager redirects 301 when built and returns plain 503 when no bundle exists; `/manager/` HTML must not get a REST envelope.
3. Fix success annotations throughout `internal/httpapi` using `envelope{data=ExistingResponse}` or `envelope{data=[]ExistingResponse}`. Preserve typed models. Keep actual no-body 204 and binary media schemas. `/readyz` uses `data` on both 200 and 503. `/healthz` is a data-wrapped status object. Standard errors use `errorEnvelope`. Correct inaccurate operation content types (e.g. body-less requests) and annotate response headers only where actually sent. Do not change runtime logic to satisfy docs.
4. Correct the global description and credential descriptions: normal REST JSON successes/errors use envelopes, exceptions have own formats, global/instance apikey scopes differ, session cookie is accepted by dual-auth API routes. Swagger 2.0 cannot declare cookie security. Header parameters must not make apikey mandatory where cookie authentication suffices; retain valid apikey security for machine test calls.
5. Regenerate with `PATH="/usr/local/go/bin:$PATH" go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs`. If using optional manager entry annotations, ensure this command discovers them without changing CI flags. Fix documentation generation errors. Update README only to clarify full coverage, response formats and pinned regeneration workflow.
6. GREEN: run focused `go test ./internal/httpapi -run TestSwagger -count=1`, full httpapi package tests, `go test ./manager -count=1`; run generator again and compare output byte-for-byte. Format changed Go files. Controller runs the whole suite once.
7. Self-review own edits relative to `/tmp/wzap-swagger-baseline-9rvgvrup`, not HEAD (which includes user changes). Write a report containing files/lines, RED/GREEN command outputs, operation counts, exceptions, freshness evidence and concerns. Do not commit.

### Task 2: Verify, review and deliver (controller; tasks 2.1–2.2)

1. Generate a review patch relative to the initial snapshot, have independent review verify spec compliance and quality, and fix substantive findings.
2. Run gofmt, go vet, pinned golangci-lint v2.13.2 (install only if absent), `go test ./... -count=1`, `go build ./...`; record exact evidence and any pre-existing failures. Do not claim optional integration tests were run.
3. Write verify.md and retrospective.md, update only tasks whose checks pass. Validate OpenSpec strictly.
4. Confirm original changed files still match the initial snapshot; apply only this task's patch plus its new OpenSpec directory back to the original workspace. Run focused Swagger tests and a generation freshness check there. No push, PR or merge is requested.
