import type { TableColumn, TableRow } from '@nuxt/ui'
import type { AccountUser } from '~/types/api'

// Table state for the accounts list: column definitions plus the sorting /
// filter / visibility / selection / pagination state. The page binds every
// state with UTable v-models (sorting, global-filter, column-filters,
// column-visibility, row-selection, pagination) and passes the loaded users
// straight into :data; cell rendering lives in
// components/accounts/AccountsTable*.vue.
export function useAccountsTable() {
  const { t } = useI18n()

  const sorting = ref<{ id: string, desc: boolean }[]>([{ id: 'email', desc: false }])
  const globalFilter = ref('')
  const columnFilters = ref<{ id: string, value: unknown }[]>([])
  const columnVisibility = ref<Record<string, boolean>>({})
  const rowSelection = ref<Record<string, boolean>>({})
  const pagination = ref({ pageIndex: 0, pageSize: 10 })
  // No virtualization: pageSize slicing bounds render cost on large account lists; virtualize only if lists outgrow this.

  // Quota display rule, mirroring quotaLabel in pages/accounts/index.vue:
  // quota 0 means unlimited, any other quota renders as its number. The quota
  // cell (AccountsTableQuotaCell.vue) applies the same rule; keep both in
  // sync. Sorting never uses this display string (see numeric sortingFn on
  // the quota column below).
  function quotaLabel(user: AccountUser): string {
    return user.instance_quota === 0 ? t('accounts.unlimited') : String(user.instance_quota)
  }

  // Global search over the already-loaded users (no API call): matches the
  // email, case-insensitively. Wired as the native global filter via
  // :global-filter-options, so UTable owns matching.
  function accountsGlobalFilterFn(
    row: { original: AccountUser },
    _columnId: string,
    filterValue: unknown
  ): boolean {
    const query = String(filterValue ?? '').trim().toLowerCase()
    if (query === '') {
      return true
    }
    return row.original.email.toLowerCase().includes(query)
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
        rows: TableRow<AccountUser>[]
        flatRows: TableRow<AccountUser>[]
        rowsById: Record<string, TableRow<AccountUser>>
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

  const globalFilterOptions = computed(() => ({ globalFilterFn: accountsGlobalFilterFn }))
  const paginationOptions = computed(() => ({ getPaginationRowModel: clientPaginationRowModel() }))

  const columns = computed<TableColumn<AccountUser>[]>(() => {
    // Built inside the computed so headers follow runtime locale switches.
    // pnpm keeps @tanstack/* transitive-only (unresolvable from app code), so
    // ColumnMeta cannot be augmented here; the role filter variant travels as
    // untyped meta for the page to read.
    const roleColumn: TableColumn<AccountUser> = {
      id: 'role',
      accessorKey: 'role',
      header: t('common.role'),
      enableSorting: true,
      enableHiding: true,
      filterFn: 'equalsString'
    }
    roleColumn.meta = { filterVariant: 'select' } as unknown as TableColumn<AccountUser>['meta']
    const quotaColumn: TableColumn<AccountUser> = {
      id: 'instance_quota',
      accessorFn: (row: AccountUser) => quotaLabel(row),
      header: t('accounts.quotaLabel'),
      enableSorting: true,
      enableHiding: true,
      // Numeric ordering over instance_quota (0 = Unlimited sorts as 0),
      // never over the display string from the accessorFn above.
      sortingFn: (rowA: TableRow<AccountUser>, rowB: TableRow<AccountUser>) =>
        rowA.original.instance_quota - rowB.original.instance_quota
    }
    return [
      // Selection checkboxes render via the page's #select-header/#select-cell
      // slots; the column itself only opts out of sorting and hiding.
      {
        id: 'select',
        enableSorting: false,
        enableHiding: false
      },
      {
        id: 'email',
        accessorKey: 'email',
        header: t('common.email'),
        enableSorting: true,
        enableHiding: true
      },
      roleColumn,
      quotaColumn,
      // Display column: no accessor, never sorts, never hides (the page keeps
      // openQuota / openDelete; AccountsTableActionsCell only emits).
      {
        id: 'actions',
        header: '',
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
