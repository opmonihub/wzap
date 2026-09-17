## 1. Types and i18n foundations

- [x] 1.1 Extend `manager/app/types/api.ts` with Group, Newsletter, OwnStatus, Profile, Privacy, PairPhoneResult, Chatwoot types plus rich-send/presence/revoke/mark-read/reject-call inputs and verify `pnpm --dir manager typecheck` passes
- [x] 1.2 Add `instances.*` keys for groups/channels/profile/privacy/chatwoot/pairPhone/actions to `manager/i18n/locales/en.json` (no template literals) and verify `pnpm --dir manager lint` passes

## 2. Composables

- [x] 2.1 Create `useInstanceMessaging` (location/contact/rich sends with fresh Idempotency-Key, revoke, mark-read, presence, media download via `raw()` blob URL) and verify typecheck passes
- [x] 2.2 Create `useInstanceGroups` (create/get/update/photo/participants/invite/reset/join/leave with 25-char name rule and empty-invite reconcile) and verify typecheck passes
- [x] 2.3 Create `useInstanceChannels` (newsletters follow/unfollow/get/list; status publish/list/delete with 1..700 text rule and image|video kinds) and verify typecheck passes
- [x] 2.4 Create `useInstanceProfile` (profile get/patch/photo octet-stream PUT, privacy get/put allowlists, pair-phone with expiry, reject-call with 501 notice) and verify typecheck passes
- [x] 2.5 Create `useInstanceChatwoot` (get/put/import/command with `chatwoot_disabled` 400 detection and masked token) and verify typecheck passes

## 3. Leaf components

- [x] 3.1 Build `InstanceHeaderStats` (4x `UPageCard subtle` with verbatim `HomeStats.vue` `:ui`) plus `PairPhoneCard` (Connect-first ordering, 8-digit code + expiry) and verify they render in isolation via `pnpm --dir manager build`
- [x] 3.2 Build `ConversationList` (shared WhatsApp-style selectable rows) plus `MessageDetail` (InboxMail pattern) and `MessageComposer` (footer reply card with text/media/location/contact/poll/reaction/list/buttons sub-tabs) and verify `pnpm --dir manager build` succeeds
- [x] 3.3 Build `MessageActionsCard` (revoke/mark-read/presence row) plus `GroupDetail` (update/invite/photo/participants/leave) and `ChannelsCard` (newsletters + status bubbles) and verify `pnpm --dir manager build` succeeds
- [x] 3.4 Build `ProfileCard`, `PrivacyCard`, `DeviceActionsCard` (presence + reject-call), and `ChatwootCard` (config/import/command + computed webhook URL) preserving `data-testid`s and verify `pnpm --dir manager build` succeeds

## 4. Page wiring

- [x] 4.1 Rework `[id].vue` to header stats + toolbar `UNavigationMenu` with 7 sections while preserving load/notFound/failure/skeleton, `deleteOpen`, `onPaired`, `onWebhookUpdated`, `messagesRefresh`, admin-gating, and the CSS-gated aside/slideover contract and verify `pnpm --dir manager typecheck` passes
- [x] 4.2 Refactor `TestSendCard`/`MessagesCard` to host or delegate to the new composer/detail without changing emit contracts (`paired`/`updated`/`sent`/`settled`) and verify existing detail flows still refresh history

## 5. Verification

- [x] 5.1 Run `pnpm --dir manager typecheck`, `pnpm --dir manager lint`, and `pnpm --dir manager build` (`nuxt generate`) and verify all three succeed
- [x] 5.2 Run `gofmt -l .` (verify empty output), `go vet ./...`, and `go test ./... -count=1` unit-only and verify all succeed without claiming Postgres/NATS integration
- [ ] 5.3 Manually verify connected + disconnected instances across all 7 tabs (happy-path + 409/422 display), mobile slideover + desktop, light/dark, admin vs user key hiding, and Chatwoot enabled vs disabled backend
