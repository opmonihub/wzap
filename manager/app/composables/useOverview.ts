import type { Instance, InstanceListPage, InstanceStats } from '~/types/api'

// Page cap for the cursor accumulation backing recent and the local fallback
// count. The list pages at the server default (50), so 20 pages cover 1000
// instances; beyond that the overview truncates (the stats endpoint stays
// exact server-side, so truncation only affects the fallback path and the
// recent/chart inputs).
const maxOverviewPages = 20

// How many of the accumulated instances the overview surfaces as recent.
const recentLimit = 5

// Statuses the API reports, mirroring the by_status buckets accumulated in
// handleInstanceStats (internal/httpapi/instances.go). Unknown statuses fold
// into disconnected on both sides so the four buckets always sum to total,
// even against a server newer than this client.
const knownStatuses: readonly string[] = ['connected', 'disconnected', 'pairing', 'error']

function emptyStats(): InstanceStats {
  return { total: 0, by_status: { connected: 0, disconnected: 0, pairing: 0, error: 0 } }
}

// Local count over accumulated instances with the server's folding rule
// (unknown status counts as disconnected). Buckets are written through the
// nullish default so indexed access stays safe under noUncheckedIndexedAccess.
function countLocally(items: Instance[]): InstanceStats {
  const stats = emptyStats()
  for (const item of items) {
    stats.total++
    const bucket = knownStatuses.includes(item.status) ? item.status : 'disconnected'
    stats.by_status[bucket] = (stats.by_status[bucket] ?? 0) + 1
  }
  return stats
}

// Newest first by created_at, id breaking ties — the same order the list
// endpoint serves (ORDER BY created_at DESC, id DESC), applied client-side so
// recent never depends on server ordering.
function byNewestFirst(a: Instance, b: Instance): number {
  if (a.created_at !== b.created_at) {
    return a.created_at < b.created_at ? 1 : -1
  }
  if (a.id === b.id) {
    return 0
  }
  return a.id < b.id ? 1 : -1
}

// Overview data for the Home screen: scoped totals from GET /instances/stats
// (via useApi, the session cookie travels automatically) with a local count
// over the cursor-accumulated listing (listInstances from useInstances) when
// the endpoint fails. Both attempts failing surfaces failure for the
// UAlert+retry; a stats-only failure sets fallback so the page shows its
// discrete notice and still renders. A listing-only failure keeps the stats
// cards with an empty recent. refresh() never rejects.
//
// Usage in setup (mirrors await loadFirst() in pages/instances/index.vue):
// const overview = useOverview(); await overview.refresh()
export function useOverview() {
  const { api } = useApi()
  const { listInstances } = useInstances()

  const stats = ref<InstanceStats | null>(null)
  const items = ref<Instance[]>([])
  const pending = ref(true)
  const failure = ref<string | null>(null)
  const fallback = ref(false)

  // The 5 most recent instances by created_at desc of the accumulated
  // listing, empty until the listing resolves.
  const recent = computed<Instance[]>(() => [...items.value].sort(byNewestFirst).slice(0, recentLimit))

  // Accumulates list pages up to maxOverviewPages; rejects on the first page
  // error so refresh can attribute the failure. The loop is bounded and stops
  // early when the cursor empties.
  async function accumulateListing(): Promise<Instance[]> {
    const collected: Instance[] = []
    let cursor: string | undefined
    for (let page = 0; page < maxOverviewPages; page++) {
      const listing: InstanceListPage = await listInstances(cursor)
      collected.push(...listing.items)
      if (listing.next_cursor === '') {
        break
      }
      cursor = listing.next_cursor
    }
    return collected
  }

  function messageOf(error: unknown): string {
    return error instanceof Error ? error.message : String(error)
  }

  // Loads stats and listing concurrently: stats prefer the endpoint and fall
  // back to the local count only when the endpoint fails but the listing
  // succeeded; failure is set only when neither yields data (carrying the
  // stats error as the primary source, the listing error otherwise).
  async function refresh(): Promise<void> {
    pending.value = true
    failure.value = null
    fallback.value = false
    const [statsResult, listResult] = await Promise.allSettled([
      api<InstanceStats>('/instances/stats'),
      accumulateListing()
    ])
    if (listResult.status === 'fulfilled') {
      items.value = listResult.value
    } else {
      items.value = []
    }
    if (statsResult.status === 'fulfilled') {
      stats.value = statsResult.value
    } else if (listResult.status === 'fulfilled') {
      stats.value = countLocally(listResult.value)
      fallback.value = true
    } else {
      stats.value = null
      failure.value = messageOf(statsResult.reason ?? listResult.reason)
    }
    pending.value = false
  }

  return { stats, recent, pending, failure, fallback, refresh }
}
