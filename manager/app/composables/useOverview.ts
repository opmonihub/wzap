import type { Instance, InstanceStats, StatsEnvelope } from '~/types/api'

// How many instances the overview surfaces as recent.
const recentLimit = 5

// Statuses the API reports, mirroring the by_status buckets accumulated in
// handleInstanceStats (internal/httpapi/instances.go). Unknown statuses fold
// into disconnected on both sides so the four buckets always sum to total,
// even against a server newer than this client.
const knownStatuses: readonly string[] = ['connected', 'disconnected', 'pairing', 'error']

function emptyStats(): InstanceStats {
  return { total: 0, by_status: { connected: 0, disconnected: 0, pairing: 0, error: 0 } }
}

// Normalizes the endpoint payload: a zero bucket may be omitted by the
// server, so defaults spread first and the four known buckets always exist.
// Unknown bucket keys fall through the local folding rule (as disconnected);
// total is trusted as answered.
function normalizeStats(raw: InstanceStats): InstanceStats {
  const normalized = emptyStats()
  normalized.total = raw.total
  for (const [status, count] of Object.entries(raw.by_status ?? {})) {
    const bucket = knownStatuses.includes(status) ? status : 'disconnected'
    normalized.by_status[bucket] = (normalized.by_status[bucket] ?? 0) + count
  }
  return normalized
}

// Local count over all authorized instances with the server's folding rule
// (unknown status counts as disconnected). The status lives inside the
// connection block. Buckets are written through
// the nullish default so indexed access stays safe under
// noUncheckedIndexedAccess.
function countLocally(items: Instance[]): InstanceStats {
  const stats = emptyStats()
  for (const item of items) {
    stats.total++
    const bucket = knownStatuses.includes(item.connection.status) ? item.connection.status : 'disconnected'
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
// (answered under data.stats; the session cookie travels automatically) with
// a local count over the complete listing (listInstances from useInstances,
// direct elements under instances) when the endpoint fails. Both
// attempts failing surfaces failure for the UAlert+retry; a stats-only
// failure sets fallback so the page shows its discrete notice and still
// renders. A listing-only failure sets listingFailed so the Recent block
// offers its own retry while the stats cards keep rendering. refresh() never
// rejects and never clears a previous snapshot: stale stats/items survive a
// failed re-fetch.
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
  const listingFailed = ref(false)

  // The 5 most recent instances by created_at desc of the complete
  // listing, empty until the listing resolves.
  const recent = computed<Instance[]>(() => [...items.value].sort(byNewestFirst).slice(0, recentLimit))

  function messageOf(error: unknown): string {
    return error instanceof Error ? error.message : String(error)
  }

  // Loads stats and listing concurrently: stats prefer the endpoint and fall
  // back to the local count only when the endpoint fails but the listing
  // succeeded. A failed attempt never clears a previous snapshot: stale
  // stats/items stay on screen while failure (both failed) or listingFailed
  // (listing failed but stats succeeded) surfaces the error for the
  // UAlert+retry and the Recent retry.
  async function refresh(): Promise<void> {
    pending.value = true
    failure.value = null
    fallback.value = false
    listingFailed.value = false
    const [statsResult, listResult] = await Promise.allSettled([
      api<StatsEnvelope>('/instances/stats'),
      listInstances()
    ])
    const listed = listResult.status === 'fulfilled'
      ? listResult.value.instances
      : null
    if (listed !== null) {
      items.value = listed
    } else {
      listingFailed.value = true
    }
    if (statsResult.status === 'fulfilled') {
      stats.value = normalizeStats(statsResult.value.stats)
    } else if (listed !== null) {
      stats.value = countLocally(listed)
      fallback.value = true
    } else {
      failure.value = messageOf(listResult.status === 'rejected' ? listResult.reason : statsResult.reason)
    }
    pending.value = false
  }

  return { stats, recent, items, pending, failure, fallback, listingFailed, refresh }
}
