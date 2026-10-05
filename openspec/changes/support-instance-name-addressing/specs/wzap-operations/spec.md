## ADDED Requirements

### Requirement: Authorization and documentation for instance aliases

Swagger SHALL describe instance path parameters as UUID or name and document targeted stats, validation and conflict responses. Aliases MUST preserve existing authorization; an instance key MUST only operate its own current name or UUID. Idempotent replay MUST recheck current access before returning a cached response, and using a UUID/name alias with the same key and request MUST share the canonical instance's replay.

#### Scenario: Swagger name input
- **WHEN** a reader opens an instance-scoped operation or stats
- **THEN** the documentation explains UUID/name references and exposes the optional stats target

#### Scenario: Alias replay
- **WHEN** an authorized client repeats an identical idempotent request and key using the other reference for the same instance
- **THEN** the previous response is replayed without repeating the operation

#### Scenario: Access revoked before replay
- **WHEN** a client lacks current ownership or uses a foreign instance key against a cached operation
- **THEN** the response is 403 and the cached response is not exposed
