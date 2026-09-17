<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { AccountUser } from '~/types/api'

const props = defineProps<{
  target: AccountUser | null
}>()
const emit = defineEmits<{
  deleted: [id: string]
}>()
const open = defineModel<boolean>('open', { default: false })

const { t } = useI18n()
const toast = useToast()
const { deleteUser } = useAccounts()

const deleting = ref(false)
const deleteFailure = ref<string | null>(null)

watch(open, (value) => {
  if (value) {
    deleting.value = false
    deleteFailure.value = null
  }
})

async function onDelete() {
  if (!props.target || deleting.value) {
    return
  }
  deleting.value = true
  deleteFailure.value = null
  try {
    await deleteUser(props.target.id)
    emit('deleted', props.target.id)
    open.value = false
    toast.add({ title: t('accounts.delete.deleted'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    deleteFailure.value = error instanceof ApiError ? friendlyDeleteError(error) : t('accounts.delete.failed')
  } finally {
    deleting.value = false
  }
}

function friendlyDeleteError(error: ApiError): string {
  if (error.status === 409) {
    return t('accounts.delete.ownsInstances')
  }
  return error.message
}
</script>

<template>
  <UModal
    v-model:open="open"
    :title="t('accounts.delete.title')"
    :description="t('accounts.delete.body', { email: target?.email ?? '' })"
    :ui="{ footer: 'justify-end' }"
  >
    <template #body>
      <UAlert
        v-if="deleteFailure"
        color="error"
        variant="subtle"
        :title="deleteFailure"
      />
    </template>
    <template #footer="{ close }">
      <UButton
        color="neutral"
        variant="outline"
        :label="t('common.cancel')"
        @click="close"
      />
      <UButton
        color="error"
        :loading="deleting"
        :label="deleting ? t('accounts.delete.deleting') : t('accounts.delete.submit')"
        @click="onDelete"
      />
    </template>
  </UModal>
</template>
