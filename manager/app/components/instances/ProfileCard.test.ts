import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../../composables/useApi'
import { useInstanceProfile } from '../../composables/useInstanceProfile'
import {
  installComponentGlobals, loadComponent, mountComponent, translate, unmountComponents
} from '../../../tests/componentHarness'

const instanceId = '94a904c9-ecbd-49c4-9839-ec4e1eec1ce2'
let stored: { name: string, status_text: string }
let requests: { path: string, method?: string, body?: unknown }[]
let notices: { title: string, color: string }[]

beforeEach(() => {
  installComponentGlobals()
  stored = { name: 'Shop', status_text: 'Open' }
  requests = []
  notices = []
  vi.stubGlobal('useToast', () => ({ add: (notice: typeof notices[number]) => notices.push(notice) }))
  vi.stubGlobal('useApi', () => ({
    api: async (path: string, options?: { method: string, body?: unknown }) => {
      requests.push({ path, ...options })
      if (options?.method === 'PUT') throw new ApiError(501, 'not_supported', 'Photo is unsupported')
      if (options?.method === 'PATCH') {
        const body = options.body as { name?: string, status_text?: string }
        // The real contract is atomic: name makes the whole request unsupported.
        if (body.name !== undefined) throw new ApiError(501, 'not_supported', 'Name is unsupported')
        stored = { ...stored, ...body }
      }
      return { ...stored }
    }
  }))
  vi.stubGlobal('useInstanceProfile', useInstanceProfile)
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function profileCard() {
  return await mountComponent(loadComponent('components/instances/ProfileCard.vue'), { instanceId, status: 'connected' })
}

describe('profile edits', () => {
  it.each(['Closed', ''])('saves only the changed recado %j without requesting unsupported name', async (recado) => {
    const card = await profileCard()
    await card.trigger(card.findAll('UInput')[1]!, 'onUpdate:modelValue', recado)
    await card.trigger(card.button(translate('common.save')), 'onClick')
    expect(requests.filter(request => request.method === 'PATCH')).toEqual([
      { path: `/instances/${instanceId}/profile`, method: 'PATCH', body: { status_text: recado } }
    ])
    expect(stored).toEqual({ name: 'Shop', status_text: recado })
    expect(notices).toEqual([expect.objectContaining({ title: translate('instances.profile.saved'), color: 'success' })])
    expect(card.findAll('UAlert')).toHaveLength(0)
  })

  it('allows recado-only edits when the loaded name is empty', async () => {
    stored.name = ''
    const card = await profileCard()
    await card.trigger(card.findAll('UInput')[1]!, 'onUpdate:modelValue', 'Available')
    await card.trigger(card.button(translate('common.save')), 'onClick')
    expect(stored.status_text).toBe('Available')
    expect(card.findAll('UAlert')).toHaveLength(0)
  })

  it('does not submit unchanged fields or show a success notice', async () => {
    const card = await profileCard()
    await card.trigger(card.button(translate('common.save')), 'onClick')
    expect(requests.filter(request => request.method === 'PATCH')).toHaveLength(0)
    expect(notices).toHaveLength(0)
  })

  it('validates only changed fields when the loaded recado exceeds the editing limit', async () => {
    stored.status_text = 'x'.repeat(501)
    const card = await profileCard()
    await card.trigger(card.findAll('UInput')[0]!, 'onUpdate:modelValue', 'New shop')
    await card.trigger(card.button(translate('common.save')), 'onClick')
    expect(requests.filter(request => request.method === 'PATCH')).toEqual([
      { path: `/instances/${instanceId}/profile`, method: 'PATCH', body: { name: 'New shop' } }
    ])
    expect(card.findAll('UAlert').map(alert => alert.props.title)).toEqual([translate('instances.profile.unsupported')])
  })

  it.each([false, true])('keeps a real name edit unsupported and atomic, with recado changed: %s', async (changeRecado) => {
    const card = await profileCard()
    await card.trigger(card.findAll('UInput')[0]!, 'onUpdate:modelValue', 'New shop')
    if (changeRecado) await card.trigger(card.findAll('UInput')[1]!, 'onUpdate:modelValue', 'Closed')
    await card.trigger(card.button(translate('common.save')), 'onClick')
    expect(requests.filter(request => request.method === 'PATCH')).toEqual([
      { path: `/instances/${instanceId}/profile`, method: 'PATCH', body: changeRecado ? { name: 'New shop', status_text: 'Closed' } : { name: 'New shop' } }
    ])
    expect(stored).toEqual({ name: 'Shop', status_text: 'Open' })
    expect(notices).toHaveLength(0)
    expect(card.findAll('UAlert').map(alert => alert.props.title)).toEqual([translate('instances.profile.unsupported')])
    expect(card.findAll('UInput')[0]!.props.modelValue).toBe('New shop')
  })

  it('keeps unsupported photo edits visible without reporting success', async () => {
    const card = await profileCard()
    await card.trigger(card.findAll('UFileUpload')[0]!, 'onUpdate:modelValue', new File(['image'], 'photo.png', { type: 'image/png' }))
    await card.trigger(card.button(translate('instances.profile.uploadPhoto')), 'onClick')
    expect(requests.filter(request => request.method === 'PUT')).toHaveLength(1)
    expect(notices).toHaveLength(0)
    expect(card.findAll('UAlert').map(alert => alert.props.title)).toEqual([translate('instances.profile.unsupported')])
  })
})
