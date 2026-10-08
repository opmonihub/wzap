import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useInstanceMessaging } from '../../composables/useInstanceMessaging'
import { useMessages } from '../../composables/useMessages'
import {
  flushComponent, installComponentGlobals, loadComponent, mountComponent, translate, unmountComponents
} from '../../../tests/componentHarness'

const instanceId = '94a904c9-ecbd-49c4-9839-ec4e1eec1ce2'
let requests: { path: string, method?: string, body?: unknown, headers?: Record<string, string> }[]
let sent: string[]
let settled: string[]

beforeEach(() => {
  installComponentGlobals()
  vi.useFakeTimers()
  requests = []
  sent = []
  settled = []
  vi.stubGlobal('useToast', () => ({ add: () => {} }))
  vi.stubGlobal('useApi', () => ({
    api: async (path: string, options?: Omit<typeof requests[number], 'path'>) => {
      requests.push({ path, ...options })
      if (options?.method === 'POST') {
        return { message: { id: 'message-1', instance_id: instanceId, send_status: 'queued' } }
      }
      return {
        message: {
          id: 'message-1', instance_id: instanceId, message_type: 'location', recipient_jid: 'recipient',
          send_status: 'sent', retry_count: 0, created_at: '2026-10-07T12:00:00Z', updated_at: '2026-10-07T12:00:00Z'
        }
      }
    }
  }))
  vi.stubGlobal('useInstanceMessaging', useInstanceMessaging)
  vi.stubGlobal('useMessages', useMessages)
})

afterEach(() => {
  unmountComponents()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

async function sendCoordinates(latitude: string, longitude: string) {
  const composer = await mountComponent(loadComponent('components/instances/MessageComposer.vue'), {
    instanceId, status: 'connected', onSent: (id: string) => sent.push(id), onSettled: (id: string) => settled.push(id)
  })
  await composer.trigger(composer.findAll('UInput')[0]!, 'onUpdate:modelValue', '5511999999999')
  await composer.trigger(composer.findAll('UTabs')[0]!, 'onUpdate:modelValue', 'location')
  await composer.trigger(composer.findAll('UInput')[1]!, 'onUpdate:modelValue', latitude)
  await composer.trigger(composer.findAll('UInput')[2]!, 'onUpdate:modelValue', longitude)
  const sending = composer.trigger(composer.button(translate('instances.send.send')), 'onClick')
  await flushComponent()
  await vi.advanceTimersByTimeAsync(2000)
  await sending
  return composer
}

describe('location composition', () => {
  it.each([
    ['', ''], ['  ', '\t'], ['', '0'], ['0', ''], [' ', '12'], ['12', '  ']
  ])('rejects absent coordinates %j, %j before numeric conversion or sending', async (latitude, longitude) => {
    const composer = await sendCoordinates(latitude, longitude)
    expect(requests).toHaveLength(0)
    expect(sent).toHaveLength(0)
    expect(settled).toHaveLength(0)
    expect(composer.findAll('UAlert').map(alert => alert.props.title)).toEqual([translate('instances.send.invalidLocation')])
  })

  it.each([
    ['0', '0', 0, 0], [' 0 ', ' 0 ', 0, 0], ['-90', '180', -90, 180], ['90', '-180', 90, -180]
  ])('sends explicit valid coordinates %j, %j', async (latitude, longitude, expectedLatitude, expectedLongitude) => {
    const composer = await sendCoordinates(latitude, longitude)
    const sends = requests.filter(request => request.method === 'POST')
    expect(sends).toHaveLength(1)
    expect(sends[0]).toEqual(expect.objectContaining({
      path: `/instances/${instanceId}/messages/location`, method: 'POST',
      body: { to: '5511999999999', latitude: expectedLatitude, longitude: expectedLongitude }
    }))
    expect(sends[0]!.headers?.['Idempotency-Key']).toBeTruthy()
    expect(sent).toEqual(['message-1'])
    expect(settled).toEqual(['message-1'])
    expect(composer.findAll('UAlert')).toHaveLength(0)
  })

  it.each([
    ['-91', '0'], ['91', '0'], ['0', '-181'], ['0', '181'], ['NaN', '0'], ['0', 'Infinity']
  ])('rejects non-finite or out-of-bounds coordinates %j, %j', async (latitude, longitude) => {
    const composer = await sendCoordinates(latitude, longitude)
    expect(requests).toHaveLength(0)
    expect(composer.findAll('UAlert').map(alert => alert.props.title)).toEqual([translate('instances.send.invalidLocation')])
  })
})
