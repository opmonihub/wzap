<script setup lang="ts">
import type { DropdownMenuItem, TableRow } from '@nuxt/ui'
import { useInstancesTable } from '~/composables/useInstancesTable'
import InstancesTableActionsCell from '~/components/instances/InstancesTableActionsCell.vue'
import InstancesTableNameCell from '~/components/instances/InstancesTableNameCell.vue'
import InstancesTableStatusCell from '~/components/instances/InstancesTableStatusCell.vue'
import DataTableFooter from '~/components/shared/DataTableFooter.vue'
import DataTableToolbar from '~/components/shared/DataTableToolbar.vue'
import type { Instance, InstanceStatus } from '~/types/api'

const props = defineProps<{
  items: Instance[]
  isAdmin: boolean
}>()

const emit = defineEmits<{
  connect: [instance: Instance]
  edit: [instance: Instance]
  remove: [instance: Instance]
  // Empty-state create button (the navbar create button stays in the page, so
  // this is the only create entry owned here).
  create: []
}>()

// Structural view of the UTable API this table drives (stable component
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
const { copy } = useClipboard()

const itemsRef = computed(() => props.items)
const isAdminRef = computed(() => props.isAdmin)

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
} = useInstancesTable(itemsRef, isAdminRef)

const table = useTemplateRef<{ tableApi?: InstancesTableApi }>('table')

// The remodeled public instance hides owner_user_id, external_ref and
// whatsapp_jid, so only name/status/actions remain: the column-visibility
// merge stays for the dropdown even though every column is always visible.
const columnVisibility = computed<Record<string, boolean>>({
  get: () => ({ ...visibilityOverrides.value }),
  set: (next) => {
    visibilityOverrides.value = { ...next }
  }
})

function getRowId(row: Instance): string {
  return row.id
}

// Native table state, read from the table API (UTable owns sorting, filtering
// and pagination; the table only binds state and renders). Before the first
// mount tableApi is null, so every read falls back to the loaded items.
function filteredCount(): number {
  return table.value?.tableApi?.getFilteredRowModel().rows.length ?? props.items.length
}

// UPagination :total subscribes to the v-model state because tableApi reads
// are not reactive: sorting/filter/pagination/items changes recompute it.
const totalFiltered = computed(() => {
  void sorting.value
  void globalFilter.value
  void columnFilters.value
  void pagination.value
  void props.items
  return filteredCount()
})
const pageCount = computed(() => Math.max(1, Math.ceil(totalFiltered.value / pagination.value.pageSize)))

// Status column filter behind a USelect. Reka SelectItem forbids an empty
// value string (it throws and unmounts the page), so the "all" option uses
// the 'all' sentinel, mapped back to "no column filter" in the setter.
const statusFilter = computed<string>({
  get: () => {
    const current = columnFilters.value.find(entry => entry.id === 'status')?.value
    return typeof current === 'string' && current !== '' ? current : 'all'
  },
  set: (value) => {
    const rest = columnFilters.value.filter(entry => entry.id !== 'status')
    columnFilters.value = value === '' || value === 'all' ? rest : [...rest, { id: 'status', value }]
  }
})

const instanceStatuses: InstanceStatus[] = ['disconnected', 'pairing', 'connected', 'error']

const statusFilterItems = computed(() => [
  { label: t('instances.table.allStatuses'), value: 'all' },
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

// The toolbar owns the debounced search input (v-model:filter writes straight
// into globalFilter and syncs back on external resets), so clearing the
// search is a single globalFilter reset; the status filter stays immediate.
function clearFilters() {
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
// a shrunken result only clamps an out-of-range page. Adding a created
// instance never resets the page the user is on.
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

// Row navigation stays inside the table (no open emit): row click and the
// name link go straight to the detail screen, except when the click lands on
// an interactive element (links, buttons, inputs, checkboxes, menu items),
// which keeps its own behavior.
function openDetails(instance: Instance) {
  void navigateTo(`/instances/${instance.id}`)
}

function onRowSelect(event: Event, row: { original: Instance }) {
  const target = event.target as HTMLElement | null
  if (target?.closest('a, button, input, [role="menuitem"], [role="menuitemcheckbox"]')) {
    return
  }
  openDetails(row.original)
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <DataTableToolbar
      v-model:filter="globalFilter"
      :search-placeholder="t('instances.table.search')"
      :search-aria="t('instances.table.search')"
      :filters-label="t('instances.table.filtersLabel')"
      :display-label="t('instances.table.display')"
      :column-items="columnItems"
    >
      <template #bulk>
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
      </template>

      <template #filters>
        <USelect
          v-model="statusFilter"
          :items="statusFilterItems"
          :aria-label="t('instances.table.statusFilter')"
          :placeholder="t('instances.table.statusFilter')"
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

      <template #status-cell="{ row }">
        <InstancesTableStatusCell :instance="row.original" @connect="(instance: Instance) => emit('connect', instance)" />
      </template>

      <template #actions-cell="{ row }">
        <InstancesTableActionsCell
          :instance="row.original"
          @open="(instance: Instance) => openDetails(instance)"
          @connect="(instance: Instance) => emit('connect', instance)"
          @edit="(instance: Instance) => emit('edit', instance)"
          @remove="(instance: Instance) => emit('remove', instance)"
        />
      </template>

      <template #empty>
        <UEmpty
          v-if="items.length === 0"
          icon="i-lucide-search-x"
          :title="t('instances.empty')"
        >
          <template #actions>
            <UButton icon="i-lucide-plus" :label="t('instances.create.title')" @click="emit('create')" />
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

    <DataTableFooter
      :status-text="t('instances.table.selectedOf', { selected: selectedCount, filtered: totalFiltered })"
      :page="pagination.pageIndex + 1"
      :page-size="pagination.pageSize"
      :total="totalFiltered"
      :page-count="pageCount"
      @update:page="onUpdatePage"
    />
  </div>
</template>
