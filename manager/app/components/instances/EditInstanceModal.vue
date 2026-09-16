<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
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

const schema = z.object({
  name: z.string().min(1, t('instances.create.nameRequired')).max(255),
  external_ref: z.string().max(255)
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ name: '', external_ref: '' })
const saving = ref(false)
const failure = ref<string | null>(null)

watch([open, () => props.target], ([isOpen]) => {
  if (isOpen && props.target) {
    state.name = props.target.name
    state.external_ref = props.target.external_ref
    saving.value = false
    failure.value = null
  }
})

async function onSubmit(event: FormSubmitEvent<Schema>) {
  if (!props.target || saving.value) {
    return
  }
  const trimmedName = (event.data.name ?? '').trim()
  if (trimmedName === '') {
    failure.value = t('instances.create.nameRequired')
    return
  }
  saving.value = true
  failure.value = null
  try {
    const updated = await updateInstance(props.target.id, {
      name: trimmedName,
      external_ref: (event.data.external_ref ?? '').trim()
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
      <UForm
        id="edit-instance"
        :schema="schema"
        :state="state"
        class="flex flex-col gap-4"
        @submit="onSubmit"
      >
        <UAlert
          v-if="failure"
          color="error"
          variant="subtle"
          :title="failure"
        />

        <UFormField :label="t('instances.fields.name')" name="name" required>
          <UInput
            v-model="state.name"
            maxlength="255"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('instances.fields.externalRef')" :hint="t('instances.fields.externalRefHint')" name="external_ref">
          <UInput v-model="state.external_ref" maxlength="255" class="w-full" />
        </UFormField>
      </UForm>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2">
        <UButton
          type="button"
          color="neutral"
          variant="ghost"
          :label="t('common.cancel')"
          @click="open = false"
        />
        <UButton
          type="submit"
          form="edit-instance"
          :loading="saving"
          :label="saving ? t('common.saving') : t('common.save')"
        />
      </div>
    </template>
  </UModal>
</template>
