# Manager Page Componentization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Repo apply route is `openspec-apply-change` (git worktree + subagents); this plan satisfies its plan.md requirement.

**Goal:** Reduce every page in `manager/app/pages` to page-only duty (SEO/meta, guard, composable orchestration, composition) with all table/toolbar/modal/section markup in reusable components.

**Architecture:** Hybrid: new `components/shared/` holds only the identical shell (`PageState`, `DataTableToolbar`, `DataTableFooter`); typed `UTable`s, modals and detail sections live per domain; pages keep data orchestration via existing composables.

**Tech Stack:** Nuxt 4, Vue 3.5 SFC `<script setup lang="ts">`, @nuxt/ui 4.11 (UDashboardPanel, UTable/TanStack, UModal, UAuthForm, UNavigationMenu), zod 4, existing `use*` composables.

**Spec:** `openspec/changes/manager-page-componentization/design.md` (decisions + risks); dialogue in `brainstorm.md`; scope contract in `tasks.md`.

## Global Constraints

- No Go, route, auth/RBAC, migration, NATS, or `openspec/specs/` changes.
- No new dependencies; no new i18n keys (reuse existing `en.json` copy, EN only).
- Existing child emit contracts and `data-testid`s stay stable.
- Manager quality gates per task: `pnpm --dir manager typecheck`, `pnpm --dir manager lint`; `pnpm --dir manager build` at milestones (after 1.3, 2.5, 3.1, 4.8, 5.2, 6.1).
- Conventional commits: `refactor(manager): <what>`.
- `gofmt -l .` must stay empty (no Go files touched; verified in 6.1).

---

## File structure (target)

```
manager/app/
  components/
    shared/
      PageState.vue            # skeleton + error+retry, default slot = content
      DataTableToolbar.vue     # debounced search + #bulk/#filters slots + columns dropdown
      DataTableFooter.vue      # selectedOf text + UPagination
    accounts/
      AccountsTable.vue        # owns useAccountsTable + toolbar/table/footer composition
      CreateAccountModal.vue   # ex-modal inline 1
      EditQuotaModal.vue       # ex-modal inline 2
      DeleteAccountModal.vue   # ex-modal inline 3
      (existing cells reused verbatim)
    instances/
      InstancesTable.vue       # owns useInstancesTable + viewport column rules
      InstancesCards.vue       # restored cards/grid view (cookie view state lives in page)
      detail/
        InstanceSectionNav.vue
        InstanceOverviewSection.vue
        InstanceMessagesSection.vue
        InstanceGroupsSection.vue
        InstanceChannelsSection.vue
        InstanceProfileSection.vue
        InstanceIntegrationsSection.vue
        InstanceSettingsSection.vue
      (existing modals/cards reused)
    overview/
      OverviewHeader.vue        # navbar + notifications + create + period toolbar
    auth/
      LoginForm.vue             # ex-login.vue form logic
  utils/
    accountQuota.ts             # parseQuota shared by the two account modals
  pages/
    accounts/index.vue          # ~110 lines after
    instances/index.vue         # ~110 lines after
    instances/[id].vue          # ~120 lines after
    index.vue                   # ~70 lines after
    login.vue                   # ~10 lines after
```

Order matters: shared (1.x) → accounts (2.x) → instances list (3.x) → detail (4.x) → overview/login (5.x) → gates (6.x). Each task leaves the app building and its screen manually verifiable.

---

### Task 1.1: `components/shared/PageState.vue`

**Files:**
- Create: `manager/app/components/shared/PageState.vue`
- Modify: `manager/app/pages/index.vue` (replace pending/failure blocks with it; keep cards branch)

**Interfaces:**
- Consumes: nothing.
- Produces: `PageState` props `{ pending: boolean, error: string | null }`, emit `retry: []`, default slot = loaded content. Later tasks reuse it in accounts/instances/detail pages.

- [ ] **Step 1: Create `PageState.vue` with this exact content**

```vue
<script setup lang="ts">
const { t } = useI18n()

withDefaults(defineProps<{
  pending: boolean
  error: string | null
  skeletonRows?: number
}>(), {
  error: null,
  skeletonRows: 3
})

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <div v-if="pending" class="flex flex-col gap-2">
    <USkeleton v-for="n in skeletonRows" :key="n" class="h-12 w-full" />
  </div>

  <UAlert
    v-else-if="error"
    color="error"
    variant="subtle"
    :title="error"
  >
    <template #actions>
      <UButton
        color="error"
        variant="soft"
        :label="t('common.retry')"
        @click="emit('retry')"
      />
    </template>
  </UAlert>

  <slot v-else />
</template>
```

