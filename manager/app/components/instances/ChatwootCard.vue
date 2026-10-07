<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import type { ChatwootConfig, ChatwootSetInput } from '~/types/api'

// Chatwoot connector card: config get/put, history import, operator command
// and the server-computed webhook URL. The token is write-only — it never
// appears in any response, so the form keeps a separate password field that
// starts empty and is only sent when the operator typed a value; saving
// without a token is blocked so an empty string never wipes the stored
// secret. Every field the card does not edit round-trips from the loaded
// config on save, so hidden flags are never wiped. A 400 chatwoot_disabled
// disables every form.
const props = defineProps<{
  instanceId: string
}>()

const { t } = useI18n()
const toast = useToast()
const { getChatwoot, setChatwoot, importHistory, sendCommand, isChatwootDisabled } = useInstanceChatwoot()

const config = ref<ChatwootConfig | null>(null)
const enabled = ref(false)
const url = ref('')
const accountId = ref('')
const nameInbox = ref('')
const daysLimit = ref('7')
const tokenInput = ref('')
const command = ref('')
const conversationId = ref('')
const disabled = ref(false)
const failure = ref<string | null>(null)
const imported = ref<number | null>(null)
const saving = ref(false)
const importing = ref(false)
const running = ref(false)

const canSave = computed(() => !disabled.value && !saving.value && tokenInput.value.trim() !== '')

async function load() {
  failure.value = null
  try {
    config.value = await getChatwoot(props.instanceId)
    enabled.value = config.value.is_enabled
    url.value = config.value.url ?? ''
    accountId.value = config.value.account_id ?? ''
    nameInbox.value = config.value.inbox_name
    daysLimit.value = String(config.value.import_days)
  } catch (error) {
    if (isChatwootDisabled(error)) {
      disabled.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.chatwoot.loadFailed')
    }
  }
}

