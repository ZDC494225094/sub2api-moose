<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header filters -->
      <div class="flex flex-wrap items-center justify-between gap-3">
        <!-- Date range quick shortcuts -->
        <div class="flex items-center gap-2">
          <button
            v-for="s in SHORTCUTS"
            :key="s.key"
            type="button"
            class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-200 dark:border-dark-600 transition-colors"
            :class="activeShortcut === s.key
              ? 'bg-primary-600 text-white border-primary-600'
              : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="applyShortcut(s.key)"
          >
            {{ s.label }}
          </button>
          <div class="flex items-center gap-1.5">
            <input
              v-model="startDate"
              type="date"
              class="input input-sm text-xs"
              @change="onCustomRange"
            />
            <span class="text-xs text-gray-400">-</span>
            <input
              v-model="endDate"
              type="date"
              class="input input-sm text-xs"
              @change="onCustomRange"
            />
          </div>
        </div>
        <button @click="loadDashboard" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <!-- Dashboard Content -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <template v-else-if="stats">
        <OrderStatsCards :stats="stats" :start-date="startDate" :end-date="endDate" />

        <!-- Daily revenue chart -->
        <DailyRevenueChart :data="stats.daily_series || []" :loading="loading" />

        <!-- Daily new/returning users chart -->
        <DailyUserChart :data="stats.daily_series || []" :loading="loading" />

        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <!-- Payment methods: compact list -->
          <div class="card p-4">
            <h3 class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.paymentDistribution') }}</h3>
            <div v-if="!stats.payment_methods?.length" class="flex h-16 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.admin.noData') }}</div>
            <div v-else class="flex flex-wrap gap-3">
              <div
                v-for="method in stats.payment_methods"
                :key="method.type"
                class="flex items-center gap-1.5 rounded-lg bg-gray-50 dark:bg-dark-700 px-3 py-1.5"
              >
                <span :class="['inline-block h-2.5 w-2.5 rounded-full', methodColor(method.type)]"></span>
                <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('payment.methods.' + method.type, method.type) }}</span>
                <span class="text-xs text-gray-900 dark:text-white font-semibold">&yen;{{ method.amount.toFixed(2) }}</span>
                <span class="text-xs text-gray-400">({{ method.count }})</span>
              </div>
            </div>
          </div>

          <!-- Top users -->
          <div class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('payment.admin.topUsers') }}</h3>
            <div v-if="!stats.top_users?.length" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('payment.admin.noData') }}</div>
            <div v-else class="space-y-2">
              <div v-for="(user, idx) in stats.top_users" :key="user.user_id" class="flex items-center justify-between rounded-lg px-3 py-2 hover:bg-gray-50 dark:hover:bg-dark-700">
                <div class="flex items-center gap-3">
                  <span :class="['flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold', rankClass(idx)]">{{ idx + 1 }}</span>
                  <span class="text-sm text-gray-700 dark:text-gray-300">{{ user.email }}</span>
                </div>
                <span class="text-sm font-medium text-gray-900 dark:text-white">&yen;{{ user.amount.toFixed(2) }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatLocalDate } from '@/utils/localDate'
import type { DashboardStats } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatsCards from '@/components/admin/payment/OrderStatsCards.vue'
import DailyRevenueChart from '@/components/admin/payment/DailyRevenueChart.vue'
import DailyUserChart from '@/components/admin/payment/DailyUserChart.vue'

const { t } = useI18n()
const appStore = useAppStore()

type ShortcutKey = 'current-month' | 'last-month' | '7d' | '30d' | '90d'

const SHORTCUTS = computed<{ key: ShortcutKey; label: string }[]>(() => [
  { key: 'current-month', label: t('payment.admin.currentMonth') },
  { key: 'last-month', label: t('payment.admin.lastMonth') },
  { key: '7d', label: `7${t('payment.admin.daySuffix')}` },
  { key: '30d', label: `30${t('payment.admin.daySuffix')}` },
  { key: '90d', label: `90${t('payment.admin.daySuffix')}` },
])

const now = new Date()
const startDate = ref(formatLocalDate(new Date(now.getFullYear(), now.getMonth(), 1)))
const endDate = ref(formatLocalDate(now))
const activeShortcut = ref<ShortcutKey>('current-month')
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)

function applyShortcut(key: ShortcutKey) {
  activeShortcut.value = key
  const today = new Date()
  if (key === 'current-month') {
    startDate.value = formatLocalDate(new Date(today.getFullYear(), today.getMonth(), 1))
    endDate.value = formatLocalDate(today)
  } else if (key === 'last-month') {
    const first = new Date(today.getFullYear(), today.getMonth() - 1, 1)
    const last = new Date(today.getFullYear(), today.getMonth(), 0)
    startDate.value = formatLocalDate(first)
    endDate.value = formatLocalDate(last)
  } else if (key === '7d') {
    startDate.value = formatLocalDate(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 6))
    endDate.value = formatLocalDate(today)
  } else if (key === '30d') {
    startDate.value = formatLocalDate(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 29))
    endDate.value = formatLocalDate(today)
  } else if (key === '90d') {
    startDate.value = formatLocalDate(new Date(today.getFullYear(), today.getMonth(), today.getDate() - 89))
    endDate.value = formatLocalDate(today)
  }
  loadDashboard()
}

function onCustomRange() {
  activeShortcut.value = '' as ShortcutKey
  loadDashboard()
}

function methodColor(type: string): string {
  const c: Record<string, string> = {
    alipay: 'bg-blue-500', wxpay: 'bg-green-500',
    alipay_direct: 'bg-blue-400', wxpay_direct: 'bg-green-400',
    stripe: 'bg-purple-500',
  }
  return c[type] || 'bg-gray-400'
}

function rankClass(idx: number): string {
  if (idx === 0) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (idx === 1) return 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  if (idx === 2) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

async function loadDashboard() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getDashboard({
      start_date: startDate.value,
      end_date: endDate.value,
    })
    stats.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(() => loadDashboard())
</script>
