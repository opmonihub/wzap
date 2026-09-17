# Retrospective — manager page componentization

Date: 2026-09-17. Branch: `refactor/manager-page-componentization-wt` (head `681c083`).
Scope: move-only componentization of all 5 manager pages; zero Go files, no new dependencies.

## §0 Evidence

- `tasks.md`: 22/22 items `[x]` (1.1–6.1).
- `verify.md` §§1–9: `pnpm --dir manager typecheck` PASS; `lint` PASS (0 errors, 2 pre-existing `withDefaults` warnings); `build` PASS (7 prerendered routes); `go vet ./...` PASS; `gofmt -l .` empty; `go test ./... -count=1` PASS (all packages ok, Postgres integration skipped without `WZAP_TEST_DATABASE_URL`); `grep UTable|UModal|UForm manager/app/pages/` zero hits; page sizes 76/8/192/165/284 lines.
- Final code review (subagent, 2026-09-17): **Ready with notes**, 6 minors, 0 critical/important.
- NOT RUN (no live backend/browser here): 5 routes × admin/user × light/dark, 7 detail sections × connected/disconnected, mobile slideover vs lg aside, login/delete/quota/pairing interactive paths — all listed in `verify.md` §4.
- Commits: `refactor(manager):` × per-task + `fix(manager):` for cards restoration (`b61f50e`), review fix wave (`077d759`), settings redundancy (`69963a5`) + `docs(specs):` verify/tasks commits.

## 1. What went well

- Plan-as-contract worked: per-task exact files, line ranges, code blocks and gates kept 22 tasks move-only and auditable; the read-only audit found 13/16 fully conformant with the rest as compatible extensions.
- Shared shell stayed minimal (`PageState`, `DataTableToolbar`, `DataTableFooter`, each reused 2–3×) while typed `UTable`s lived per domain — the hybrid decision from `design.md` held up in practice.
- Emit/`data-testid`/i18n discipline: child contracts and test ids stayed stable, `parseQuota` moved verbatim, one-time key handling preserved; reviewer confirmed zero contract breaks.
- Gates stayed green at every milestone (typecheck/lint per task, build at 1.3/2.5/3.1/4.8/5.2/6.1 + final), so the finish session needed no code fix.

## 2. What didn't go as planned

- Line budgets in `plan.md` were underestimated for 3 pages: accounts 192 vs ~110 (guard + cursor `loadUsage` loop + bulk confirm/loop stayed by design), detail 165 vs ~120 (7-section switch is pure composition but verbose), instances list 284 vs ~110 (cards/table dual view with `KeepAlive` restored in `b61f50e`). Budgets measured markup extraction, not orchestration weight.
- Two scope extensions arrived mid-flight: instances cards view restoration (`InstanceCard.vue`, `InstancesCards.vue`, view toggle + `wzap-instances-view` cookie, 4 new i18n keys) and UCard→UPageCard conversions from the carried-in baseline — both behavior-adjacent, both now pending human visual/live matrix.
- `verify.md` needed post-hoc repair in the finish session (duplicated §5 numbering, stale toolchain note, missing final Go gates + review record) — evidence existed but was not integration-ready.

## 3. Decisions worth keeping

- Move-only tasks with verbatim template moves + per-task gates: prevented logic drift across 44 touched files.
- `v-model:open` + emits for all modals, `defineExpose({ clearSelection })` / `tableApi` for tables, section emits (`paired`/`updated`/`changed`/`webhook-updated`/`delete-requested`/`select`): uniform contracts the reviewer could verify mechanically.
- Honest-deviation records in `verify.md` (§§3–7: over-budget pages, 4 i18n keys, UPageCard, cards duplication) instead of silent scope creep.

## 4. Risks carried to release (all human-matrix-gated)

- UPageCard visual neutrality unconfirmed in this environment.
- Cards-view live behavior unconfirmed (filters/sort/pagination parity, connect/edit/delete from cards, cookie persistence, `loadMore`).
- Overview initial-error cosmetics: raw `failure` as `PageState` title vs former `title=t('overview.loadFailed') + description` — minor i18n regression on one branch.
- SSR first-paint viewport flicker (owner/JID hidden until mount) and pre-mount empty-copy edge — both minor, both documented.

## 5. Follow-ups proposed (out of scope for this change)

- Shared `useInstanceListFilter` to unify TanStack (`InstancesTable`) vs hand-rolled (`InstancesCards`) filter/sort/pagination state.
- Restore localized overview error title inside `PageState` usage (pass title prop or slot) — 5-line fix, needs no architecture change.
- Reconsider line budgets as "markup budget" vs "orchestration budget" in future componentization plans.

## 6. Process notes

- `tasks.md` as scope contract + `plan.md` as executable detail + `verify.md` post-apply only: the split held; the only gap was verify maintenance (numbering/toolchain/final gates), fixed in the finish session without rewriting history.
- Forward-pointer: if the human live matrix finds visual/behavioral breaks, append to `verify.md` §4/§9 — never rewrite this retrospective.