async function onSave() {
  if (!config.value || tokenInput.value.trim() === '') {
    return
  }
  saving.value = true
  failure.value = null
  const previous = config.value
  // The card edits only enabled/url/account_id/inbox_name/import_days plus
  // the write-only token: every other flag round-trips from the loaded
  // config so hidden fields are preserved.
  const body: ChatwootSetInput = {
    is_enabled: enabled.value,
    url: url.value.trim(),
    account_id: accountId.value.trim(),
    token: tokenInput.value,
    inbox_name: nameInbox.value.trim(),
    is_sign_enabled: previous.is_sign_enabled,
    sign_delimiter: previous.sign_delimiter,
    is_reopen_enabled: previous.is_reopen_enabled,
    is_pending_enabled: previous.is_pending_enabled,
    is_merge_enabled: previous.is_merge_enabled,
    is_import_contacts: previous.is_import_contacts,
    is_import_messages: previous.is_import_messages,
    import_days: Number(daysLimit.value) || 0,
    is_auto_create: previous.is_auto_create,
    organization: previous.organization,
    logo: previous.logo,
    ignored_jids: previous.ignored_jids
  }
  try {
    config.value = await setChatwoot(props.instanceId, body)
    tokenInput.value = ''
    enabled.value = config.value.is_enabled
    toast.add({ title: t('instances.chatwoot.saved'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    if (isChatwootDisabled(error)) {
      disabled.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.chatwoot.saveFailed')
    }
  } finally {
    saving.value = false
  }
}

async function onImport() {
  if (importing.value) {
    return
  }
  importing.value = true
  failure.value = null
  try {
    const result = await importHistory(props.instanceId)
    imported.value = result.imported
  } catch (error) {
    if (isChatwootDisabled(error)) {
      disabled.value = true
    } else {
      // A partial import answers imported:N alongside the error; the last
      // known count stays visible so progress is never hidden.
      failure.value = error instanceof ApiError ? error.message : t('instances.chatwoot.importFailed')
    }
  } finally {
    importing.value = false
  }
}

async function onCommand() {
  if (running.value || command.value.trim() === '') {
    return
  }
  running.value = true
  failure.value = null
  try {
    await sendCommand(props.instanceId, command.value.trim(), Number(conversationId.value) || 0)
    toast.add({ title: t('instances.chatwoot.commandSent'), icon: 'i-lucide-check', color: 'success' })
  } catch (error) {
    if (isChatwootDisabled(error)) {
      disabled.value = true
    } else {
      failure.value = error instanceof ApiError ? error.message : t('instances.chatwoot.commandFailed')
    }
  } finally {
    running.value = false
  }
}

// The 400 chatwoot_disabled gate is service-wide and sticky: once known,
// skip refetches on section revisits so the console stays quiet (the info
// banner already explains the state). Switching instances still resets every
// per-instance field first: resetting only the token leaks the previous
// instance's config, imported count and failure into the new instance when
// its load() fails or is skipped.
watch(() => props.instanceId, () => {
  config.value = null
  enabled.value = false
  url.value = ''
  accountId.value = ''
  nameInbox.value = ''
  daysLimit.value = '7'
  tokenInput.value = ''
  command.value = ''
  conversationId.value = ''
  failure.value = null
  imported.value = null
  if (!disabled.value) {
    void load()
  }
}, { immediate: true })
</script>

<template>
  <UPageCard :title="t('instances.chatwoot.cardTitle')" variant="subtle" data-testid="chatwoot-card">
    <div class="flex flex-col gap-3">
      <UAlert
        v-if="disabled"
        color="info"
        variant="subtle"
        :title="t('instances.chatwoot.disabled')"
      />
      <UAlert
        v-if="failure"
        color="error"
        variant="subtle"
        :title="failure"
      />
      <UAlert
        v-if="imported !== null"
        color="success"
        variant="subtle"
        :title="t('instances.chatwoot.imported', { count: imported })"
      />
      <p v-if="config?.webhook_url" class="truncate font-mono text-xs text-muted">
        {{ t('instances.chatwoot.webhookUrl') }}: {{ config.webhook_url }}
      </p>
      <UFormField :label="t('instances.chatwoot.enabled')">
        <UCheckbox v-model="enabled" :disabled="disabled" />
      </UFormField>
      <UFormField :label="t('instances.chatwoot.url')">
        <UInput v-model="url" :disabled="disabled" class="w-full font-mono" />
      </UFormField>
      <UFormField :label="t('instances.chatwoot.accountId')">
        <UInput v-model="accountId" :disabled="disabled" class="w-full font-mono" />
      </UFormField>
      <UFormField :label="t('instances.chatwoot.token')" :hint="t('instances.chatwoot.tokenHint')">
        <UInput
          v-model="tokenInput"
          :disabled="disabled"
          class="w-full font-mono"
          type="password"
          autocomplete="new-password"
        />
      </UFormField>
      <UFormField :label="t('instances.chatwoot.nameInbox')">
        <UInput v-model="nameInbox" :disabled="disabled" class="w-full" />
      </UFormField>
      <UFormField :label="t('instances.chatwoot.daysLimit')">
        <UInput v-model="daysLimit" :disabled="disabled" class="w-full font-mono" />
      </UFormField>
      <div class="flex flex-wrap gap-2">
        <UButton
          :disabled="!canSave"
          :loading="saving"
          :label="t('common.save')"
          @click="onSave"
        />
        <UButton
          :disabled="disabled"
          variant="soft"
          :loading="importing"
          :label="importing ? t('instances.chatwoot.importing') : t('instances.chatwoot.import')"
          @click="onImport"
        />
      </div>
      <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
        <UInput
          v-model="command"
          :disabled="disabled"
          :placeholder="t('instances.chatwoot.commandPlaceholder')"
          class="w-full font-mono"
        />
        <UInput
          v-model="conversationId"
          :disabled="disabled"
          :placeholder="t('instances.chatwoot.conversationId')"
          class="w-full font-mono sm:w-40"
        />
        <UButton
          :disabled="disabled"
          class="w-fit shrink-0"
          variant="soft"
          :loading="running"
          :label="t('instances.chatwoot.run')"
          @click="onCommand"
        />
      </div>
    </div>
  </UPageCard>
</template>
