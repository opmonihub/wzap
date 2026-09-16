<script setup lang="ts">
import type { InstanceStats } from '~/types/api'

// Home stat cards: scoped totals from useOverview rendered as the template's
// HomeStats (UPageGrid + UPageCard per bucket, each linking to /instances).
// The four buckets always sum to total (useOverview normalizes the endpoint
// payload and folds unknown statuses into disconnected locally), so
// pairing/disconnected merge into one pending card.
const props = defineProps<{
  stats: InstanceStats
}>()

const { t } = useI18n()

const cards = computed(() => [
  {
    key: 'total',
    icon: 'i-lucide-smartphone',
    title: t('overview.stats.total'),
    value: props.stats.total
  },
  {
    key: 'connected',
    icon: 'i-lucide-signal',
    title: t('overview.stats.connected'),
    value: props.stats.by_status.connected ?? 0
  },
  {
    key: 'pending',
    icon: 'i-lucide-qr-code',
    title: t('overview.stats.pending'),
    value: (props.stats.by_status.pairing ?? 0) + (props.stats.by_status.disconnected ?? 0)
  },
  {
    key: 'error',
    icon: 'i-lucide-triangle-alert',
    title: t('overview.stats.error'),
    value: props.stats.by_status.error ?? 0
  }
])
</script>

<template>
  <UPageGrid class="lg:grid-cols-4 gap-4 sm:gap-6 lg:gap-px">
    <UPageCard
      v-for="card in cards"
      :key="card.key"
      :icon="card.icon"
      :title="card.title"
      to="/instances"
      variant="subtle"
      :ui="{
        container: 'gap-y-1.5',
        wrapper: 'items-start',
        leading: 'p-2.5 rounded-full bg-primary/10 ring ring-inset ring-primary/25 flex-col',
        title: 'font-normal text-muted text-xs uppercase'
      }"
      class="lg:rounded-none first:rounded-l-lg last:rounded-r-lg hover:z-1"
    >
      <span class="text-2xl font-semibold text-highlighted">
        {{ card.value }}
      </span>
    </UPageCard>
  </UPageGrid>
</template>
