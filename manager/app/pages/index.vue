<script setup lang="ts">
import { sub } from 'date-fns'
import type { OverviewPeriod, OverviewRange } from '~/components/overview/OverviewChart.client.vue'

// Home overview orchestration only: scoped stat cards from GET
// /instances/stats (local fallback over the accumulated listing), a
// created_at histogram regrouped client-side by period, and the 5 most
// recent instances. Shell (navbar + period toolbar + welcome) lives in
// OverviewHeader; snapshot rendering lives in OverviewStats/OverviewChart/
// OverviewRecent behind PageState. No invented data: when both attempts fail
// the page shows the error with retry; when stats fall back to the local
// count a discrete notice says so.
const { t } = useI18n()
const { user, scope } = useAuth()
const { stats, recent, items, pending, failure, fallback, listingFailed, refresh } = useOverview()

const range = shallowRef<OverviewRange>({
  start: sub(new Date(), { months: 3 }),
  end: new Date()
})
const period = ref<OverviewPeriod>('daily')

const periodItems = computed<{ label: string, value: OverviewPeriod }[]>(() => [
  { label: t('overview.chart.daily'), value: 'daily' },
  { label: t('overview.chart.weekly'), value: 'weekly' },
  { label: t('overview.chart.monthly'), value: 'monthly' }
])

useSeoMeta({
  title: 'Overview'
})

await refresh()
</script>

<template>
  <OverviewHeader
    v-model:period="period"
    :period-items="periodItems"
    :user-email="user?.email ?? ''"
    :scope="scope ?? ''"
  >
    <!-- Initial load renders skeletons; after the first fetch the snapshot
      stays on screen across retries (useOverview never clears stats/items
      on failure), with the alert and retry rendered inline. -->
    <PageState :pending="pending && !stats" :error="failure && !stats ? failure : null" @retry="refresh">
      <div v-if="stats" class="flex flex-col gap-4 sm:gap-6">
        <UAlert
          v-if="failure"
          color="error"
          variant="subtle"
          :title="t('overview.loadFailed')"
          :description="failure"
        >
          <template #actions>
            <UButton
              color="error"
              variant="soft"
              :label="t('common.retry')"
              @click="refresh"
            />
          </template>
        </UAlert>
        <UAlert
          v-if="fallback"
          color="warning"
          variant="subtle"
          :title="t('overview.fallbackNotice')"
        />
        <OverviewStats :stats="stats" />
        <OverviewChart :period="period" :range="range" :items="items" />
        <OverviewRecent :recent="recent" :listing-failed="listingFailed" @retry="refresh" />
      </div>
    </PageState>
  </OverviewHeader>
</template>