- [ ] **Step 2: Use it in `pages/index.vue` for the initial-load branch only**

Replace the `v-if="pending && !stats"` skeleton div (lines 90-98) and the
`v-else-if="failure && !stats"` alert (lines 100-115) with:

```vue
<PageState :pending="pending && !stats" :error="failure && !stats ? failure : null" @retry="refresh">
  <div v-if="stats" class="flex flex-col gap-4 sm:gap-6">
    <!-- existing stats branch, lines 117-143, unchanged -->
  </div>
</PageState>
```

Keep the inline `v-if="failure"` alert inside the stats branch (lines
118-133) untouched — it covers background-refresh failure with data on screen.

- [ ] **Step 3: Typecheck**

Run: `pnpm --dir manager typecheck`
Expected: PASS with no errors.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/shared/PageState.vue manager/app/pages/index.vue openspec/changes/manager-page-componentization/tasks.md
git commit -m "refactor(manager): add shared PageState and use it on overview"
```

---

### Task 1.2: `components/shared/DataTableToolbar.vue`

**Files:**
- Create: `manager/app/components/shared/DataTableToolbar.vue`

**Interfaces:**
- Consumes: `PageState`-era patterns only (no dependency).
- Produces: toolbar with `defineModel<string>('filter', { default: '' })`
  (debounced 300ms write-back), props `{ searchPlaceholder, searchAria, filtersLabel, displayLabel, columnItems }`, slots `#bulk`, `#filters`. Consumed by Tasks 2.5 and 3.1.

- [ ] **Step 1: Create the component with this exact script**

```vue
<script setup lang="ts">
import type { DropdownMenuItem } from '@nuxt/ui'

withDefaults(defineProps<{
  searchPlaceholder: string
  searchAria: string
  filtersLabel: string
  displayLabel: string
  columnItems: DropdownMenuItem[]
}>(), {
  columnItems: () => []
})

// External filter state (the table's globalFilter). The visible input is
// local so keystrokes never re-filter per keystroke; it syncs back debounced.
const filter = defineModel<string>('filter', { default: '' })
const input = ref(filter.value)
watchDebounced(input, (value) => {
  filter.value = value
}, { debounce: 300 })
watch(filter, (value) => {
  if (value !== input.value) {
    input.value = value
  }
})
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-1.5" role="group" :aria-label="filtersLabel">
    <UInput
      v-model="input"
      icon="i-lucide-search"
      :placeholder="searchPlaceholder"
      :aria-label="searchAria"
      class="max-w-sm"
    />

    <div class="flex flex-wrap items-center gap-1.5">
      <slot name="bulk" />
      <slot name="filters" />
      <UDropdownMenu
        :items="[columnItems]"
        :content="{ align: 'end' }"
      >
        <UButton
          :label="displayLabel"
          color="neutral"
          variant="outline"
          trailing-icon="i-lucide-settings-2"
        />
      </UDropdownMenu>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Lint**

Run: `pnpm --dir manager lint`
Expected: PASS (component unused yet — no unused warnings for new files).

- [ ] **Step 3: Commit**

```bash
git add manager/app/components/shared/DataTableToolbar.vue
git commit -m "refactor(manager): add shared DataTableToolbar"
```

---

### Task 1.3: `components/shared/DataTableFooter.vue`

**Files:**
- Create: `manager/app/components/shared/DataTableFooter.vue`

**Interfaces:**
- Consumes: nothing.
- Produces: props `{ statusText: string, page: number, pageSize: number, total: number, pageCount: number }`, emit `update:page: [page: number]`. Consumed by Tasks 2.5 and 3.1.

- [ ] **Step 1: Create the component with this exact content**

```vue
<script setup lang="ts">
defineProps<{
  statusText: string
  page: number
  pageSize: number
  total: number
  pageCount: number
}>()

const emit = defineEmits<{
  'update:page': [page: number]
}>()
</script>

<template>
  <div class="flex items-center justify-between gap-3 border-t border-default pt-4 mt-auto">
    <div class="text-sm text-muted">
      {{ statusText }}
    </div>

    <div class="flex items-center gap-1.5">
      <UPagination
        v-if="pageCount > 1"
        :page="page"
        :items-per-page="pageSize"
        :total="total"
        size="sm"
        @update:page="(value: number) => emit('update:page', value)"
      />
    </div>
  </div>
