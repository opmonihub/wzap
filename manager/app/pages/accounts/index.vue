<script setup lang="ts">
import type { DropdownMenuItem, TableRow } from '@nuxt/ui'
import { ApiError } from '~/composables/useApi'
import { useAccountsTable } from '~/composables/useAccountsTable'
import AccountsTableActionsCell from '~/components/accounts/AccountsTableActionsCell.vue'
import AccountsTableQuotaCell from '~/components/accounts/AccountsTableQuotaCell.vue'
import AccountsTableRoleCell from '~/components/accounts/AccountsTableRoleCell.vue'
import type { AccountRole, AccountUser } from '~/types/api'

// Structural view of the UTable API this page drives (stable component
// instance typing without importing @tanstack/*, which stays transitive-only).
interface AccountsTableApi {
  getFilteredRowModel: () => { rows: TableRow<AccountUser>[] }
  getFilteredSelectedRowModel: () => { rows: TableRow<AccountUser>[] }
  setPageIndex: (index: number) => void
  setPageSize: (size: number) => void
  resetRowSelection: () => void
}

// Account management is admin-only: the sidebar hides this screen for user
// accounts and the guard below turns a direct visit back to the overview.
// Quotas are per user (0 means unlimited); deleting an owner that still owns
// instances answers 409 and removes nothing.
const { t } = useI18n()
const toast = useToast()
const { isAdmin } = useAuth()
const { listUsers, createUser, deleteUser, updateUserQuota } = useAccounts()

if (!isAdmin.value) {
  await navigateTo('/')
}

const users = ref<AccountUser[]>([])
const pending = ref(true)
const failure = ref<string | null>(null)

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

// Role column filter behind a USelect: '' means Todos (no column filter).
const roleFilter = computed<string>({
  get: () => {
    const current = columnFilters.value.find(entry => entry.id === 'role')?.value
    return typeof current === 'string' ? current : ''
  },
  set: (value) => {
    const rest = columnFilters.value.filter(entry => entry.id !== 'role')
    columnFilters.value = value === '' ? rest : [...rest, { id: 'role', value }]
  }
})

const accountRoles: AccountRole[] = ['admin', 'user']

function roleOptionLabel(role: AccountRole): string {
  return role === 'admin' ? t('userMenu.roleAdmin') : t('userMenu.roleUser')
}

const roleFilterItems = computed(() => [
  { label: t('accounts.table.allRoles'), value: '' },
  ...accountRoles.map(role => ({ label: roleOptionLabel(role), value: role }))
])

// Select and actions never hide (bulk bar + per-row edit/delete stay
// reachable); every other column is user-toggleable.
const hideableColumnIds = computed<string[]>(() =>
  columns.value
    .filter(column => column.id !== 'select' && column.id !== 'actions')
    .map(column => column.id ?? '')
    .filter(id => id !== '')
)

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

const visibilityItems = computed<DropdownMenuItem[]>(() =>
  hideableColumnIds.value.map(columnId => ({
    label: columnLabel(columnId),
    type: 'checkbox' as const,
    checked: columnVisibility.value[columnId] ?? true,
    onUpdateChecked: (checked: boolean) => {
      columnVisibility.value = { ...columnVisibility.value, [columnId]: checked }
    },
    onSelect: (event: Event) => {
      event.preventDefault()
    }
  }))
)

const selectedCount = computed(() => {
  const fallback = Object.values(rowSelection.value).filter(Boolean).length
  return table.value?.tableApi?.getFilteredSelectedRowModel().rows.length ?? fallback
})

