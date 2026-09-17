<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const { revokeMessage, markRead, sendPresence } = useInstanceMessaging()

const chat = ref('')
const messageId = ref('')
const failure = ref<string | null>(null)
const done = ref<string | null>(null)
const canAct = computed(() => props.status === 'connected')

async function onRevoke() {
  failure.value = null
  done.value = null
  try {
    await revokeMessage(props.instanceId, { chat: chat.value.trim(), message_id: messageId.value.trim() })
    done.value = t('common.done')
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onMarkRead() {
  failure.value = null
  done.value = null
  try {
    await markRead(props.instanceId, { chat: chat.value.trim(), message_id: messageId.value.trim() })
    done.value = t('common.done')
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}

async function onPresence(state: 'composing' | 'paused' | 'available' | 'unavailable') {
  failure.value = null
  done.value = null
  try {
    await sendPresence(props.instanceId, { chat: chat.value.trim(), state })
    done.value = t('common.done')
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  }
}
</script>

<template>
  <UCard variant="subtle" data-testid="message-actions-card">
    <template #header>
      <h3 class="font-medium text-highlighted">
        {{ t('instances.actions.presence') }}
      </h3>
    </template>
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
        v-if="done"
        color="success"
        variant="subtle"
        :title="done"
      />
      <UInput v-model="chat" placeholder="chat" class="w-full font-mono" />
      <UInput v-model="messageId" placeholder="message_id" class="w-full font-mono" />
      <div class="flex flex-wrap gap-2">
        <UButton :disabled="!canAct" :label="t('instances.actions.revoke')" @click="onRevoke" />
        <UButton
          :disabled="!canAct"
          variant="soft"
          :label="t('instances.actions.markRead')"
          @click="onMarkRead"
        />
        <UButton
          :disabled="!canAct"
          variant="soft"
          label="composing"
          @click="onPresence('composing')"
        />
        <UButton
          :disabled="!canAct"
          variant="soft"
          label="paused"
          @click="onPresence('paused')"
        />
      </div>
    </div>
  </UCard>
</template>