</template>
```

- [ ] **Step 2: Typecheck + build (shared shell complete)**

Run: `pnpm --dir manager typecheck && pnpm --dir manager build`
Expected: both PASS.

- [ ] **Step 3: Commit**

```bash
git add manager/app/components/shared/DataTableFooter.vue
git commit -m "refactor(manager): add shared DataTableFooter"
```

---

### Task 2.1: `manager/app/utils/accountQuota.ts`

**Files:**
- Create: `manager/app/utils/accountQuota.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `parseQuota(raw: string): number | undefined | null` — empty = undefined (server default), valid int ≥ 0, else null. Consumed by Tasks 2.2 and 2.3.

- [ ] **Step 1: Create with the exact function moved from `pages/accounts/index.vue:303-312`**

```ts
// Empty stays omitted so the server default applies; 0 is a valid explicit
// value meaning unlimited.
export function parseQuota(raw: string): number | undefined | null {
  const trimmed = raw.trim()
  if (trimmed === '') {
    return undefined
  }
  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed) || parsed < 0) {
    return null
  }
  return parsed
}
```

- [ ] **Step 2: Typecheck**

Run: `pnpm --dir manager typecheck`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add manager/app/utils/accountQuota.ts
git commit -m "refactor(manager): extract parseQuota helper for account modals"
```

---

### Task 2.2: `components/accounts/CreateAccountModal.vue`

**Files:**
- Create: `manager/app/components/accounts/CreateAccountModal.vue`
- Modify: `manager/app/pages/accounts/index.vue` (delete create state/handlers/modal; wire new component)

**Interfaces:**
- Consumes: `parseQuota` (Task 2.1), `useAccounts().createUser`, `AccountUser` type.
- Produces: `defineModel<boolean>('open')`, emit `created: [user: AccountUser]`. Page prepends the user and toasts stay inside the modal. Consumed by the thinned page (Task 2.5).

- [ ] **Step 1: Create the modal moving page lines 220-229 (schema/state), 285-298 (reset+watch), 314-351 (onCreate+friendly error), 736-798 (template)** adapted to this exact contract

Script contract (move bodies verbatim, replacing `createOpen` with the `open` model and importing `parseQuota` from `~/utils/accountQuota`):

```ts
const emit = defineEmits<{
  created: [user: AccountUser]
}>()
const open = defineModel<boolean>('open', { default: false })
// schema/state/creating/failure + reset() + watch(open) + onCreate + friendlyCreateError moved verbatim;
// success branch: users-list prepend is replaced by emit('created', created) + open.value = false;
// success toast t('accounts.create.createdToast') stays here.
```

Template: page lines 736-798 verbatim with `v-model:open="open"`.

- [ ] **Step 2: Rewire the page — delete `createSchema`, `CreateSchema`, `createState`, `creating`, `createFailure`, `resetCreate`, `watch(createOpen)`, `onCreate`, `friendlyCreateError`, `parseQuota` (moved), and the first `UModal` block (lines 736-798); add**

```vue
<CreateAccountModal v-model:open="createOpen" @created="(user) => { users = [user, ...users] }" />
```

keeping `const createOpen = ref(false)` in page state.

- [ ] **Step 3: Lint + manual verify**

Run: `pnpm --dir manager lint`
Expected: PASS. Manual: open `/accounts` as admin, create a user, see success toast + new row; submit an taken email, see `accounts.create.emailTaken`.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/accounts/CreateAccountModal.vue manager/app/pages/accounts/index.vue
git commit -m "refactor(manager): extract CreateAccountModal from accounts page"
```

---

### Task 2.3: `components/accounts/EditQuotaModal.vue`

**Files:**
- Create: `manager/app/components/accounts/EditQuotaModal.vue`
- Modify: `manager/app/pages/accounts/index.vue`

**Interfaces:**
- Consumes: `parseQuota`, `useAccounts().updateUserQuota`.
- Produces: props `{ target: AccountUser | null }`, `defineModel<boolean>('open')`, emit `updated: [user: AccountUser]`.

- [ ] **Step 1: Create the modal moving page lines 233-239 (schema/state), 353-359 (openQuota init becomes a watcher), 361-383 (onSaveQuota), 800-842 (template)**

Watcher replacing `openQuota`:

```ts
watch([open, () => props.target], ([isOpen, target]) => {
  if (isOpen && target) {
    quotaState.instance_quota = String(target.instance_quota)
    quotaSaving.value = false
    quotaFailure.value = null
  }
})
```

