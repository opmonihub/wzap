<script setup lang="ts">
import type { DropdownMenuItem, TableRow } from '@nuxt/ui'
import { ApiError } from '~/composables/useApi'
import { useAccountsTable } from '~/composables/useAccountsTable'
import AccountsTableActionsCell from '~/components/accounts/AccountsTableActionsCell.vue'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import AccountsTableQuotaCell from '~/components/accounts/AccountsTableQuotaCell.vue'
import AccountsTableRoleCell from '~/components/accounts/AccountsTableRoleCell.vue'
import type { AccountRole, AccountUser } from '~/types/api'

// Structural view of the UTable API this page drives (stable component
// instance typing as a structural view without importing table-core types directly).
interface AccountsTableColumn {
  id: string
  getCanHide: () => boolean
  getIsVisible: () => boolean
  toggleVisibility: (visible: boolean) => void
}

interface AccountsTableApi {
  getFilteredRowModel: () => { rows: TableRow<AccountUser>[] }
  getFilteredSelectedRowModel: () => { rows: TableRow<AccountUser>[] }
  setPageIndex: (index: number) => void
  setPageSize: (size: number) => void
  resetRowSelection: () => void
  getAllColumns: () => AccountsTableColumn[]
}

// Account management is admin-only: the sidebar hides this screen for user
// accounts and the guard below turns a direct visit back to the overview.
// Quotas are per user (0 means unlimited); deleting an owner that still owns
// instances answers 409 and removes nothing.
const { t } = useI18n()
const toast = useToast()
const { isAdmin, user: sessionUser } = useAuth()
const { listUsers, deleteUser } = useAccounts()
const { confirmDelete } = useConfirmDelete()
const { listInstances } = useInstances()
const { copy } = useClipboard()

if (!isAdmin.value) {
  await navigateTo('/')
}

const users = ref<AccountUser[]>([])
const pending = ref(true)
const failure = ref<string | null>(null)
// Instances owned per account, counted client-side: GET /users exposes no
// usage field, so the quota cell pairs the stored quota with this count.
const usageByOwner = ref<Record<string, number>>({})

const {
  columns,
  sorting,
  globalFilter,
  columnFilters,
  columnVisibility,
  rowSelection,
  pagination,
  globalFilterOptions,
  paginationOptions
} = useAccountsTable()

const table = useTemplateRef<{ tableApi?: AccountsTableApi }>('table')

// Search input debounced into the table's global filter (300ms): typing never
// re-filters per keystroke on 1000+ rows; the role select stays immediate.
const searchInput = ref('')
watchDebounced(searchInput, (value) => {
  globalFilter.value = value
}, { debounce: 300 })

function getRowId(row: AccountUser): string {
  return row.id
}

// Native table state, read from the table API (UTable owns sorting, filtering
// and pagination; the page only binds state and renders). Before the first
// mount tableApi is null, so every read falls back to the loaded users.
function filteredCount(): number {
  return table.value?.tableApi?.getFilteredRowModel().rows.length ?? users.value.length
}

// UPagination :total subscribes to the v-model state because tableApi reads
// are not reactive: sorting/filter/pagination/users changes recompute it.
const totalFiltered = computed(() => {
  void sorting.value
  void globalFilter.value
  void columnFilters.value
  void pagination.value
  void users.value
  return filteredCount()
})
const pageCount = computed(() => Math.max(1, Math.ceil(totalFiltered.value / pagination.value.pageSize)))

// Role column filter behind a USelect. Reka SelectItem forbids an empty
// value string (it throws and unmounts the page), so the "all" option uses
// the 'all' sentinel, mapped back to "no column filter" in the setter.
const roleFilter = computed<string>({
  get: () => {
    const current = columnFilters.value.find(entry => entry.id === 'role')?.value
    return typeof current === 'string' && current !== '' ? current : 'all'
  },
  set: (value) => {
    const rest = columnFilters.value.filter(entry => entry.id !== 'role')
    columnFilters.value = value === '' || value === 'all' ? rest : [...rest, { id: 'role', value }]
  }
})

const accountRoles: AccountRole[] = ['admin', 'user']

function roleOptionLabel(role: AccountRole): string {
  return role === 'admin' ? t('userMenu.roleAdmin') : t('userMenu.roleUser')
}

