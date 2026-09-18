<template>
  <UAuthForm
    :schema="schema"
    :fields="fields"
    :submit="{ label: pending ? t('auth.signingIn') : t('auth.submit'), block: true, loading: pending }"
    :title="t('auth.loginTitle')"
    :description="t('auth.loginSubtitle')"
    @submit="onSubmit"
  >
    <template #validation>
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
    </template>
  </UAuthForm>
</template>

<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'

const { t } = useI18n()
const { login } = useAuth()
const pending = ref(false)
const failure = ref<string | null>(null)

const schema = z.object({
  email: z.email(),
  password: z.string().min(1)
})
type Schema = z.output<typeof schema>

const fields = computed(() => [
  { name: 'email', type: 'email' as const, label: t('auth.email'), placeholder: 'admin@example.com', required: true, autocomplete: 'username' },
  { name: 'password', type: 'password' as const, label: t('auth.password'), required: true, autocomplete: 'current-password' }
])

async function onSubmit(event: FormSubmitEvent<Schema>) {
  if (pending.value) {
    return
  }
  pending.value = true
  failure.value = null
  try {
    await login(event.data.email.trim(), event.data.password)
  } catch (error) {
    // A plain Error (not ApiError) means the request never reached the Go
    // backend: offline service or network failure. Showing the credential
    // hint here misdirects the user into retrying a correct password.
    if (!(error instanceof ApiError)) {
      failure.value = t('auth.serviceUnavailable')
      pending.value = false
      return
    }
    failure.value = error.message || t('auth.loginFailed')
    pending.value = false
    return
  }
  try {
    await navigateTo('/')
  } catch {
    // Post-login navigation failures (e.g. a route chunk that fails to load)
    // are not credential problems: the session already exists, so route home
    // with a full reload instead of stranding an authenticated user on the
    // form, which invites a duplicate login and a second session.
    await navigateTo('/', { external: true })
  } finally {
    pending.value = false
  }
}
</script>
