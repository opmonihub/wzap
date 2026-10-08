import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as z from 'zod'
import { WEBHOOK_EVENT_TYPES } from '../../types/api'
import {
  installComponentGlobals, loadComponent, mountComponent, primitive, translate, unmountComponents
} from '../../../tests/componentHarness'

const instanceId = '94a904c9-ecbd-49c4-9839-ec4e1eec1ce2'
const updateInstanceWebhook = vi.fn()

beforeEach(() => {
  installComponentGlobals()
  updateInstanceWebhook.mockReset().mockResolvedValue({ id: instanceId })
  vi.stubGlobal('useToast', () => ({ add: vi.fn() }))
  vi.stubGlobal('useInstances', () => ({ updateInstanceWebhook }))
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function webhookCard() {
  const component = Object.assign(loadComponent('components/instances/WebhookCard.vue', {
    'zod': z,
    '~/types/api': { WEBHOOK_EVENT_TYPES }
  }), { components: Object.fromEntries(['UForm', 'UCheckbox', 'USwitch'].map(name => [name, primitive(name)])) })
  return await mountComponent(component, {
    instance: { id: instanceId, integration: { webhook: { url: 'https://hooks.example.com/wzap', enabled: true, events: ['message'] } } }
  })
}

type Card = Awaited<ReturnType<typeof webhookCard>>

async function save(card: Card) {
  const form = card.findAll('UForm')[0]!
  const schema = form.props.schema as z.ZodType
  await card.trigger(form, 'onSubmit', { data: schema.parse(form.props.state) })
}

async function changeSubscription(card: Card, type: string, value: boolean | 'indeterminate') {
  const checkbox = card.findAll('UCheckbox').find(entry => entry.props.label === type)!
  await card.trigger(checkbox, 'onUpdate:modelValue', value)
}

describe('webhook subscriptions', () => {
  it('saves independent event changes in contract order without selecting message', async () => {
    const card = await webhookCard()
    await card.trigger(card.button(translate('instances.webhook.selectNone')), 'onClick')
    await changeSubscription(card, 'group.info', true)
    await changeSubscription(card, 'receipt', true)
    await save(card)
    expect(updateInstanceWebhook).toHaveBeenCalledExactlyOnceWith(instanceId, {
      url: 'https://hooks.example.com/wzap', enabled: true, events: ['receipt', 'group.info']
    })
  })

  it('keeps All, removal and None payloads deterministic', async () => {
    const card = await webhookCard()
    await card.trigger(card.button(translate('instances.webhook.selectAll')), 'onClick')
    await changeSubscription(card, 'receipt', 'indeterminate')
    await save(card)
    expect(updateInstanceWebhook).toHaveBeenLastCalledWith(instanceId, {
      url: 'https://hooks.example.com/wzap', enabled: true,
      events: WEBHOOK_EVENT_TYPES.filter(type => type !== 'receipt')
    })
    await card.trigger(card.button(translate('instances.webhook.selectNone')), 'onClick')
    await card.trigger(card.findAll('USwitch')[0]!, 'onUpdate:modelValue', false)
    await card.trigger(card.findAll('UInput')[0]!, 'onUpdate:modelValue', '')
    await save(card)
    expect(updateInstanceWebhook).toHaveBeenLastCalledWith(instanceId, { url: '', enabled: false, events: [] })
  })
})
