## Context

See `proposal.md` (Why) and `specs/wzap-manager/spec.md` for requirements. Current state: `[id].vue` is a stacked-cards scroll covering GET/PATCH/DELETE instance, connect/disconnect, QR (`PairingCard`), `GET /status`, webhook PATCH (`WebhookCard`), apikey rotate/revoke (admin), and `numbers/check` + `messages/text` + `messages/media` + list/get (`TestSendCard`, `MessagesCard`, `useMessages`). Missing: generic `POST /messages` (poll/reaction/list/buttons), location, contact, revoke, mark-read, presence, pair-phone, 8 groups routes, 4 newsletters routes, 4 status routes, calls/reject, profile get/patch/photo, privacy get/put, `GET /media/{id}`, Chatwoot PUT/GET/import/command. Upstream template at `/home/obsidian/dev/research/dashboard` (outside repo) provides the verified patterns; manager already mirrors home/customers/inbox-shell. Constraints: session-cookie client only (`useApi().api` JSON, `raw` for 204/binary); `data-testid`s stable; i18n EN-only under `instances.*`; no Go changes; message aside/slideover mounts via CSS gating (documented redundant fetch on mobile accepted).

## Goals / Non-Goals

- Goals: 7-section detail composition on template primitives with verbatim `:ui` passthroughs; full instance-scoped API coverage with per-endpoint validation mirroring Go handlers; preserved load/notFound/failure/skeleton, admin-gating, and child emit contracts (`paired`/`updated`/`sent`/`settled` → `messagesRefresh`).
- Non-Goals: no backend, auth, RBAC, migration, or event-contract work; no `has_api_key` flag; no new test runner; no visual redesign beyond the template (no invented variants).

## Decisions

- **Section nav = `UNavigationMenu` in `UDashboardToolbar` (settings.vue pattern), not `UTabs`.** Rationale: user-confirmed; toolbar pattern already proven in template settings and keeps header/stats stable across sections. Alternative (`UTabs` content-switch) rejected: breaks toolbar alignment and duplicates tab state.
- **Stats = `HomeStats.vue` `UPageGrid`/`UPageCard` passthrough verbatim (4 cards, `lg:grid-cols-4`, `variant="subtle"` + exact `:ui`).** Rationale: free theming/dark mode via tokens; no invented counts (Connection/Identity/Webhook/Chatwoot only). Alternative (custom stat cards) rejected: visual drift + token debt.
- **Messages = inbox master-detail full-width; groups/channels = WhatsApp-conversation rows on `UPageCard subtle` + `divide-y`.** Rationale: reuses `inbox.vue`/`InboxList`/`InboxMail` (selectable rows, arrow-key shortcuts, `USlideover` + `useBreakpoints` mobile) and `settings/members.vue` rows; only exception to `lg:max-w-2xl` body is messages. Alternative (uniform `lg:max-w-2xl` cards for messages) rejected: history + composer need the split width.
- **Types → composables → leaf components → page wiring → i18n order.** Rationale: extends `types/api.ts` first (no behavior change), then 5 composables mirroring `useMessages.ts` idempotency + `ApiError` style, then leaves (`PairPhoneCard`, `MessageComposer`, `MessageDetail`, `ConversationList`, `MessageActionsCard`, `GroupDetail`, `ChannelsCard`, `ProfileCard`, `PrivacyCard`, `DeviceActionsCard`, `ChatwootCard`, `InstanceHeaderStats`), then `[id].vue` wiring. `TestSendCard`/`MessagesCard` delegate rather than fork to keep `data-testid`s. Alternative (page-first) rejected: child contracts would churn.
- **Idempotency helper kept local per composable (not shared via `useMessages`).** Rationale: avoids contract churn on the existing helper; each send path mints `crypto.randomUUID()` per click. Alternative (export shared helper) rejected: touches a stable contract for no behavioral gain.
- **Group-create 201 with empty `invite_code` reconciles via GET invite, never retries create.** Rationale: mirrors `handleCreateGroup` partial-create semantics; retry would duplicate the group. Same for status fire-and-forget and import partial-count display.

## Risks / Trade-offs

- [Risk] Desktop aside mounts + fetches on every viewport (CSS gating), mobile slideover adds one redundant fetch → Accepted and documented; mitigation: keep as-is, do not "fix" with `v-if` (reintroduces SSR double-mount) or lifted fetch (breaks child contracts).
- [Risk] `501 not_supported` (profile name/photo, reject-call) and `chatwoot_disabled` 400 confuse operators → Mitigation: explicit unsupported/info notices, forms disabled, Chatwoot GET masks token.
- [Risk] Reused `Idempotency-Key` across payloads → 422 → Mitigation: fresh key per click, never cache.
- [Risk] Phone/token/key leakage in logs/UI → Mitigation: never log phones/tokens; pair-phone honors `expires_at`; Chatwoot token write-only.
- [Risk] Scope creep (media cleaner, webhook worker, relay) → Mitigation: transport-only change; shutdown order, outbox, and relay untouched.

## Migration Plan

- Frontend-only, no migration: deploy with the service binary (manager is embedded via `go:embed`, SPA fallback). Rollback = previous binary. No feature flag; old stacked layout is fully replaced.
- Validation sequence: `pnpm --dir manager typecheck` → `pnpm --dir manager lint` → `pnpm --dir manager build` (`nuxt generate`) → `gofmt -l .` empty → `go test ./... -count=1` (unit only). Manual matrix: connected/disconnected × each tab happy-path + 409/422 × mobile/desktop × light/dark × admin/user × Chatwoot enabled/disabled.

## Open Questions

- None — template source, route inventory (`internal/httpapi/server.go`), validation limits, and section map are all settled in the planning handover.
