<script setup lang="ts">
import type { AccountUser } from '~/types/api'

// Role badge with distinct treatments: admin reads as primary with a shield,
// user as neutral with a person icon. Labels come from the shared
// userMenu.roleAdmin / userMenu.roleUser strings.
defineProps<{
  user: AccountUser
}>()

const { t } = useI18n()

function roleLabel(role: AccountUser['role']): string {
  return role === 'admin' ? t('userMenu.roleAdmin') : t('userMenu.roleUser')
}
</script>

<template>
  <UBadge
    :color="user.role === 'admin' ? 'primary' : 'neutral'"
    variant="subtle"
    :icon="user.role === 'admin' ? 'i-lucide-shield-check' : 'i-lucide-user'"
  >
    {{ roleLabel(user.role) }}
  </UBadge>
</template>
