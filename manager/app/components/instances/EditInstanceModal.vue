<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { Instance } from '~/types/api'

// Inline edit dialog for the instances table: renames the instance or edits
// its external reference without leaving the list. The detail screen keeps
// its own fuller form; this modal only mirrors its save path.
const props = defineProps<{
  target: Instance | null
}>()

const emit = defineEmits<{
  updated: [instance: Instance]
}>()

const { t } = useI18n()
const { updateInstance } = useInstances()

const open = defineModel<boolean>('open', { default: false })

const name = ref('')
const externalRef = ref('')
const saving = ref(false)
const failure = ref<string | null>(null)

watch([open, () => props.target], ([isOpen]) => {
  if (isOpen && props.target) {
    name.value = props.target.name
    externalRef.value = props.target.external_ref
    saving.value = false
    failure.value = null
  }
})

async function onSubmit() {
  if (!props.target || saving.value) {
    return
  }
  const trimmedName = name.value.trim()
  if (trimmedName === '') {
    failure.value = t('instances.create.nameRequired')
    return
  }
  saving.value = true
  failure.value = null
  try {
    const updated = await updateInstance(props.target.id, {
      name: trimmedName,
      external_ref: externalRef.value.trim()
    })
    open.value = false
    emit('updated', updated)
  } catch (error) {
    failure.value = error instanceof ApiError ? friendlySaveError(error) : t('instances.edit.saveFailed')
  } finally {
    saving.value = false
  }
}

function friendlySaveError(error: ApiError): string {
  if (error.status === 409) {
    return t('instances.detail.externalRefTaken')
  }
  return error.message
}
</script>

<template>
  <UModal v-model:open="open" :title="t('instances.edit.title')" :description="t('instances.edit.body', { name: target?.name ?? '' })">
    <template #body>
      <form class="flex flex-col gap-4" @submit.prevent="onSubmit">
        <UAlert
          v-if="failure"
          color="error"
          variant="subtle"
          :title="failure"
        />

        <UFormField :label="t('instances.fields.name')" name="name" required>
          <UInput
            v-model="name"
            required
            maxlength="255"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('instances.fields.externalRef')" :hint="t('instances.fields.externalRefHint')" name="external_ref">
          <UInput v-model="externalRef" maxlength="255" class="w-full" />
        </UFormField>

        <div class="flex justify-end gap-2">
          <UButton
            type="button"
            color="neutral"
            variant="ghost"
            :label="t('common.cancel')"
            @click="open = false"
          />
          <UButton type="submit" :loading="saving" :label="saving ? t('common.saving') : t('common.save')" />
        </div>
      </form>
    </template>
  </UModal>
</template>
