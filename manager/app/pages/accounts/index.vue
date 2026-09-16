<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import type { DropdownMenuItem, TableRow } from '@nuxt/ui'
import { ApiError } from '~/composables/useApi'
import { useAccountsTable } from '~/composables/useAccountsTable'
import AccountsTableActionsCell from '~/components/accounts/AccountsTableActionsCell.vue'
import AccountsTableQuotaCell from '~/components/accounts/AccountsTableQuotaCell.vue'
import AccountsTableRoleCell from '~/components/accounts/AccountsTableRoleCell.vue'
import type { AccountRole, AccountUser } from '~/types/api'

// Structural view of the UTable API this page drives (stable component
// instance typing without importing @tanstack/*, which stays transitive-only).
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
const { listUsers, createUser, deleteUser, updateUserQuota } = useAccounts()
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
    toast.add({ title: t('accounts.table.copiedEmails'), color: 'success' })
  } catch {
    toast.add({ title: t('accounts.table.copyFailed'), color: 'error' })
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
// USelect may emit a string, so the signature accepts both and normalizes.
function onUpdatePageSize(size: number | string) {
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
const loadedLabel = computed(() => t('accounts.table.loadedCount', { count: users.value.length }))
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
const createSchema = z.object({
  email: z.string().min(1, t('accounts.create.emailRequired')).max(255),
  password: z.string().min(1, t('accounts.create.passwordRequired')),
  role: z.enum(['admin', 'user']),
  instance_quota: z.string()
})
type CreateSchema = z.output<typeof createSchema>
const createState = reactive<Partial<CreateSchema>>({ email: '', password: '', role: 'user', instance_quota: '' })
const creating = ref(false)
const createFailure = ref<string | null>(null)

const quotaTarget = ref<AccountUser | null>(null)
const quotaOpen = ref(false)
const quotaSchema = z.object({
  instance_quota: z.string()
})
type QuotaSchema = z.output<typeof quotaSchema>
const quotaState = reactive<Partial<QuotaSchema>>({ instance_quota: '' })
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

function resetCreate() {
  createState.email = ''
  createState.password = ''
  createState.role = 'user'
  createState.instance_quota = ''
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

async function onCreate(event: FormSubmitEvent<CreateSchema>) {
  if (creating.value) {
    return
  }
  const email = (event.data.email ?? '').trim()
  if (email === '') {
    createFailure.value = t('accounts.create.emailRequired')
    return
  }
  if ((event.data.password ?? '') === '') {
    createFailure.value = t('accounts.create.passwordRequired')
    return
  }
  const quota = parseQuota(event.data.instance_quota ?? '')
  if (quota === null) {
    createFailure.value = t('accounts.create.quotaInvalid')
    return
  }
  creating.value = true
  createFailure.value = null
  try {
    const created = await createUser({ email, password: event.data.password ?? '', role: event.data.role ?? 'user', instance_quota: quota })
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
  quotaState.instance_quota = String(user.instance_quota)
  quotaSaving.value = false
  quotaFailure.value = null
  quotaOpen.value = true
}

async function onSaveQuota(event: FormSubmitEvent<QuotaSchema>) {
  if (!quotaTarget.value || quotaSaving.value) {
    return
  }
  const quota = parseQuota(event.data.instance_quota ?? '')
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
  // Self-delete is blocked client-side (the row menu also disables it): an
  // admin removing their own account would lock themselves out.
  if (sessionUser.value && user.id === sessionUser.value.id) {
    toast.add({ title: t('accounts.delete.selfBlocked'), color: 'error' })
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

// Bulk delete over the selected rows: the signed-in account is always
// skipped, owners that still own instances fail with the 409 message, and the
// summary toast reports deleted vs failed counts.
const bulkOpen = ref(false)
const bulkDeleting = ref(false)
const bulkFailure = ref<string | null>(null)

function openBulkDelete() {
  bulkFailure.value = null
  bulkDeleting.value = false
  bulkOpen.value = true
}

async function onBulkDelete() {
  if (bulkDeleting.value) {
    return
  }
  const selfId = sessionUser.value?.id
  const targets = selectedUsers().filter(user => user.id !== selfId)
  if (targets.length === 0) {
    bulkOpen.value = false
    toast.add({ title: t('accounts.bulkDelete.selfSkipped'), color: 'warning' })
    return
  }
  bulkDeleting.value = true
  bulkFailure.value = null
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
    toast.add({ title: t('accounts.bulkDelete.deleted', { count: deleted }), color: 'success' })
  } else {
    // Partial run: reload so the rows reflect exactly what the server kept.
    await load()
    toast.add({ title: t('accounts.bulkDelete.partial', { deleted, failed }), color: 'warning' })
  }
  clearSelection()
  bulkOpen.value = false
  bulkDeleting.value = false
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
        <UDashboardToolbar role="group" :aria-label="t('accounts.table.filtersLabel')">
          <template #left>
            <UInput
              v-model="searchInput"
              icon="i-lucide-search"
              :placeholder="t('accounts.table.search')"
              :aria-label="t('accounts.table.search')"
              class="min-w-52 flex-1"
            />
          </template>
          <template #right>
            <USelect
              v-model="roleFilter"
              :items="roleFilterItems"
              :aria-label="t('accounts.table.roleFilter')"
              class="min-w-40"
            />
            <UDropdownMenu
              :items="[columnItems]"
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
          </template>
        </UDashboardToolbar>

        <p class="text-sm text-muted">
          {{ loadedLabel }}
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
            icon="i-lucide-copy"
            :label="t('accounts.table.copyEmails')"
            @click="copySelectedEmails"
          />
          <UButton
            color="error"
            variant="ghost"
            size="sm"
            icon="i-lucide-trash-2"
            :label="t('accounts.table.deleteSelected')"
            @click="openBulkDelete"
          />
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
              :color="column.getIsSorted() ? 'primary' : 'neutral'"
              :variant="column.getIsSorted() ? 'soft' : 'ghost'"
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
              :color="column.getIsSorted() ? 'primary' : 'neutral'"
              :variant="column.getIsSorted() ? 'soft' : 'ghost'"
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
              :color="column.getIsSorted() ? 'primary' : 'neutral'"
              :variant="column.getIsSorted() ? 'soft' : 'ghost'"
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
            <AccountsTableQuotaCell :user="row.original" :usage="usageByOwner[row.original.id] ?? 0" />
          </template>

          <template #actions-cell="{ row }">
            <AccountsTableActionsCell
              :user="row.original"
              :is-self="sessionUser?.id === row.original.id"
              @edit-quota="openQuota($event)"
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
      <UForm
        id="create-account"
        :schema="createSchema"
        :state="createState"
        class="flex flex-col gap-4"
        @submit="onCreate"
      >
        <UAlert
          v-if="createFailure"
          color="error"
          variant="subtle"
          :title="createFailure"
        />

        <UFormField :label="t('common.email')" name="email" required>
          <UInput
            v-model="createState.email"
            type="email"
            maxlength="255"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('auth.password')" name="password" required>
          <UInput
            v-model="createState.password"
            type="password"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('common.role')" name="role" required>
          <USelect
            v-model="createState.role"
            :items="['admin', 'user']"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('accounts.quotaLabel')" :hint="t('accounts.create.quotaHint')" name="instance_quota">
          <UInput
            v-model="createState.instance_quota"
            type="number"
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
      </UForm>
    </template>
  </UModal>

  <UModal v-model:open="quotaOpen" :title="t('accounts.quota.title')" :description="t('accounts.quota.body', { email: quotaTarget?.email ?? '' })">
    <template #body>
      <UForm
        id="edit-quota"
        :schema="quotaSchema"
        :state="quotaState"
        class="flex flex-col gap-4"
        @submit="onSaveQuota"
      >
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
            v-model="quotaState.instance_quota"
            type="number"
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
      </UForm>
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

  <UModal v-model:open="bulkOpen" :title="t('accounts.bulkDelete.title')" :description="t('accounts.bulkDelete.body', { count: selectedCount })">
    <template #body>
      <UAlert
        v-if="bulkFailure"
        color="error"
        variant="subtle"
        :title="bulkFailure"
      />
    </template>
    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton
          color="neutral"
          variant="ghost"
          :label="t('common.cancel')"
          @click="bulkOpen = false"
        />
        <UButton
          color="error"
          :loading="bulkDeleting"
          :label="bulkDeleting ? t('accounts.bulkDelete.deleting') : t('accounts.bulkDelete.submit')"
          @click="onBulkDelete"
        />
      </div>
    </template>
  </UModal>
</template>