function clearSelection() {
  if (table.value?.tableApi) {
    table.value.tableApi.resetRowSelection()
  } else {
    rowSelection.value = {}
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

// Page size travels through the table API when mounted (keeps v-model in
// sync) and resets to the first page; fallback writes the ref directly.
function onUpdatePageSize(size: number) {
  const next = Number(size) || 10
  if (table.value?.tableApi) {
    table.value.tableApi.setPageSize(next)
  } else {
    pagination.value.pageSize = next
  }
  pagination.value.pageIndex = 0
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
const countLabel = computed(() => t('accounts.table.loadedCount', { count: users.value.length }))
const pageLabel = computed(() => t('accounts.table.pageOf', { page: pagination.value.pageIndex + 1, pages: pageCount.value }))

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
const createEmail = ref('')
const createPassword = ref('')
const createRole = ref<AccountRole>('user')
const createQuota = ref('')
const creating = ref(false)
const createFailure = ref<string | null>(null)

const quotaTarget = ref<AccountUser | null>(null)
const quotaOpen = ref(false)
const quotaValue = ref('')
const quotaSaving = ref(false)
const quotaFailure = ref<string | null>(null)

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
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('accounts.loadFailed')
  } finally {
    pending.value = false
  }
}

function resetCreate() {
  createEmail.value = ''
  createPassword.value = ''
  createRole.value = 'user'
  createQuota.value = ''
  creating.value = false
  createFailure.value = null
}

watch(createOpen, (value) => {
  if (value) {
    resetCreate()
  }
})

// An empty quota stays omitted so the server default applies; 0 is a valid
// explicit value meaning unlimited.
function parseQuota(raw: string): number | undefined | null {
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

async function onCreate() {
  if (creating.value) {
    return
  }
  const email = createEmail.value.trim()
  if (email === '') {
    createFailure.value = t('accounts.create.emailRequired')
    return
  }
  if (createPassword.value === '') {
    createFailure.value = t('accounts.create.passwordRequired')
    return
  }
  const quota = parseQuota(createQuota.value)
  if (quota === null) {
    createFailure.value = t('accounts.create.quotaInvalid')
    return
  }
  creating.value = true
  createFailure.value = null
  try {
    const created = await createUser({ email, password: createPassword.value, role: createRole.value, instance_quota: quota })
    users.value = [created, ...users.value]
    createOpen.value = false
    toast.add({ title: t('accounts.create.createdToast'), color: 'success' })
  } catch (error) {
    createFailure.value = error instanceof ApiError ? friendlyCreateError(error) : t('accounts.create.failed')
  } finally {
    creating.value = false
  }
}

function friendlyCreateError(error: ApiError): string {
  if (error.status === 409) {
    return t('accounts.create.emailTaken')
  }
  return error.message
}

function openQuota(user: AccountUser) {
  quotaTarget.value = user
  quotaValue.value = String(user.instance_quota)
  quotaSaving.value = false
  quotaFailure.value = null
  quotaOpen.value = true
}

async function onSaveQuota() {
  if (!quotaTarget.value || quotaSaving.value) {
    return
  }
  const quota = parseQuota(quotaValue.value)
  if (quota === null || quota === undefined) {
    quotaFailure.value = t('accounts.quota.quotaInvalid')
    return
  }
  quotaSaving.value = true
  quotaFailure.value = null
  try {
    const updated = await updateUserQuota(quotaTarget.value.id, quota)
    users.value = users.value.map(user => user.id === updated.id ? updated : user)
    quotaOpen.value = false
    quotaTarget.value = null
    toast.add({ title: t('accounts.quota.updated'), color: 'success' })
  } catch (error) {
    quotaFailure.value = error instanceof ApiError ? error.message : t('accounts.quota.saveFailed')
  } finally {
    quotaSaving.value = false
  }
}

function openDelete(user: AccountUser) {
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
    toast.add({ title: t('accounts.delete.deleted'), color: 'success' })
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
        <div role="group" :aria-label="t('accounts.table.filtersLabel')" class="flex flex-wrap items-center gap-2">
          <UInput
            v-model="searchInput"
            icon="i-lucide-search"
            :placeholder="t('accounts.table.search')"
            :aria-label="t('accounts.table.search')"
            class="min-w-52 flex-1"
          />
          <USelect
            v-model="roleFilter"
            :items="roleFilterItems"
            :aria-label="t('accounts.table.roleFilter')"
            class="min-w-40"
          />
          <UDropdownMenu
            :items="visibilityItems"
            :content="{ align: 'end' }"
          >
            <UButton
              :label="t('accounts.table.visibility')"
              color="neutral"
              variant="outline"
              trailing-icon="i-lucide-chevron-down"
              class="min-h-11"
              :aria-label="t('accounts.table.visibility')"
            />
          </UDropdownMenu>
        </div>

        <p class="text-sm text-muted">
          {{ countLabel }}
        </p>

        <div
          v-if="selectedCount > 0"
          class="flex flex-wrap items-center gap-2 rounded-lg bg-elevated px-3 py-2"
        >
          <p class="text-sm">
            {{ t('accounts.table.selectedCount', { count: selectedCount }) }}
          </p>
          <UButton
            color="neutral"
            variant="ghost"
            size="sm"
            icon="i-lucide-x"
            :label="t('accounts.table.clearSelection')"
            @click="clearSelection"
          />
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
              size="sm"
              class="min-h-11"
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
              size="sm"
              class="min-h-11"
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
              size="sm"
              class="min-h-11"
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
            <AccountsTableQuotaCell :user="row.original" />
          </template>

          <template #actions-cell="{ row }">
            <AccountsTableActionsCell
              :user="row.original"
              @edit-quota="openQuota($event)"
              @remove="openDelete($event)"
            />
          </template>

          <template #empty>
            <div class="flex flex-col items-center gap-3 py-8 text-center">
              <template v-if="users.length === 0">
                <p class="text-sm text-muted">
                  {{ t('accounts.empty') }}
                </p>
                <UButton icon="i-lucide-plus" :label="t('accounts.create.title')" @click="createOpen = true" />
              </template>
              <template v-else>
                <p class="text-sm text-muted">
                  {{ t('accounts.table.noResults') }}
                </p>
                <UButton
                  color="neutral"
                  variant="soft"
                  :label="t('accounts.table.clearFilters')"
                  @click="clearFilters"
                />
              </template>
            </div>
          </template>
        </UTable>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-sm text-muted">
            {{ pageLabel }}
          </p>
          <div class="flex flex-wrap items-center gap-3">
            <USelect
              :model-value="pagination.pageSize"
              :items="[10, 25, 50]"
              :aria-label="t('accounts.table.pageSize')"
              @update:model-value="onUpdatePageSize"
            />
            <UPagination
              v-if="pageCount > 1"
              :page="pagination.pageIndex + 1"
              :items-per-page="pagination.pageSize"
              :total="totalFiltered"
              @update:page="onUpdatePage"
            />
          </div>
        </div>
      </div>
    </template>
  </UDashboardPanel>

  <UModal v-model:open="createOpen" :title="t('accounts.create.title')" :description="t('accounts.create.body')">
    <template #body>
      <form class="flex flex-col gap-4" @submit.prevent="onCreate">
        <UAlert
          v-if="createFailure"
          color="error"
          variant="subtle"
          :title="createFailure"
        />

        <UFormField :label="t('common.email')" name="email" required>
          <UInput
            v-model="createEmail"
            type="email"
            required
            maxlength="255"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('auth.password')" name="password" required>
          <UInput
            v-model="createPassword"
            type="password"
            required
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('common.role')" name="role" required>
          <USelect
            v-model="createRole"
            :items="['admin', 'user']"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('accounts.quotaLabel')" :hint="t('accounts.create.quotaHint')" name="instance_quota">
          <UInput
            v-model="createQuota"
            type="number"
            min="0"
            step="1"
            class="w-full"
          />
        </UFormField>

        <div class="flex justify-end gap-2">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            :label="t('common.cancel')"
            @click="createOpen = false"
          />
          <UButton type="submit" :loading="creating" :label="creating ? t('accounts.create.creating') : t('accounts.create.submit')" />
        </div>
      </form>
    </template>
  </UModal>

  <UModal v-model:open="quotaOpen" :title="t('accounts.quota.title')" :description="t('accounts.quota.body', { email: quotaTarget?.email ?? '' })">
    <template #body>
      <form class="flex flex-col gap-4" @submit.prevent="onSaveQuota">
        <UAlert
          v-if="quotaFailure"
          color="error"
          variant="subtle"
          :title="quotaFailure"
        />

        <UFormField
          :label="t('accounts.quotaLabel')"
          :hint="t('accounts.quota.hint')"
          name="instance_quota"
          required
        >
          <UInput
            v-model="quotaValue"
            type="number"
            required
            min="0"
            step="1"
            class="w-full"
          />
        </UFormField>

        <div class="flex justify-end gap-2">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            :label="t('common.cancel')"
            @click="quotaOpen = false"
          />
          <UButton type="submit" :loading="quotaSaving" :label="quotaSaving ? t('common.saving') : t('common.save')" />
        </div>
      </form>
    </template>
  </UModal>

  <UModal v-model:open="deleteOpen" :title="t('accounts.delete.title')" :description="t('accounts.delete.body', { email: deleteTarget?.email ?? '' })">
    <template #body>
      <UAlert
        v-if="deleteFailure"
        color="error"
        variant="subtle"
        :title="deleteFailure"
      />
    </template>
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton
          color="neutral"
          variant="ghost"
          :label="t('common.cancel')"
          @click="deleteOpen = false"
        />
        <UButton
          color="error"
          :loading="deleting"
          :label="deleting ? t('accounts.delete.deleting') : t('accounts.delete.submit')"
          @click="onDelete"
        />
      </div>
    </template>
  </UModal>
</template>
