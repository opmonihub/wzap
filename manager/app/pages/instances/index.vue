<script setup lang="ts">
import type { DropdownMenuItem, TableRow } from '@nuxt/ui'
import { useMediaQuery } from '@vueuse/core'
import { ApiError } from '~/composables/useApi'
import { useInstancesTable } from '~/composables/useInstancesTable'
import CreateInstanceModal from '~/components/instances/CreateInstanceModal.vue'
import DeleteInstanceModal from '~/components/instances/DeleteInstanceModal.vue'
import EditInstanceModal from '~/components/instances/EditInstanceModal.vue'
import InstancesTableActionsCell from '~/components/instances/InstancesTableActionsCell.vue'
import InstancesTableJidCell from '~/components/instances/InstancesTableJidCell.vue'
import InstancesTableNameCell from '~/components/instances/InstancesTableNameCell.vue'
import InstancesTableOwnerCell from '~/components/instances/InstancesTableOwnerCell.vue'
import InstancesTableStatusCell from '~/components/instances/InstancesTableStatusCell.vue'
import type { CreatedInstance, Instance, InstanceStatus } from '~/types/api'

// Structural view of the UTable API this page drives (stable component
// instance typing as a structural view without importing table-core types directly).
interface InstancesTableColumn {
  id: string
  getCanHide: () => boolean
  getIsVisible: () => boolean
  toggleVisibility: (visible: boolean) => void
}

interface InstancesTableApi {
  getFilteredRowModel: () => { rows: TableRow<Instance>[] }
  getFilteredSelectedRowModel: () => { rows: TableRow<Instance>[] }
  setPageIndex: (index: number) => void
  setPageSize: (size: number) => void
  resetRowSelection: () => void
  getAllColumns: () => InstancesTableColumn[]
}

const { t } = useI18n()
const toast = useToast()
const { isAdmin } = useAuth()
const { listInstances, listAccounts, connectInstance } = useInstances()
const { copy } = useClipboard()

const items = ref<Instance[]>([])
const nextCursor = ref('')
const pending = ref(true)
const loadingMore = ref(false)
const failure = ref<string | null>(null)
const createOpen = ref(false)
const ownerEmails = ref<Record<string, string>>({})

const {
  columns,
  sorting,
  globalFilter,
  columnFilters,
  columnVisibility: visibilityOverrides,
  rowSelection,
  pagination,
  globalFilterOptions,
  paginationOptions
} = useInstancesTable(items, ownerEmails, isAdmin)

const table = useTemplateRef<{ tableApi?: InstancesTableApi }>('table')

// Search input debounced into the table's global filter (300ms): typing never
// re-filters per keystroke on 1000+ rows; the status select stays immediate.
const searchInput = ref('')
watchDebounced(searchInput, (value) => {
  globalFilter.value = value
}, { debounce: 300 })

// Client-side viewport mirrors the old cards (owner hidden below md, JID
// below lg). useMediaQuery is mobile-first on SSR (false until mount), so the
// first paint already hides both columns on small screens.
const isMdViewport = useMediaQuery('(min-width: 768px)')
const isLgViewport = useMediaQuery('(min-width: 1024px)')

// The admin-only marker travels as untyped column meta (see
// useInstancesTable); read it with a cast, as a structural view without importing table-core types directly.
function isAdminOnly(columnId: string): boolean {
  const column = columns.value.find(entry => entry.id === columnId)
  return (column?.meta as unknown as { ifAdmin?: boolean } | undefined)?.ifAdmin ?? false
}

function baseColumnVisibility(): Record<string, boolean> {
  return {
    owner: (!isAdminOnly('owner') || isAdmin.value) && isMdViewport.value,
    whatsapp_jid: isLgViewport.value
  }
}

// Viewport defaults merged with the user's dropdown overrides (kept in the
// composable ref). Untouched columns keep tracking the viewport; a column the
// user toggled stays on their choice until toggled back to the default.
const columnVisibility = computed<Record<string, boolean>>({
  get: () => ({ ...baseColumnVisibility(), ...visibilityOverrides.value }),
  set: (next) => {
    const base = baseColumnVisibility()
    const diff: Record<string, boolean> = {}
    for (const key of Object.keys(next)) {
      const value = next[key] ?? true
      if (value !== (base[key] ?? true)) {
        diff[key] = value
      }
    }
    visibilityOverrides.value = diff
  }
})

function getRowId(row: Instance): string {
  return row.id
}

// Native table state, read from the table API (UTable owns sorting, filtering
// and pagination; the page only binds state and renders). Before the first
// mount tableApi is null, so every read falls back to the loaded items.
function filteredCount(): number {
  return table.value?.tableApi?.getFilteredRowModel().rows.length ?? items.value.length
}