Template: lines 800-842 verbatim with `v-model:open="open"` and description using `target?.email ?? ''`. Success toast stays inside; emit `updated`.

- [ ] **Step 2: Rewire the page — delete quota schema/state/refs, `openQuota`, `onSaveQuota`, and the quota `UModal`; change row-action handler to `quotaTarget = $event; quotaOpen = true`; add**

```vue
<EditQuotaModal v-model:open="quotaOpen" :target="quotaTarget" @updated="(user) => { users = users.map(entry => entry.id === user.id ? user : entry) }" />
```

- [ ] **Step 3: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual: edit a quota, invalid input shows `accounts.quota.quotaInvalid`, save shows `accounts.quota.updated`.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/accounts/EditQuotaModal.vue manager/app/pages/accounts/index.vue
git commit -m "refactor(manager): extract EditQuotaModal from accounts page"
```

---

### Task 2.4: `components/accounts/DeleteAccountModal.vue`

**Files:**
- Create: `manager/app/components/accounts/DeleteAccountModal.vue`
- Modify: `manager/app/pages/accounts/index.vue`

**Interfaces:**
- Consumes: `useAccounts().deleteUser`.
- Produces: props `{ target: AccountUser | null }`, `defineModel<boolean>('open')`, emit `deleted: [id: string]`. The self-delete guard (page lines 385-396) stays in the page.

- [ ] **Step 1: Create the modal moving page lines 241-244 (deleting/deleteFailure refs), 398-422 (onDelete+friendly error), 844-872 (template)**

Template lines 844-872 verbatim with `v-model:open="open"`; on delete success emit `deleted` with the target id (page removes the row + toasts stay inside the modal per the new contract — move the `accounts.delete.deleted` toast into the modal to match Tasks 2.2/2.3 ownership).

- [ ] **Step 2: Rewire the page — keep `openDelete` guard + toast, delete `onDelete`/`friendlyDeleteError`/refs/modal block; add**

```vue
<DeleteAccountModal v-model:open="deleteOpen" :target="deleteTarget" @deleted="(id) => { users = users.filter(user => user.id !== id); deleteTarget = null }" />
```

- [ ] **Step 3: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual: delete a user (success toast + row gone); try deleting an owner with instances → `accounts.delete.ownsInstances`; try self-delete from row menu → `accounts.delete.selfBlocked` toast, no modal.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/accounts/DeleteAccountModal.vue manager/app/pages/accounts/index.vue
git commit -m "refactor(manager): extract DeleteAccountModal from accounts page"
```

---

### Task 2.5: `components/accounts/AccountsTable.vue` + thinned page

**Files:**
- Create: `manager/app/components/accounts/AccountsTable.vue`
- Modify: `manager/app/pages/accounts/index.vue` (≈110 lines after)

**Interfaces:**
- Consumes: `useAccountsTable`, shared toolbar/footer, existing `AccountsTable*Cell` components, `useClipboard`, `useConfirmDelete` stays in page.
- Produces: props `{ users: AccountUser[], usageByOwner: Record<string, number>, sessionUserId: string | undefined }`; emits `edit-quota: [user: AccountUser]`, `remove: [user: AccountUser]`, `bulk-delete: [users: AccountUser[]]`; exposes `clearSelection()` via `defineExpose`. Page owns guard/load/users/usage/modals/bulk confirm+loop. (The navbar create button stays in the page — no `create` emit.)

- [ ] **Step 1: Create `AccountsTable.vue` moving page lines 54-64 (table state), 66-73 (ref+search), 75-96 (row id + counts), 101-214 (role filter, labels, column items, selection, copy, clear, pagination, watchers, sort labels), and template lines 528-731 (toolbar→shared component, UTable slots 614-713 verbatim, footer→shared component)**

Adaptations: `searchInput`/`globalFilter` bind to toolbar `v-model:filter`; role select + bulk buttons go in `#bulk`/`#filters` slots; `USelect` role filter and copy-emails stay inside; `columnItems` feeds the toolbar prop; footer receives `:status-text="t('accounts.table.selectedOf', { selected: selectedCount, filtered: totalFiltered })" :page="pagination.pageIndex + 1" :page-size :total="totalFiltered" :page-count` and `@update:page="onUpdatePage"`; bulk-delete button emits `bulk-delete` with `selectedUsers()` (page confirms + loops); `create` button is NOT here (page navbar keeps it — emit unused; drop the `create` emit).

