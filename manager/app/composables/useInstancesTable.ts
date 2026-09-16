import type { TableColumn, TableRow } from '@nuxt/ui'
import type { ComputedRef, Ref } from 'vue'
import type { Instance } from '~/types/api'

// Table state for the instances list: column definitions plus the TanStack
// sorting / filter / visibility / selection / pagination state. The page binds
// every state with UTable v-models (sorting, global-filter, column-filters,
// column-visibility, row-selection, pagination) and passes the loaded items
// straight into :data; cell rendering lives in
// components/instances/InstancesTable*.vue.
export function useInstancesTable(
  _items: Ref<Instance[]> | ComputedRef<Instance[]>,
  ownerEmails: Ref<Record<string, string>> | ComputedRef<Record<string, string>>,
  _isAdmin: Ref<boolean> | ComputedRef<boolean>
) {
  const { t } = useI18n()

  // The loaded items travel straight into UTable as :data (client-side
  // sorting/filtering/pagination act on the accumulated cursor pages).
  // Admin gating lives in the page (owner column meta + viewport merge), so
  // the items and role travel unused here beyond keeping the shared
  // signature.

  const sorting = ref<{ id: string, desc: boolean }[]>([{ id: 'name', desc: false }])
  const globalFilter = ref('')
  const columnFilters = ref<{ id: string, value: unknown }[]>([])
  // User visibility overrides from the columns dropdown; the page merges them
  // over the viewport defaults (owner/jid collapse on small screens).
  const columnVisibility = ref<Record<string, boolean>>({})
  const rowSelection = ref<Record<string, boolean>>({})
  const pagination = ref({ pageIndex: 0, pageSize: 10 })
  // No virtualization: cursor accumulation plus pageSize slicing bounds render cost; virtualize only if lists outgrow this.

  // Resolves the owner column value, mirroring the card list it replaces
  // (ownerLabel in pages/instances/index.vue): the account email when known,
  // the short id as best-effort fallback, Not set when ownerless.
  function ownerLabel(instance: Instance): string {
    if (!instance.owner_user_id) {
      return t('common.notSet')
    }
    return ownerEmails.value[instance.owner_user_id] ?? instance.owner_user_id.slice(0, 8)
  }

  // Global search over the already-loaded items (no API call): matches the
  // instance name or the external reference, case-insensitively. Wired as the
  // native global filter via :global-filter-options, so UTable owns matching.
  function instancesGlobalFilterFn(
    row: { original: Instance },
    _columnId: string,
    filterValue: unknown
  ): boolean {
    const query = String(filterValue ?? '').trim().toLowerCase()
    if (query === '') {
      return true
    }
    return [row.original.name, row.original.external_ref].some(value =>
      (value ?? '').toLowerCase().includes(query)
    )
  }

  // Client-side pagination for UTable: Nuxt UI wires TanStack's core, filtered
  // and sorted models but no pagination model, and @tanstack/* stays
  // transitive-only (pnpm strict, no new dependency). This local model slices
  // the pre-pagination (filtered+sorted) rows exactly like TanStack's own
  // getPaginationRowModel, so :data keeps the FULL list and sorting, filtering
  // and pagination all stay native inside the table (no page-level engine).
  function clientPaginationRowModel() {
    return (table: {
      getState: () => { pagination: { pageIndex: number, pageSize: number } }
      getPrePaginationRowModel: () => {
        rows: TableRow<Instance>[]
        flatRows: TableRow<Instance>[]
        rowsById: Record<string, TableRow<Instance>>
      }
    }) => () => {
      const { pageIndex, pageSize } = table.getState().pagination
      const pre = table.getPrePaginationRowModel()
      const start = pageIndex * pageSize
      const rows = pre.rows.slice(start, start + pageSize)
      const pageIds = new Set(rows.map(row => row.id))
      return {
        rows,
        flatRows: pre.flatRows.filter(row => pageIds.has(row.id)),
        rowsById: Object.fromEntries(rows.map(row => [row.id, row]))
      }
    }
  }

  const globalFilterOptions = computed(() => ({ globalFilterFn: instancesGlobalFilterFn }))
  const paginationOptions = computed(() => ({ getPaginationRowModel: clientPaginationRowModel() }))

  const columns = computed<TableColumn<Instance>[]>(() => {
    // Built inside the computed so headers follow runtime locale switches.
    // pnpm keeps @tanstack/* transitive-only (unresolvable from app code), so
    // ColumnMeta cannot be augmented here; the admin-only marker and the
    // status filter variant travel as untyped meta for the page to read.
    const ownerColumn: TableColumn<Instance> = {
      id: 'owner',
      accessorFn: (row: Instance) => ownerLabel(row),
      header: t('instances.columns.owner'),
      enableSorting: false,
      enableHiding: true
    }
    ownerColumn.meta = { ifAdmin: true } as unknown as TableColumn<Instance>['meta']
    const statusColumn: TableColumn<Instance> = {
      id: 'status',
      accessorKey: 'status',
      header: t('instances.columns.status'),
      enableSorting: true,
      enableHiding: false,
      filterFn: 'equalsString'
    }
    statusColumn.meta = { filterVariant: 'select' } as unknown as TableColumn<Instance>['meta']
    return [
      // Selection checkboxes render via the page's #select-header/#select-cell
      // slots; the column itself only opts out of sorting and hiding.
      {
        id: 'select',
        enableSorting: false,
        enableHiding: false
      },
      {
        id: 'name',
        accessorKey: 'name',
        header: t('instances.columns.name'),
        enableSorting: true,
        enableHiding: false
      },
      statusColumn,
      {
        id: 'external_ref',
        accessorKey: 'external_ref',
        header: t('instances.columns.externalRef'),
        enableSorting: false,
        enableHiding: true
      },
      ownerColumn,
      {
        id: 'whatsapp_jid',
        accessorKey: 'whatsapp_jid',
        header: t('instances.columns.jid'),
        enableSorting: false,
        enableHiding: true
      }
    ]
  })

  return {
    columns,
    sorting,
    globalFilter,
    columnFilters,
    columnVisibility,
    rowSelection,
    pagination,
    globalFilterOptions,
    paginationOptions
  }
}