const roleFilterItems = computed(() => [
  { label: t('accounts.table.allRoles'), value: 'all' },
  ...accountRoles.map(role => ({ label: roleOptionLabel(role), value: role }))
])

// Select and actions never hide (bulk bar + per-row edit/delete stay
// reachable); the dropdown only lists columns where the table API reports
// getCanHide().
function columnLabel(columnId: string): string {
  switch (columnId) {
    case 'email':
      return t('common.email')
    case 'role':
      return t('common.role')
    case 'instance_quota':
      return t('accounts.quotaLabel')
    default:
      return columnId
  }
}

const columnItems = computed<DropdownMenuItem[]>(() =>
  table.value?.tableApi?.getAllColumns()
    .filter(column => column.getCanHide())
    .map(column => ({
      label: columnLabel(column.id),
      type: 'checkbox' as const,
      checked: column.getIsVisible(),
      onUpdateChecked: (checked: boolean) => column.toggleVisibility(checked),
      onSelect: (event: Event) => {
        event.preventDefault()
      }
    })) ?? []
)

const selectedCount = computed(() => {
  const fallback = Object.values(rowSelection.value).filter(Boolean).length
  return table.value?.tableApi?.getFilteredSelectedRowModel().rows.length ?? fallback
})

function selectedUsers(): AccountUser[] {
  return table.value?.tableApi?.getFilteredSelectedRowModel().rows.map(row => row.original)
    ?? users.value.filter(user => rowSelection.value[user.id])
}

function clearSelection() {
  if (table.value?.tableApi) {
    table.value.tableApi.resetRowSelection()
  } else {
    rowSelection.value = {}
  }
}