- [ ] **Step 2: Thin the page to guard + load/loadUsage + users/usage state + modal targets + bulk confirm/loop (lines 427-480 adapted to accept emitted targets and call exposed `clearSelection`) + panel/navbar/subtitle + `<AccountsTable>` + 3 modals**

- [ ] **Step 3: Lint + manual matrix + build**

Run: `pnpm --dir manager lint && pnpm --dir manager build`
Expected: PASS. Manual on `/accounts` (admin): search debounce, role filter, column display dropdown, sort headers, select-all + copy emails, single delete, bulk delete clean + partial (reload toast), empty-state create button, no-results clear-filters, pagination.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/accounts/AccountsTable.vue manager/app/pages/accounts/index.vue
git commit -m "refactor(manager): extract AccountsTable and thin accounts page"
```

---

### Task 3.1: `components/instances/InstancesTable.vue` + thinned page

**Files:**
- Create: `manager/app/components/instances/InstancesTable.vue`
- Modify: `manager/app/pages/instances/index.vue` (≈110 lines after)

**Interfaces:**
- Consumes: `useInstancesTable(items, ownerEmails, isAdmin)`, shared toolbar/footer, existing `InstancesTable*Cell` components.
- Produces: props `{ items: Instance[], ownerEmails: Record<string,string>, isAdmin: boolean, loadingMore: boolean, hasMore: boolean }`; emits `connect: [instance: Instance]`, `edit: [instance: Instance]`, `remove: [instance: Instance]`, `load-more: []`. Row click and name link navigate directly inside the table (no `open` emit); `onCreated/onUpdated/onDeleted/onConnect` stay in the page with the existing modals.

- [ ] **Step 1: Create `InstancesTable.vue` moving page lines 48-73 (table state + search + viewport), 77-128 (admin meta, visibility merge, counts), 133-236 (status filter, labels, column items, selection, copy, clear, pagination, watchers, sort labels, onRowSelect with direct `navigateTo`), and template lines 419-623 (toolbar→shared, UTable slots 491-596 verbatim, footer→shared, loadMore button emitting `load-more`)**

`statusFilter`/`statusFilterItems` feed the `#filters` slot `USelect`; bulk copy button in `#bulk`; `onUpdatePage`, `clearFilters`, `copySelectedNames` internal.

- [ ] **Step 2: Thin the page to `loadFirst`/`loadMore`/`loadOwners`, `onCreated` (strip one-time key), `onConnect` (connectingId + toasts + navigate), edit/delete targets + existing modals, panel/navbar/subtitle + `PageState` (Task 1.1 pattern) + `<InstancesTable>`**

- [ ] **Step 3: Lint + manual matrix + build**

Run: `pnpm --dir manager lint && pnpm --dir manager build`
Expected: PASS. Manual on `/instances` (admin + user): search, status filter, owner column hidden below md / JID below lg, row click → detail, connect inline + pairing toast, edit modal, delete modal, loadMore append, empty + no-results states.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/InstancesTable.vue manager/app/pages/instances/index.vue
git commit -m "refactor(manager): extract InstancesTable and thin instances page"
```

---

### Task 4.1: `components/instances/detail/InstanceSectionNav.vue`

**Files:**
- Create: `manager/app/components/instances/detail/InstanceSectionNav.vue`
- Modify: `manager/app/pages/instances/[id].vue` (replace `sections` computed + `UNavigationMenu` block)

**Interfaces:**
- Consumes: nothing (builds its own items with `useI18n`).
- Produces: props `{ section: string }`, emit `select: [value: string]` (keeps the documented onSelect workaround: no `v-model` reliance).

- [ ] **Step 1: Create moving page lines 57-72 (section state stays in page; items move here)**

```vue
<script setup lang="ts">
const { t } = useI18n()
defineProps<{ section: string }>()
const emit = defineEmits<{ select: [value: string] }>()
const items = computed(() => [
  { label: t('instances.sections.overview'), value: 'overview', onSelect: () => emit('select', 'overview') },
  // ... messages, groups, channels, profile, integrations, settings (same pattern)
])
</script>

<template>
  <UDashboardToolbar>
    <UNavigationMenu
      :model-value="section"
      highlight
      class="-mx-1 flex-1 min-w-0 overflow-x-auto"
      :items="items"
    />
  </UDashboardToolbar>
