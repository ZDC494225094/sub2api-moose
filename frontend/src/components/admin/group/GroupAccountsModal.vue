<template>
  <ExtensionGroupAccountsModal
    v-bind="$props"
    :enabled="enabled"
    :list-accounts="listAccounts"
    :update-account="updateAccount"
    @close="emit('close')"
    @success="emit('success')"
  />
</template>
<script setup lang="ts">
import { adminAPI } from '@/api/admin'
import { useAdminEfficiency } from '@/extensions/useAdminEfficiency'
import ExtensionGroupAccountsModal from '@/extensions/modules/admin-efficiency/GroupAccountsModal.vue'
import type { AdminGroup } from '@/types'
defineProps<{ show: boolean; group: AdminGroup | null }>()
const emit = defineEmits<{ close: []; success: [] }>()
const enabled = useAdminEfficiency()
const listAccounts = adminAPI.accounts.list
const updateAccount = adminAPI.accounts.update
</script>
