## Context

See proposal.md for motivation. Instance paths currently parse UUIDs; static `/instances/stats` shadows a name `stats`. Idempotency uses route pattern and instance UUID but currently replays before handler ownership checks. Name columns have no unique constraint. Existing live legacy names and the user's uncommitted migration 00007 must be preserved.

## Goals / Non-Goals

Resolve HTTP references at the transport boundary and keep all downstream contracts UUID-based. Preserve lazy UUID handler validation ordering. No cross-request name cache, URL rewriting, database migration, record normalization or unrelated route changes.

## Decisions

1. Domain name validator shared by create/rename and HTTP name reference checks: exact case-sensitive ASCII grammar, 1–64 characters, alphanumeric ends, reserved stats/UUID namespace. Manager duplicates this small explicit contract in one shared helper. No silent slug conversion.
2. Add `GetByName(ctx,name)` to repository/domain/HTTP contracts; exact SQL equality with at most two rows identifies legacy ambiguity. Domain/storage sentinels distinguish invalid, taken and ambiguous names.
3. PostgreSQL Create and Update use transactions and schema-scoped exact-name advisory locks. After obtaining the lock, a separate statement checks occupancy before writing under READ COMMITTED. Update first locks/reads the current row so unchanged legacy values are accepted using the current database name. A unique index was considered but would reject existing duplicates and conflict with pending migration numbering; no migration is introduced. Guarantee applies to repository writers following this protocol, not direct SQL or older binaries.
4. A mux registration adapter wraps only instance path patterns after Go's mux sets PathValue. Names resolve to canonical UUID; ordinary UUID paths remain lazy. Idempotency registrations require current access before any replay when a replay key is supplied. Preserve instance-key anti-enumeration by resolving its own UUID and comparing the requested name. Public Chatwoot uses resolution without authentication, then retains canonical limiter/connector identity.
5. `r.Pattern` already normalizes route identity for idempotency, so only PathValue changes; no URL mutation. No extra query/body fingerprint change.
6. Stats checks collection authorization first, then resolves and authorizes optional `instance`; it reuses the existing status aggregation and response shape. Existing `/instances/{id}/status` remains the detailed status route.
7. Manager edit forms omit exactly unchanged names and show distinct server name-conflict errors. UUID links remain stable through renames.

## Risks / Trade-offs

- [Risk] Direct SQL or older code can introduce duplicates -> lookup rejects ambiguity; repository atomic concurrency tests verify participating writers.
- [Risk] Legacy unsafe names cannot be used as new aliases -> retain UUID access and permit deliberate valid renames without rewriting rows.
- [Risk] New replay access checks expose fixture assumptions -> test actual router auth and preserve ordinary lazy UUID validation ordering.
- [Risk] Root has unrelated dirty work and migration 00007 -> implement/build in a clean `.worktrees/` checkout; protect original files before integration and adapt only required interface methods.

## Migration Plan

No SQL migration or existing-data rewrite. Deploy reviewed branch after Go/Manager/Swagger verification; test live read-only UUID/name and stats requests. Rollback by rebuilding prior committed source; UUIDs and stored names remain compatible. Merge locally to main as already requested, retain unrelated dirty work, then remove the manual worktree.
