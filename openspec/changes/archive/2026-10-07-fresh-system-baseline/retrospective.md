# retrospective.md — fresh-system-baseline

## Outcome

Fresh-install baseline landed: single initial migration, legacy compatibility paths removed, current gateway flows preserved (events v1, Chatwoot import, `pending:{uuid}`, RBAC/privacy tests). Tasks **25/26** complete; **7.3 E2E WhatsApp pending** (no test number).

## What went well

- Sequential waves kept shared packages stable; media bucket semantics clarified with focused tests.
- Full `go test ./...` with Postgres on host port **5435** matched the post-reset compose mapping.
- Manager contract aligned with Go DTOs (ambiguous name handling removed, revoke `reason` dropped).

## Friction

- Dual SDD controllers earlier in the change risked duplicate commits; resumed from ledger + `git log`.
- Local compose default ports conflicted with `opmoni-nats` on **4222**; 7.2 used a ephemeral compose override for host binds only.
- S3 integration test initially failed after removing empty-bucket fallback until tests passed explicit row buckets.

## Follow-ups

- Run **7.3** when a test number is available (pair → send → webhook capture with redacted evidence).
- Consider documenting optional host port overrides for multi-project NATS on one machine.
- Merge branch after review; main checkout should stay stopped or use disjoint DB ports when developing in parallel.
