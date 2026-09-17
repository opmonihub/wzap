<script setup lang="ts">
import MessageActionsCard from '~/components/instances/MessageActionsCard.vue'
import MessageComposer from '~/components/instances/MessageComposer.vue'
import MessagesCard from '~/components/instances/MessagesCard.vue'
import TestSendCard from '~/components/instances/TestSendCard.vue'
import type { InstanceStatus } from '~/types/api'

defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const { t } = useI18n()
const route = useRoute()

const messagesRefresh = ref(0)

// Message history split (template inbox pattern): at lg+ it renders as a
// side panel, below lg it opens as a slideover via the messages button.
// Visibility is CSS-gated (hidden/lg: wrappers), which does not prevent
// mount: the desktop aside mounts and fetches once per page load on every
// viewport, while the slideover content mounts lazily on first open (its
// Presence unmounts on close). Opening the slideover on mobile therefore
// issues one redundant history fetch; accepted (no v-if gating, which would
// reintroduce the SSR-breakpoint double-mount, and no lifted fetch, which
// would change child-card contracts).
const isMessagesOpen = ref(false)

// The slideover closes on navigation, mirroring the dashboard slideover.
watch(() => route.fullPath, () => {
  isMessagesOpen.value = false
})

// A test send emits on accept and again when the message settles; either
// refresh bumps the message history so the new row shows up.
function onMessagesRefresh() {
  messagesRefresh.value += 1
}
</script>

<template>
  <div class="flex w-full flex-col gap-4">
    <div class="flex w-full flex-col gap-4 lg:flex-row lg:items-start lg:gap-6">
      <div class="flex min-w-0 flex-1 flex-col gap-4 lg:max-w-2xl">
        <TestSendCard
          :instance-id="instanceId"
          :status="status"
          @sent="onMessagesRefresh"
          @settled="onMessagesRefresh"
        />

        <MessageComposer
          :instance-id="instanceId"
          :status="status"
          @sent="onMessagesRefresh"
          @settled="onMessagesRefresh"
        />

        <MessageActionsCard :instance-id="instanceId" :status="status" />

        <UButton
          class="lg:hidden"
          icon="i-lucide-message-square-text"
          :label="t('instances.messages.cardTitle')"
          @click="isMessagesOpen = true"
        />
      </div>

      <aside class="hidden min-w-0 flex-1 lg:block lg:max-w-md lg:shrink-0">
        <div class="lg:sticky lg:top-4">
          <ClientOnly>
            <MessagesCard :instance-id="instanceId" :refresh-key="messagesRefresh" />
          </ClientOnly>
        </div>
      </aside>
    </div>

    <div class="flex w-full flex-col gap-4 lg:hidden">
      <ClientOnly>
        <USlideover v-model:open="isMessagesOpen" :title="t('instances.messages.cardTitle')">
          <template #content>
            <MessagesCard :instance-id="instanceId" :refresh-key="messagesRefresh" />
          </template>
        </USlideover>
      </ClientOnly>
    </div>
  </div>
</template>
