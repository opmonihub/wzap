import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { computed, ref } from 'vue'
import type { Instance } from '../types/api'
import { useOverview } from './useOverview'

const instance: Instance = {
  id: 'instance-1',
  name: 'Loja',
  created_at: '2026-10-07T12:00:00Z',
  updated_at: '2026-10-07T12:00:00Z',
  connection: { status: 'disconnected' },
  integration: { webhook: { enabled: false, events: [] } }
}

const api = vi.fn()
const listInstances = vi.fn()

beforeEach(() => {
  api.mockReset()
  listInstances.mockReset()
  vi.stubGlobal('ref', ref)
  vi.stubGlobal('computed', computed)
  vi.stubGlobal('useApi', () => ({ api }))
  vi.stubGlobal('useInstances', () => ({ listInstances }))
})

afterEach(() => vi.unstubAllGlobals())

describe('overview collections', () => {
  it('shows direct instances with absent settings and preserves empty webhook events', async () => {
    api.mockResolvedValue({ stats: { total: 1, by_status: { disconnected: 1, connected: 0, pairing: 0, error: 0 } } })
    listInstances.mockResolvedValue({ instances: [instance] })
    const overview = useOverview()
    await overview.refresh()
    expect(overview.items.value).toEqual([instance])
    expect(overview.recent.value[0]?.integration.webhook).toEqual({ enabled: false, events: [] })
    expect(overview.listingFailed.value).toBe(false)
    expect(overview.stats.value?.by_status.connected).toBe(0)
  })

  it('counts the direct collection when the stats endpoint fails', async () => {
    api.mockRejectedValue(new Error('stats unavailable'))
    listInstances.mockResolvedValue({ instances: [instance] })
    const overview = useOverview()
    await overview.refresh()
    expect(overview.stats.value?.total).toBe(1)
    expect(overview.stats.value?.by_status.disconnected).toBe(1)
    expect(overview.fallback.value).toBe(true)
    expect(overview.failure.value).toBeNull()
  })

  it('accepts an empty collection without marking the listing as failed', async () => {
    api.mockRejectedValue(new Error('stats unavailable'))
    listInstances.mockResolvedValue({ instances: [] })
    const overview = useOverview()
    await overview.refresh()
    expect(overview.items.value).toEqual([])
    expect(overview.stats.value?.total).toBe(0)
    expect(overview.listingFailed.value).toBe(false)
  })
})
