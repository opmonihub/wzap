<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { InstanceStatus, MediaKind } from '~/types/api'

// Footer reply card for the messages split: shared recipient plus sub-tabs
// for text/media/location/contact/poll/reaction/list/buttons. Every send mints
// a fresh Idempotency-Key per click inside the composables; 422 answers
// surface the server text verbatim. Sends are gated on a connected instance
// and each accepted message is polled until sent/failed (settled).
const props = defineProps<{
  instanceId: string
  status: InstanceStatus
}>()

const emit = defineEmits<{
  sent: [messageId: string]
  settled: [messageId: string]
}>()

const { t } = useI18n()
const toast = useToast()
const { sendLocation, sendContact, sendRich } = useInstanceMessaging()
const { sendText, sendMedia, getMessage } = useMessages()

const canSend = computed(() => props.status === 'connected')
const to = ref('')
const failure = ref<string | null>(null)
const sending = ref(false)

type ComposerTab = 'text' | 'media' | 'location' | 'contact' | 'poll' | 'reaction' | 'list' | 'buttons'
const tab = ref<ComposerTab>('text')
const tabs = computed(() => [
  { label: t('instances.send.tabText'), value: 'text' },
  { label: t('instances.send.tabMedia'), value: 'media' },
  { label: t('instances.send.tabLocation'), value: 'location' },
  { label: t('instances.send.tabContact'), value: 'contact' },
  { label: t('instances.send.tabPoll'), value: 'poll' },
  { label: t('instances.send.tabReaction'), value: 'reaction' },
  { label: t('instances.send.tabList'), value: 'list' },
  { label: t('instances.send.tabButtons'), value: 'buttons' }
])

const text = ref('')
const mediaFile = ref<File | null>(null)
const mediaCaption = ref('')
const latitude = ref('')
const longitude = ref('')
const displayName = ref('')
const vcard = ref('')
const question = ref('')
const options = ref('')
const selectableCount = ref('1')
const reactionTarget = ref('')
const emoji = ref('')
const listTitle = ref('')
const listDescription = ref('')
const listButton = ref('')
const listSection = ref('')
const listRows = ref('')
const listFooter = ref('')
const buttonsText = ref('')
const buttonsRows = ref('')
const buttonsFooter = ref('')

function recipient(): string | null {
  const value = to.value.trim()
  if (value === '') {
    failure.value = t('instances.send.phoneRequired')
    return null
  }
  return value
}

function friendlySendError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) {
      return t('instances.send.notConnected')
    }
    return error.message
  }
  return t('instances.send.failed')
}

async function accepted(messageId: string) {
  toast.add({ title: t('instances.send.sentToast'), icon: 'i-lucide-check', color: 'success' })
  emit('sent', messageId)
  await settle(messageId)
}

async function settle(messageId: string) {
  for (let attempt = 0; attempt < 30; attempt += 1) {
    await new Promise(resolve => setTimeout(resolve, 2000))
    try {
      const current = await getMessage(props.instanceId, messageId)
      if (current.status === 'sent' || current.status === 'failed') {
        emit('settled', messageId)
        return
      }
    } catch {
      return
    }
  }
}

