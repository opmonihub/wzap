<script setup lang="ts">
import DeviceActionsCard from '~/components/instances/DeviceActionsCard.vue'
import PrivacyCard from '~/components/instances/PrivacyCard.vue'
import ProfileCard from '~/components/instances/ProfileCard.vue'
import type { InstanceStatus } from '~/types/api'

defineProps<{
  instanceId: string
  status: InstanceStatus
}>()
</script>

<template>
  <div class="flex w-full min-w-0 flex-col gap-4 sm:gap-6">
    <ProfileCard :instance-id="instanceId" :status="status" />
    <PrivacyCard :instance-id="instanceId" :status="status" />
    <DeviceActionsCard :instance-id="instanceId" :status="status" />
  </div>
</template>
