<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useConfirmDelete } from '~/components/instances/ConfirmDelete'
import OneTimeKeyDisplay from '~/components/instances/OneTimeKeyDisplay.vue'
import type { Instance, RotatedInstanceKey, StatusPrivacy } from '~/types/api'

const props = defineProps<{
  instance: Instance
  isAdmin: boolean
}>()

const emit = defineEmits<{
  'changed': []
  'delete-requested': []
}>()

const { t } = useI18n()
const toast = useToast()
const { rotateInstanceKey, revokeInstanceKey } = useInstances()
const { setDefaultDisappearing } = useInstanceProfile()
const { confirmDelete } = useConfirmDelete()

const disappearingOptions = computed(() => [
  { label: t('instances.settings.disappearingOff'), value: '0' },
  { label: t('instances.settings.disappearing24h'), value: '24h' },
  { label: t('instances.settings.disappearing7d'), value: '168h' },
  { label: t('instances.settings.disappearing90d'), value: '2160h' }
])
const disappearingChoice = ref('0')
const savingDisappearing = ref(false)
const disappearingFailure = ref<string | null>(null)

watch(() => props.instance.settings?.default_disappearing, (value) => {
  const options = disappearingOptions.value
  disappearingChoice.value = value && options.some(o => o.value === value) ? value : '0'
}, { immediate: true })

async function onSaveDefaultDisappearing() {
  if (savingDisappearing.value || props.instance.connection.status !== 'connected') {
    return
  }
  savingDisappearing.value = true
  disappearingFailure.value = null
  try {
    await setDefaultDisappearing(props.instance.id, disappearingChoice.value)
    toast.add({ title: t('instances.settings.defaultDisappearingSaved'), icon: 'i-lucide-check', color: 'success' })
    emit('changed')
  } catch (error) {
    disappearingFailure.value = error instanceof ApiError ? error.message : t('instances.settings.defaultDisappearingFailed')
  } finally {
    savingDisappearing.value = false
  }
}

const freshKey = ref<RotatedInstanceKey | null>(null)
const keySeen = ref(false)
const generating = ref(false)
const keyFailure = ref<string | null>(null)
const revoking = ref(false)

