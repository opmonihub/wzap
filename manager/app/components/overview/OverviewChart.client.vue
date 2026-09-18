<script setup lang="ts">
import { eachDayOfInterval, eachMonthOfInterval, eachWeekOfInterval, format } from 'date-fns'
import { VisArea, VisAxis, VisCrosshair, VisLine, VisTooltip, VisXYContainer } from '@unovis/vue'
import type { Instance } from '~/types/api'

export type OverviewPeriod = 'daily' | 'weekly' | 'monthly'

export interface OverviewRange {
  start: Date
  end: Date
}

// Histogram of instance creation over the selected range, regrouped
// client-side from the accumulated listing (no time-series endpoint): each
// created_at falling on or before the range end lands in its day/week/month
// bucket. Buckets outside the range are never created, so counts only ever
// move between buckets when the period changes.
const props = defineProps<{
  period: OverviewPeriod
  range: OverviewRange
  items: Instance[]
}>()

const { t } = useI18n()

interface BucketPoint {
  date: Date
  count: number
}

const bucketStarts = computed<Date[]>(() => {
  if (props.range.start > props.range.end) {
    return []
  }
  const interval = { start: props.range.start, end: props.range.end }
  if (props.period === 'weekly') {
    return eachWeekOfInterval(interval)
  }
  if (props.period === 'monthly') {
    return eachMonthOfInterval(interval)
  }
  return eachDayOfInterval(interval)
})

const data = computed<BucketPoint[]>(() => {
  const starts = bucketStarts.value
  const counts = starts.map(() => 0)
  for (const item of props.items) {
    const created = new Date(item.created_at)
    if (Number.isNaN(created.getTime()) || created > props.range.end) {
      continue
    }
    let bucket = -1
    for (let index = starts.length - 1; index >= 0; index--) {
      const start = starts[index]
      if (start !== undefined && start <= created) {
        bucket = index
        break
      }
    }
    if (bucket >= 0) {
      counts[bucket] = (counts[bucket] ?? 0) + 1
    }
  }
  return starts.map((date, index) => ({ date, count: counts[index] ?? 0 }))
})

const x = (_: BucketPoint, i: number) => i
const y = (point: BucketPoint) => point.count

const total = computed(() => data.value.reduce((sum, point) => sum + point.count, 0))

function formatBucket(date: Date): string {
  if (props.period === 'monthly') {
    return format(date, 'MMM yyyy')
  }
  return format(date, 'd MMM')
}

function xTicks(index: number): string {
  const point = data.value[index]
  if (index === 0 || index === data.value.length - 1 || !point) {
    return ''
  }
  return formatBucket(point.date)
}

function tooltip(point: BucketPoint): string {
  return `${formatBucket(point.date)}: ${point.count}`
}
</script>

<template>
  <UCard :ui="{ root: 'overflow-visible', body: 'px-0! pt-0! pb-3!' }">
    <template #header>
      <div>
        <p class="text-xs text-muted uppercase mb-1.5">
          {{ t('overview.chart.title') }}
        </p>
        <p class="text-3xl text-highlighted font-semibold">
          {{ total }}
        </p>
        <p class="mt-1 text-xs text-muted">
          {{ t('overview.chart.rangeNote') }}
        </p>
      </div>
    </template>

    <VisXYContainer
      :data="data"
      :padding="{ top: 40 }"
      :margin="{ left: -5, right: -5 }"
      class="h-96"
    >
      <VisLine
        :x="x"
        :y="y"
        color="var(--ui-primary)"
      />
      <VisArea
        :x="x"
        :y="y"
        color="var(--ui-primary)"
        :opacity="0.1"
      />

      <VisAxis
        type="x"
        :x="x"
        :tick-format="xTicks"
      />

      <VisCrosshair
        :x="x"
        :y="y"
        color="var(--ui-primary)"
        :template="tooltip"
      />

      <VisTooltip />
    </VisXYContainer>
  </UCard>
</template>

<style scoped>
.unovis-xy-container {
  --vis-crosshair-line-stroke-color: var(--ui-primary);
  --vis-crosshair-circle-stroke-color: var(--ui-bg);

  --vis-axis-grid-color: var(--ui-border);
  --vis-axis-tick-color: var(--ui-border);
  --vis-axis-tick-label-color: var(--ui-text-dimmed);

  --vis-tooltip-background-color: var(--ui-bg);
  --vis-tooltip-border-color: var(--ui-border);
  --vis-tooltip-text-color: var(--ui-text-highlighted);
}
</style>
