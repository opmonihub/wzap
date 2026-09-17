<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus } from '~/types/api'

const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const emit = defineEmits<{
  sent: [messageId: string]
  settled: [messageId: string]
}>()

const { t } = useI18n()
const toast = useToast()
const { sendLocation, sendContact, sendRich } = useInstanceMessaging()
const { sendText, sendMedia, getMessage } = useMessages()

const canSend = computed(() => props.status === 'connected')
const to = ref('')
const text = ref('')
const failure = ref<string | null>(null)
const sending = ref(false)

async function onSendText() {
  if (sending.value || to.value.trim() === '' || text.value.trim() === '') {
    return
  }
  sending.value = true
  failure.value = null
  try {
    const accepted = await sendText(props.instanceId, to.value.trim(), text.value)
    toast.add({ title: t('instances.send.sentToast'), icon: 'i-lucide-check', color: 'success' })
    emit('sent', accepted.message_id)
    void settle(accepted.message_id)
  } catch (error) {
    failure.value = error instanceof ApiError ? error.message : t('instances.send.failed')
  } finally {
    sending.value = false
  }
}

async function settle(messageId: string) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await new Promise(resolve => setTimeout(resolve, 2000))
    try {
      const current = await getMessage(props.instanceId, messageId)
      if (current.status === 'sent' || current.status === 'failed') {
        emit('settled', messageId)
        return
      }
    } catch {
      return
    }
  }
}

void sendLocation
void sendContact
void sendRich
void sendMedia
</script>

<template>
  <UCard variant="subtle" data-testid="message-composer">
    <div class="flex flex-col gap-3">
      <UAlert
        v-if="!canSend"
        color="warning"
        variant="subtle"
        :title="t('instances.send.notConnected')"
      />
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
      <UInput v-model="to" :placeholder="t('instances.send.phonePlaceholder')" class="w-full font-mono" />
      <UTextarea
        v-model="text"
        variant="none"
        :rows="3"
        :placeholder="t('instances.send.text')"
        class="w-full"
      />
      <div class="flex justify-end">
        <UButton
          icon="i-lucide-send"
          :disabled="!canSend"
          :loading="sending"
          :label="sending ? t('instances.send.sending') : t('instances.send.send')"
          @click="onSendText"
        />
      </div>
    </div>
  </UCard>
</template>
