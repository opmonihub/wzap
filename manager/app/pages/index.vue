<script setup lang="ts">
import { sub } from 'date-fns'
import type { OverviewPeriod, OverviewRange } from '~/components/overview/OverviewChart.client.vue'

// Home overview: scoped stat cards from GET /instances/stats (local fallback
// over the accumulated listing), a created_at histogram regrouped
// client-side by period, and the 5 most recent instances. No invented data:
// when both attempts fail the page shows the error with retry; when stats
// fall back to the local count a discrete notice says so.
const { t } = useI18n()
const { user, scope } = useAuth()
const { isNotificationsSlideoverOpen } = useDashboard()
const { stats, recent, items, pending, failure, fallback, listingFailed, refresh } = useOverview()

const range = shallowRef<OverviewRange>({
  start: sub(new Date(), { months: 3 }),
  end: new Date()
})
const period = ref<OverviewPeriod>('daily')

const periodItems = computed(() => [
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
  <UDashboardPanel id="overview">
    <template #header>
      <UDashboardNavbar :title="t('overview.title')" :ui="{ right: 'gap-3' }">
        <template #leading>
          <UDashboardSidebarCollapse />
        </template>

        <template #right>
          <UTooltip :text="t('nav.notifications')" :shortcuts="['N']">
            <UButton
              color="neutral"
              variant="ghost"
              square
              :aria-label="t('nav.notifications')"
              @click="isNotificationsSlideoverOpen = true"
            >
              <UChip color="error" inset>
                <UIcon name="i-lucide-bell" class="size-5 shrink-0" />
              </UChip>
            </UButton>
          </UTooltip>

          <UButton
            icon="i-lucide-plus"
            size="md"
            class="rounded-full"
            :aria-label="t('instances.create.title')"
            to="/instances"
          />
        </template>
      </UDashboardNavbar>

      <UDashboardToolbar>
        <template #left>
          <USelect
            v-model="period"
            :items="periodItems"
            variant="ghost"
            class="data-[state=open]:bg-elevated -ms-1"
            :aria-label="t('overview.chart.periodLabel')"
            :ui="{ value: 'capitalize', itemLabel: 'capitalize', trailingIcon: 'group-data-[state=open]:rotate-180 transition-transform duration-200' }"
          />
        </template>
      </UDashboardToolbar>
    </template>

    <template #body>
      <p class="text-sm text-muted">
        {{ t('overview.welcome', { email: user?.email ?? '' }) }}
        {{ scope === 'global' ? t('overview.scopeGlobal') : t('overview.scopeInstance') }}
      </p>

      <!-- Initial load renders skeletons; after the first fetch the snapshot
        stays on screen across retries (useOverview never clears stats/items
        on failure), with the alert and retry rendered inline. -->
      <div v-if="pending && !stats" class="flex flex-col gap-4">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4 sm:gap-6">
          <USkeleton class="h-28 w-full" />
          <USkeleton class="h-28 w-full" />
          <USkeleton class="h-28 w-full" />
          <USkeleton class="h-28 w-full" />
        </div>
        <USkeleton class="h-96 w-full" />
      </div>

      <UAlert
        v-else-if="failure && !stats"
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

      <div v-else-if="stats" class="flex flex-col gap-4 sm:gap-6">
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
    </template>
  </UDashboardPanel>
</template>
