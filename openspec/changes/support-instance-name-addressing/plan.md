# Instance name addressing implementation plan

## Global Constraints

- Names are exact case-sensitive ASCII strings of 1–64 characters matching `^[A-Za-z0-9](?:[A-Za-z0-9_-]{0,62}[A-Za-z0-9])?$`; reserve exact `stats` and all UUID-parseable strings.
- Invalid names return `422 invalid_instance_name`; occupied names return `409 instance_name_taken`; ambiguous valid legacy name references return `409 instance_name_ambiguous`; missing references return 404.
- New names and actual renames are globally unique through transactional repository writers; unchanged legacy names and all existing UUIDs remain usable.
- All instance paths support UUID/name; downstream identities remain UUIDs. User/media IDs and collection authorization remain unchanged.
- Optional `GET /instances/stats?instance=<uuid-or-name>` preserves response shape and ownership checks; instance keys remain forbidden on stats.
- Aliases share replay by canonical UUID and route pattern; current access is checked before cached replay. Ordinary UUID requests preserve lazy handler validation ordering.
- No SQL migration, record normalization, cross-request name cache, committed secrets or unrelated-user-file edits.
- Each writer has exclusive path ownership. Only the controller updates tasks, review records and integration artifacts.

## Task 1.1 — Naming/domain/storage

Write scope: `internal/instance/` name/service implementation and tests; shared validator leaf `internal/model/instance_name.go` if storage needs under-lock validation; `internal/storage/repository.go`; `internal/storage/postgres/instances.go` and name tests; interface-only fake adapters in `internal/app/runtime_test.go`, `internal/session/whatsmeow/manager_test.go`.

1. Add failing domain grammar/rename tests, including no persistence/key issuance on invalid names and unchanged legacy names.
2. Add storage sentinels `ErrInstanceNameTaken` and `ErrInstanceNameAmbiguous`, domain equivalents and `ErrInvalidInstanceName`, plus `GetByName` interfaces and exact fake adapters.
3. Implement name validator exported for HTTP, create validation and changed-name-only update validation; map storage errors consistently.
4. Add actual PostgreSQL tests for exact case, missing/ambiguous name and raw-SQL legacy rows; concurrent create/create, rename/rename, create/rename and same-row updates.
5. Implement transactions: schema+name advisory lock, separate post-lock occupancy query; Update reads current name under row lock before deciding whether to claim a new name, and validates every actual stored-name change under that lock. Preserve other error maps.
6. Run `/usr/local/go/bin/go test ./internal/instance ./internal/storage/postgres ./internal/app ./internal/session/whatsmeow -count=1`; run focused new PostgreSQL tests with `WZAP_TEST_DATABASE_URL=postgres://wzap:secret@127.0.0.1:5435/wzap_test?sslmode=disable`; format and commit only owned paths.

## Task 2.1 — Transport/docs

Write scope: `internal/httpapi/` including annotations/tests; generated `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`; `README.md`. Start once Task 1 exposes the fixed domain interfaces; transport verification waits for completed backend code.

1. Add failing actual-router tests for UUID/name reads, status, mutation, public Chatwoot; typed conflicts/missing names and reserved paths.
2. Add GetByName to HTTP InstanceService and fakes. Implement instance-only registration adapter and open webhook resolver. UUID interpretation takes precedence. For instance-key names compare its own UUID row's current name, refusing foreign names without foreign lookup.
3. Outside idempotent handlers check current authorization before keyed replay, then pass canonical UUID PathValue into existing middleware; keep ordinary UUID operations lazy and avoid rewriting URL.
4. Add tests for UUID↔name replay, foreign ownership/key and changed ownership, rename replay, no extra operation, route/path safety.
5. Add optional stats query, collection-first authorization and single-target aggregation tests; no query retains existing totals.
6. Update every instance-reference annotation (including open webhook), create/update name rules and errors, Swagger contracts and README BREAKING note; keep other IDs unchanged.
7. Focused HTTP tests, `/usr/local/go/bin/go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -g internal/httpapi/swagger.go -o docs`, Swagger contract/freshness checks; commit owned paths only.

## Task 3.1 — Manager

Write scope: shared `manager/app/utils/instanceName.ts` and tests; three name forms CreateInstanceModal.vue, EditInstanceModal.vue, detail/InstanceOverviewSection.vue; `manager/app/types/api.ts`, `manager/app/composables/useInstances.ts`, `manager/i18n/locales/en.json` as needed. No dependency/version changes.

1. Add failing helper checks for grammar, UUID reservations including compact forms, exact stats, case, bounds and unchanged-legacy name omission.
2. Implement helper and use in all three schemas. Preserve exactly unchanged originals rather than trimming them into renames; submit changed names only.
3. Show rule guidance and handle `instance_name_taken` separately from external-ref conflicts using existing error contract.
4. Run meaningful helper behavioral tests using existing/Node tooling without new dependency, `pnpm --dir manager typecheck` and `pnpm --dir manager build`; commit owned paths only.

## Task 4.1 — Reviews and verification

Controller writes planning artifacts and seven-check verify plus evidence-first retrospective. Fresh scoped reviewer per task, then full branch review. Run gofmt, vet, lint v2.13.2, default complete Go suite, build, Manager gates and generated Swagger freshness. PostgreSQL tests executed only against reachable isolated `_test` database; no live NATS integration against shared stream.

## Task 4.2 — Local integration/deployment

Protect all current root dirty files and compare after merge. Build app image from reviewed clean worktree, recreate only local wzap service. Merge using only feature commit deltas into root preserving unrelated changes; required private fake method adapter stays user-owned/untracked. Re-run integrated gates and selected name PostgreSQL tests. Browser reload Swagger, authorize once, execute name status and targeted stats with equivalent UUID checks. Preserve existing live rows and UUIDs. Record outcome and clean only feature worktree/branch.
