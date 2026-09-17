<script setup lang="ts">
defineProps<{
  statusText: string
  page: number
  pageSize: number
  total: number
  pageCount: number
}>()

const emit = defineEmits<{
  'update:page': [page: number]
}>()
</script>

<template>
  <div class="flex items-center justify-between gap-3 border-t border-default pt-4 mt-auto">
    <div class="text-sm text-muted">
      {{ statusText }}
    </div>

    <div class="flex items-center gap-1.5">
      <UPagination
        v-if="pageCount > 1"
        :page="page"
        :items-per-page="pageSize"
        :total="total"
        size="sm"
        @update:page="(value: number) => emit('update:page', value)"
      />
    </div>
  </div>
</template>
