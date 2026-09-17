<script setup lang="ts">
import type { Instance } from '~/types/api'

const props = defineProps<{
  instance: Instance
  chatwootEnabled: boolean | null
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
  <UPageGrid class="lg:grid-cols-4 gap-4 sm:gap-6 lg:gap-px">
    <UPageCard
      variant="subtle"
      data-testid="stat-connection"
      :ui="{ container: 'gap-y-1.5', wrapper: 'items-start', leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col', title: 'font-normal text-muted text-xs uppercase' }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <template #leading>
        <UIcon name="i-lucide-signal" />
      </template>
      <template #title>
        {{ t('instances.stats.connection') }}
      </template>
      <template #description>
        {{ props.instance.status }} · {{ props.instance.whatsapp_jid || t('common.notSet') }}
      </template>
    </UPageCard>
    <UPageCard
      variant="subtle"
      data-testid="stat-identity"
      :ui="{ container: 'gap-y-1.5', wrapper: 'items-start', leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col', title: 'font-normal text-muted text-xs uppercase' }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <template #leading>
        <UIcon name="i-lucide-fingerprint" />
      </template>
      <template #title>
        {{ t('instances.stats.identity') }}
      </template>
      <template #description>
        {{ formatDateTime(props.instance.created_at) }} · {{ props.instance.external_ref || t('common.notSet') }}
      </template>
    </UPageCard>
    <UPageCard
      variant="subtle"
      data-testid="stat-webhook"
      :ui="{ container: 'gap-y-1.5', wrapper: 'items-start', leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col', title: 'font-normal text-muted text-xs uppercase' }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <template #leading>
        <UIcon name="i-lucide-webhook" />
      </template>
      <template #title>
        {{ t('instances.stats.webhook') }}
      </template>
      <template #description>
        {{ t('instances.stats.subscribed', { count: props.instance.webhook_events.length }) }}
      </template>
    </UPageCard>
    <UPageCard
      variant="subtle"
      data-testid="stat-chatwoot"
      :ui="{ container: 'gap-y-1.5', wrapper: 'items-start', leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col', title: 'font-normal text-muted text-xs uppercase' }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <template #leading>
        <UIcon name="i-lucide-messages-square" />
      </template>
      <template #title>
        {{ t('instances.stats.chatwoot') }}
      </template>
      <template #description>
        {{ props.chatwootEnabled === null ? t('instances.stats.globalOff') : String(props.chatwootEnabled) }}
      </template>
    </UPageCard>
  </UPageGrid>
</template>
