<template>
  <div class="card" data-test="ip-sharing-alert">
    <div class="flex flex-wrap items-center gap-3 px-4 py-3 sm:px-6">
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <span
          class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.8">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m0 3h.008v.008H12v-.008zM10.34 3.94 2.7 17.06A1.5 1.5 0 0 0 4 19.5h16a1.5 1.5 0 0 0 1.3-2.44L13.66 3.94a1.5 1.5 0 0 0-2.6 0z" />
          </svg>
        </span>
        <div class="min-w-0">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.usage.ipRisk.title') }}</p>
          <p class="truncate text-xs" :class="summaryClass">{{ summaryText }}</p>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.usage.ipRisk.minAccounts') }}</label>
        <select v-model.number="minAccounts" class="input h-8 w-32 py-1 text-sm" @change="load">
          <option v-for="option in thresholdOptions" :key="option" :value="option">
            {{ t('admin.usage.ipRisk.thresholdOption', { count: option }) }}
          </option>
        </select>
        <button
          type="button"
          class="btn btn-secondary px-2 py-1"
          :disabled="loading"
          :title="t('admin.usage.ipRisk.refresh')"
          data-test="ip-sharing-refresh"
          @click="load"
        >
          <svg class="h-4 w-4" :class="loading ? 'animate-spin' : ''" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.8">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992V4.356m-1.98 5.34a8.25 8.25 0 1 0 1.94 6.072" />
          </svg>
        </button>
        <button
          v-if="items.length > 0"
          type="button"
          class="btn btn-secondary px-2 py-1 text-xs"
          data-test="ip-sharing-toggle"
          @click="expanded = !expanded"
        >
          {{ expanded ? t('admin.usage.ipRisk.collapse') : t('admin.usage.ipRisk.expand') }}
        </button>
      </div>
    </div>

    <div
      v-if="expanded && items.length > 0"
      class="border-t border-gray-100 px-4 py-3 dark:border-dark-700 sm:px-6"
      data-test="ip-sharing-list"
    >
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="text-left text-xs text-gray-500 dark:text-gray-400">
              <th class="py-2 pr-4 font-medium">{{ t('admin.usage.ipRisk.ip') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('admin.usage.ipRisk.accounts') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('admin.usage.ipRisk.requests') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('admin.usage.ipRisk.lastSeen') }}</th>
              <th class="py-2 pr-4 font-medium">{{ t('admin.usage.ipRisk.sampleUsers') }}</th>
              <th class="py-2 text-right font-medium"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="item in items" :key="item.ip_address">
              <td class="py-2 pr-4 font-mono text-gray-900 dark:text-gray-100">{{ item.ip_address }}</td>
              <td class="py-2 pr-4">
                <span class="inline-flex items-center rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-300">
                  {{ item.user_count }}
                </span>
              </td>
              <td class="py-2 pr-4 text-gray-600 dark:text-gray-300">{{ item.request_count }}</td>
              <td class="py-2 pr-4 text-gray-600 dark:text-gray-300">{{ formatTime(item.last_seen) }}</td>
              <td class="py-2 pr-4">
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="user in item.users || []"
                    :key="user.user_id"
                    class="inline-flex max-w-[220px] items-center truncate rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                    :title="user.email || `#${user.user_id}`"
                  >
                    {{ user.email || `#${user.user_id}` }}
                  </span>
                </div>
              </td>
              <td class="py-2 text-right">
                <button
                  type="button"
                  class="text-primary-600 hover:text-primary-700 dark:text-primary-400"
                  data-test="ip-sharing-view"
                  @click="emit('select-ip', item.ip_address)"
                >
                  {{ t('admin.usage.ipRisk.viewRecords') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminUsageAPI, type UsageIPSharingItem } from '@/api/admin/usage'

const props = defineProps<{
  startDate?: string
  endDate?: string
}>()

const emit = defineEmits<{
  (e: 'select-ip', ip: string): void
}>()

const { t } = useI18n()
const loading = ref(false)
const failed = ref(false)
const expanded = ref(false)
const minAccounts = ref(2)
const items = ref<UsageIPSharingItem[]>([])
const thresholdOptions = [2, 3, 5]

const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
let requestSequence = 0

const summaryText = computed(() => {
  if (loading.value) return t('admin.usage.ipRisk.loading')
  if (failed.value) return t('admin.usage.ipRisk.error')
  if (items.value.length === 0) return t('admin.usage.ipRisk.empty')
  return t('admin.usage.ipRisk.summary', { count: items.value.length })
})

const summaryClass = computed(() =>
  failed.value
    ? 'text-red-600 dark:text-red-400'
    : items.value.length > 0
      ? 'text-amber-700 dark:text-amber-300'
      : 'text-gray-500 dark:text-gray-400'
)

const formatTime = (value: string) => {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

const load = async () => {
  const sequence = ++requestSequence
  loading.value = true
  failed.value = false
  try {
    const data = await adminUsageAPI.ipSharing({
      start_date: props.startDate || undefined,
      end_date: props.endDate || undefined,
      timezone,
      min_accounts: minAccounts.value,
      limit: 50
    })
    if (sequence !== requestSequence) return
    items.value = data.items ?? []
    if (items.value.length > 0) expanded.value = true
  } catch {
    if (sequence !== requestSequence) return
    failed.value = true
    items.value = []
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

watch(
  () => [props.startDate, props.endDate],
  () => {
    expanded.value = false
    load()
  }
)

onMounted(load)
</script>
