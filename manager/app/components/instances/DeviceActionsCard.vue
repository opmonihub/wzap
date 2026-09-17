<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus } from '~/types/api'

// Device-level actions: one-shot presence signals plus call reject. An
// upstream that cannot reject answers 501 not_supported, surfaced as an
// explicit unsupported notice instead of an error.
const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const toast = useToast()
const { sendPresence } = useInstanceMessaging()
const { rejectCall } = useInstanceProfile()

const chat = ref('')
const callId = ref('')
const from = ref('')
const failure = ref<string | null>(null)
const unsupported = ref(false)
const canAct = computed(() => props.status === 'connected')

async function onPresence(state: 'composing' | 'paused' | 'available' | 'unavailable') {
  failure.value = null
  if (chat.value.trim() === '') {
    failure.value = t('instances.send.phoneRequired')
    return
  }
  try {
    await sendPresence(props.instanceId, { chat: chat.value.trim(), state })
    toast.add({ title: t('common.done'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onReject() {
  failure.value = null
  unsupported.value = false
  if (callId.value.trim() === '' || from.value.trim() === '') {
    failure.value = t('instances.send.phoneRequired')
    return
  }
  try {
    await rejectCall(props.instanceId, { call_id: callId.value.trim(), from: from.value.trim() })
    toast.add({ title: t('common.done'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    if (error instanceof ApiError && error.status === 501) {
      unsupported.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
    }
  }
}
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
      <UInput v-model="chat" :placeholder="t('instances.actions.chatPlaceholder')" class="w-full font-mono" />
      <div class="flex flex-wrap gap-2">
        <UButton
          :disabled="!canAct"
          variant="soft"
          :label="t('instances.actions.composing')"
          @click="onPresence('composing')"
        />
        <UButton
          :disabled="!canAct"
          variant="soft"
          :label="t('instances.actions.paused')"
          @click="onPresence('paused')"
        />
        <UButton
          :disabled="!canAct"
          variant="soft"
          :label="t('instances.actions.available')"
          @click="onPresence('available')"
        />
        <UButton
          :disabled="!canAct"
          variant="soft"
          :label="t('instances.actions.unavailable')"
          @click="onPresence('unavailable')"
        />
      </div>
      <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
        <UInput v-model="callId" :placeholder="t('instances.actions.callIdPlaceholder')" class="w-full font-mono" />
        <UInput v-model="from" :placeholder="t('instances.actions.fromPlaceholder')" class="w-full font-mono" />
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
