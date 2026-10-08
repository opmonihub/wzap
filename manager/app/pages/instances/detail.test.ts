import { reactive, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../composables/useApi'
import { useInstances } from '../../composables/useInstances'
import type { Instance } from '../../types/api'
import {
  flushComponent, installComponentGlobals, loadComponent, mountComponent, primitive, unmountComponents
} from '../../../tests/componentHarness'

const firstId = '94a904c9-ecbd-49c4-9839-ec4e1eec1ce2'
const otherId = 'e4915c06-9e50-4fe3-bf28-7dc54ce06ea6'
const fixture = (id: string, name: string): Instance => ({
  id, name, created_at: '2026-10-07T12:00:00Z', updated_at: '2026-10-07T12:00:00Z',
  connection: { status: 'connected' }, integration: { webhook: { enabled: false, events: [] } }
})
let first: Instance
let route: { params: { id: string }, query: Record<string, string>, hash: string }
let requests: string[]
let replacements: { path?: string, query: Record<string, string>, hash?: string }[]

beforeEach(() => {
  installComponentGlobals()
  first = fixture(firstId, 'OldName')
  route = reactive({ params: { id: 'OldName' }, query: { section: 'overview', trace: 'keep' }, hash: '#details' })
  requests = []
  replacements = []
  vi.stubGlobal('useRoute', () => route)
  vi.stubGlobal('useRouter', () => ({
    replace: async (target: typeof replacements[number]) => {
      replacements.push(target)
      if (target.path) route.params.id = target.path.slice('/instances/'.length)
      route.query = { ...target.query }
      if (target.hash !== undefined) route.hash = target.hash
    }
  }))
  vi.stubGlobal('useToast', () => ({ add: () => {} }))
  vi.stubGlobal('useAuth', () => ({ isAdmin: ref(true) }))
  vi.stubGlobal('useSeoMeta', () => {})
  vi.stubGlobal('useApi', () => ({
    api: async (path: string) => {
      requests.push(path)
      const reference = path.slice('/instances/'.length)
      if (reference === first.id || reference === first.name) return { instance: { ...first } }
      if (reference === 'OtherName' || reference === otherId) return { instance: fixture(otherId, 'OtherName') }
      throw new ApiError(404, 'not_found', 'Alias does not exist')
    }
  }))
  vi.stubGlobal('useInstances', useInstances)
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

function detailPage() {
  const imports: Record<string, unknown> = {
    '~/components/shared/PageState.vue': { default: loadComponent('components/shared/PageState.vue') }
  }
  for (const name of ['DeleteInstanceModal', 'InstanceStatusBadge']) {
    imports[`~/components/instances/${name}.vue`] = { default: primitive(name) }
  }
  for (const name of [
    'InstanceChannelsSection', 'InstanceGroupsSection', 'InstanceIntegrationsSection',
    'InstanceMessagesSection', 'InstanceOverviewSection', 'InstanceProfileSection',
    'InstanceSectionNav', 'InstanceSettingsSection'
  ]) {
    imports[`~/components/instances/detail/${name}.vue`] = { default: primitive(name) }
  }
  return mountComponent(loadComponent('pages/instances/[id].vue', imports))
}

describe('instance detail identity', () => {
  it('replaces a renamed alias with a refreshable UUID URL and retains section, other query and hash', async () => {
    const page = await detailPage()
    first = fixture(firstId, 'NewName')
    await page.trigger(page.findAll('InstanceOverviewSection')[0]!, 'onUpdated', { ...first })
    expect(route.params.id).toBe(firstId)
    expect(replacements).toContainEqual({ path: `/instances/${firstId}`, query: { section: 'overview', trace: 'keep' }, hash: '#details' })
    await page.trigger(page.findAll('InstanceOverviewSection')[0]!, 'onPaired')
    route.query.section = 'settings'
    await flushComponent()
    await page.trigger(page.findAll('InstanceSettingsSection')[0]!, 'onChanged')
    expect(requests[0]).toBe('/instances/OldName')
    expect(requests.slice(1).every(path => path === `/instances/${firstId}`)).toBe(true)
    expect(page.findAll('InstanceSettingsSection')[0]!.props.instance).toEqual(first)

    // A new page instance represents refreshing the canonical browser URL.
    const refreshed = await detailPage()
    expect(requests.at(-1)).toBe(`/instances/${firstId}`)
    expect(refreshed.findAll('InstanceSettingsSection')[0]!.props.instance).toEqual(first)
  })

  it('reloads an unchanged alias by UUID without rewriting its valid URL', async () => {
    const page = await detailPage()
    await page.trigger(page.findAll('InstanceOverviewSection')[0]!, 'onPaired')
    expect(requests).toEqual(['/instances/OldName', `/instances/${firstId}`])
    expect(replacements).toHaveLength(0)
  })

  it('loads the new route reference when navigating to another instance and then reloads its UUID', async () => {
    const page = await detailPage()
    route.params.id = 'OtherName'
    await flushComponent()
    expect(page.findAll('InstanceOverviewSection')[0]!.props.instance).toEqual(fixture(otherId, 'OtherName'))
    await page.trigger(page.findAll('InstanceOverviewSection')[0]!, 'onPaired')
    expect(requests).toEqual(['/instances/OldName', '/instances/OtherName', `/instances/${otherId}`])
  })

  it('ignores a lookup that finishes after navigation to another instance', async () => {
    let resolveFirst!: (value: { instance: Instance }) => void
    vi.stubGlobal('useApi', () => ({
      api: async (path: string) => {
        requests.push(path)
        if (path === '/instances/OldName') {
          return await new Promise((resolve) => {
            resolveFirst = resolve
          })
        }
        return { instance: fixture(otherId, 'OtherName') }
      }
    }))
    const page = await detailPage()
    route.params.id = 'OtherName'
    await flushComponent()
    resolveFirst({ instance: first })
    await flushComponent()
    expect(page.findAll('InstanceOverviewSection')[0]!.props.instance).toEqual(fixture(otherId, 'OtherName'))
  })

  it('ignores an update emitted by the previous instance after navigation', async () => {
    const page = await detailPage()
    const previousSection = page.findAll('InstanceOverviewSection')[0]!
    route.params.id = 'OtherName'
    await flushComponent()
    await page.trigger(previousSection, 'onUpdated', fixture(firstId, 'NewName'))
    expect(page.findAll('InstanceOverviewSection')[0]!.props.instance).toEqual(fixture(otherId, 'OtherName'))
    expect(route.params.id).toBe('OtherName')
  })
})
