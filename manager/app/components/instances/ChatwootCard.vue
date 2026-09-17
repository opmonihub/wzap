<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { ChatwootConfig, InstanceStatus } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const emit = defineEmits<{
  loaded: [enabled: boolean | null]
}>()

const { t } = useI18n()
const { getChatwoot, setChatwoot, importHistory, isChatwootDisabled } = useInstanceChatwoot()

const config = ref<ChatwootConfig | null>(null)
const disabled = ref(false)
const failure = ref<string | null>(null)
const imported = ref<number | null>(null)

async function load() {
  failure.value = null
  try {
    config.value = await getChatwoot(props.instanceId)
    emit('loaded', config.value.enabled)
  } catch (error) {
    if (isChatwootDisabled(error)) {
      disabled.value = true
      emit('loaded', null)
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.messages.loadFailed')
    }
  }
}

async function onImport() {
  imported.value = null
  failure.value = null
  try {
    const result = await importHistory(props.instanceId)
    imported.value = result.imported
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onSave() {
  if (!config.value) {
    return
  }
  try {
    config.value = await setChatwoot(props.instanceId, config.value)
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

watch(() => props.instanceId, () => void load(), { immediate: true })
void onSave
void props.status
</script>

<template>
  <UCard variant="subtle" data-testid="chatwoot-card">
    <template #header>
      <h3 class="font-medium text-highlighted">
        {{ t('instances.chatwoot.cardTitle') }}
      </h3>
    </template>
    <div class="flex flex-col gap-3">
      <UAlert v-if="disabled" color="info" variant="subtle" :title="t('instances.chatwoot.disabled')" />
      <UAlert v-if="failure" color="error" variant="subtle" :title="failure" />
      <UAlert v-if="imported !== null" color="success" variant="subtle" :title="t('instances.chatwoot.imported', { count: imported })" />
      <p v-if="config?.webhook_url" class="truncate font-mono text-xs text-muted">
        {{ config.webhook_url }}
      </p>
      <UInput v-if="config" v-model="config.token" :disabled="disabled" class="w-full font-mono" type="password" :hint="t('instances.chatwoot.tokenHint')" />
      <div class="flex gap-2">
        <UButton :disabled="disabled" label="Save" @click="onSave" />
        <UButton :disabled="disabled" variant="soft" label="Import" @click="onImport" />
      </div>
    </div>
  </UCard>
</template>