// UPagination :total subscribes to the v-model state because tableApi reads
// are not reactive: sorting/filter/pagination/items changes recompute it.
const totalFiltered = computed(() => {
  void sorting.value
  void globalFilter.value
  void columnFilters.value
  void pagination.value
  void items.value
  return filteredCount()
})
const pageCount = computed(() => Math.max(1, Math.ceil(totalFiltered.value / pagination.value.pageSize)))

// Status column filter behind a USelect: '' means Todos (no column filter).
const statusFilter = computed<string>({
  get: () => {
    const current = columnFilters.value.find(entry => entry.id === 'status')?.value
    return typeof current === 'string' ? current : ''
  },
  set: (value) => {
    const rest = columnFilters.value.filter(entry => entry.id !== 'status')
    columnFilters.value = value === '' ? rest : [...rest, { id: 'status', value }]
  }
})

const instanceStatuses: InstanceStatus[] = ['disconnected', 'pairing', 'connected', 'error']

const statusFilterItems = computed(() => [
  { label: t('instances.table.allStatuses'), value: '' },
  ...instanceStatuses.map(status => ({ label: t(`instances.status.${status}`), value: status }))
])

// Name, status, select and actions are essential and locked
// (enableHiding:false in the composable); the dropdown only lists columns
// where the table API reports getCanHide().
function columnLabel(columnId: string): string {
  switch (columnId) {
    case 'name':
      return t('instances.columns.name')
    case 'status':
      return t('instances.columns.status')
    case 'external_ref':
      return t('instances.columns.externalRef')
    case 'owner':
      return t('instances.columns.owner')
    case 'whatsapp_jid':
      return t('instances.columns.jid')
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

function selectedNames(): string[] {
  return table.value?.tableApi?.getFilteredSelectedRowModel().rows.map(row => row.original.name) ?? []
}

async function copySelectedNames() {
  try {
    await copy(selectedNames().join('\n'))
    toast.add({ title: t('instances.table.copiedNames'), icon: 'i-lucide-check', color: 'success' })
  } catch {
    toast.add({ title: t('instances.table.copyFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
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
// a shrunken result only clamps an out-of-range page. Cursor accumulation
// (loadMore/onCreated) never resets the page the user is on.
watch([globalFilter, columnFilters, sorting], () => {
  pagination.value.pageIndex = 0
})

watch(pageCount, (count) => {
  if (pagination.value.pageIndex > count - 1) {
    pagination.value.pageIndex = count - 1
  }
})

// Table copy lives in instances.table.* (en.json); no UI literal stays here.
function sortActionLabel(columnId: string): string {
  const current = sorting.value[0]
  const nextDesc = current?.id === columnId && !current.desc
  const column = columnId === 'status' ? t('instances.columns.status') : t('instances.columns.name')
  return nextDesc ? t('instances.table.sortDesc', { column }) : t('instances.table.sortAsc', { column })
}

// UTable exposes no th slot, so aria-sort stays off the header buttons; sort
// state travels visibly (primary + soft when the column drives the order) and
// in the button aria-label (sortAsc/sortDesc) instead.

// Clickable rows: the table's onSelect navigates to the detail screen, except
// when the click lands on an interactive element (links, buttons, inputs,
// checkboxes, menu items), which keeps its own behavior.
function onRowSelect(event: Event, row: { original: Instance }) {
  const target = event.target as HTMLElement | null
  if (target?.closest('a, button, input, [role="menuitem"], [role="menuitemcheckbox"]')) {
    return
  }
  void navigateTo(`/instances/${row.original.id}`)
}

useSeoMeta({
  title: 'Instances'
})

async function loadOwners() {
  if (!isAdmin.value) {
    return
  }
  try {
    const accounts = await listAccounts()
    ownerEmails.value = Object.fromEntries(accounts.map(account => [account.id, account.email]))
  } catch {
    // Owner resolution is best-effort: the list still renders with short ids.
  }
}

async function loadFirst() {
  pending.value = true
  failure.value = null
  try {
    const page = await listInstances()
    items.value = page.items
    nextCursor.value = page.next_cursor
    await loadOwners()
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.loadFailed')
  } finally {
    pending.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || nextCursor.value === '') {
    return
  }
  loadingMore.value = true
  try {
    const page = await listInstances(nextCursor.value)
    items.value = [...items.value, ...page.items]
    nextCursor.value = page.next_cursor
  } catch (error) {
    toast.add({
      title: error instanceof ApiError ? error.message : t('instances.loadFailed'),
      icon: 'i-lucide-triangle-alert',
      color: 'error'
    })
  } finally {
    loadingMore.value = false
  }
}

function onCreated(instance: CreatedInstance) {
  // The one-time key lives in the modal only: strip it before the created
  // instance joins the list so it is never retained in list memory.
  const { instance_api_key: _omit, ...rest } = instance
  items.value = [rest, ...items.value]
  toast.add({ title: t('instances.create.createdToast'), icon: 'i-lucide-check', color: 'success' })
}

// Row actions: connect starts pairing inline (the QR itself lives on the
// detail screen), edit opens the inline rename modal, remove opens the typed
// delete confirmation. All API work stays in these handlers; the actions cell
// only emits.
const connectingId = ref<string | null>(null)
const editTarget = ref<Instance | null>(null)
const editOpen = ref(false)
const deleteTarget = ref<Instance | null>(null)
const deleteOpen = ref(false)

function openDetails(instance: Instance) {
  void navigateTo(`/instances/${instance.id}`)
}

async function onConnect(instance: Instance) {
  if (connectingId.value) {
    return
  }
  connectingId.value = instance.id
  try {
    const result = await connectInstance(instance.id)
    items.value = items.value.map(entry =>
      entry.id === instance.id ? { ...entry, status: result.status } : entry
    )
    if (result.status === 'connected' || !result.qr_code) {
      toast.add({ title: t('instances.connect.alreadyConnected'), icon: 'i-lucide-check', color: 'success' })
      return
    }
    toast.add({ title: t('instances.connect.pairingStarted'), icon: 'i-lucide-check', color: 'success' })
    await navigateTo(`/instances/${instance.id}`)
  } catch (error) {
    toast.add({
      title: error instanceof ApiError ? error.message : t('instances.connect.failed'),
      icon: 'i-lucide-triangle-alert',
      color: 'error'
    })
  } finally {
    connectingId.value = null
  }
}

function openEdit(instance: Instance) {
  editTarget.value = instance
  editOpen.value = true
}

function onUpdated(updated: Instance) {
  items.value = items.value.map(entry => entry.id === updated.id ? updated : entry)
  editTarget.value = null
  toast.add({ title: t('instances.edit.saved'), icon: 'i-lucide-check', color: 'success' })
}

function openDelete(instance: Instance) {
  deleteTarget.value = instance
  deleteOpen.value = true
}

function onDeleted(id: string) {
  items.value = items.value.filter(entry => entry.id !== id)
  deleteTarget.value = null
  toast.add({ title: t('instances.detail.deleted'), icon: 'i-lucide-check', color: 'success' })
}

await loadFirst()
</script>

<template>
  <UDashboardPanel id="instances">
    <template #header>
      <UDashboardNavbar :title="t('instances.title')">
        <template #leading>
          <UDashboardSidebarCollapse />
        </template>
        <template #right>
          <UButton icon="i-lucide-plus" :label="t('instances.create.title')" @click="createOpen = true" />
        </template>
      </UDashboardNavbar>
    </template>

    <template #body>
      <p class="mb-4 text-sm text-muted">
        {{ isAdmin ? t('instances.subtitleAdmin') : t('instances.subtitleUser') }}
      </p>

      <!-- Initial load renders skeletons; the table mounts only after load, so no :loading prop (loadMore owns its button spinner). -->
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
            @click="loadFirst"
          />
        </template>
      </UAlert>

      <div v-else class="flex flex-col gap-3">
        <div class="flex flex-wrap items-center justify-between gap-1.5" role="group" :aria-label="t('instances.table.filtersLabel')">
          <UInput
            v-model="searchInput"
            icon="i-lucide-search"
            :placeholder="t('instances.table.search')"
            :aria-label="t('instances.table.search')"
            class="max-w-sm"
          />

          <div class="flex flex-wrap items-center gap-1.5">
            <UButton
              v-if="selectedCount > 0"
              color="neutral"
              variant="subtle"
              icon="i-lucide-copy"
              :label="t('instances.table.copyNames')"
              @click="copySelectedNames"
            >
              <template #trailing>
                <UKbd>
                  {{ selectedCount }}
                </UKbd>
              </template>
            </UButton>
            <USelect
              v-model="statusFilter"
              :items="statusFilterItems"
              :aria-label="t('instances.table.statusFilter')"
              :placeholder="t('instances.table.statusFilter')"
              class="min-w-28"
              :ui="{ trailingIcon: 'group-data-[state=open]:rotate-180 transition-transform duration-200' }"
            />
            <UDropdownMenu
              :items="[columnItems]"
              :content="{ align: 'end' }"
            >
              <UButton
                :label="t('instances.table.display')"
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
          :data="items"
          :columns="columns"
          :global-filter-options="globalFilterOptions"
          :pagination-options="paginationOptions"
          :get-row-id="getRowId"
          :auto-reset-all="false"
          :on-select="onRowSelect"
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
              :aria-label="t('instances.table.selectAll')"
              @update:model-value="(value: boolean | 'indeterminate') => api.toggleAllPageRowsSelected(!!value)"
            />
          </template>

          <template #select-cell="{ row }">
            <UCheckbox
              :model-value="row.getIsSelected()"
              :aria-label="t('instances.table.selectRow')"
              @update:model-value="(value: boolean | 'indeterminate') => row.toggleSelected(!!value)"
            />
          </template>

          <template #name-header="{ column }">
            <UButton
              color="neutral"
              variant="ghost"
              class="-mx-2.5"
              :label="t('instances.columns.name')"
              :icon="column.getIsSorted() ? (column.getIsSorted() === 'asc' ? 'i-lucide-arrow-up-narrow-wide' : 'i-lucide-arrow-down-wide-narrow') : 'i-lucide-arrow-up-down'"
              :aria-label="sortActionLabel('name')"
              @click="column.toggleSorting(column.getIsSorted() === 'asc')"
            />
          </template>

          <template #status-header="{ column }">
            <UButton
              color="neutral"
              variant="ghost"
              class="-mx-2.5"
              :label="t('instances.columns.status')"
              :icon="column.getIsSorted() ? (column.getIsSorted() === 'asc' ? 'i-lucide-arrow-up-narrow-wide' : 'i-lucide-arrow-down-wide-narrow') : 'i-lucide-arrow-up-down'"
              :aria-label="sortActionLabel('status')"
              @click="column.toggleSorting(column.getIsSorted() === 'asc')"
            />
          </template>

          <template #name-cell="{ row }">
            <NuxtLink
              :to="`/instances/${row.original.id}`"
              class="block min-w-0 rounded text-current no-underline hover:no-underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
              <InstancesTableNameCell :instance="row.original" />
            </NuxtLink>
          </template>

          <template #external_ref-cell="{ row }">
            <span v-if="row.original.external_ref" class="block min-w-0 truncate" :title="row.original.external_ref ?? ''">
              {{ row.original.external_ref }}
            </span>
            <span v-else class="text-sm text-muted" aria-hidden="true">—</span>
          </template>

          <template #status-cell="{ row }">
            <InstancesTableStatusCell :instance="row.original" @connect="onConnect" />
          </template>

          <template #owner-cell="{ row }">
            <InstancesTableOwnerCell :instance="row.original" :email="ownerEmails[row.original.owner_user_id ?? '']" />
          </template>

          <template #whatsapp_jid-cell="{ row }">
            <div class="min-w-0 truncate" :title="row.original.whatsapp_jid">
              <InstancesTableJidCell :instance="row.original" />
            </div>
          </template>

          <template #actions-cell="{ row }">
            <InstancesTableActionsCell
              :instance="row.original"
              @open="openDetails"
              @connect="onConnect"
              @edit="openEdit"
              @remove="openDelete"
            />
          </template>

          <template #empty>
            <UEmpty
              v-if="items.length === 0"
              icon="i-lucide-search-x"
              :title="t('instances.empty')"
            >
              <template #actions>
                <UButton icon="i-lucide-plus" :label="t('instances.create.title')" @click="createOpen = true" />
              </template>
            </UEmpty>
            <UEmpty
              v-else
              icon="i-lucide-search-x"
              :title="t('instances.table.noResults')"
            >
              <template #actions>
                <UButton
                  color="neutral"
                  variant="soft"
                  :label="t('instances.table.clearFilters')"
                  @click="clearFilters"
                />
              </template>
            </UEmpty>
          </template>
        </UTable>

        <div class="flex items-center justify-between gap-3 border-t border-default pt-4 mt-auto">
          <div class="text-sm text-muted">
            {{ t('instances.table.selectedOf', { selected: selectedCount, filtered: totalFiltered }) }}
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

        <div v-if="nextCursor !== ''" class="flex justify-center pt-2">
          <UButton
            color="neutral"
            variant="soft"
            :loading="loadingMore"
            :label="t('common.loadMore')"
            @click="loadMore"
          />
        </div>
      </div>
    </template>
  </UDashboardPanel>

  <CreateInstanceModal v-model:open="createOpen" @created="onCreated" />

  <EditInstanceModal v-model:open="editOpen" :target="editTarget" @updated="onUpdated" />

  <DeleteInstanceModal
    v-if="deleteTarget"
    v-model:open="deleteOpen"
    :instance="deleteTarget"
    @deleted="onDeleted"
  />
</template>
