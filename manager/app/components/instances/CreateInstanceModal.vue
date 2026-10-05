<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import { instanceNameErrorKey, isValidInstanceName } from '~/utils/instanceName'
import OneTimeKeyDisplay from '~/components/instances/OneTimeKeyDisplay.vue'
import type { CreatedInstance } from '~/types/api'

// Creation dialog: form first, then the created instance with its one-time
// key for copy. Closing the dialog drops the key from memory by design.
const emit = defineEmits<{
  created: [instance: CreatedInstance]
}>()

const { t } = useI18n()
const { createInstance } = useInstances()

const open = defineModel<boolean>('open', { default: false })

const schema = z.object({
  name: z.string().refine(name => isValidInstanceName(name), t('instances.fields.nameHint')),
  external_ref: z.string().max(255)
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ name: '', external_ref: '' })
const pending = ref(false)
const failure = ref<string | null>(null)
const created = ref<CreatedInstance | null>(null)

function reset() {
  state.name = ''
  state.external_ref = ''
  pending.value = false
  failure.value = null
  created.value = null
}

watch(open, (value) => {
  if (value) {
    reset()
  }
})

async function onSubmit(event: FormSubmitEvent<Schema>) {
  if (pending.value) {
    return
  }
  const name = event.data.name
  if (!isValidInstanceName(name)) {
    failure.value = t('instances.fields.nameHint')
    return
  }
  pending.value = true
  failure.value = null
  try {
    created.value = await createInstance({ name, external_ref: event.data.external_ref ?? '' })
    markInstanceKeySeen(created.value.id)
    emit('created', created.value)
  } catch (error) {
    failure.value = error instanceof ApiError ? friendlyCreateError(error) : t('instances.create.failed')
  } finally {
    pending.value = false
  }
}

function friendlyCreateError(error: ApiError): string {
  const nameError = instanceNameErrorKey(error)
  if (nameError) {
    return t(nameError)
  }
  if (error.status === 403 && error.code === 'quota_exceeded') {
    return t('instances.create.quotaExceeded')
  }
  if (error.status === 409 && error.code === 'conflict') {
    return t('instances.create.externalRefTaken')
  }
  return error.message
}
</script>

<template>
  <UModal v-model:open="open" :title="created ? t('instances.create.successTitle') : t('instances.create.title')" :description="created ? t('instances.create.successBody') : t('instances.create.body')">
    <template #body>
      <UForm
        v-if="!created"
        id="create-instance"
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

        <UFormField
          :label="t('instances.fields.name')"
          :description="t('instances.fields.nameHint')"
          name="name"
          required
        >
          <UInput
            v-model="state.name"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('instances.fields.externalRef')" :hint="t('instances.fields.externalRefHint')" name="external_ref">
          <UInput v-model="state.external_ref" maxlength="255" class="w-full" />
        </UFormField>
      </UForm>

      <OneTimeKeyDisplay v-else :api-key="created.instance_api_key" />
    </template>

    <template #footer>
      <div v-if="!created" class="flex justify-end gap-2">
        <UButton
          type="button"
          color="neutral"
          variant="ghost"
          :label="t('common.cancel')"
          @click="open = false"
        />
        <UButton
          type="submit"
          form="create-instance"
          :loading="pending"
          :label="pending ? t('instances.create.creating') : t('instances.create.submit')"
        />
      </div>
      <div v-else class="flex justify-end">
        <UButton :label="t('common.done')" @click="open = false" />
      </div>
    </template>
  </UModal>
</template>
