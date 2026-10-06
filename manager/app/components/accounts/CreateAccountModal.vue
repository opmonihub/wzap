<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import { parseQuota } from '~/utils/accountQuota'
import type { AccountUser } from '~/types/api'

const emit = defineEmits<{
  created: [user: AccountUser]
}>()
const open = defineModel<boolean>('open', { default: false })

const { t } = useI18n()
const toast = useToast()
const { createUser } = useAccounts()

const createSchema = z.object({
  email: z.string().min(1, t('accounts.create.emailRequired')).max(255),
  password: z.string().min(1, t('accounts.create.passwordRequired')),
  role: z.enum(['admin', 'user']),
  instance_limit: z.string()
})
type CreateSchema = z.output<typeof createSchema>
const createState = reactive<Partial<CreateSchema>>({ email: '', password: '', role: 'user', instance_limit: '' })
const creating = ref(false)
const createFailure = ref<string | null>(null)

function resetCreate() {
  createState.email = ''
  createState.password = ''
  createState.role = 'user'
  createState.instance_limit = ''
  creating.value = false
  createFailure.value = null
}

watch(open, (value) => {
  if (value) {
    resetCreate()
  }
})

async function onCreate(event: FormSubmitEvent<CreateSchema>) {
  if (creating.value) {
    return
  }
  const email = (event.data.email ?? '').trim()
  if (email === '') {
    createFailure.value = t('accounts.create.emailRequired')
    return
  }
  if ((event.data.password ?? '') === '') {
    createFailure.value = t('accounts.create.passwordRequired')
    return
  }
  const quota = parseQuota(event.data.instance_limit ?? '')
  if (quota === null) {
    createFailure.value = t('accounts.create.quotaInvalid')
    return
  }
  creating.value = true
  createFailure.value = null
  try {
    const created = await createUser({ email, password: event.data.password ?? '', role: event.data.role ?? 'user', instance_limit: quota })
    emit('created', created)
    open.value = false
    toast.add({ title: t('accounts.create.createdToast'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    createFailure.value = error instanceof ApiError ? friendlyCreateError(error) : t('accounts.create.failed')
  } finally {
    creating.value = false
  }
}

function friendlyCreateError(error: ApiError): string {
  if (error.status === 409) {
    return t('accounts.create.emailTaken')
  }
  return error.message
}
</script>

<template>
  <UModal v-model:open="open" :title="t('accounts.create.title')" :description="t('accounts.create.body')">
    <template #body>
      <UForm
        id="create-account"
        :schema="createSchema"
        :state="createState"
        class="flex flex-col gap-4"
        @submit="onCreate"
      >
        <UAlert
          v-if="createFailure"
          color="error"
          variant="subtle"
          :title="createFailure"
        />

        <UFormField :label="t('common.email')" name="email" required>
          <UInput
            v-model="createState.email"
            type="email"
            maxlength="255"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('auth.password')" name="password" required>
          <UInput
            v-model="createState.password"
            type="password"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('common.role')" name="role" required>
          <USelect
            v-model="createState.role"
            :items="['admin', 'user']"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('accounts.quotaLabel')" :hint="t('accounts.create.quotaHint')" name="instance_limit">
          <UInput
            v-model="createState.instance_limit"
            type="number"
            step="1"
            class="w-full"
          />
        </UFormField>

        <div class="flex justify-end gap-2">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            :label="t('common.cancel')"
            @click="open = false"
          />
          <UButton type="submit" :loading="creating" :label="creating ? t('accounts.create.creating') : t('accounts.create.submit')" />
        </div>
      </UForm>
    </template>
  </UModal>
</template>