async function copySelectedEmails() {
  try {
    await copy(selectedUsers().map(user => user.email).join('\n'))
    toast.add({ title: t('accounts.table.copiedEmails'), icon: 'i-lucide-check', color: 'success' })
  } catch {
    toast.add({ title: t('accounts.table.copyFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
  }
}

function clearFilters() {
  searchInput.value = ''
  globalFilter.value = ''
  columnFilters.value = []
}

function onUpdatePage(page: number) {
  if (table.value?.tableApi) {
    table.value.tableApi.setPageIndex(page - 1)
  } else {
    pagination.value.pageIndex = page - 1
  }
}

// The table owns ordering, so filter/sort changes restart at the first page;
// a shrunken result only clamps an out-of-range page.
watch([globalFilter, columnFilters, sorting], () => {
  pagination.value.pageIndex = 0
})

watch(pageCount, (count) => {
  if (pagination.value.pageIndex > count - 1) {
    pagination.value.pageIndex = count - 1
  }
})

// Table copy lives in accounts.table.* (en.json); no UI literal stays here.
function sortActionLabel(columnId: string): string {
  const current = sorting.value[0]
  const nextDesc = current?.id === columnId && !current.desc
  const column = columnId === 'role'
    ? t('common.role')
    : columnId === 'instance_quota' ? t('accounts.quotaLabel') : t('common.email')
  return nextDesc ? t('accounts.table.sortDesc', { column }) : t('accounts.table.sortAsc', { column })
}

// UTable exposes no th slot, so aria-sort stays off the header buttons; sort
// state travels in the button aria-label (sortAsc/sortDesc) instead.

const createOpen = ref(false)

const quotaTarget = ref<AccountUser | null>(null)
const quotaOpen = ref(false)

const deleteTarget = ref<AccountUser | null>(null)
const deleteOpen = ref(false)
const deleting = ref(false)
const deleteFailure = ref<string | null>(null)

useSeoMeta({
  title: 'Accounts'
})

async function load() {
  pending.value = true
  failure.value = null
  try {
    users.value = await listUsers()
    await loadUsage()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('accounts.loadFailed')
  } finally {
    pending.value = false
  }
}

// Usage comes from the instance list (no usage field on GET /users):
// accumulate every cursor page best-effort and count owners. A failed usage
// load never fails the accounts list; cells fall back to 0 used.
async function loadUsage() {
  const counts: Record<string, number> = {}
  try {
    let cursor: string | undefined
    do {
      const page = await listInstances(cursor)
      for (const instance of page.items) {
        if (instance.owner_user_id) {
          counts[instance.owner_user_id] = (counts[instance.owner_user_id] ?? 0) + 1
        }
      }
      cursor = page.next_cursor === '' ? undefined : page.next_cursor
    } while (cursor !== undefined)
    usageByOwner.value = counts
  } catch {
    usageByOwner.value = counts
  }
}

function openDelete(user: AccountUser) {
  // Self-delete is blocked client-side (the row menu also disables it): an
  // admin removing their own account would lock themselves out.
  if (sessionUser.value && user.id === sessionUser.value.id) {
    toast.add({ title: t('accounts.delete.selfBlocked'), icon: 'i-lucide-triangle-alert', color: 'error' })
    return
  }
  deleteTarget.value = user
  deleting.value = false
  deleteFailure.value = null
  deleteOpen.value = true
}

async function onDelete() {
  if (!deleteTarget.value || deleting.value) {
    return
  }
  deleting.value = true
  deleteFailure.value = null
  try {
    await deleteUser(deleteTarget.value.id)
    users.value = users.value.filter(user => user.id !== deleteTarget.value?.id)
    deleteOpen.value = false
    deleteTarget.value = null
    toast.add({ title: t('accounts.delete.deleted'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    deleteFailure.value = error instanceof ApiError ? friendlyDeleteError(error) : t('accounts.delete.failed')
  } finally {
    deleting.value = false
  }
}

function friendlyDeleteError(error: ApiError): string {
  if (error.status === 409) {
    return t('accounts.delete.ownsInstances')
  }
  return error.message
}

// Bulk delete runs behind the programmatic confirm like the detail-page
// confirms: the overlay resolves true only on the confirm button, then the
// loop runs here with the loading state on the bulk-bar button.
const bulkDeleting = ref(false)

async function openBulkDelete() {
  if (bulkDeleting.value) {
    return
  }
  const confirmed = await confirmDelete({
    title: t('accounts.bulkDelete.title'),
    description: t('accounts.bulkDelete.body', { count: selectedCount.value }),
    confirmLabel: t('accounts.bulkDelete.submit')
  })
  if (!confirmed) {
    return
  }
  await onBulkDelete()
}

async function onBulkDelete() {
  if (bulkDeleting.value) {
    return
  }
  const selfId = sessionUser.value?.id
  const targets = selectedUsers().filter(user => user.id !== selfId)
  if (targets.length === 0) {
    toast.add({ title: t('accounts.bulkDelete.selfSkipped'), icon: 'i-lucide-triangle-alert', color: 'warning' })
    return
  }
  bulkDeleting.value = true
  try {
    let deleted = 0
    let failed = 0
    for (const target of targets) {
      try {
        await deleteUser(target.id)
        deleted += 1
      } catch {
        failed += 1
      }
    }
    if (failed === 0) {
      // Clean run: drop the rows locally without a reload.
      const removedIds = new Set(targets.map(target => target.id))
      users.value = users.value.filter(user => !removedIds.has(user.id))
      toast.add({ title: t('accounts.bulkDelete.deleted', { count: deleted }), icon: 'i-lucide-check', color: 'success' })
    } else {
      // Partial run: reload so the rows reflect exactly what the server kept.
      await load()
      toast.add({ title: t('accounts.bulkDelete.partial', { deleted, failed }), icon: 'i-lucide-triangle-alert', color: 'warning' })
    }
    clearSelection()
  } finally {
    bulkDeleting.value = false
  }
}

if (isAdmin.value) {
  await load()
}
</script>

<template>
  <UDashboardPanel id="accounts">
    <template #header>
      <UDashboardNavbar :title="t('accounts.title')">
        <template #leading>
          <UDashboardSidebarCollapse />
        </template>
        <template #right>
          <UButton icon="i-lucide-plus" :label="t('accounts.create.title')" @click="createOpen = true" />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <p class="mb-4 text-sm text-muted">
        {{ t('accounts.subtitle') }}
      </p>

      <!-- Initial load renders skeletons; the table mounts only after load, so no :loading prop (no background reload on this screen). -->
      <div v-if="pending" class="flex flex-col gap-2">
        <USkeleton class="h-12 w-full" />
        <USkeleton class="h-12 w-full" />
        <USkeleton class="h-12 w-full" />
      </div>

      <UAlert
        v-else-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      >
        <template #actions>
          <UButton
            color="error"
            variant="soft"
            :label="t('common.retry')"
            @click="load"
          />
        </template>
      </UAlert>

      <div v-else class="flex flex-col gap-3">
        <div class="flex flex-wrap items-center justify-between gap-1.5" role="group" :aria-label="t('accounts.table.filtersLabel')">
          <UInput
            v-model="searchInput"
            icon="i-lucide-search"
            :placeholder="t('accounts.table.search')"
            :aria-label="t('accounts.table.search')"
            class="max-w-sm"
          />

          <div class="flex flex-wrap items-center gap-1.5">
            <UButton
              v-if="selectedCount > 0"
              color="neutral"
              variant="subtle"
              icon="i-lucide-copy"
              :label="t('accounts.table.copyEmails')"
              @click="copySelectedEmails"
            >
              <template #trailing>
                <UKbd>
                  {{ selectedCount }}
                </UKbd>
              </template>
            </UButton>
            <UButton
              v-if="selectedCount > 0"
              color="error"
              variant="subtle"
              icon="i-lucide-trash-2"
              :loading="bulkDeleting"
              :label="t('accounts.table.deleteSelected')"
              @click="openBulkDelete"
            >
              <template #trailing>
                <UKbd>
                  {{ selectedCount }}
                </UKbd>
              </template>
            </UButton>
            <USelect
              v-model="roleFilter"
              :items="roleFilterItems"
              :aria-label="t('accounts.table.roleFilter')"
              :placeholder="t('accounts.table.roleFilter')"
              class="min-w-28"
              :ui="{ trailingIcon: 'group-data-[state=open]:rotate-180 transition-transform duration-200' }"
            />
            <UDropdownMenu
              :items="[columnItems]"
              :content="{ align: 'end' }"
            >
              <UButton
                :label="t('accounts.table.display')"
                color="neutral"
                variant="outline"
                trailing-icon="i-lucide-settings-2"
              />
            </UDropdownMenu>
          </div>
        </div>

        <UTable
          ref="table"
          v-model:sorting="sorting"
          v-model:global-filter="globalFilter"
          v-model:column-filters="columnFilters"
          v-model:column-visibility="columnVisibility"
          v-model:row-selection="rowSelection"
          v-model:pagination="pagination"
          :data="users"
          :columns="columns"
          :global-filter-options="globalFilterOptions"
          :pagination-options="paginationOptions"
          :get-row-id="getRowId"
          :auto-reset-all="false"
          class="shrink-0"
          :ui="{
            base: 'table-fixed border-separate border-spacing-0',
            thead: '[&>tr]:bg-elevated/50 [&>tr]:after:content-none',
            tbody: '[&>tr]:last:[&>td]:border-b-0',
            th: 'py-2 first:rounded-l-lg last:rounded-r-lg border-y border-default first:border-l last:border-r',
            td: 'border-b border-default',
            separator: 'h-0'
          }"
        >
          <template #select-header="{ table: api }">
            <UCheckbox
              :model-value="api.getIsSomePageRowsSelected() ? 'indeterminate' : api.getIsAllPageRowsSelected()"
              :aria-label="t('accounts.table.selectAll')"
              @update:model-value="(value: boolean | 'indeterminate') => api.toggleAllPageRowsSelected(!!value)"
            />
          </template>

          <template #select-cell="{ row }">
            <UCheckbox
              :model-value="row.getIsSelected()"
              :aria-label="t('accounts.table.selectRow')"
              @update:model-value="(value: boolean | 'indeterminate') => row.toggleSelected(!!value)"
            />
          </template>

          <template #email-header="{ column }">
            <UButton
              color="neutral"
              variant="ghost"
              class="-mx-2.5"
              :label="t('common.email')"
              :icon="column.getIsSorted() ? (column.getIsSorted() === 'asc' ? 'i-lucide-arrow-up-narrow-wide' : 'i-lucide-arrow-down-wide-narrow') : 'i-lucide-arrow-up-down'"
              :aria-label="sortActionLabel('email')"
              @click="column.toggleSorting(column.getIsSorted() === 'asc')"
            />
          </template>

          <template #role-header="{ column }">
            <UButton
              color="neutral"
              variant="ghost"
              class="-mx-2.5"
              :label="t('common.role')"
              :icon="column.getIsSorted() ? (column.getIsSorted() === 'asc' ? 'i-lucide-arrow-up-narrow-wide' : 'i-lucide-arrow-down-wide-narrow') : 'i-lucide-arrow-up-down'"
              :aria-label="sortActionLabel('role')"
              @click="column.toggleSorting(column.getIsSorted() === 'asc')"
            />
          </template>

          <template #instance_quota-header="{ column }">
            <UButton
              color="neutral"
              variant="ghost"
              class="-mx-2.5"
              :label="t('accounts.quotaLabel')"
              :icon="column.getIsSorted() ? (column.getIsSorted() === 'asc' ? 'i-lucide-arrow-up-narrow-wide' : 'i-lucide-arrow-down-wide-narrow') : 'i-lucide-arrow-up-down'"
              :aria-label="sortActionLabel('instance_quota')"
              @click="column.toggleSorting(column.getIsSorted() === 'asc')"
            />
          </template>

          <template #email-cell="{ row }">
            <span class="block min-w-0 truncate" :title="row.original.email">
              {{ row.original.email }}
            </span>
          </template>

          <template #role-cell="{ row }">
            <AccountsTableRoleCell :user="row.original" />
          </template>

          <template #instance_quota-cell="{ row }">
            <AccountsTableQuotaCell :user="row.original" :usage="usageByOwner[row.original.id] ?? 0" />
          </template>

          <template #actions-cell="{ row }">
            <AccountsTableActionsCell
              :user="row.original"
              :is-self="sessionUser?.id === row.original.id"
              @edit-quota="quotaTarget = $event; quotaOpen = true"
              @remove="openDelete($event)"
            />
          </template>

          <template #empty>
            <UEmpty
              v-if="users.length === 0"
              icon="i-lucide-search-x"
              :title="t('accounts.empty')"
            >
              <template #actions>
                <UButton icon="i-lucide-plus" :label="t('accounts.create.title')" @click="createOpen = true" />
              </template>
            </UEmpty>
            <UEmpty
              v-else
              icon="i-lucide-search-x"
              :title="t('accounts.table.noResults')"
            >
              <template #actions>
                <UButton
                  color="neutral"
                  variant="soft"
                  :label="t('accounts.table.clearFilters')"
                  @click="clearFilters"
                />
              </template>
            </UEmpty>
          </template>
        </UTable>

        <div class="flex items-center justify-between gap-3 border-t border-default pt-4 mt-auto">
          <div class="text-sm text-muted">
            {{ t('accounts.table.selectedOf', { selected: selectedCount, filtered: totalFiltered }) }}
          </div>

          <div class="flex items-center gap-1.5">
            <UPagination
              v-if="pageCount > 1"
              :page="pagination.pageIndex + 1"
              :items-per-page="pagination.pageSize"
              :total="totalFiltered"
              size="sm"
              @update:page="onUpdatePage"
            />
          </div>
        </div>
      </div>
    </template>
  </UDashboardPanel>

  <CreateAccountModal v-model:open="createOpen" @created="(user) => { users = [user, ...users] }" />

  <EditQuotaModal v-model:open="quotaOpen" :target="quotaTarget" @updated="(user) => { users = users.map(entry => entry.id === user.id ? user : entry) }" />

  <UModal
    v-model:open="deleteOpen"
    :title="t('accounts.delete.title')"
    :description="t('accounts.delete.body', { email: deleteTarget?.email ?? '' })"
    :ui="{ footer: 'justify-end' }"
  >
    <template #body>
      <UAlert
        v-if="deleteFailure"
        color="error"
        variant="subtle"
        :title="deleteFailure"
      />
    </template>
    <template #footer="{ close }">
      <UButton
        color="neutral"
        variant="outline"
        :label="t('common.cancel')"
        @click="close"
      />
      <UButton
        color="error"
        :loading="deleting"
        :label="deleting ? t('accounts.delete.deleting') : t('accounts.delete.submit')"
        @click="onDelete"
      />
    </template>
  </UModal>
</template>
