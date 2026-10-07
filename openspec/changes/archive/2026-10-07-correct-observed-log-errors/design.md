## Context

See proposal.md for why. The local compose already mounts `natsdata` at `/data`, but the NATS command is `-js -m 8222`, so JetStream uses `/tmp/nats/jetstream`. `SetGroupPhoto` sends the body to the session and `classifySessionError` turns every unknown upstream error, including "not a valid image", into a transient error that the HTTP layer answers as `500`. `GetProfile` calls user-info and profile-picture with `ParseJID` of the stored JID, which after pairing includes the device suffix (`:N`). That query returned `400 bad-request` and the whole read became `500`, even though the push name was already on the session. `GroupDetail` trims the JID and calls `GET /instances/{id}/groups/{jid}`; an empty field becomes `GET .../groups/` and `404`.

## Goals / Non-Goals

**Goals:**

- Broker store on the mounted volume in both compose files.
- Invalid group-photo bytes stay a client error (`422`) at the session boundary, without a new image decoder in the HTTP handler.
- Profile read uses the user JID (no device suffix) and degrades to the known push name instead of `500` when recado or photo are absent.
- The groups console does not issue a request for a blank JID.

**Non-Goals:**

- A general trailing-slash redirect. See proposal.md, Out of Scope.
- Changing the Chatwoot global gate.
- Reclassifying every transient session error. Only the invalid-image case and the profile read change.

## Decisions

### 1. Pass `-sd /data` to NATS

Both `docker-compose.yml` and `docker-compose.dev.yml` gain `-sd /data` on the existing command. The volume mount stays. No NATS config file.

Alternative: a `nats.conf` with `store_dir`. Rejected because the only missing piece is the store path, and a file would be a second source of truth next to the command.

### 2. Classify "not a valid image" as invalid input in the session adapter

`SetGroupPhoto` maps that upstream failure to the existing invalid-recipient sentinel before the generic transient wrap. The service already turns that sentinel into invalid input, and the HTTP layer already answers invalid input with `422`. The handler keeps the `Content-Type` and size checks it has today. No magic-byte sniff in the transport: Go's image decoder and the upstream decoder disagree on formats (WebP, in particular), and a pre-check would reject photos the upstream would accept.

Alternative: decode in `httpapi` and return `422` before the session call. Rejected for the format mismatch above.

### 3. Profile read uses the bare user JID and stays best-effort

`GetProfile` converts the stored JID to the user JID without the device suffix before user-info and profile-picture. A user-info or picture failure that is not "disconnected" does not fail the read: the response still carries the push name already on the session, with empty recado or empty photo URL, and the adapter logs a warning with the instance identifier only. Disconnected stays `409` via the existing not-connected check.

Alternative: retry the device JID and map `400` to `422`. Rejected because the console would still have no profile, and the `400` is from querying the wrong JID, not from a bad client payload.

### 4. Blank group lookup stops in the console

`onLookup` returns before `getGroup` when the trimmed JID is empty and shows the missing-identifier copy. No new API route for `/groups/`.

Alternative: register `GET /groups/` as the group list. Rejected because the observed call is an empty lookup, not a list, and a slash alias would hide the bug.

## Risks / Trade-offs

- [Risk] Recreating the NATS container drops the stream that currently lives in `/tmp` -> Mitigation: the Postgres outbox remains the source for events not yet published; the fresh volume starts empty and is the durable store from then on. Rollback is reverting the command flag.
- [Risk] Classifying the image error by the upstream text couples us to that wording -> Mitigation: cover it with a unit test that feeds that error and expects the invalid-input sentinel, not the transient one. Do not match a broader substring.
- [Risk] Swallowing a real user-info failure hides an upstream outage behind a `200` with empty recado -> Mitigation: still log a warning; only the disconnected sentinel stays a hard `409`. The spec asks for the push name, not a guaranteed recado.

## Migration Plan

1. Change the NATS command in both compose files.
2. Recreate the broker container so the new flag applies. Do not copy `/tmp` into the volume.
3. Ship the session and console changes with the service image. No schema migration.

## Open Questions

None.
