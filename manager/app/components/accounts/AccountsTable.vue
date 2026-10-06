<script setup lang="ts">
import type { DropdownMenuItem, TableRow } from '@nuxt/ui'
import { useAccountsTable } from '~/composables/useAccountsTable'
import AccountsTableActionsCell from '~/components/accounts/AccountsTableActionsCell.vue'
import AccountsTableQuotaCell from '~/components/accounts/AccountsTableQuotaCell.vue'
import AccountsTableRoleCell from '~/components/accounts/AccountsTableRoleCell.vue'
import DataTableFooter from '~/components/shared/DataTableFooter.vue'
import DataTableToolbar from '~/components/shared/DataTableToolbar.vue'
import type { AccountRole, AccountUser } from '~/types/api'

const props = defineProps<{
  users: AccountUser[]
  sessionUserId: string | undefined
  // Owned by the page (it runs the bulk-delete loop); drives the bulk-bar
  // button loading state while the confirm + loop resolve.
  bulkDeleting: boolean
}>()

const emit = defineEmits<{
  'edit-quota': [user: AccountUser]
  'remove': [user: AccountUser]
  'bulk-delete': [users: AccountUser[]]
  // Empty-state create button (the navbar create button stays in the page, so
  // this is the only create entry owned here).
  'create': []
}>()

// Structural view of the UTable API this table drives (stable component
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

const { t } = useI18n()
const toast = useToast()
const { copy } = useClipboard()

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

function getRowId(row: AccountUser): string {
  return row.id
}

// Native table state, read from the table API (UTable owns sorting, filtering
// and pagination; the table only binds state and renders). Before the first
// mount tableApi is null, so every read falls back to the loaded users.
function filteredCount(): number {
  return table.value?.tableApi?.getFilteredRowModel().rows.length ?? props.users.length
}

// UPagination :total subscribes to the v-model state because tableApi reads
// are not reactive: sorting/filter/pagination/users changes recompute it.
const totalFiltered = computed(() => {
  void sorting.value
  void globalFilter.value
  void columnFilters.value
  void pagination.value
  void props.users
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
    case 'instance_limit':
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
    ?? props.users.filter(user => rowSelection.value[user.id])
}

function clearSelection() {
  if (table.value?.tableApi) {
    table.value.tableApi.resetRowSelection()
  } else {
    rowSelection.value = {}
  }
}

defineExpose({ clearSelection })

async function copySelectedEmails() {
  try {
    await copy(selectedUsers().map(user => user.email).join('\n'))
    toast.add({ title: t('accounts.table.copiedEmails'), icon: 'i-lucide-check', color: 'success' })
  } catch {
    toast.add({ title: t('accounts.table.copyFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
  }
}

// The toolbar owns the debounced search input (v-model:filter writes straight
// into globalFilter and syncs back on external resets), so clearing the
// search is a single globalFilter reset; the role filter stays immediate.
function clearFilters() {
  globalFilter.value = ''
  columnFilters.value = []
}

function onBulkDeleteClick() {
  emit('bulk-delete', selectedUsers())
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
    : columnId === 'instance_limit' ? t('accounts.quotaLabel') : t('common.email')
  return nextDesc ? t('accounts.table.sortDesc', { column }) : t('accounts.table.sortAsc', { column })
}

// UTable exposes no th slot, so aria-sort stays off the header buttons; sort
// state travels in the button aria-label (sortAsc/sortDesc) instead.
</script>

<template>
  <div class="flex flex-col gap-3">
    <DataTableToolbar
      v-model:filter="globalFilter"
      :search-placeholder="t('accounts.table.search')"
      :search-aria="t('accounts.table.search')"
      :filters-label="t('accounts.table.filtersLabel')"
      :display-label="t('accounts.table.display')"
      :column-items="columnItems"
    >
      <template #bulk>
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
          @click="onBulkDeleteClick"
        >
          <template #trailing>
            <UKbd>
              {{ selectedCount }}
            </UKbd>
          </template>
        </UButton>
      </template>

      <template #filters>
        <USelect
          v-model="roleFilter"
          :items="roleFilterItems"
          :aria-label="t('accounts.table.roleFilter')"
          :placeholder="t('accounts.table.roleFilter')"
          class="min-w-28"
          :ui="{ trailingIcon: 'group-data-[state=open]:rotate-180 transition-transform duration-200' }"
        />
      </template>
    </DataTableToolbar>

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

      <template #instance_limit-header="{ column }">
        <UButton
          color="neutral"
          variant="ghost"
          class="-mx-2.5"
          :label="t('accounts.quotaLabel')"
          :icon="column.getIsSorted() ? (column.getIsSorted() === 'asc' ? 'i-lucide-arrow-up-narrow-wide' : 'i-lucide-arrow-down-wide-narrow') : 'i-lucide-arrow-up-down'"
          :aria-label="sortActionLabel('instance_limit')"
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

      <template #instance_limit-cell="{ row }">
        <AccountsTableQuotaCell :user="row.original" />
      </template>

      <template #actions-cell="{ row }">
        <AccountsTableActionsCell
          :user="row.original"
          :is-self="sessionUserId === row.original.id"
          @edit-quota="(user: AccountUser) => emit('edit-quota', user)"
          @remove="(user: AccountUser) => emit('remove', user)"
        />
      </template>

      <template #empty>
        <UEmpty
          v-if="users.length === 0"
          icon="i-lucide-search-x"
          :title="t('accounts.empty')"
        >
          <template #actions>
            <UButton icon="i-lucide-plus" :label="t('accounts.create.title')" @click="emit('create')" />
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

    <DataTableFooter
      :status-text="t('accounts.table.selectedOf', { selected: selectedCount, filtered: totalFiltered })"
      :page="pagination.pageIndex + 1"
      :page-size="pagination.pageSize"
      :total="totalFiltered"
      :page-count="pageCount"
      @update:page="onUpdatePage"
    />
  </div>
</template>
