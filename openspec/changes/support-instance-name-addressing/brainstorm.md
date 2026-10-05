# Request and decisions

The user requested UUID or instance name in API paths, a UUID/name option for instance stats, and names suitable for URLs, referencing Evolution API. Its router uses `:instanceName`; wzap will retain UUID addressing and add exact names.

Names use ASCII letters, digits, hyphen and underscore, 1–64 characters, starting and ending with a letter or digit. Case is significant. An optional case-policy question was offered; the stated default permits both cases. `stats` and strings accepted as UUIDs are reserved. New names and renames must be globally unique. Existing names and UUIDs remain unchanged; unrelated updates may retain an invalid legacy name. Ambiguous legacy names return 409 rather than selecting a row.

An observed live instance has a legacy name outside the grammar. The user's uncommitted migration 00007 also exists outside this feature. This change adds no migration: transactional repository writers serialize exact-name claims. Direct SQL and older binaries are outside that enforcement protocol.

`GET /instances/{id}/status` already gives individual status. All instance paths will accept a name; `GET /instances/stats?instance=<uuid-or-name>` will count that target while preserving the existing totals response and collection authorization.
