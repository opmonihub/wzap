## ADDED Requirements

### Requirement: URL-safe unique instance names

**BREAKING**: New names and actual renames SHALL be globally unique, case-sensitive ASCII strings of 1–64 characters containing letters, digits, hyphens or underscores, with an alphanumeric first and last character. The exact name `stats` and every string interpreted as a UUID SHALL be reserved. Invalid names MUST return `422 invalid_instance_name`; occupied names MUST return `409 instance_name_taken`, without changing the instance or issuing a key. Existing rows MUST retain their names and UUIDs, and updates with an exactly unchanged legacy name MUST remain possible.

#### Scenario: Valid new name
- **WHEN** an authorized client creates `Loja_SP-1` with an available name
- **THEN** the instance is created with that exact name

#### Scenario: Invalid or reserved name
- **WHEN** a client creates or renames to a name outside the grammar, `stats`, or a UUID-shaped string
- **THEN** the response is 422 and no write or key issuance occurs

#### Scenario: Concurrent name claims
- **WHEN** creates or renames concurrently claim the same available exact name
- **THEN** exactly one succeeds and the other receives 409

#### Scenario: Legacy value retained
- **WHEN** an existing instance with an invalid or duplicate legacy name receives an unrelated update
- **THEN** its name and UUID remain unchanged and the update succeeds

### Requirement: Instance references by UUID or name

Every instance-scoped API path, including the public Chatwoot webhook, SHALL accept either an instance UUID or its exact valid name. UUIDs SHALL take precedence and remain immutable identity for keys, sessions, messages, events, rate limits and idempotency. Missing references MUST return 404; ambiguous legacy valid names MUST return `409 instance_name_ambiguous`. Renaming SHALL immediately change name resolution without changing UUID access.

#### Scenario: Equivalent path references
- **WHEN** a client reads or operates an instance using its UUID or exact name
- **THEN** both address the same instance and preserve the operation's response and authorization

#### Scenario: Rename
- **WHEN** an instance is renamed
- **THEN** the old name no longer resolves, the new name resolves, and its original UUID still resolves

#### Scenario: Ambiguous legacy name
- **WHEN** a name reference matches multiple existing rows
- **THEN** the response is 409 and UUID access remains available

### Requirement: Targeted instance stats

`GET /instances/stats` SHALL accept an optional `instance` UUID/name query. With a target it SHALL return the existing `total` and `by_status` response counting only that authorized instance; without one it SHALL preserve aggregate stats over the authorized collection. Collection authorization MUST remain required, including rejection of instance keys.

#### Scenario: Target by UUID or name
- **WHEN** an authorized collection client selects an accessible instance by either reference
- **THEN** total is one and the correct status bucket is one

#### Scenario: Missing or foreign target
- **WHEN** the target is missing or belongs to another non-admin user's scope
- **THEN** the response is 404 or 403 respectively

#### Scenario: Unfiltered stats
- **WHEN** the query is absent
- **THEN** existing collection totals and status buckets are returned
