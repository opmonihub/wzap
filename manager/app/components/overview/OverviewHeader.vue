<script setup lang="ts">
import type { OverviewPeriod } from '~/components/overview/OverviewChart.client.vue'

// Home overview shell: dashboard navbar (sidebar collapse, notifications
// slideover shortcut, create shortcut to /instances), the chart period
// toolbar and the vision-scope line. Owns the UDashboardPanel slots so the
// page stays orchestration-only; the snapshot body (PageState + stats/chart/
// recent) renders through the default slot. Notifications state lives here
// via useDashboard (no prop drilling); the create button navigates to
// /instances directly (no emit). The scope line is the only global-vs-account
// vision indicator on the home screen — users without it cannot tell a scoped
// listing from missing instances.
defineProps<{
  scope: string | null
  periodItems: { label: string, value: OverviewPeriod }[]
}>()

const period = defineModel<OverviewPeriod>('period', { required: true })

const { t } = useI18n()
const { isNotificationsSlideoverOpen } = useDashboard()
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
      <p v-if="scope" class="text-sm text-muted">
        {{ scope === 'global' ? t('overview.scopeGlobal') : t('overview.scopeInstance') }}
      </p>

      <slot />
    </template>
  </UDashboardPanel>
</template>
