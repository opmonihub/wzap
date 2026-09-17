<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, Privacy } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const { getPrivacy, setPrivacy } = useInstanceProfile()

const privacy = ref<Privacy | null>(null)
const failure = ref<string | null>(null)
const canAct = computed(() => props.status === 'connected')
const OPTIONS = ['all', 'contacts', 'contact_blacklist', 'none']
const RECEIPTS = ['all', 'none']

async function load() {
  try {
    privacy.value = await getPrivacy(props.instanceId)
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.messages.loadFailed')
  }
}

async function onSave() {
  if (!privacy.value) {
    failure.value = t('instances.privacy.atLeastOne')
    return
  }
  try {
    privacy.value = await setPrivacy(props.instanceId, privacy.value)
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

watch(() => props.instanceId, () => void load(), { immediate: true })
void OPTIONS
void RECEIPTS
</script>

<template>
  <UPageCard variant="subtle" data-testid="privacy-card">
    <template #header>
      <h3 class="font-medium text-highlighted">
        {{ t('instances.privacy.cardTitle') }}
      </h3>
    </template>
    <div class="flex flex-col gap-3">
      <UAlert v-if="!canAct" color="warning" variant="subtle" :title="t('instances.actions.notConnected')" />
      <UAlert v-if="failure" color="error" variant="subtle" :title="failure" />
      <UButton :disabled="!canAct" :label="t('common.save')" @click="onSave" />
    </div>
  </UPageCard>
</template>
