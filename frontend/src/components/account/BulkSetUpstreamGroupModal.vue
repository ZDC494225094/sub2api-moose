<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkSetUpstreamGroup.title')"
    width="normal"
    @close="handleClose"
  >
    <form id="bulk-set-upstream-group-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div class="rounded-lg bg-primary-50 p-4 dark:bg-primary-900/20">
        <p class="text-sm text-primary-800 dark:text-primary-200">
          {{ t('admin.accounts.bulkSetUpstreamGroup.selectionInfo', { count: accountIds.length }) }}
        </p>
      </div>

      <UpstreamGroupField
        v-if="!clearUpstreamGroup"
        v-model="upstreamGroup"
        :groups="upstreamGroups"
        :loading="loading"
      />

      <div v-else class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-900/20 dark:text-amber-200">
        {{ t('admin.accounts.bulkSetUpstreamGroup.clearNotice') }}
      </div>

      <label class="flex cursor-pointer items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:border-dark-600 dark:text-dark-200 dark:hover:bg-dark-800">
        <input
          id="bulk-set-upstream-group-clear"
          v-model="clearUpstreamGroup"
          type="checkbox"
          class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          data-testid="clear-upstream-group"
        />
        <span>
          <span class="block font-medium">{{ t('admin.accounts.bulkSetUpstreamGroup.clear') }}</span>
          <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.bulkSetUpstreamGroup.clearHint') }}</span>
        </span>
      </label>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="bulk-set-upstream-group-form"
          :disabled="submitting || accountIds.length === 0"
          class="btn btn-primary"
          data-testid="submit-upstream-group"
        >
          {{ submitting ? t('admin.accounts.bulkSetUpstreamGroup.submitting') : t('admin.accounts.bulkSetUpstreamGroup.submit') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAppStore } from '@/stores/app'
import type { AccountUpstreamGroup } from '@/types'

import UpstreamGroupField from './UpstreamGroupField.vue'

interface Props {
  show: boolean
  accountIds: number[]
  upstreamGroups: AccountUpstreamGroup[]
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  upstreamGroups: () => [],
  loading: false
})

const emit = defineEmits<{
  close: []
  updated: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const upstreamGroup = ref('')
const clearUpstreamGroup = ref(false)
const submitting = ref(false)

const resetForm = () => {
  upstreamGroup.value = ''
  clearUpstreamGroup.value = false
}

const handleClose = () => {
  emit('close')
}

const handleSubmit = async () => {
  if (props.accountIds.length === 0) {
    appStore.showError(t('admin.accounts.bulkEdit.noSelection'))
    return
  }

  const value = upstreamGroup.value.trim()
  if (!clearUpstreamGroup.value && !value) {
    appStore.showError(t('admin.accounts.bulkSetUpstreamGroup.required'))
    return
  }

  submitting.value = true
  try {
    const result = await adminAPI.accounts.bulkUpdate(props.accountIds, {
      upstream_group: clearUpstreamGroup.value ? '' : value
    })
    const success = result.success || 0
    const failed = result.failed || 0

    if (success > 0 && failed === 0) {
      appStore.showSuccess(t('admin.accounts.bulkEdit.success', { count: success }))
    } else if (success > 0) {
      appStore.showError(t('admin.accounts.bulkEdit.partialSuccess', { success, failed }))
    } else {
      appStore.showError(t('admin.accounts.bulkEdit.failed'))
    }

    if (success > 0) {
      emit('updated')
      handleClose()
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.bulkEdit.failed'))
    console.error('Failed to set upstream group in bulk:', error)
  } finally {
    submitting.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) resetForm()
  }
)
</script>
