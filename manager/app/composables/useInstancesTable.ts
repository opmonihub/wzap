import { getPaginationRowModel } from '@tanstack/table-core'
import type { TableColumn } from '@nuxt/ui'
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
  _isAdmin: Ref<boolean> | ComputedRef<boolean>
) {
  const { t } = useI18n()

  // The loaded items travel straight into UTable as :data (client-side
  // sorting/filtering/pagination act on the accumulated cursor pages). The
  // remodeled public DTO hides owner_user_id, external_ref and whatsapp_jid,
  // so the owner/JID/external-ref columns are gone: the table renders name,
  // the nested connection status and actions.

  // Default sorting is empty, mirroring the template customers table: the
  // list renders in server order until the account sorts a header.
  const sorting = ref<{ id: string, desc: boolean }[]>([])
  const globalFilter = ref('')
  const columnFilters = ref<{ id: string, value: unknown }[]>([])
  // User visibility overrides from the columns dropdown; the page merges them
  // over the viewport defaults.
  const columnVisibility = ref<Record<string, boolean>>({})
  const rowSelection = ref<Record<string, boolean>>({})
  const pagination = ref({ pageIndex: 0, pageSize: 10 })
  // No virtualization: cursor accumulation plus pageSize slicing bounds render cost; virtualize only if lists outgrow this.

  // Global search over the already-loaded items (no API call): matches the
  // instance name, case-insensitively (external_ref is no longer public).
  // Wired as the native global filter via :global-filter-options, so UTable
  // owns matching.
  function instancesGlobalFilterFn(
    row: { original: Instance },
    _columnId: string,
    filterValue: unknown
  ): boolean {
    const query = String(filterValue ?? '').trim().toLowerCase()
    if (query === '') {
      return true
    }
    return row.original.name.toLowerCase().includes(query)
  }

  const globalFilterOptions = computed(() => ({ globalFilterFn: instancesGlobalFilterFn }))
  const paginationOptions = computed(() => ({ getPaginationRowModel: getPaginationRowModel() }))

  const columns = computed<TableColumn<Instance>[]>(() => {
    // Built inside the computed so headers follow runtime locale switches.
    // The status filter variant travels as untyped meta for the page to read.
    const statusColumn: TableColumn<Instance> = {
      id: 'status',
      accessorFn: (row: Instance) => row.connection.status,
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
      // Display column: no accessor, never sorts, never hides (the page keeps
      // the connect/edit/delete flows; InstancesTableActionsCell only emits).
      {
        id: 'actions',
        header: t('instances.table.actions'),
        enableSorting: false,
        enableHiding: false
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
