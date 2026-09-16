<script setup lang="ts">
import type { AccountUser } from '~/types/api'

// Role badge for the accounts table. The role renders raw (admin | user),
// exactly as the card list it replaced; the role filter labels live in
// userMenu.roleAdmin / userMenu.roleUser.
defineProps<{
  user: AccountUser
}>()
</script>

<template>
  <UBadge variant="subtle">
    {{ user.role }}
  </UBadge>
</template>