async function onSend() {
  if (sending.value) {
    return
  }
  const target = recipient()
  if (!target) {
    return
  }
  sending.value = true
  failure.value = null
  try {
    switch (tab.value) {
      case 'text': {
        if (text.value.trim() === '') {
          failure.value = t('instances.send.textRequired')
          return
        }
        await accepted((await sendText(props.instanceId, target, text.value)).message_id)
        break
      }
      case 'media': {
        if (!mediaFile.value) {
          failure.value = t('instances.send.fileRequired')
          return
        }
        const kind = kindForFile(mediaFile.value)
        if (!kind) {
          failure.value = t('instances.send.unsupportedFile')
          return
        }
        await accepted((await sendMedia(props.instanceId, {
          to: target,
          type: kind,
          caption: mediaCaption.value,
          file: mediaFile.value
        })).message_id)
        break
      }
      case 'location': {
        const lat = Number(latitude.value)
        const lng = Number(longitude.value)
        if (!Number.isFinite(lat) || !Number.isFinite(lng) || lat < -90 || lat > 90 || lng < -180 || lng > 180) {
          failure.value = t('instances.send.invalidLocation')
          return
        }
        await accepted((await sendLocation(props.instanceId, { to: target, latitude: lat, longitude: lng })).message_id)
        break
      }
      case 'contact': {
        if (displayName.value.trim() === '' || vcard.value.trim() === '') {
          failure.value = t('instances.send.contactRequired')
          return
        }
        await accepted((await sendContact(props.instanceId, {
          to: target,
          display_name: displayName.value.trim(),
          vcard: vcard.value
        })).message_id)
        break
      }
      case 'poll': {
        const choices = options.value.split('\n').map(line => line.trim()).filter(line => line !== '')
        if (question.value.trim() === '' || choices.length < 2) {
          failure.value = t('instances.send.pollRequired')
          return
        }
        await accepted((await sendRich(props.instanceId, {
          type: 'poll',
          to: target,
          question: question.value.trim(),
          options: choices,
          selectable_count: Math.max(1, Number(selectableCount.value) || 1)
        })).message_id)
        break
      }
      case 'reaction': {
        if (reactionTarget.value.trim() === '') {
          failure.value = t('instances.send.reactionRequired')
          return
        }
        // An empty emoji removes the reaction on the target message.
        await accepted((await sendRich(props.instanceId, {
          type: 'reaction',
          to: target,
          target: reactionTarget.value.trim(),
          emoji: emoji.value
        })).message_id)
        break
      }
      case 'list': {
        const rows = parseRows(listRows.value)
        if (listTitle.value.trim() === '' || listButton.value.trim() === '' || rows.length === 0) {
          failure.value = t('instances.send.listRequired')
          return
        }
        await accepted((await sendRich(props.instanceId, {
          type: 'list',
          to: target,
          title: listTitle.value.trim(),
          description: listDescription.value.trim(),
          button_text: listButton.value.trim(),
          sections: [{ title: listSection.value.trim() || listTitle.value.trim(), rows }],
          footer: listFooter.value.trim()
        })).message_id)
        break
      }
      case 'buttons': {
        const rows = parseRows(buttonsRows.value)
        if (buttonsText.value.trim() === '' || rows.length === 0) {
          failure.value = t('instances.send.buttonsRequired')
          return
        }
        await accepted((await sendRich(props.instanceId, {
          type: 'buttons',
          to: target,
          text: buttonsText.value.trim(),
          buttons: rows.map(row => ({ id: row.id, title: row.title })),
          footer: buttonsFooter.value.trim()
        })).message_id)
        break
      }
    }
  } catch (error) {
    failure.value = friendlySendError(error)
  } finally {
    sending.value = false
  }
}

// Rows are typed one per line as "id | title" for list sections and buttons.
function parseRows(value: string): Array<{ id: string, title: string }> {
  return value.split('\n')
    .map(line => line.split('|').map(part => part.trim()))
    .filter(parts => (parts[0] ?? '') !== '' && (parts[1] ?? '') !== '')
    .map(parts => ({ id: parts[0] as string, title: parts[1] as string }))
}

// Same mapping as media.Kind on the Go side; unknown MIME would answer 422
// so it is rejected before anything is sent.
function kindForFile(file: File): MediaKind | null {
  const mime = file.type.toLowerCase().trim()
  if (mime.startsWith('image/')) {
    return 'image'
  }
  if (mime.startsWith('video/')) {
    return 'video'
  }
  if (mime.startsWith('audio/')) {
    return 'audio'
  }
  if (mime === 'application/pdf' || mime === 'text/plain' || mime.startsWith('application/vnd.') || mime === 'application/msword') {
    return 'document'
  }
  return null
}

watch(() => props.instanceId, () => {
  failure.value = null
})
</script>

