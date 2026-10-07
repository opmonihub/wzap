<script setup lang="ts">
import MessageStatusBadge from '~/components/instances/MessageStatusBadge.vue'
import type { OutboundMessage } from '~/types/api'

const props = defineProps<{
  message: OutboundMessage | null
}>()

const { t } = useI18n()

function formatDateTime(value: string | undefined): string {
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
      <UAvatar :alt="props.message.recipient_jid" />
      <div class="min-w-0 flex-1">
        <p class="truncate font-mono text-sm text-highlighted">
          {{ props.message.recipient_jid }}
        </p>
        <p class="text-xs text-muted">
          {{ formatDateTime(props.message.created_at) }}
        </p>
      </div>
      <MessageStatusBadge :status="props.message.send_status" />
    </div>
    <dl class="flex flex-col gap-3 text-sm sm:gap-2">
      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <dt class="shrink-0 text-muted">
          {{ t('instances.messages.type') }}
        </dt>
        <dd class="min-w-0 font-mono break-all text-highlighted sm:text-right">
          {{ props.message.message_type }}
        </dd>
      </div>
      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <dt class="shrink-0 text-muted">
          {{ t('instances.messages.whatsappId') }}
        </dt>
        <dd class="min-w-0 font-mono break-all text-highlighted sm:text-right">
          {{ props.message.wa_id || t('common.notSet') }}
        </dd>
      </div>
      <div v-if="props.message.last_error" class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <dt class="shrink-0 text-muted">
          {{ t('instances.messages.lastError') }}
        </dt>
        <dd class="min-w-0 break-all text-highlighted sm:text-right">
          {{ props.message.last_error.message }}
        </dd>
      </div>
      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <dt class="shrink-0 text-muted">
          {{ t('instances.messages.attempts') }}
        </dt>
        <dd class="min-w-0 text-highlighted sm:text-right">
          {{ props.message.retry_count }}
        </dd>
      </div>
      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <dt class="shrink-0 text-muted">
          {{ t('instances.messages.deliveredAt') }}
        </dt>
        <dd class="min-w-0 break-all text-highlighted sm:text-right">
          {{ formatDateTime(props.message.delivered_at) }}
        </dd>
      </div>
      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-baseline sm:justify-between sm:gap-4">
        <dt class="shrink-0 text-muted">
          {{ t('instances.messages.readAt') }}
        </dt>
        <dd class="min-w-0 break-all text-highlighted sm:text-right">
          {{ formatDateTime(props.message.read_at) }}
        </dd>
      </div>
    </dl>
  </div>
  <UEmpty v-else icon="i-lucide-inbox" :title="t('instances.messages.empty')" />
</template>