</template>
```

Page usage: `<InstanceSectionNav :section="section" @select="(value) => { section = value }" />` replacing lines 314-321.

- [ ] **Step 2: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual: all 7 sections switch, active pill follows.

- [ ] **Step 3: Commit**

```bash
git add manager/app/components/instances/detail/InstanceSectionNav.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract InstanceSectionNav from detail page"
```

---

### Task 4.2: `components/instances/detail/InstanceOverviewSection.vue`

**Files:**
- Create: `manager/app/components/instances/detail/InstanceOverviewSection.vue`
- Modify: `manager/app/pages/instances/[id].vue`

**Interfaces:**
- Consumes: `useInstances().updateInstance/disconnectInstance`, `useConfirmDelete`, `PairingCard`, `PairPhoneCard`.
- Produces: props `{ instance: Instance }`; emits `paired: []`, `updated: [instance: Instance]`, `changed: []` (after disconnect, page reloads).

- [ ] **Step 1: Create moving page lines 38-45 (schema/state), 114-146 (onSave + friendly error), 152-175 (onDisconnect → emit `changed` instead of `load()`), 248-254 (formatDateTime), and template lines 323-412 verbatim (info card + PairingCard + PairPhoneCard + rename form)**

- [ ] **Step 2: Rewire page — delete moved code; wire `<InstanceOverviewSection :instance="instance" @paired="load" @updated="(value) => { instance = value }" @changed="load" />`**

- [ ] **Step 3: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual: rename save + 409 externalRefTaken, disconnect confirm flow, pairing card still emits.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/detail/InstanceOverviewSection.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract InstanceOverviewSection from detail page"
```

---

### Task 4.3: `components/instances/detail/InstanceMessagesSection.vue`

**Files:**
- Create: `manager/app/components/instances/detail/InstanceMessagesSection.vue`
- Modify: `manager/app/pages/instances/[id].vue`

**Interfaces:**
- Consumes: `TestSendCard`, `MessageComposer`, `MessageActionsCard`, `MessagesCard`.
- Produces: props `{ instanceId: string, status: InstanceStatus }`; fully self-contained (owns `messagesRefresh`, `isMessagesOpen`, route watcher closing the slideover). No emits.

- [ ] **Step 1: Create moving page lines 55 (messagesRefresh), 83-88 (isMessagesOpen + route watcher), 196-198 (onMessagesRefresh), and template lines 414-459 verbatim**

- [ ] **Step 2: Rewire page — delete moved state; wire `<InstanceMessagesSection :instance-id="instance.id" :status="instance.status" />`**

- [ ] **Step 3: Lint + manual verify (both viewports)**

Run: `pnpm --dir manager lint`
Expected: PASS. Manual: send via TestSend + Composer bumps history; lg shows aside, mobile shows slideover button (accepted single redundant fetch preserved — no `v-if` gating).

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/detail/InstanceMessagesSection.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract InstanceMessagesSection from detail page"
```

---

### Task 4.4: Groups + Channels sections (thin wrappers)

**Files:**
- Create: `manager/app/components/instances/detail/InstanceGroupsSection.vue`, `manager/app/components/instances/detail/InstanceChannelsSection.vue`
- Modify: `manager/app/pages/instances/[id].vue`

**Interfaces:**
- Consumes: `GroupDetail`, `ChannelsCard`.
- Produces (each): props `{ instanceId: string, status: InstanceStatus }`, no emits.

- [ ] **Step 1: Create both wrappers (template-only around the existing card)**

```vue
<script setup lang="ts">
import type { InstanceStatus } from '~/types/api'
defineProps<{ instanceId: string, status: InstanceStatus }>()
</script>

<template>
  <div class="mx-auto flex w-full flex-col gap-4 sm:gap-6 lg:max-w-2xl">
    <GroupDetail :instance-id="instanceId" :status="status" />
  </div>
</template>
```

(Channels mirrors it with `ChannelsCard`; moves page lines 461-467.)

- [ ] **Step 2: Rewire page branches to the two components**

- [ ] **Step 3: Typecheck**

Run: `pnpm --dir manager typecheck`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/detail/InstanceGroupsSection.vue manager/app/components/instances/detail/InstanceChannelsSection.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract groups and channels sections"
```

---

### Task 4.5: `components/instances/detail/InstanceProfileSection.vue`

**Files:**
- Create: `manager/app/components/instances/detail/InstanceProfileSection.vue`
- Modify: `manager/app/pages/instances/[id].vue`

