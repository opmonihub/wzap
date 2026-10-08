<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import { parseQuota } from '~/utils/accountQuota'
import type { AccountUser } from '~/types/api'

const props = defineProps<{
  target: AccountUser | null
}>()
const emit = defineEmits<{
  updated: [user: AccountUser]
}>()
const open = defineModel<boolean>('open', { default: false })

const { t } = useI18n()
const toast = useToast()
const { updateUserQuota } = useAccounts()

const quotaSchema = z.object({
  instance_limit: z.union([z.string(), z.number()], { error: t('accounts.quota.quotaInvalid') })
    .optional()
    .refine((value) => {
      const quota = parseQuota(value)
      return quota !== null && quota !== undefined
    }, t('accounts.quota.quotaInvalid'))
})
type QuotaSchema = z.output<typeof quotaSchema>
const quotaState = reactive<Partial<QuotaSchema>>({ instance_limit: '' })
const quotaSaving = ref(false)
const quotaFailure = ref<string | null>(null)

watch([open, () => props.target], ([isOpen, target]) => {
  if (isOpen && target) {
    quotaState.instance_limit = String(target.instance_limit)
    quotaSaving.value = false
    quotaFailure.value = null
  }
})

async function onSaveQuota(event: FormSubmitEvent<QuotaSchema>) {
  if (!props.target || quotaSaving.value) {
    return
  }
  const quota = parseQuota(event.data.instance_limit ?? '')
  if (quota === null || quota === undefined) {
    quotaFailure.value = t('accounts.quota.quotaInvalid')
    return
  }
  quotaSaving.value = true
  quotaFailure.value = null
  try {
    const updated = await updateUserQuota(props.target.id, quota)
    emit('updated', updated)
    open.value = false
    toast.add({ title: t('accounts.quota.updated'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    quotaFailure.value = error instanceof ApiError ? error.message : t('accounts.quota.saveFailed')
  } finally {
    quotaSaving.value = false
  }
}
</script>

<template>
  <UModal v-model:open="open" :title="t('accounts.quota.title')" :description="t('accounts.quota.body', { email: target?.email ?? '' })">
    <template #body>
      <UForm
        id="edit-quota"
        :schema="quotaSchema"
        :state="quotaState"
        class="flex flex-col gap-4"
        @submit="onSaveQuota"
      >
        <UAlert
          v-if="quotaFailure"
          color="error"
          variant="subtle"
          :title="quotaFailure"
        />

        <UFormField
          :label="t('accounts.quotaLabel')"
          :hint="t('accounts.quota.hint')"
          name="instance_limit"
          required
        >
          <UInput
            v-model="quotaState.instance_limit"
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
          <UButton type="submit" :loading="quotaSaving" :label="quotaSaving ? t('common.saving') : t('common.save')" />
        </div>
      </UForm>
    </template>
  </UModal>
</template>
