<script setup lang="ts">
const { t } = useI18n()
</script>

<template>
  <UCard class="shrink-0" :ui="{ body: 'px-0! pt-0! pb-3!' }">
    <template #header>
      <div>
        <p class="text-xs text-muted uppercase mb-1.5">
          {{ t('overview.chart.title') }}
        </p>
        <p class="text-3xl text-highlighted font-semibold">
          ---
        </p>
      </div>
    </template>

    <USkeleton class="h-96 w-full" />
  </UCard>
</template>