**Interfaces:**
- Consumes: `ProfileCard`, `PrivacyCard`, `DeviceActionsCard`.
- Produces: props `{ instanceId: string, status: InstanceStatus }`, no emits.

- [ ] **Step 1: Create wrapping page lines 469-473 verbatim in the section container**

- [ ] **Step 2: Rewire the profile branch to `<InstanceProfileSection :instance-id :status />`**

- [ ] **Step 3: Typecheck**

Run: `pnpm --dir manager typecheck`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/detail/InstanceProfileSection.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract InstanceProfileSection from detail page"
```

---

### Task 4.6: `components/instances/detail/InstanceIntegrationsSection.vue`

**Files:**
- Create: `manager/app/components/instances/detail/InstanceIntegrationsSection.vue`
- Modify: `manager/app/pages/instances/[id].vue`

**Interfaces:**
- Consumes: `WebhookCard`, `ChatwootCard`.
- Produces: props `{ instance: Instance }`; re-emits `webhook-updated: [instance: Instance]` (page sets `instance = updated`, replacing `onWebhookUpdated` lines 190-192).

- [ ] **Step 1: Create wrapping page lines 475-482 verbatim, forwarding `@updated` as `webhook-updated`**

- [ ] **Step 2: Rewire page — delete `onWebhookUpdated`; wire `<InstanceIntegrationsSection :instance="instance" @webhook-updated="(value) => { instance = value }" />`**

- [ ] **Step 3: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual: webhook save persists + shows stored config.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/detail/InstanceIntegrationsSection.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract InstanceIntegrationsSection from detail page"
```

---

### Task 4.7: `components/instances/detail/InstanceSettingsSection.vue`

**Files:**
- Create: `manager/app/components/instances/detail/InstanceSettingsSection.vue`
- Modify: `manager/app/pages/instances/[id].vue`

**Interfaces:**
- Consumes: `useInstances().rotateInstanceKey/revokeInstanceKey`, `useConfirmDelete`, `OneTimeKeyDisplay`, `hasSeenInstanceKey`/`markInstanceKeySeen`/`forgetInstanceKeySeen` helpers.
- Produces: props `{ instance: Instance, isAdmin: boolean }`; emits `changed: []` (revoke succeeded → page reloads + resets keySeen via `load()`), `delete-requested: []` (page opens existing `DeleteInstanceModal`).

- [ ] **Step 1: Create moving page lines 47-55 (deleteOpen stays page; move freshKey/keySeen/generating/keyFailure/revoking), 200-246 (onGenerate/onRevoke → revoke emits `changed`), and template lines 484-547 verbatim with delete button emitting `delete-requested`**

`keySeen` initializes from `hasSeenInstanceKey(instance.id)` via watcher/`onMounted` inside the section (replacing page line 101 assignment).

- [ ] **Step 2: Rewire page — keep `deleteOpen` + `DeleteInstanceModal` + `onDeleted`; wire `<InstanceSettingsSection :instance :is-admin="isAdmin" @changed="load" @delete-requested="deleteOpen = true" />`**

- [ ] **Step 3: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual (admin): keyless banner → generate → one-time display → revoke confirm; delete card opens modal.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/instances/detail/InstanceSettingsSection.vue "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): extract InstanceSettingsSection from detail page"
```

---

### Task 4.8: Thinned `pages/instances/[id].vue` + build

**Files:**
- Modify: `manager/app/pages/instances/[id].vue` (≈120 lines after)

**Interfaces:**
- Consumes: all Task 4.x sections + `DeleteInstanceModal`.
- Produces: page keeps `load`/`notFound`/`failure`, `section`, navbar + `InstanceStatusBadge`, `PageState` for pending/failure/notFound branches, section switch, delete modal wiring.

- [ ] **Step 1: Remove every moved block; keep imports of sections + `DeleteInstanceModal`, `useConfirmDelete` only if still used (disconnect moved out — drop it), `load`, `onDeleted`, seo meta, template panel/header/body skeleton**

- [ ] **Step 2: Lint + build + detail matrix**

Run: `pnpm --dir manager lint && pnpm --dir manager build`
Expected: PASS. Manual: direct visit with bad id → notFound; API down → failure + retry; admin vs user (key card hidden); all 7 sections render; no `UTable`/`UModal`/`UForm` markup left in the page (grep to confirm).

- [ ] **Step 3: Commit**

```bash
git add "manager/app/pages/instances/[id].vue"
git commit -m "refactor(manager): thin instance detail page to composition only"
```

---

### Task 5.1: `components/overview/OverviewHeader.vue` + thinned `pages/index.vue`

**Files:**
- Create: `manager/app/components/overview/OverviewHeader.vue`
- Modify: `manager/app/pages/index.vue` (≈70 lines after)

**Interfaces:**
- Consumes: `useDashboard().isNotificationsSlideoverOpen` directly (no prop drilling).
- Produces: props `{ userEmail: string, scope: string }`, `defineModel<OverviewPeriod>('period')`, props `{ periodItems: { label: string, value: OverviewPeriod }[] }`. Renders navbar (collapse + notifications + create→`/instances`) + period toolbar + welcome line (moves template lines 35-85).

- [ ] **Step 1: Create the header moving `pages/index.vue` template lines 35-85 verbatim, with `period` as model and `useDashboard()` called inside**

- [ ] **Step 2: Thin the page to `useOverview` + range/period state + `PageState` + 3 overview components**

- [ ] **Step 3: Lint + manual verify**

Run: `pnpm --dir manager lint`
Expected: PASS. Manual: welcome line, period switch regroups chart, notifications slideover opens, fallback notice + retry path intact.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/overview/OverviewHeader.vue manager/app/pages/index.vue
git commit -m "refactor(manager): extract OverviewHeader and thin overview page"
```

