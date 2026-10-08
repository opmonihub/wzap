<script setup lang="ts">
import { useObjectUrl } from '@vueuse/core'

const props = defineProps<{
  file: File
}>()
const appConfig = useAppConfig()
const previewUrl = useObjectUrl(() => props.file)
</script>

<template>
  <!-- Grid previews hide the filename, so the image identifies the selected file. -->
  <UAvatar
    :as="{ img: 'img' }"
    :src="previewUrl"
    :alt="file.name"
    :icon="appConfig.ui.icons.file"
    data-slot="fileLeadingAvatar"
  />
</template>
