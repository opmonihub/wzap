<script setup lang="ts">
const { isNotificationsSlideoverOpen } = useDashboard()
const { t } = useI18n()
</script>

<template>
  <USlideover v-model:open="isNotificationsSlideoverOpen" :title="t('nav.notifications')">
    <template #body>
      <p class="text-sm text-muted">
        {{ t('nav.noNotifications') }}
      </p>
    </template>
  </USlideover>
</template>
