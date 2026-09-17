<script setup lang="ts">
const { t } = useI18n()

withDefaults(defineProps<{
  pending: boolean
  error: string | null
  skeletonRows?: number
}>(), {
  error: null,
  skeletonRows: 3
})

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <div v-if="pending" class="flex flex-col gap-2">
    <USkeleton v-for="n in skeletonRows" :key="n" class="h-12 w-full" />
  </div>

  <UAlert
    v-else-if="error"
    color="error"
    variant="subtle"
    :title="error"
  >
    <template #actions>
      <UButton
        color="error"
        variant="soft"
        :label="t('common.retry')"
        @click="emit('retry')"
      />
    </template>
  </UAlert>

  <slot v-else />
</template>