async function onGenerate() {
  if (generating.value) {
    return
  }
  generating.value = true
  keyFailure.value = null
  try {
    freshKey.value = await rotateInstanceKey(props.instance.id)
    markInstanceKeySeen(props.instance.id)
    keySeen.value = true
    toast.add({ title: t('instances.key.generated'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    keyFailure.value = error instanceof ApiError ? error.message : t('instances.key.generateFailed')
  } finally {
    generating.value = false
  }
}

// Revoke runs behind the programmatic confirm like disconnect: failures
// surface as toasts, while keyFailure stays owned by the generate path whose
// alert renders in the card.
async function onRevoke() {
  if (revoking.value) {
    return
  }
  const confirmed = await confirmDelete({
    title: t('instances.key.revokeConfirmTitle'),
    description: t('instances.key.revokeConfirmBody'),
    confirmLabel: t('instances.key.revoke')
  })
  if (!confirmed || revoking.value) {
    return
  }
  const instanceId = props.instance.id
  revoking.value = true
  try {
    await revokeInstanceKey(instanceId)
    forgetInstanceKeySeen(instanceId)
    keySeen.value = false
    freshKey.value = null
    toast.add({ title: t('instances.key.revoked'), icon: 'i-lucide-check', color: 'success' })
    emit('changed')
  } catch (error) {
    toast.add({ title: error instanceof ApiError ? error.message : t('instances.key.revokeFailed'), icon: 'i-lucide-triangle-alert', color: 'error' })
  } finally {
    revoking.value = false
  }
}

watch(() => props.instance.id, (nextId: string) => {
  keySeen.value = hasSeenInstanceKey(nextId)
}, { immediate: true })

// Compact read-only snapshot of the available instance.settings blocks.
// Purely presentational: profile/privacy edits stay under the Profile
function audienceLabel(value: string | null | undefined): string {
  switch (value) {
    case 'all':
      return t('instances.privacy.allowAll')
    case 'contacts':
      return t('instances.privacy.allowContacts')
    case 'contact_blacklist':
      return t('instances.privacy.allowBlacklist')
    case 'none':
      return t('instances.privacy.allowNone')
    default:
      return value || t('common.notSet')
  }
}

function statusPrivacyLabel(value: StatusPrivacy | undefined): string {
  if (!value) {
    return t('common.notSet')
  }
  return value.jids.length > 0
    ? `${value.mode} · ${t('instances.settings.statusPrivacyJids', { count: value.jids.length })}`
    : value.mode
}

const settingsRows = computed(() => {
  const settings = props.instance.settings
  return [
    { key: 'profileName', label: t('instances.settings.profileName'), value: settings?.profile?.name ?? t('common.notSet') },
    { key: 'statusText', label: t('instances.profile.recado'), value: settings?.profile?.status_text ?? t('common.notSet') },
    { key: 'lastSeen', label: t('instances.privacy.lastSeen'), value: audienceLabel(settings?.privacy?.last_seen) },
    { key: 'profilePhoto', label: t('instances.privacy.profilePhoto'), value: audienceLabel(settings?.privacy?.profile_photo) },
    { key: 'status', label: t('instances.privacy.status'), value: audienceLabel(settings?.privacy?.status) },
    { key: 'readReceipts', label: t('instances.privacy.readReceipts'), value: audienceLabel(settings?.privacy?.read_receipts) },
    { key: 'groupsAdd', label: t('instances.privacy.groupsAdd'), value: audienceLabel(settings?.privacy?.groups_add) },
    { key: 'statusPrivacy', label: t('instances.settings.statusPrivacy'), value: statusPrivacyLabel(settings?.status_privacy) }
  ]
})

const isConnected = computed(() => props.instance.connection.status === 'connected')
</script>

<template>
  <div class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
    <UPageCard :title="t('instances.settings.cardTitle')" variant="subtle" data-testid="instance-settings-card">
      <div class="flex flex-col gap-3">
        <p class="text-sm text-muted">
          {{ t('instances.settings.hint') }}
        </p>
        <dl class="flex flex-col gap-3 text-sm sm:gap-2">
          <div
            v-for="row in settingsRows"
            :key="row.key"
            class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4"
          >
            <dt class="shrink-0 text-muted">
              {{ row.label }}
            </dt>
            <dd class="min-w-0 break-all text-highlighted sm:text-right">
              {{ row.value }}
            </dd>
          </div>
        </dl>
        <div v-if="isConnected" class="flex flex-col gap-2 border-t border-default pt-4">
          <p class="text-sm font-medium text-highlighted">
            {{ t('instances.settings.defaultDisappearing') }}
          </p>
          <UAlert
            v-if="disappearingFailure"
            color="error"
            variant="subtle"
            :title="disappearingFailure"
          />
          <div class="flex flex-wrap items-end gap-2">
            <USelect
              v-model="disappearingChoice"
              :items="disappearingOptions"
              class="min-w-48"
            />
            <UButton
              :loading="savingDisappearing"
              :label="t('instances.settings.saveDefaultDisappearing')"
              @click="onSaveDefaultDisappearing"
            />
          </div>
          <p class="text-xs text-muted">
            {{ t('instances.settings.defaultDisappearingHint') }}
          </p>
        </div>
        <p v-else class="border-t border-default pt-4 text-sm text-muted">
          {{ t('instances.settings.defaultDisappearing') }}:
          {{ props.instance.settings?.default_disappearing || t('common.notSet') }}
        </p>
      </div>
    </UPageCard>

    <UPageCard v-if="isAdmin" :title="t('instances.key.cardTitle')" variant="subtle">
      <div class="flex flex-col gap-4">
        <UAlert
          v-if="keyFailure"
          color="error"
          variant="subtle"
          :title="keyFailure"
        />

        <OneTimeKeyDisplay v-if="freshKey" :api-key="freshKey.instance_api_key" />

        <template v-else>
          <!-- keySeen tracks whether the operator saved a freshly rotated key
              in this browser; it does not reflect server-side key existence. -->
          <UAlert
            v-if="!keySeen"
            color="info"
            variant="subtle"
            :title="t('instances.key.keylessTitle')"
            :description="t('instances.key.keylessBody')"
          />

          <p v-else class="text-sm text-muted">
            {{ t('instances.key.rotateHint') }}
          </p>

          <div class="flex flex-wrap gap-2">
            <UButton
              icon="i-lucide-key-round"
              :loading="generating"
              :label="generating ? t('instances.key.generating') : t('instances.key.generate')"
              @click="onGenerate"
            />
            <UButton
              v-if="keySeen"
              color="error"
              variant="soft"
              :loading="revoking"
              :label="revoking ? t('instances.key.revoking') : t('instances.key.revoke')"
              @click="onRevoke"
            />
          </div>
        </template>
      </div>
    </UPageCard>

    <UPageCard :title="t('instances.detail.delete')" variant="subtle" :ui="{ title: 'text-error' }">
      <div class="flex min-w-0 flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
        <p class="min-w-0 text-sm text-muted">
          {{ t('instances.delete.warning') }}
        </p>
        <UButton
          color="error"
          variant="soft"
          icon="i-lucide-trash-2"
          class="w-fit shrink-0"
          :label="t('instances.detail.delete')"
          @click="emit('delete-requested')"
        />
      </div>
    </UPageCard>
  </div>
</template>
