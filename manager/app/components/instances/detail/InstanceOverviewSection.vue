<script setup lang="ts">
import * as z from 'zod'
import type { FormSubmitEvent } from '#ui/types'
import { ApiError } from '~/composables/useApi'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import PairingCard from '~/components/instances/PairingCard.vue'
import PairPhoneCard from '~/components/instances/PairPhoneCard.vue'
import type { Instance } from '~/types/api'

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

const schema = z.object({
  name: z.string().min(1, t('instances.create.nameRequired')).max(255),
  external_ref: z.string().max(255)
})
type Schema = z.output<typeof schema>
const state = reactive<Partial<Schema>>({ name: props.instance.name, external_ref: props.instance.external_ref })
const saving = ref(false)
const saveFailure = ref<string | null>(null)

const disconnecting = ref(false)

async function onSave(event: FormSubmitEvent<Schema>) {
  if (saving.value) {
    return
  }
  const trimmedName = (event.data.name ?? '').trim()
  if (trimmedName === '') {
    saveFailure.value = t('instances.create.nameRequired')
    return
  }
  saving.value = true
  saveFailure.value = null
  try {
    const updated = await updateInstance(props.instance.id, {
      name: trimmedName,
      external_ref: (event.data.external_ref ?? '').trim()
    })
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
    if (error.status === 409) {
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
// connected; the page reloads to refresh the badge, JID and disconnect action.
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
// replaces the edited values with the stored configuration.
watch(() => props.instance, (next) => {
  state.name = next.name
  state.external_ref = next.external_ref
  saveFailure.value = null
})
</script>

<template>
  <div class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
    <UPageCard :title="instance.name" variant="subtle">
      <dl class="flex flex-col gap-3 text-sm sm:gap-2">
        <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <dt class="shrink-0 text-muted">
            {{ t('instances.fields.jid') }}
          </dt>
          <dd class="min-w-0 font-mono break-all text-highlighted sm:text-right">
            {{ instance.whatsapp_jid || t('common.notSet') }}
          </dd>
        </div>
        <div v-if="instance.last_error" class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
          <dt class="shrink-0 text-muted">
            {{ t('instances.fields.lastError') }}
          </dt>
          <dd class="min-w-0 break-all text-highlighted sm:text-right">
            {{ instance.last_error }}
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
      <template v-if="instance.status === 'connected'" #footer>
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
      :status="instance.status"
      :whatsapp-jid="instance.whatsapp_jid"
      @paired="onPaired"
    />

    <PairPhoneCard :instance-id="instance.id" :status="instance.status" />

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

        <div class="flex justify-end">
          <UButton type="submit" :loading="saving" :label="saving ? t('common.saving') : t('common.save')" />
        </div>
      </UForm>
    </UPageCard>
  </div>
</template>
