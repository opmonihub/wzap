import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useInstanceChannels } from '../../composables/useInstanceChannels'
import type { OwnStatus } from '../../types/api'
import {
  installComponentGlobals, loadComponent, mountComponent, translate, unmountComponents
} from '../../../tests/componentHarness'

const instanceId = '94a904c9-ecbd-49c4-9839-ec4e1eec1ce2'
let statuses: OwnStatus[]
let requests: { path: string, method?: string }[]

beforeEach(() => {
  installComponentGlobals()
  statuses = []
  requests = []
  vi.stubGlobal('useApi', () => ({
    api: async (path: string, options?: { method?: string }) => {
      requests.push({ path, method: options?.method })
      if (options?.method === 'DELETE') {
        const id = decodeURIComponent(path.split('/').at(-1)!)
        statuses = statuses.filter(status => status.id !== id)
        return { deleted: true }
      }
      if (path.endsWith('/newsletters')) return { channels: [] }
      return { statuses: statuses.map(status => ({ ...status })) }
    }
  }))
  vi.stubGlobal('useInstanceChannels', useInstanceChannels)
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function channelsCard() {
  return await mountComponent(loadComponent('components/instances/ChannelsCard.vue'), { instanceId, status: 'connected' })
}

describe('own status rows', () => {
  it.each(['image', 'video'])('keeps a %s status without caption identifiable and deletable', async (type) => {
    statuses = [{ id: 'media-status-1', type, created_at: '2026-10-07T12:00:00Z' }]
    const card = await channelsCard()
    const rows = card.findAll('li')
    expect(rows).toHaveLength(1)
    expect(card.text(rows[0])).toContain('media-status-1')
    expect(card.text(rows[0])).toContain(type)
    expect(card.findAll('UEmpty')).toHaveLength(0)
    await card.trigger(card.button(translate('instances.channels.delete')), 'onClick')
    expect(requests.filter(request => request.method === 'DELETE')).toEqual([
      { path: `/instances/${instanceId}/status/updates/media-status-1`, method: 'DELETE' }
    ])
    expect(statuses).toEqual([])
    expect(card.findAll('li')).toHaveLength(0)
    expect(card.findAll('UEmpty')).toHaveLength(1)
  })

  it('shows text and captions alongside status identity and separate delete actions', async () => {
    statuses = [
      { id: 'text-status-1', type: 'text', text: 'A written status', created_at: '2026-10-07T12:00:00Z' },
      { id: 'caption-status-1', type: 'image', caption: 'A caption', created_at: '2026-10-07T12:00:00Z' }
    ]
    const card = await channelsCard()
    const rows = card.findAll('li')
    expect(rows).toHaveLength(2)
    expect(card.text(rows[0])).toContain('A written status')
    expect(card.text(rows[0])).toContain('text-status-1')
    expect(card.text(rows[1])).toContain('A caption')
    expect(card.text(rows[1])).toContain('caption-status-1')
    expect(card.findAll('UButton').filter(button => button.props.label === translate('instances.channels.delete'))).toHaveLength(2)
    expect(card.findAll('UEmpty')).toHaveLength(0)
  })

  it('shows the empty state only when the returned collections are empty', async () => {
    const card = await channelsCard()
    expect(card.findAll('li')).toHaveLength(0)
    expect(card.findAll('UEmpty').map(empty => empty.props.title)).toEqual([translate('instances.channels.empty')])
  })
})
