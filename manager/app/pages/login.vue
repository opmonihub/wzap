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

definePageMeta({ layout: 'auth' })

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
    await navigateTo('/')
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('auth.loginFailed')
  } finally {
    pending.value = false
  }
}
</script>
