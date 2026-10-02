<template>
  <ExtensionBulkSetUpstreamGroupModal
    v-bind="$props"
    :show="show && enabled"
    :update-accounts="updateAccounts"
    @close="emit('close')"
    @updated="emit('updated')"
  />
</template>

<script setup lang="ts">
import { useAdminEfficiency } from '@/extensions/useAdminEfficiency'
const enabled = useAdminEfficiency()
import { adminAPI } from '@/api/admin'
import ExtensionBulkSetUpstreamGroupModal from '@/extensions/modules/admin-efficiency/BulkSetUpstreamGroupModal.vue'
import type { AccountUpstreamGroup, BulkUpstreamGroupUpdater } from '@/extensions/modules/admin-efficiency/types'

defineProps<{
  show: boolean
  accountIds: number[]
  upstreamGroups: AccountUpstreamGroup[]
  loading?: boolean
}>()
const emit = defineEmits<{ close: []; updated: [] }>()
// Generic account writes remain host-owned; the extension receives only this narrow port.
const updateAccounts: BulkUpstreamGroupUpdater = (ids, update) => {
  if (!enabled.value) return Promise.reject(new Error('管理效率插件已关闭'))
  return adminAPI.accounts.bulkUpdate(ids, update)
}
</script>