---

### Task 5.2: `components/auth/LoginForm.vue` + thinned `pages/login.vue`

**Files:**
- Create: `manager/app/components/auth/LoginForm.vue`
- Modify: `manager/app/pages/login.vue` (≈10 lines after)

**Interfaces:**
- Consumes: `useAuth().login`, `ApiError`, zod.
- Produces: no props/emits (owns schema/fields/pending/failure + `navigateTo('/')` on success). Moves `login.vue` lines 1-58 verbatim.

- [ ] **Step 1: Create `LoginForm.vue` with the page's template + script verbatim (minus `definePageMeta`/`useSeoMeta`, which stay in the page)**

- [ ] **Step 2: Thin the page to**

```vue
<script setup lang="ts">
definePageMeta({ layout: 'auth' })
useSeoMeta({ title: 'Sign in' })
</script>

<template>
  <LoginForm />
</template>
```

- [ ] **Step 3: Typecheck + manual verify**

Run: `pnpm --dir manager typecheck`
Expected: PASS. Manual: successful login → `/`; wrong password → inline alert; double-submit guarded.

- [ ] **Step 4: Commit**

```bash
git add manager/app/components/auth/LoginForm.vue manager/app/pages/login.vue
git commit -m "refactor(manager): extract LoginForm and thin login page"
```

---

### Task 6.1: Final gates + manual matrix

**Files:**
- None (verification only)

- [ ] **Step 1: Run the full manager gates**

Run: `pnpm --dir manager typecheck && pnpm --dir manager lint && pnpm --dir manager build`
Expected: all PASS.

- [ ] **Step 2: Confirm no Go surface changed**

Run: `gofmt -l .` (expect empty output) and `git status --short` (expect only `manager/` + `openspec/` paths).

- [ ] **Step 3: Manual matrix — 5 routes × admin/user × light/dark; detail × 7 sections × connected/disconnected; mobile width for messages slideover; confirm page sizes (≈110/110/120/70/10 lines) and grep pages for leftover `UTable|UModal|UForm` (expect zero hits)**

- [ ] **Step 4: Write `verify.md` (post-apply evidence per repo workflow) and commit it**

```bash
git add openspec/changes/manager-page-componentization/verify.md
git commit -m "docs(specs): verify manager page componentization"
```

---

## Self-review

- **Spec coverage:** every Goals bullet maps — page line budgets (2.5/3.1/4.8/5.1/5.2), dedup shell (1.1–1.3), modal pattern (2.2–2.4), 7 detail sections + nav (4.1–4.7), untouched contracts (Global Constraints + per-task verbatim moves), verification (6.1). Non-Goals/Out-of-Scope enforced by move-only tasks with no new keys/deps.
- **Placeholder scan:** no TBD/TODO; every step names exact files, line ranges, code, or commands. Cross-task references name concrete IDs and interfaces instead of "similar to".
- **Type consistency:** `tableApi` exposure via `defineExpose({ clearSelection })` matches page usage; modal contracts uniformly `v-model:open` + emits; section emits (`paired`/`updated`/`changed`/`webhook-updated`/`delete-requested`/`select`) defined once per task and wired in the consuming page task.
