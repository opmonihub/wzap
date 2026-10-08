import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import * as z from 'zod'
import { ApiError } from '../../composables/useApi'
import { useAuth } from '../../composables/useAuth'
import {
  installComponentGlobals, loadComponent, mountComponent, primitive, translate, unmountComponents
} from '../../../tests/componentHarness'

let requests: { path: string, method?: string, body?: unknown }[]
let serverError: Error | null
let navigations: { path: string, options?: unknown }[]

beforeEach(() => {
  installComponentGlobals()
  requests = []
  serverError = null
  navigations = []
  const states = new Map()
  vi.stubGlobal('useState', (key: string, initial: () => unknown) => {
    if (!states.has(key)) states.set(key, ref(initial()))
    return states.get(key)
  })
  vi.stubGlobal('useApi', () => ({
    api: async (path: string, options?: { method: string, body?: unknown }) => {
      requests.push({ path, ...options })
      if (serverError) throw serverError
      return { me: { id: '42b4c925-dcba-455c-9669-296143778321', email: 'admin@example.com', role: 'admin' } }
    }
  }))
  vi.stubGlobal('useAuth', useAuth)
  vi.stubGlobal('navigateTo', async (path: string, options?: unknown) => {
    navigations.push({ path, options })
  })
})

afterEach(() => {
  unmountComponents()
  vi.unstubAllGlobals()
})

async function loginForm() {
  const component = Object.assign(loadComponent('components/auth/LoginForm.vue', { zod: z }), {
    components: { UAuthForm: primitive('UAuthForm') }
  })
  return await mountComponent(component)
}

type MountedForm = Awaited<ReturnType<typeof loginForm>>

async function submit(form: MountedForm, data?: Record<string, unknown>) {
  const target = form.findAll('UAuthForm')[0]!
  const fields = target.props.fields as { name: string, defaultValue?: unknown }[]
  const untouched = Object.fromEntries(fields.map(field => [field.name, field.defaultValue]))
  const schema = target.props.schema as z.ZodType<Record<string, unknown>>
  const result = schema.safeParse(data ?? untouched)
  if (result.success) await form.trigger(target, 'onSubmit', { data: result.data })
  return result
}

describe('login validation', () => {
  it.each([undefined, { email: '', password: '' }])('explains missing fields for %j without a login request', async (data) => {
    const form = await loginForm()
    const result = await submit(form, data)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map(issue => ({ path: issue.path, message: issue.message }))).toEqual([
        { path: ['email'], message: translate('auth.emailRequired') },
        { path: ['password'], message: translate('auth.passwordRequired') }
      ])
    }
    expect(requests).toHaveLength(0)
  })

  it('guides an invalid email without sending credentials', async () => {
    const form = await loginForm()
    const result = await submit(form, { email: 'invalid-email', password: 'test-password' })
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map(issue => ({ path: issue.path, message: issue.message }))).toEqual([
        { path: ['email'], message: translate('auth.emailInvalid') }
      ])
    }
    expect(requests).toHaveLength(0)
  })

  it('accepts the existing one-character password minimum and completes login', async () => {
    const form = await loginForm()
    expect((await submit(form, { email: 'admin@example.com', password: 'x' })).success).toBe(true)
    expect(requests).toEqual([
      { path: '/auth/login', method: 'POST', body: { email: 'admin@example.com', password: 'x' } }
    ])
    expect(navigations).toEqual([{ path: '/', options: undefined }])
    expect(form.findAll('UAlert')).toHaveLength(0)
  })

  it.each([
    [new ApiError(401, 'unauthorized', 'Credentials were rejected'), 'Credentials were rejected'],
    [new ApiError(401, 'unauthorized', ''), translate('auth.loginFailed')],
    [new Error('Offline'), translate('auth.serviceUnavailable')]
  ])('preserves sign-in failure guidance for %j', async (error, expected) => {
    serverError = error as Error
    const form = await loginForm()
    expect((await submit(form, { email: 'admin@example.com', password: 'test-password' })).success).toBe(true)
    expect(form.findAll('UAlert').map(alert => alert.props.title)).toEqual([expected])
    expect(navigations).toHaveLength(0)
    expect(form.findAll('UAuthForm')[0]!.props.submit).toEqual(expect.objectContaining({ loading: false }))
  })
})
