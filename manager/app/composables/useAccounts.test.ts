import { afterEach, describe, expect, it, vi } from 'vitest'
import { useAccounts } from './useAccounts'

afterEach(() => vi.unstubAllGlobals())

describe('account collections', () => {
  it('loads direct users while keeping mandatory zero counters', async () => {
    const user = { id: 'user-1', email: 'user@example.com', role: 'user', instance_limit: 0, instances_used: 0 }
    const api = vi.fn().mockResolvedValue({ users: [user] })
    vi.stubGlobal('useApi', () => ({ api }))
    expect(await useAccounts().listUsers()).toEqual([user])
    expect(api).toHaveBeenCalledWith('/users')
  })

  it('accepts an empty users collection', async () => {
    vi.stubGlobal('useApi', () => ({ api: vi.fn().mockResolvedValue({ users: [] }) }))
    expect(await useAccounts().listUsers()).toEqual([])
  })
})
