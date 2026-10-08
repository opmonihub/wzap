# wzap

Multi-instance WhatsApp gateway (Go): REST + NATS JetStream + embedded Nuxt
manager + optional Chatwoot. One replica. Contract/envs: [README.md](README.md).
Specs: `openspec/specs/`. Workflow: `openspec/config.yaml`. Manager:
[manager/AGENTS.md](manager/AGENTS.md).

## Setup commands

- `docker compose up -d wzap` — Postgres 18, NATS, MinIO, `127.0.0.1:8081` (key `dev-wzap-token`)
- `docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d --build wzap manager-dev` — Air + Nuxt at `127.0.0.1:8081/manager/`; insert local override between base/dev when present
- `docker compose exec postgres createdb -U wzap wzap_test` — once per volume
- Binary: `serve` (default), `migrate`, `healthcheck`

## Testing

- `go test ./... -count=1` · focused: `go test ./internal/<pkg> -run TestName -count=1`
- After every frontend interface change, use `agent-browser` against the updated
  local UI to manually exercise affected flows and visually validate rendering,
  relevant states and responsive layouts before declaring completion. Capture
  screenshots and record the tested flows/results as evidence.
- Postgres (skip if unset; FAIL unless DB name ends in `_test`; isolated schema via `postgrestest` — never touch the shared schema):
  `WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5432/wzap_test?sslmode=disable' go test ./... -count=1`
- NATS: `WZAP_TEST_NATS_URL='nats://127.0.0.1:4222' go test ./internal/events/ -count=1`
- Do not claim integration tests ran unless the var was set and the DB reachable.
- CI: `gofmt` clean, `go vet`, golangci-lint v2.13.2 (govet/staticcheck/errcheck/ineffassign/unused), `go test` with the Postgres URL, swagger freshness, `go build`.
- After handler annotation edits: `go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs` then `git diff --exit-code -- docs/`.

## Code style

Go 1.26, `gofmt`, imports `wzap/...`, logger from `internal/logger` by value.
Handlers stay thin. Chi v5 is the only HTTP router. Keep transport organized in
`internal/httpapi/core`, `representation` and resource packages `instances`,
`messages`, `groups`, `contacts`, `channels`, `chats`, `statuses`, `profile`,
`users`, `authsession`, `media` and `chatwoot`; resource packages own their
handlers and route registration and must not depend on the root composition
package. Instance writes are per satellite (identity / connection / webhook /
chat_settings) — never snapshot-UPDATE. Commits: `feat(api):` / `fix(events):` /
`docs(specs):`. Mark contract breaks **BREAKING** in the change and delivery.

## Architecture

`cmd/wzap/main.go` wires: `httpapi` (REST, `apikey:` or cookie, RBAC, idempotency),
`auth`, `instance` (lifecycle + parity), `session`/`whatsmeow`, `app` (inbound),
`message` (outbound), `events` (envelope v1 + relay), `webhook` (Writer fan-out;
relay reads DB), `media` (S3 if `WZAP_S3_ENDPOINT`, else disk), `storage`,
`chatwoot`, `replicalock`.

- `{id}` = UUID (wins) or exact name. Keys never query a foreign name. `stats` and UUID-shaped names reserved. Rename 404s the old alias.
- REST collections are direct named arrays in `data`: `instances`, `users`, `groups`, `messages`, `channels`, `statuses`, `contacts` and `blocked_jids`; elements have no singular wrapper. Channel updates return `messages`. `next_cursor`, when present, is a sibling of the collection in `data` and is omitted at the end.
- Required arrays are non-nil, have no `omitempty`, and serialize empty as `[]`, including `events`. Omit optional fields and objects without values, including optional error details. Preserve valid `false`, `0`, and `"0"` values; model optional zero-capable fields with pointers. `integration.webhook` remains required, including when disabled. `settings` and `chatwoot_config` are omitted when unavailable.
- Hide `external_ref`, `owner_user_id`, `device_jid` and tokens. `GET /instances` is unpaginated.
- Boot: lock → migrate → NATS (serve if down) → seed → S3 → WA version → restore. Shutdown 10s: HTTP → outbox → cleaner → webhook → Chatwoot mirror → import scheduler → relay. Second signal aborts. `chatimport.Pool.Close` only in `serve()`.
- Schema: fresh-system baseline `00001`; the historical migration/cutover chain is not shipped.
- Session writes are sync (no outbox) except status publish (`202`, fire-and-forget). Profile name/photo → `501`.
- Chatwoot: seal tokens with `WZAP_CHATWOOT_TOKEN_KEY`; commands only on `POST .../chatwoot/command`; import inert without `WZAP_CHATWOOT_IMPORT_DB_URL`.

## Boundaries

- Always: focused tests + `gofmt`; regen `docs/` after Swagger edits; stable `event_id`.
- Ask first: new deps beyond the approved Chi v5 router, `.github/workflows/`, unplanned migrations.
- Never: commit secrets/`.env`; shared-schema tests; second replica on the same DB; leak token/password/`external_ref`/`owner_user_id`/`device_jid`; invent tenant/account domain; write plans to `docs/superpowers/`.

## Spec-driven workflow

OpenSpec = WHAT/WHY; `tasks.md` is the scope contract.
Starting/resuming `openspec-apply-change` MUST invoke and follow Superpowers:
`using-git-worktrees` → `writing-plans` (create/reuse `plan.md` before code) →
`subagent-driven-development` (implementer + spec/quality reviewer subagents;
TDD, `systematic-debugging` on failures) → `requesting-code-review` →
`verification-before-completion` → `finishing-a-development-branch`.
Artifacts: `openspec/changes/<name>/` (`plan.md`, `verify.md`, `retrospective.md`);
archive last. Worktrees: `.worktrees/<name>` only — never `.kilo/worktrees/`.

## Local agents

Machine-local skills (`.factory/skills`, `.codex/skills` symlink). Droids:
explorer/log-detective (RO), implementer (scoped write), griller. One writer per
file. Cross Go+manager: fix the contract first, then one implementer per side.
Briefs return `file:line` evidence; do not redo a subagent search.
