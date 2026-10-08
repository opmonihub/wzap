# Fix frontend QA regressions

Spec: `specs/wzap-manager/spec.md`. Evidence: main checkout `dogfood-output/frontend-2026-10-07/report.md`.

## Global constraints

Work only in `/home/obsidian/dev/wzap/.worktrees/fix-frontend-qa`. Preserve its baseline manager snapshot and all prior fixes. No dependency, backend, workflow, migration, lockfile or generated asset changes. One writer per file; implementer dispatches run sequentially. No commits, merges, pushes or deployment. The controller owns `manager/i18n/locales/en.json`; request copy keys rather than editing it. Manual visual verification uses the controller's owned agent-browser session and the updated local build. Do not spawn children, use other browser sessions, expose secrets or send real WhatsApp commands. Reports and test logs live in this plan's ignored SDD directory.

## Task 1: Forms (1.1)

Files: `manager/app/utils/accountQuota.ts`, `manager/app/components/accounts/{CreateAccountModal,EditQuotaModal}.vue`, `manager/app/components/auth/LoginForm.vue` and focused tests in those same directories. Read this worktree's AGENTS files, debugging/TDD skills and existing component harness.

1. Reproduce current numeric failures using the shared parser and compiled real form schemas/components; log a failing regression before fixing.
2. Accept the actual number/string/undefined numeric input model; empty creation stays omitted, explicit 0 stays 0, positive integers stay numeric; reject negative, fractional, non-finite and invalid text values. Editing remains required.
3. Make untouched login email/password display the existing auth validation copy, including when the model is undefined; preserve invalid-email/min-password and server-error behavior.
4. Run focused Vitest and self-review. Report exact file/line root causes, red and green commands/results, remaining concerns and needed locale keys.

## Task 2: Label associations and names (1.2)

Files: `manager/app/components/shared/DataTableToolbar.vue` (1.4), `manager/app/components/instances/WebhookCard.vue`, `ProfileCard.vue`, `GroupDetail.vue`, `ChannelsCard.vue`, `MessageComposer.vue`, `TestSendCard.vue`, `detail/InstanceSettingsSection.vue`, related focused tests only if they verify behavior rather than copying implementation.

1. Inspect installed Nuxt UI Checkbox/FileUpload/Select injection and attribute forwarding.
2. Give every event checkbox a unique component-scoped id that targets its own label; preserve subscription order, All/None, enabled and URL logic.
3. Add locale-backed accessible names to the actual role-button file upload pickers and the confirmed Groups participants action, Channels status media type and Settings default disappearing selectors.
4. Use existing copy; tell controller any new keys. Check all visible file picker locations in the listed files. Do not change the instance page or section navigation.
5. ISSUE-008: all instance columns are required, so its Display menu is empty; hide the shared column-display dropdown when the provided optional-column list is empty, while keeping Accounts display/toggling working. Do not unlock required instance columns. Browser before evidence is screenshots/30-instance-display-empty-menu.png; verify both tables after build.
6. Use browser pre-fix evidence as the failing accessibility regression for simple attribute changes; focused meaningful webhook tests should cover subscription changes/save payload. The controller then clicks actual labels and checks actual accessible names in the built browser.

## Task 3: Theme and responsive navigation (1.3)

Files: `manager/app/assets/css/main.css`, `manager/app/app.config.ts` if necessary, `manager/app/pages/instances/[id].vue`, `manager/app/components/instances/detail/InstanceSectionNav.vue`, meaningful navigation tests.

1. Inspect installed Nuxt UI semantic aliases and NavigationMenu/SelectMenu APIs; use pre-fix computed contrast and mobile screenshots as failing visual evidence.
2. Override light semantic tokens to suitable existing darker shades; preserve dark aliases, ensure white normal text on primary solid and error actions and colored text on subtle feedback meet 4.5:1 including hover. Avoid blanket global overrides of component internals.
3. Name back action using existing `instances.title` copy (or request a dedicated locale key).
4. At mobile widths provide a labelled selector displaying the full selected section and all seven full choices; keep desktop navigation and section/URL behavior. Minimum accessible mobile tap height 44px. Do not rely on truncated labels or hidden core sections.
5. Run focused tests for actual emitted section selection/current choice. Controller visually checks 320/390px and desktop, light/dark, form errors and alerts, with rendered contrast measurements.

## Task 4: Complete gates (2.1)

Run once after all writers finish:
`pnpm --config.verify-deps-before-run=false --dir manager test`;
`node --test manager/tests/instanceName.test.mjs`;
`pnpm --config.verify-deps-before-run=false --dir manager lint`;
`pnpm --config.verify-deps-before-run=false --dir manager typecheck`;
`pnpm --config.verify-deps-before-run=false --dir manager build`.
Save logs, note existing two lint warnings. Do not rebuild unrelated Go packages for manager-only source changes.

## Task 5: Review and manual QA (2.2)

Create a baseline-only diff package, have an independent reviewer check all three fixes for spec compliance and quality, and resolve material regressions in a scoped pass. Copy fresh static output into the disposable QA server. Retest all seven issues and important sibling states, all seven instance sections, search/navigation, instances/accounts filters, cancel/delete guard, error/retry and logout/login. Use real API only for owned disposable QA records; explicitly label WhatsApp fixtures and collection adapter. Capture after screenshots, measured contrast, accessible names, request-field-only evidence and console errors. Update the report with per-issue results and limits.

## Task 6: Apply and deliver (2.3)

Compare main manager files to `/tmp/wzap-frontend-fix-baseline.json`; copy only files that differ from the isolated baseline, failing on concurrent conflicting changes. No deletion of prior edits. Confirm built-source hash parity and clean diff whitespace. Remove only ledger-owned QA users/instances, close owned browser, stop disposable proxy, remove private credentials/state. Write verify.md with seven checks and evidence-first retrospective with §0 plus six sections. Retain new worktree and review artifacts because no commit was requested. Leave archive for a separate request.

## Task 7: Browser-discovered row semantics (1.5)

Files: manager/app/components/instances/InstancesTable.vue and a focused test if it proves genuine behavior; no other writer overlaps. Before browser WCAG axe evidence: after-instances-table-light-a11y.json nested-interactive on both rows. Nuxt UI Table.vue:321 adds role=button only when onSelect is present, while existing rows contain named links, Connect/row-action buttons and checkboxes. Restore native row semantics without DOM role-mutation hacks or losing normal interactive targets. Preserve opening detail by name/row where feasible, checkbox keyboard selection, actions, filtering and sorting (row identity must follow displayed rows). Inspect the actual installed Table API, choose the smallest supported solution and record rationale. Before implementation create/observe a meaningful regression test if a behavior helper is introduced; simple semantic attribute changes use real browser RED evidence rather than unit mirrors. No dependencies, generated files, locale changes or commits. Then independent scoped review, fresh frontend gates/build and actual axe/selection/navigation re-test.

## Task 8: selected file preview accessibility (1.6)

- Browser discovery: `after-profile-file-picker-a11y.json` reports image-alt on the selected image thumbnail. Installed Nuxt UI FileUpload uses an Avatar without alt.
- One scoped writer owns the five existing picker components and an optional shared FileUploadPreview component. Use existing file-leading slot and VueUse object-URL lifecycle to retain preview and file removal, with valid alternative text. No dependency or contract change.
- Root owns actual image uploads/removal and axe WCAG checks in all five locations, independent review and fresh final build.
