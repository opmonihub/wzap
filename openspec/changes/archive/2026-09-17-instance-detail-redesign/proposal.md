## Why

The instance detail page (`manager/app/pages/instances/[id].vue`) is still a stacked-cards scroll while the rest of the manager console already mirrors the upstream Nuxt UI dashboard template (home, customers/instances list, inbox shell). Only ~8 of the ~30 instance-scoped API routes are reachable from the UI, forcing operators back to raw REST for groups, newsletters, statuses, profile/privacy, presence, pair-phone, media download, and Chatwoot.

## What Changes

- Rework `[id].vue` into the template pattern: `UDashboardPanel` + `UDashboardNavbar` (kept) + `UDashboardToolbar` with horizontal `UNavigationMenu` section nav (exact `settings.vue` pattern) for 7 sections: overview, messages, groups, channels, profile, integrations, settings.
- Add a stats strip under the toolbar (`UPageGrid` 4x `UPageCard variant="subtle"` with the `HomeStats.vue` `:ui` passthrough verbatim): Connection (status/JID/last_connected), Identity (created/updated/external_ref), Webhook (enabled + subscribed count), Chatwoot (enabled/disabled / global-off notice).
- Expose every instance-scoped resource in `internal/httpapi/server.go` through template primitives: extended message composer (text/media/location/contact/poll/reaction/list/buttons) + revoke/mark-read/presence; groups CRUD + photo/participants/invite/join/leave; newsletters follow/unfollow/get/list + status publish/list/delete; profile get/patch/photo + privacy get/put + presence + reject-call; webhook + Chatwoot config/import/command.
- Messages section adopts the full-width inbox master-detail pattern (`InboxList` selectable rows + `InboxMail` detail + footer composer, mobile `USlideover` via `useBreakpoints`); groups/channels adopt the WhatsApp-conversation rows on `UPageCard subtle` + `divide-y` (`settings/members.vue` pattern).
- Extend `app/types/api.ts` and add 5 composables (`useInstanceMessaging`, `useInstanceGroups`, `useInstanceChannels`, `useInstanceProfile`, `useInstanceChatwoot`); add leaf components (`PairPhoneCard`, `MessageComposer`, `MessageDetail`, `ConversationList`, `MessageActionsCard`, `GroupDetail`, `ChannelsCard`, `ProfileCard`, `PrivacyCard`, `DeviceActionsCard`, `ChatwootCard`, `InstanceHeaderStats`) while preserving existing child contracts and `data-testid`s.
- Add i18n copy under `instances.*` in `en.json` only; no literals in templates; server 422 messages surface verbatim.

## Capabilities

### New Capabilities

- None — no new backend capability; all server routes already exist and their specs are unchanged.

### Modified Capabilities

- `wzap-manager`: instance detail composition changes from stacked-cards scroll to header + stats + 7-section toolbar nav with full API coverage (overview, messages, groups, channels, profile, integrations, settings), keeping load/notFound/failure/skeleton, admin-gating, and the CSS-gated messages aside/slideover contract.

## Impact

- Affected code: `manager/app/pages/instances/[id].vue`, `manager/app/types/api.ts`, 5 new composables in `manager/app/composables/`, ~12 new/refactored components in `manager/app/components/instances/`, `manager/i18n/locales/en.json`. No Go changes.
- APIs: no route, auth, RBAC, envelope, or event-contract changes; manager uses the existing session-cookie client (`useApi().api` for JSON, `raw` for 204/binary).
- Dependencies: none new; reuses upstream template `:ui` passthroughs verbatim and existing Nuxt UI tokens (light/dark free).
- Validation: `pnpm --dir manager typecheck`, `pnpm --dir manager lint`, `pnpm --dir manager build` (`nuxt generate`), `gofmt -l .` empty, `go test ./... -count=1` (unit only).

## Out-of-Scope

- No Go, route, auth/RBAC, migration, NATS, or `openspec/specs/` contract changes; no `has_api_key` API flag (browser `localStorage` marker stays, debt noted).
- No new design system and no invented `:ui` variants — copy template passthroughs verbatim.
- Open `POST /chatwoot/webhook/{id}` stays out of the UI (open-by-design).
- No frontend test runner (manager has no vitest script); no `v-if` gating or lifted fetch for the messages aside/slideover (documented no-double-fetch rule stays).
