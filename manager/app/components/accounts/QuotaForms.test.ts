import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as z from 'zod'
import { ApiError } from '../../composables/useApi'
import { useAccounts } from '../../composables/useAccounts'
import type { AccountUser } from '../../types/api'
import { parseQuota } from '../../utils/accountQuota'
import {
  installComponentGlobals, loadComponent, mountComponent, primitive, translate, unmountComponents
} from '../../../tests/componentHarness'

const accountId = '67a5b146-74e8-4fa5-b77d-dcd92fba7012'
let stored: AccountUser
let requests: { path: string, method?: string, body?: unknown }[]
let notices: { title: string, color: string }[]
let serverError: Error | null

beforeEach(() => {
  installComponentGlobals()
  stored = {
    id: accountId,
    email: 'quota@example.com',
    role: 'user',
    instance_limit: 4,
    instances_used: 0,
    created_at: '2026-10-07T12:00:00Z',
    updated_at: '2026-10-07T12:00:00Z'
  }
  requests = []
  notices = []
  serverError = null
  vi.stubGlobal('useToast', () => ({ add: (notice: typeof notices[number]) => notices.push(notice) }))
  vi.stubGlobal('useApi', () => ({
    api: async (path: string, options?: { method: string, body?: unknown }) => {
      requests.push({ path, ...options })
      if (serverError) throw serverError
      const body = options?.body as { email?: string, instance_limit?: number }
      stored = { ...stored, email: body.email ?? stored.email, instance_limit: body.instance_limit ?? stored.instance_limit }
      return { user: { ...stored } }
    }
  }))
  vi.stubGlobal('useAccounts', useAccounts)
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function quotaForm(name: 'CreateAccountModal' | 'EditQuotaModal') {
  const component = Object.assign(loadComponent(`components/accounts/${name}.vue`, {
    'zod': z,
    '~/utils/accountQuota': { parseQuota }
  }), { components: { UForm: primitive('UForm') } })
  return await mountComponent(component, { target: stored, open: true })
}

type MountedForm = Awaited<ReturnType<typeof quotaForm>>

async function submit(form: MountedForm) {
  const target = form.findAll('UForm')[0]!
  const schema = target.props.schema as z.ZodType<Record<string, unknown>>
  const result = schema.safeParse(target.props.state)
  if (result.success) await form.trigger(target, 'onSubmit', { data: result.data })
  return result
}

async function fillCreation(form: MountedForm, quota: string | number | undefined) {
  const inputs = form.findAll('UInput')
  await form.trigger(inputs[0]!, 'onUpdate:modelValue', ' quota@example.com ')
  await form.trigger(inputs[1]!, 'onUpdate:modelValue', 'test-password')
  await form.trigger(inputs[2]!, 'onUpdate:modelValue', quota)
}

describe('account creation quota', () => {
  it.each([undefined, '', '   '])('leaves a blank quota %j out of the API body', async (quota) => {
    const form = await quotaForm('CreateAccountModal')
    await fillCreation(form, quota)
    const result = await submit(form)
    expect(result.success).toBe(true)
    expect(requests).toEqual([
      { path: '/users', method: 'POST', body: { email: 'quota@example.com', password: 'test-password', role: 'user' } }
    ])
    expect(stored.instance_limit).toBe(4)
  })

  it.each([[0, 0], ['0', 0], [2, 2], ['2', 2]])('sends quota %j as the number %j', async (quota, expected) => {
    const form = await quotaForm('CreateAccountModal')
    await fillCreation(form, quota)
    const result = await submit(form)
    expect(result.success).toBe(true)
    expect(requests).toEqual([
      { path: '/users', method: 'POST', body: { email: 'quota@example.com', password: 'test-password', role: 'user', instance_limit: expected } }
    ])
    expect(stored.instance_limit).toBe(expected)
    expect(notices).toEqual([expect.objectContaining({ title: translate('accounts.create.createdToast'), color: 'success' })])
  })

  it.each([-1, '-1', 1.5, '1.5', NaN, Infinity, 'Infinity', 'quota'])('rejects quota %j with the account guidance before sending', async (quota) => {
    const form = await quotaForm('CreateAccountModal')
    await fillCreation(form, quota)
    const result = await submit(form)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map(issue => ({ path: issue.path, message: issue.message }))).toEqual([
        { path: ['instance_limit'], message: translate('accounts.create.quotaInvalid') }
      ])
    }
    expect(requests).toHaveLength(0)
    expect(notices).toHaveLength(0)
  })

  it('keeps the entered fields when the server rejects a duplicate email', async () => {
    serverError = new ApiError(409, 'conflict', 'Duplicate email')
    const form = await quotaForm('CreateAccountModal')
    await fillCreation(form, 2)
    expect((await submit(form)).success).toBe(true)
    expect(form.findAll('UAlert').map(alert => alert.props.title)).toEqual([translate('accounts.create.emailTaken')])
    expect(form.findAll('UInput')[2]!.props.modelValue).toBe(2)
    expect(notices).toHaveLength(0)
  })
})

describe('editing an account quota', () => {
  it.each([[0, 0], ['0', 0], [3, 3], ['3', 3]])('sends quota %j as the number %j', async (quota, expected) => {
    const form = await quotaForm('EditQuotaModal')
    await form.trigger(form.findAll('UInput')[0]!, 'onUpdate:modelValue', quota)
    const result = await submit(form)
    expect(result.success).toBe(true)
    expect(requests).toEqual([
      { path: `/users/${accountId}`, method: 'PATCH', body: { instance_limit: expected } }
    ])
    expect(stored.instance_limit).toBe(expected)
    expect(notices).toEqual([expect.objectContaining({ title: translate('accounts.quota.updated'), color: 'success' })])
  })

  it.each([undefined, '', '   ', -1, '-1', 1.5, '1.5', NaN, Infinity, 'Infinity', 'quota'])('requires a valid quota for %j before sending', async (quota) => {
    const form = await quotaForm('EditQuotaModal')
    await form.trigger(form.findAll('UInput')[0]!, 'onUpdate:modelValue', quota)
    const result = await submit(form)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map(issue => ({ path: issue.path, message: issue.message }))).toEqual([
        { path: ['instance_limit'], message: translate('accounts.quota.quotaInvalid') }
      ])
    }
    expect(requests).toHaveLength(0)
    expect(stored.instance_limit).toBe(4)
    expect(notices).toHaveLength(0)
  })

  it('keeps the entered quota and server error when saving fails', async () => {
    serverError = new ApiError(422, 'unprocessable_entity', 'Quota could not be saved')
    const form = await quotaForm('EditQuotaModal')
    await form.trigger(form.findAll('UInput')[0]!, 'onUpdate:modelValue', 3)
    expect((await submit(form)).success).toBe(true)
    expect(form.findAll('UAlert').map(alert => alert.props.title)).toEqual(['Quota could not be saved'])
    expect(form.findAll('UInput')[0]!.props.modelValue).toBe(3)
    expect(notices).toHaveLength(0)
  })
})
