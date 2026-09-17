<script setup lang="ts">
import type { Instance } from '~/types/api'

// Props-only stats strip for the instance detail header: 4 UPageCard subtle
// cards with the HomeStats/OverviewStats :ui passthrough verbatim. Values
// come from the loaded instance plus the webhook/chatwoot state passed by the
// page — no invented counts, no fetching here. chatwootState null means the
// backend answered 400 chatwoot_disabled (global off).
const props = defineProps<{
  instance: Instance
  webhookEnabled: boolean
  webhookCount: number
  chatwootState: boolean | null
}>()

const { t } = useI18n()

const webhookTitle = computed(() => props.webhookEnabled ? t('instances.stats.enabled') : t('instances.stats.disabled'))

const chatwootTitle = computed(() => {
  if (props.chatwootState === null) {
    return t('instances.stats.globalOff')
  }
  return props.chatwootState ? t('instances.stats.enabled') : t('instances.stats.disabled')
})

function formatDateTime(value: string | null): string {
  if (!value) {
    return t('common.notSet')
  }
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString()
}
</script>

<template>
  <UPageGrid class="lg:grid-cols-4 gap-4 sm:gap-6 lg:gap-px">
    <UPageCard
      icon="i-lucide-signal"
      :title="t('instances.stats.connection')"
      variant="subtle"
      data-testid="stat-connection"
      :ui="{
        container: 'gap-y-1.5',
        wrapper: 'items-start',
        leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col',
        title: 'font-normal text-muted text-xs uppercase'
      }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <span class="block text-2xl font-semibold text-highlighted">
        {{ props.instance.status }}
      </span>
      <span class="block truncate font-mono text-xs text-muted">
        {{ props.instance.whatsapp_jid || t('common.notSet') }}
      </span>
      <span class="block text-xs text-muted">
        {{ t('instances.stats.lastConnected') }}: {{ formatDateTime(props.instance.last_connected_at) }}
      </span>
    </UPageCard>
    <UPageCard
      icon="i-lucide-fingerprint"
      :title="t('instances.stats.identity')"
      variant="subtle"
      data-testid="stat-identity"
      :ui="{
        container: 'gap-y-1.5',
        wrapper: 'items-start',
        leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col',
        title: 'font-normal text-muted text-xs uppercase'
      }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <span class="block text-2xl font-semibold text-highlighted">
        {{ props.instance.external_ref || t('common.notSet') }}
      </span>
      <span class="block text-xs text-muted">
        {{ t('instances.fields.createdAt') }}: {{ formatDateTime(props.instance.created_at) }}
      </span>
      <span class="block text-xs text-muted">
        {{ t('instances.fields.updatedAt') }}: {{ formatDateTime(props.instance.updated_at) }}
      </span>
    </UPageCard>
    <UPageCard
      icon="i-lucide-webhook"
      :title="t('instances.stats.webhook')"
      variant="subtle"
      data-testid="stat-webhook"
      :ui="{
        container: 'gap-y-1.5',
        wrapper: 'items-start',
        leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col',
        title: 'font-normal text-muted text-xs uppercase'
      }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <span class="block text-2xl font-semibold text-highlighted">
        {{ webhookTitle }}
      </span>
      <span class="block text-xs text-muted">
        {{ t('instances.stats.subscribed', { count: props.webhookCount }) }}
      </span>
    </UPageCard>
    <UPageCard
      icon="i-lucide-messages-square"
      :title="t('instances.stats.chatwoot')"
      variant="subtle"
      data-testid="stat-chatwoot"
      :ui="{
        container: 'gap-y-1.5',
        wrapper: 'items-start',
        leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col',
        title: 'font-normal text-muted text-xs uppercase'
      }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <span class="block text-2xl font-semibold text-highlighted">
        {{ props.chatwootState === null ? t('instances.stats.disabled') : chatwootTitle }}
      </span>
      <span class="block text-xs text-muted">
        {{ chatwootTitle }}
      </span>
    </UPageCard>
  </UPageGrid>
</template>
