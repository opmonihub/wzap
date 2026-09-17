<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const { sendPresence } = useInstanceMessaging()
const { rejectCall } = useInstanceProfile()

const chat = ref('')
const callId = ref('')
const from = ref('')
const failure = ref<string | null>(null)
const unsupported = ref(false)
const canAct = computed(() => props.status === 'connected')

async function onReject() {
  failure.value = null
  unsupported.value = false
  try {
    await rejectCall(props.instanceId, { call_id: callId.value.trim(), from: from.value.trim() })
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      unsupported.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
    }
  }
}

async function onPresence(state: 'composing' | 'paused' | 'available' | 'unavailable') {
  try {
    await sendPresence(props.instanceId, { chat: chat.value.trim(), state })
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

void onPresence
</script>

<template>
  <UPageCard :title="t('instances.actions.presence')" variant="subtle" data-testid="device-actions-card">
    <div class="flex flex-col gap-3">
      <UAlert
        v-if="!canAct"
        color="warning"
        variant="subtle"
        :title="t('instances.actions.notConnected')"
      />
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
      <UAlert
        v-if="unsupported"
        color="warning"
        variant="subtle"
        :title="t('instances.actions.unsupported')"
      />
      <UInput v-model="chat" class="w-full font-mono" />
      <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
        <UInput v-model="callId" class="w-full font-mono" />
        <UInput v-model="from" class="w-full font-mono" />
        <UButton
          :disabled="!canAct"
          class="w-fit shrink-0"
          :label="t('instances.actions.rejectCall')"
          @click="onReject"
        />
      </div>
    </div>
  </UPageCard>
</template>
