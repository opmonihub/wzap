<script setup lang="ts">
import ChannelsCard from '~/components/instances/ChannelsCard.vue'
import type { InstanceStatus } from '~/types/api'

defineProps<{ instanceId: string, status: InstanceStatus }>()
</script>

<template>
  <div class="mx-auto flex w-full flex-col gap-4 sm:gap-6 lg:max-w-2xl">
    <ChannelsCard :instance-id="instanceId" :status="status" />
  </div>
</template>
