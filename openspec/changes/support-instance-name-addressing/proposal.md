## Why

API clients currently need UUIDs even when they know an instance's name, and stats cannot select one instance. Names used as path references need an explicit safe, unique contract.

## What Changes

- Accept UUID or exact instance name in all instance-scoped API paths, including the open Chatwoot webhook, retaining canonical UUID identity internally.
- Add optional `instance` UUID/name query to `/instances/stats`; retain the existing response shape and authorization.
- **BREAKING**: validate new names and renames as unique, case-sensitive URL-safe ASCII names of 1–64 characters; reserve `stats` and UUID-shaped strings.
- Preserve existing rows, UUID access and unchanged legacy names; reject ambiguous legacy-name references with 409.
- Explain validation in Manager forms and generated Swagger, including separate name/external-reference conflict messages.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `wzap-instances`: safe unique names, UUID/name references, scoped stats and canonical identity.
- `wzap-manager`: consistent name validation and legacy-name editing.
- `wzap-operations`: documented name references and unchanged authentication across aliases and replay.

## Impact

Instance service and repositories, HTTP route resolution, stats, idempotency authorization, Manager forms and Swagger annotations/artifacts. No database migration or event-format change.

## Out-of-Scope

Renaming existing records automatically, changing user/media identifiers, adding another display-name field, changing collection authorization, changing unrelated pagination, external integrations or direct-SQL enforcement.
