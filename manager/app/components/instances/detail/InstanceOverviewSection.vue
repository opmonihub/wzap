<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import { instanceNameErrorKey, instanceNameUpdate, isValidInstanceName } from '~/utils/instanceName'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import PairingCard from '~/components/instances/PairingCard.vue'
import PairPhoneCard from '~/components/instances/PairPhoneCard.vue'
import type { Instance, UpdateInstanceInput } from '~/types/api'

const props = defineProps<{
  instance: Instance
}>()

const emit = defineEmits<{
  paired: []
  updated: [instance: Instance]
  changed: []
}>()

const { t } = useI18n()
const toast = useToast()
const { updateInstance, disconnectInstance } = useInstances()
const { confirmDelete } = useConfirmDelete()

// external_ref never comes back on reads (it is write-only in the public
// DTO), so the field always starts empty and an empty submit is never sent:
// only a typed value reaches the API, so the stored reference is never
// erased implicitly.
const schema = z.object({
  name: z.string().refine(name => isValidInstanceName(name, props.instance.name), t('instances.fields.nameHint')),
  external_ref: z.string().max(255)
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ name: props.instance.name, external_ref: '' })
const saving = ref(false)
const saveFailure = ref<string | null>(null)

const disconnecting = ref(false)

async function onSave(event: FormSubmitEvent<Schema>) {
  if (saving.value) {
    return
  }
  const name = event.data.name
  if (!isValidInstanceName(name, props.instance.name)) {
    saveFailure.value = t('instances.fields.nameHint')
    return
  }
  saving.value = true
  saveFailure.value = null
  try {
    const patch: UpdateInstanceInput = instanceNameUpdate(name, props.instance.name)
    const externalRef = (event.data.external_ref ?? '').trim()
    // Omitted keeps the stored external_ref; the field starts empty (the
    // read never echoes it), so only a typed value is sent — never an
    // implicit clear.
    if (externalRef !== '') {
      patch.external_ref = externalRef
    }
    const updated = await updateInstance(props.instance.id, patch)
    emit('updated', updated)
    toast.add({ title: t('instances.detail.saved'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    saveFailure.value = friendlySaveError(error)
  } finally {
    saving.value = false
  }
}

function friendlySaveError(error: unknown): string {
  if (error instanceof ApiError) {
    const nameError = instanceNameErrorKey(error)
    if (nameError) {
      return t(nameError)
    }
    if (error.status === 409 && error.code === 'conflict') {
      return t('instances.detail.externalRefTaken')
    }
    return error.message
  }
  return t('instances.detail.saveFailed')
}

// Disconnect runs behind the programmatic confirm: the overlay resolves true
// only on the confirm button, and the API (with its loading state and toasts)
// runs on the trigger afterwards.
async function onDisconnect() {
  if (disconnecting.value) {
    return
  }
  const confirmed = await confirmDelete({
    title: t('instances.detail.disconnectConfirmTitle'),
    description: t('instances.detail.disconnectConfirmBody'),
    confirmLabel: t('instances.detail.disconnect'),
    confirmColor: 'warning'
  })
  if (!confirmed || disconnecting.value) {
    return
  }
  const instanceId = props.instance.id
  disconnecting.value = true
  try {
    await disconnectInstance(instanceId)
    toast.add({ title: t('instances.detail.disconnected'), icon: 'i-lucide-check', color: 'success' })
    emit('changed')
  } catch (error) {
    toast.add({ title: error instanceof ApiError ? error.message : t('instances.detail.disconnectFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
  } finally {
    disconnecting.value = false
  }
}

// The pairing card emits after the phone scan flips the session to
// connected; the page reloads to refresh the badge and disconnect action.
function onPaired() {
  emit('paired')
}

function formatDateTime(value: string | null): string {
  if (!value) {
    return t('common.notSet')
  }
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}

// A fresh instance row (after pairing/disconnect reloads or navigation)
// replaces the edited name; the external_ref field resets to empty because
// the read never echoes the stored value.
watch(() => props.instance, (next) => {
  state.name = next.name
  state.external_ref = ''
  saveFailure.value = null
})
</script>

<template>
  <div class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
    <UPageCard :title="instance.name" variant="subtle">
      <dl class="flex flex-col gap-3 text-sm sm:gap-2">
        <div v-if="instance.connection.last_error" class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <dt class="shrink-0 text-muted">
            {{ t('instances.fields.lastError') }}
          </dt>
          <dd class="min-w-0 break-all text-highlighted sm:text-right">
            {{ instance.connection.last_error.message }}
          </dd>
        </div>
        <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <dt class="shrink-0 text-muted">
            {{ t('instances.fields.lastConnectedAt') }}
          </dt>
          <dd class="min-w-0 break-all text-highlighted sm:text-right">
            {{ formatDateTime(instance.connection.last_connected_at) }}
          </dd>
        </div>
        <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <dt class="shrink-0 text-muted">
            {{ t('instances.fields.createdAt') }}
          </dt>
          <dd class="min-w-0 break-all text-highlighted sm:text-right">
            {{ formatDateTime(instance.created_at) }}
          </dd>
        </div>
        <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <dt class="shrink-0 text-muted">
            {{ t('instances.fields.updatedAt') }}
          </dt>
          <dd class="min-w-0 break-all text-highlighted sm:text-right">
            {{ formatDateTime(instance.updated_at) }}
          </dd>
        </div>
      </dl>
      <template v-if="instance.connection.status === 'connected'" #footer>
        <UButton
          color="warning"
          variant="soft"
          icon="i-lucide-unplug"
          :loading="disconnecting"
          :label="disconnecting ? t('instances.detail.disconnecting') : t('instances.detail.disconnect')"
          @click="onDisconnect"
        />
      </template>
    </UPageCard>

    <PairingCard
      :instance-id="instance.id"
      :status="instance.connection.status"
      @paired="onPaired"
    />

    <PairPhoneCard :instance-id="instance.id" :status="instance.connection.status" />

    <UPageCard :title="t('instances.fields.name')" variant="subtle">
      <UForm
        id="instance-name"
        :schema="schema"
        :state="state"
        class="flex flex-col gap-4"
        @submit="onSave"
      >
        <UAlert
          v-if="saveFailure"
          color="error"
          variant="subtle"
          :title="saveFailure"
        />

        <UFormField
          :label="t('instances.fields.name')"
          :description="`${t('instances.fields.nameHint')} ${t('instances.fields.nameLegacyHint')}`"
          name="name"
          required
        >
          <UInput
            v-model="state.name"
            class="w-full"
          />
        </UFormField>

        <UFormField :label="t('instances.fields.externalRef')" :hint="t('instances.fields.externalRefWriteOnlyHint')" name="external_ref">
          <UInput v-model="state.external_ref" maxlength="255" class="w-full" />
        </UFormField>

        <div class="flex justify-end">
          <UButton type="submit" :loading="saving" :label="saving ? t('common.saving') : t('common.save')" />
        </div>
      </UForm>
    </UPageCard>
  </div>
</template>
