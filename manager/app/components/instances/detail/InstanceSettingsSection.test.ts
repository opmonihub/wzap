import { ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../../composables/useApi'
import {
  forgetInstanceKeySeen, hasSeenInstanceKey, markInstanceKeySeen, useInstances
} from '../../../composables/useInstances'
import { useInstanceProfile } from '../../../composables/useInstanceProfile'
import type { Instance } from '../../../types/api'
import {
  flushComponent, installComponentGlobals, loadComponent, loadScript, mountComponent, primitive,
  translate, unmountComponents
} from '../../../../tests/componentHarness'

const instanceId = '94a904c9-ecbd-49c4-9839-ec4e1eec1ce2'
const instance: Instance = {
  id: instanceId, name: 'Shop', created_at: '2026-10-07T12:00:00Z', updated_at: '2026-10-07T12:00:00Z',
  connection: { status: 'connected' }, integration: { webhook: { enabled: false, events: [] } }
}
const cacheKey = `wzap.manager.keySeen.${instanceId}`
const freshSecret = 'synthetic-one-time-key'
let cache: Map<string, string>
let storageUnavailable: boolean
let revokeFailure: boolean
let requests: { path: string, method: string }[]
let notices: { title: string, color: string }[]
let confirmations: { title: string, description: string, confirmLabel: string }[]
let resolveConfirmation: (value: boolean) => void
let changed: number

beforeEach(() => {
  installComponentGlobals()
  cache = new Map()
  storageUnavailable = false
  revokeFailure = false
  requests = []
  notices = []
  confirmations = []
  changed = 0
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => {
      if (storageUnavailable) throw new Error('Storage unavailable')
      return cache.get(key) ?? null
    },
    setItem: (key: string, value: string) => {
      if (storageUnavailable) throw new Error('Storage unavailable')
      cache.set(key, value)
    },
    removeItem: (key: string) => {
      if (storageUnavailable) throw new Error('Storage unavailable')
      cache.delete(key)
    }
  })
  vi.stubGlobal('useToast', () => ({ add: (notice: typeof notices[number]) => notices.push(notice) }))
  vi.stubGlobal('useClipboard', () => ({ copy: async () => {}, copied: ref(false) }))
  vi.stubGlobal('useOverlay', () => ({
    create: () => ({
      open: (options: typeof confirmations[number]) => {
        confirmations.push(options)
        return {
          result: new Promise<boolean>((resolve) => {
            resolveConfirmation = resolve
          })
        }
      }
    })
  }))
  vi.stubGlobal('useApi', () => ({
    api: async (path: string, options: { method: string }) => {
      requests.push({ path, method: options.method })
      return { id: instanceId, instance_api_key: freshSecret }
    },
    raw: async (path: string, options: { method: string }) => {
      requests.push({ path, method: options.method })
      if (revokeFailure) throw new ApiError(500, 'internal_error', 'Revocation failed')
    }
  }))
  vi.stubGlobal('useInstances', useInstances)
  vi.stubGlobal('useInstanceProfile', useInstanceProfile)
  vi.stubGlobal('hasSeenInstanceKey', hasSeenInstanceKey)
  vi.stubGlobal('markInstanceKeySeen', markInstanceKeySeen)
  vi.stubGlobal('forgetInstanceKeySeen', forgetInstanceKeySeen)
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function settingsSection(isAdmin = true) {
  const confirmDelete = loadScript('components/instances/ConfirmDelete.ts', {
    '#components': { UButton: primitive('UButton'), UModal: primitive('UModal') }
  })
  const component = loadComponent('components/instances/detail/InstanceSettingsSection.vue', {
    '~/components/instances/ConfirmDelete': confirmDelete,
    '~/components/instances/OneTimeKeyDisplay.vue': { default: loadComponent('components/instances/OneTimeKeyDisplay.vue') }
  })
  return await mountComponent(component, {
    instance,
    isAdmin,
    onChanged: () => {
      changed += 1
    }
  })
}

async function confirmRevoke(section: Awaited<ReturnType<typeof settingsSection>>, confirmed = true) {
  expect(section.findAll('UButton').map(button => button.props.label)).toContain(translate('instances.key.revoke'))
  const revoking = section.trigger(section.button(translate('instances.key.revoke')), 'onClick')
  await flushComponent()
  expect(requests.filter(request => request.method === 'DELETE')).toHaveLength(0)
  expect(confirmations.at(-1)).toEqual({
    title: translate('instances.key.revokeConfirmTitle'), description: translate('instances.key.revokeConfirmBody'),
    confirmLabel: translate('instances.key.revoke')
  })
  resolveConfirmation(confirmed)
  await revoking
}

describe('authorized key revocation', () => {
  it.each([false, true])('lets an admin revoke without cached key, storage unavailable: %s', async (unavailable) => {
    storageUnavailable = unavailable
    const section = await settingsSection()
    await confirmRevoke(section)
    expect(requests).toEqual([{ path: `/instances/${instanceId}/apikey`, method: 'DELETE' }])
    expect(changed).toBe(1)
    expect(notices).toEqual([expect.objectContaining({ title: translate('instances.key.revoked'), color: 'success' })])
  })

  it('requires confirmation and keeps a cancelled revocation from changing key state', async () => {
    cache.set(cacheKey, '1')
    const section = await settingsSection()
    await confirmRevoke(section, false)
    expect(requests).toHaveLength(0)
    expect(cache.get(cacheKey)).toBe('1')
    expect(changed).toBe(0)
    expect(notices).toHaveLength(0)
  })

  it('lets an admin revoke the displayed fresh key and clears cache and one-time presentation on success', async () => {
    const section = await settingsSection()
    await section.trigger(section.button(translate('instances.key.generate')), 'onClick')
    expect(section.findAll('UInput').some(input => input.props.modelValue === freshSecret)).toBe(true)
    expect(cache.get(cacheKey)).toBe('1')
    await confirmRevoke(section)
    expect(requests).toEqual([
      { path: `/instances/${instanceId}/apikey/rotate`, method: 'POST' },
      { path: `/instances/${instanceId}/apikey`, method: 'DELETE' }
    ])
    expect(cache.has(cacheKey)).toBe(false)
    expect(section.findAll('UInput').some(input => input.props.modelValue === freshSecret)).toBe(false)
    expect(changed).toBe(1)
  })

  it('preserves the fresh key and cache when revocation fails', async () => {
    const section = await settingsSection()
    await section.trigger(section.button(translate('instances.key.generate')), 'onClick')
    revokeFailure = true
    await confirmRevoke(section)
    expect(cache.get(cacheKey)).toBe('1')
    expect(section.findAll('UInput').some(input => input.props.modelValue === freshSecret)).toBe(true)
    expect(changed).toBe(0)
    expect(notices.at(-1)).toEqual(expect.objectContaining({ title: 'Revocation failed', color: 'error' }))
    expect(section.button(translate('instances.key.revoke')).props.loading).toBe(false)
  })

  it('hides key generation and revocation from a regular user even when its browser has cached the key', async () => {
    cache.set(cacheKey, '1')
    const section = await settingsSection(false)
    const labels = section.findAll('UButton').map(button => button.props.label)
    expect(labels).not.toContain(translate('instances.key.generate'))
    expect(labels).not.toContain(translate('instances.key.revoke'))
    expect(requests).toHaveLength(0)
  })

  it('does not revoke after admin access is removed while confirmation is open', async () => {
    cache.set(cacheKey, '1')
    const section = await settingsSection()
    const revoking = section.trigger(section.button(translate('instances.key.revoke')), 'onClick')
    await flushComponent()
    section.props.isAdmin = false
    await flushComponent()
    resolveConfirmation(true)
    await revoking
    expect(requests).toHaveLength(0)
    expect(cache.get(cacheKey)).toBe('1')
    expect(changed).toBe(0)
  })

  it('prevents revocation during generation so the displayed key cannot be invalidated mid-operation', async () => {
    cache.set(cacheKey, '1')
    let finishGeneration!: (result: { id: string, instance_api_key: string }) => void
    vi.stubGlobal('useApi', () => ({
      api: async (path: string, options: { method: string }) => {
        requests.push({ path, method: options.method })
        return await new Promise((resolve) => {
          finishGeneration = resolve
        })
      },
      raw: async (path: string, options: { method: string }) => { requests.push({ path, method: options.method }) }
    }))
    const section = await settingsSection()
    const generating = section.trigger(section.button(translate('instances.key.generate')), 'onClick')
    await flushComponent()
    const revoke = section.button(translate('instances.key.revoke'))
    const disabled = revoke.props.disabled
    const revoking = section.trigger(revoke, 'onClick')
    await flushComponent()
    if (confirmations.length > 0) resolveConfirmation(false)
    finishGeneration({ id: instanceId, instance_api_key: freshSecret })
    await generating
    await revoking
    expect(disabled).toBe(true)
    expect(confirmations).toHaveLength(0)
    expect(requests).toEqual([{ path: `/instances/${instanceId}/apikey/rotate`, method: 'POST' }])
  })
})
