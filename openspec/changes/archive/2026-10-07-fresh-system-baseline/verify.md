# verify.md — fresh-system-baseline

Evidence collected 2026-10-07 from worktree `codex/fresh-system-baseline` after gates and local reset.

## Check 1 — Schema and identity (tasks 2.x)

- **Real:** `WZAP_TEST_DATABASE_URL='postgres://wzap:secret@127.0.0.1:5435/wzap_test?sslmode=disable' go test ./internal/storage/postgres -run TestMigrate -count=1` — single `00001_init.sql`, 15 tables, no `remodel_report`.
- **Real:** Repository/name tests enforce global unique names and mandatory `owner_user_id` (`internal/storage/postgres/instances_name_test.go`, `internal/instance/name_test.go`).
- **Fake/skipped:** None for schema path.

## Check 2 — Boot, secrets, events (tasks 3.x)

- **Real:** Postgres integration tests for seed (`cmd/wzap/seed_test.go`), Chatwoot seal (`internal/storage/postgres/chatwoot_test.go`), log format (`internal/config/config_test.go`), outbox relay (`internal/events/relay_test.go`).
- **Real (NATS):** `WZAP_TEST_NATS_URL='nats://127.0.0.1:4224' go test ./internal/events/ -count=1` — publisher integration against compose NATS.
- **Preserved:** `pending:` correlations — `internal/chatwoot/inbound/webhook.go:67`, `internal/chatwoot/mirror/worker.go:530`.

## Check 3 — REST contract and idempotency (tasks 4.x)

- **Real:** `go test ./internal/httpapi ./internal/message ./internal/app ./internal/session/... -count=1` on port 5435; historical replay converter tests removed (`replay_contract_test.go` deleted).
- **Real:** Structured errors without `legacy_error`; revoke without `reason` field (`internal/httpapi/dto.go`, swagger regen in commit `fdf1d33`).

## Check 4 — Media and Manager (tasks 5.x)

- **Real:** `go test ./internal/media ./internal/storage/postgres -count=1`; row bucket authority, disk `local` bucket, S3 put failure does not write disk (`internal/media/objects_test.go`).
- **Real (S3):** `WZAP_TEST_S3_ENDPOINT=http://127.0.0.1:9000` MinIO integration `TestObjectStoreS3Integration` pass after explicit bucket args.
- **Real (Manager):** `pnpm --dir manager test`, `lint`, `typecheck`, `build`; `node --test manager/tests/instanceName.test.mjs`.
- **Removed:** `media-migrate` subcommand (no references in tree).

## Check 5 — CI gates (tasks 6.x)

| Gate | Command | Result |
| --- | --- | --- |
| gofmt | `gofmt -l .` empty | pass |
| go vet | `go vet ./...` | pass |
| golangci-lint | v2.13.2 `golangci-lint run` | 0 issues |
| go test | `WZAP_TEST_DATABASE_URL=…5435… go test ./... -count=1` | pass |
| go build | `go build ./...` | pass |
| Swagger | `swag init …` committed in `fdf1d33` | pass |
| Manager | pnpm test/lint/typecheck/build | pass |

**Inventory (6.3):** No matches for `media-migrate`, `BackfillOwner`, `convertReplayBody`, `legacy_error`, `DeletePublishedBefore`, `BackfillTokenSeal`, `MigrateLocalFiles`, `instance_name_ambiguous`, `ErrInstanceNameAmbiguous` in `*.go`.

## Check 6 — Local reset and empty install (tasks 7.1–7.2)

- **7.1:** `docker compose down`; removed volumes `wzap_pgdata`, `wzap_natsdata`, `wzap_miniodata`, `wzap_media`; `docker volume ls` shows no `wzap_*` volumes before recreate.
- **7.2:** Rebuilt image from worktree; stack healthy on `127.0.0.1:8081`.
  - `GET /healthz` → `{"data":{"status":"ok"}}`
  - `GET /readyz` → postgres/migrations/nats ok
  - Admin seed login `admin@wzap.local` → `GET /auth/me` role admin
  - `POST /instances` → instance `fresh-baseline-7` created
  - `POST …/messages/media` → `409 conflict` (instance disconnected) — expected without pairing; object store ready at boot (MinIO + `EnsureBucket` in logs)
- **Note:** Host port override file `.compose-7.2-ports.override.yml` used locally because `opmoni-nats-1` holds `127.0.0.1:4222`; internal service network unchanged.

## Check 7 — E2E WhatsApp (task 7.3)

- **Status:** **PENDING** — no dedicated WhatsApp test number / QR session available in this environment.
- **Evidence:** Pairing, outbound send, and inbound webhook delivery were not exercised; do not claim E2E pass.
