<script setup lang="ts">
import type { OutboundMessage } from '~/types/api'

const props = defineProps<{
  message: OutboundMessage | null
}>()

const { t } = useI18n()

function formatDateTime(value: string | null): string {
  if (!value) {
    return t('common.notSet')
  }
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}
</script>

<template>
  <div v-if="props.message" data-testid="message-detail" class="flex flex-col gap-4 p-4 sm:px-6">
    <div class="flex items-center gap-3">
      <UAvatar :alt="props.message.recipient" />
      <div class="min-w-0 flex-1">
        <p class="truncate font-mono text-sm text-highlighted">
          {{ props.message.recipient }}
        </p>
        <p class="text-xs text-muted">
          {{ formatDateTime(props.message.created_at) }}
        </p>
      </div>
      <MessageStatusBadge :status="props.message.status" />
    </div>
    <dl class="flex flex-col gap-2 text-sm">
      <div class="flex justify-between gap-4">
        <dt class="text-muted">
          {{ t('instances.messages.whatsappId') }}
        </dt>
        <dd class="font-mono text-highlighted">
          {{ props.message.whatsapp_message_id || t('common.notSet') }}
        </dd>
      </div>
      <div v-if="props.message.last_error" class="flex justify-between gap-4">
        <dt class="text-muted">
          {{ t('instances.messages.lastError') }}
        </dt>
        <dd class="text-right text-highlighted">
          {{ props.message.last_error }}
        </dd>
      </div>
      <div class="flex justify-between gap-4">
        <dt class="text-muted">
          {{ t('instances.messages.attempts') }}
        </dt>
        <dd class="text-highlighted">
          {{ props.message.attempts }}
        </dd>
      </div>
    </dl>
  </div>
  <UEmpty v-else icon="i-lucide-inbox" :title="t('instances.messages.empty')" />
</template>