<template>
  <UCard variant="subtle" data-testid="message-composer">
    <div class="flex flex-col gap-3">
      <UAlert
        v-if="!canSend"
        color="warning"
        variant="subtle"
        :title="t('instances.send.notConnected')"
      />
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
      <UInput v-model="to" :placeholder="t('instances.send.phonePlaceholder')" class="w-full font-mono" />
      <UTabs v-model="tab" :items="tabs" size="xs" />
      <UTextarea
        v-if="tab === 'text'"
        v-model="text"
        variant="none"
        :rows="3"
        :placeholder="t('instances.send.text')"
        class="w-full"
      />
      <div v-else-if="tab === 'media'" class="flex flex-col gap-2">
        <UFileUpload v-model="mediaFile" accept="image/*,video/*,audio/*,.pdf,.txt,.doc,.docx" variant="area" />
        <UInput v-model="mediaCaption" :placeholder="t('instances.send.caption')" class="w-full" />
      </div>
      <div v-else-if="tab === 'location'" class="flex flex-col gap-2 sm:flex-row">
        <UInput v-model="latitude" :placeholder="t('instances.send.latitude')" class="w-full font-mono" />
        <UInput v-model="longitude" :placeholder="t('instances.send.longitude')" class="w-full font-mono" />
      </div>
      <div v-else-if="tab === 'contact'" class="flex flex-col gap-2">
        <UInput v-model="displayName" :placeholder="t('instances.send.displayName')" class="w-full" />
        <UTextarea v-model="vcard" :rows="4" :placeholder="t('instances.send.vcard')" class="w-full font-mono" />
      </div>
      <div v-else-if="tab === 'poll'" class="flex flex-col gap-2">
        <UInput v-model="question" :placeholder="t('instances.send.question')" class="w-full" />
        <UTextarea v-model="options" :rows="3" :placeholder="t('instances.send.options')" :hint="t('instances.send.optionsHint')" class="w-full" />
        <UInput v-model="selectableCount" :placeholder="t('instances.send.selectableCount')" class="w-full font-mono" />
      </div>
      <div v-else-if="tab === 'reaction'" class="flex flex-col gap-2 sm:flex-row">
        <UInput v-model="reactionTarget" :placeholder="t('instances.send.targetMessage')" class="w-full font-mono" />
        <UInput v-model="emoji" :placeholder="t('instances.send.emoji')" :hint="t('instances.send.emojiHint')" class="w-full" />
      </div>
      <div v-else-if="tab === 'list'" class="flex flex-col gap-2">
        <UInput v-model="listTitle" :placeholder="t('instances.send.itemTitle')" class="w-full" />
        <UInput v-model="listDescription" :placeholder="t('instances.send.itemDescription')" class="w-full" />
        <UInput v-model="listButton" :placeholder="t('instances.send.buttonLabel')" class="w-full" />
        <UInput v-model="listSection" :placeholder="t('instances.send.sectionTitle')" class="w-full" />
        <UTextarea v-model="listRows" :rows="3" :placeholder="t('instances.send.options')" :hint="t('instances.send.sectionsHint')" class="w-full font-mono" />
        <UInput v-model="listFooter" :placeholder="t('instances.send.footer')" class="w-full" />
      </div>
      <div v-else class="flex flex-col gap-2">
        <UTextarea v-model="buttonsText" :rows="2" :placeholder="t('instances.send.text')" class="w-full" />
        <UTextarea v-model="buttonsRows" :rows="3" :placeholder="t('instances.send.options')" :hint="t('instances.send.sectionsHint')" class="w-full font-mono" />
        <UInput v-model="buttonsFooter" :placeholder="t('instances.send.footer')" class="w-full" />
      </div>
      <div class="flex justify-end">
        <UButton
          icon="i-lucide-send"
          :disabled="!canSend"
          :loading="sending"
          :label="sending ? t('instances.send.sending') : t('instances.send.send')"
          @click="onSend"
        />
      </div>
    </div>
  </UCard>
</template>
