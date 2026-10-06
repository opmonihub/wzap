<script setup lang="ts">
import type { TableColumn } from '@nuxt/ui'
import type { Instance } from '~/types/api'
import InstanceStatusBadge from '~/components/instances/InstanceStatusBadge.vue'

// The 5 most recent instances (useOverview.recent) next to the chart,
// mirroring the template's HomeSales mini UTable: no sorting, filtering or
// pagination of its own, name links to the detail page, status reuses the
// list badge. The remodeled public DTO hides whatsapp_jid, so the JID column
// is gone. When the listing failed but stats succeeded the parent renders
// a retry affordance instead of the empty state, so a transient listing
// error never reads as "no instances".
defineProps<{
  recent: Instance[]
  listingFailed: boolean
}>()

const emit = defineEmits<{
  retry: []
}>()

const { t } = useI18n()

const columns = computed<TableColumn<Instance>[]>(() => [
  {
    accessorKey: 'name',
    header: t('overview.recent.columns.name')
  },
  {
    id: 'status',
    accessorFn: (row: Instance) => row.connection.status,
    header: t('overview.recent.columns.status')
  },
  {
    id: 'actions',
    header: ''
  }
])

function getRowId(row: Instance): string {
  return row.id
}
</script>

<template>
  <UCard :ui="{ body: 'p-0 sm:p-0' }">
    <template #header>
      <div class="flex items-center justify-between gap-3">
        <h2 class="font-medium text-highlighted">
          {{ t('overview.recent.title') }}
        </h2>
        <UButton
          color="neutral"
          variant="ghost"
          size="sm"
          icon="i-lucide-arrow-right"
          :label="t('overview.recent.viewAll')"
          to="/instances"
        />
      </div>
    </template>

    <UAlert
      v-if="listingFailed"
      color="warning"
      variant="subtle"
      :title="t('overview.recent.loadFailed')"
      class="border-b border-default"
    >
      <template #actions>
        <UButton
          color="warning"
          variant="soft"
          :label="t('common.retry')"
          @click="emit('retry')"
        />
      </template>
    </UAlert>

    <UEmpty
      v-if="recent.length === 0 && !listingFailed"
      icon="i-lucide-smartphone"
      :title="t('overview.recent.empty')"
    >
      <template #actions>
        <UButton icon="i-lucide-plus" :label="t('instances.create.title')" to="/instances" />
      </template>
    </UEmpty>

    <UTable
      v-else-if="recent.length > 0"
      :data="recent"
      :columns="columns"
      :get-row-id="getRowId"
      class="shrink-0"
      :ui="{
        base: 'table-fixed border-separate border-spacing-0',
        thead: '[&>tr]:bg-elevated/50 [&>tr]:after:content-none',
        tbody: '[&>tr]:last:[&>td]:border-b-0',
        th: 'first:rounded-l-lg last:rounded-r-lg border-y border-default first:border-l last:border-r',
        td: 'border-b border-default'
      }"
    >
      <template #name-cell="{ row }">
        <NuxtLink
          :to="`/instances/${row.original.id}`"
          class="block min-w-0 rounded text-current no-underline hover:no-underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        >
          <InstancesTableNameCell :instance="row.original" />
        </NuxtLink>
      </template>
      <template #status-cell="{ row }">
        <InstanceStatusBadge :status="row.original.connection.status" />
      </template>
      <template #actions-cell="{ row }">
        <UButton
          :to="`/instances/${row.original.id}`"
          color="neutral"
          variant="ghost"
          size="sm"
          icon="i-lucide-arrow-right"
          :aria-label="t('overview.recent.open', { name: row.original.name })"
        />
      </template>
    </UTable>
  </UCard>
</template>
