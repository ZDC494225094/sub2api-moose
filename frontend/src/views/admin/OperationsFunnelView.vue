<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-for="shortcut in shortcuts"
            :key="shortcut.key"
            type="button"
            class="rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors"
            :class="activeShortcut === shortcut.key
              ? 'border-primary-600 bg-primary-600 text-white'
              : 'border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="applyShortcut(shortcut.key)"
          >
            {{ shortcut.label }}
          </button>
          <div class="flex items-center gap-1.5">
            <input v-model="startDate" type="date" class="input input-sm text-xs" @change="onCustomRange" />
            <span class="text-xs text-gray-400">-</span>
            <input v-model="endDate" type="date" class="input input-sm text-xs" @change="onCustomRange" />
          </div>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadFunnel">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div v-if="loading && !funnel" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="funnel">
        <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <div v-for="metric in summaryMetrics" :key="metric.key" class="card p-4">
            <div class="flex items-center gap-3">
              <div :class="['rounded-lg p-2', metric.iconBg]">
                <Icon :name="metric.icon" size="md" :class="metric.iconColor" :stroke-width="2" />
              </div>
              <div class="min-w-0">
                <p class="truncate text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</p>
                <p class="text-xl font-bold text-gray-900 dark:text-white">{{ metric.value }}</p>
                <p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ metric.hint }}</p>
              </div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,0.85fr)]">
          <section class="card p-4">
            <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.funnelTitle') }}</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.operations.funnelDescription') }}</p>
              </div>
              <div class="text-right text-xs text-gray-500 dark:text-gray-400">
                <div>{{ t('admin.operations.range') }}: {{ funnel.start_date }} - {{ funnel.end_date }}</div>
                <div>{{ t('admin.operations.generatedAt') }}: {{ formatDateTime(funnel.generated_at) }}</div>
              </div>
            </div>

            <div class="space-y-3">
              <div v-for="step in localizedSteps" :key="step.key" class="rounded-lg border border-gray-100 p-3 dark:border-dark-700">
                <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div class="flex items-center gap-2">
                    <span :class="['flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold', step.badgeClass]">
                      {{ step.index + 1 }}
                    </span>
                    <div>
                      <div class="text-sm font-semibold text-gray-900 dark:text-white">{{ step.label }}</div>
                      <div class="text-xs text-gray-500 dark:text-gray-400">
                        {{ t('admin.operations.conversionFromPrevious') }} {{ formatPercent(step.conversion_rate) }}
                      </div>
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="text-lg font-bold text-gray-900 dark:text-white">{{ formatNumber(step.count) }}</div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('admin.operations.overallConversion') }} {{ formatPercent(step.overall_conversion_rate) }}
                    </div>
                  </div>
                </div>
                <div class="h-2 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-primary-500 transition-all" :style="{ width: step.barWidth }"></div>
                </div>
                <div v-if="step.index > 0" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.operations.dropoff') }}: {{ formatNumber(step.dropoff_from_previous) }}
                </div>
              </div>
            </div>
          </section>

          <div class="space-y-6">
            <section class="card p-4">
              <div class="mb-4 flex items-center justify-between gap-3">
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.revenueTitle') }}</h2>
                <RouterLink :to="ordersLink('COMPLETED', 'paid_at')" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400">
                  {{ t('admin.operations.viewOrders') }}
                </RouterLink>
              </div>
              <div class="grid grid-cols-2 gap-3">
                <div v-for="item in revenueItems" :key="item.key" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
                  <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
                  <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ item.value }}</div>
                </div>
              </div>
            </section>

            <section class="card p-4">
              <h2 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.operations.signalsTitle') }}</h2>
              <div class="space-y-2">
                <RouterLink
                  v-for="signal in signalItems"
                  :key="signal.key"
                  :to="ordersLink(signal.status)"
                  class="flex items-center justify-between rounded-lg px-3 py-2 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700"
                >
                  <div class="flex items-center gap-2">
                    <span :class="['inline-block h-2.5 w-2.5 rounded-full', signal.dotClass]"></span>
                    <span class="text-sm text-gray-700 dark:text-gray-300">{{ signal.label }}</span>
                  </div>
                  <span class="text-sm font-semibold text-gray-900 dark:text-white">{{ formatNumber(signal.value) }}</span>
                </RouterLink>
              </div>
            </section>
          </div>
        </div>
      </template>

      <div v-else class="card flex items-center justify-center py-12 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.operations.noData') }}
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { OperationsFunnelResponse, OperationsFunnelStep } from '@/api/admin/dashboard'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'

type ShortcutKey = '7d' | '30d' | '90d' | 'custom'
type DateField = 'created_at' | 'paid_at'
type IconName = 'userPlus' | 'chart' | 'creditCard' | 'dollar'

const { t } = useI18n()
const appStore = useAppStore()

function fmtDate(date: Date): string {
  return date.toISOString().slice(0, 10)
}

function daysAgo(days: number): string {
  const today = new Date()
  return fmtDate(new Date(today.getFullYear(), today.getMonth(), today.getDate() - days + 1))
}

const startDate = ref(daysAgo(30))
const endDate = ref(fmtDate(new Date()))
const activeShortcut = ref<ShortcutKey>('30d')
const loading = ref(false)
const funnel = ref<OperationsFunnelResponse | null>(null)

const shortcuts = computed(() => [
  { key: '7d' as ShortcutKey, label: t('dates.last7Days') },
  { key: '30d' as ShortcutKey, label: t('dates.last30Days') },
  { key: '90d' as ShortcutKey, label: t('admin.operations.rangeDays', { days: 90 }) },
])

const stepLabels: Record<string, string> = {
  registered: 'admin.operations.registeredUsers',
  created_key: 'admin.operations.createdKeyUsers',
  active: 'admin.operations.activeUsers',
  paying: 'admin.operations.payingUsers',
}

const stepBadges = [
  'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
  'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
  'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
]

const localizedSteps = computed(() => {
  const steps = funnel.value?.steps ?? []
  const maxCount = Math.max(...steps.map((step) => step.count), 1)
  return steps.map((step, index) => ({
    ...step,
    index,
    label: t(stepLabels[step.key] || step.label),
    badgeClass: stepBadges[index] || stepBadges[0],
    barWidth: `${Math.max(2, Math.round((step.count / maxCount) * 100))}%`,
  }))
})

const summaryMetrics = computed<{ key: string; label: string; value: string; hint: string; icon: IconName; iconBg: string; iconColor: string }[]>(() => {
  const steps = stepMap(funnel.value?.steps ?? [])
  const revenue = funnel.value?.revenue
  return [
    {
      key: 'registered',
      label: t('admin.operations.registeredUsers'),
      value: formatNumber(steps.registered?.count ?? 0),
      hint: t('admin.operations.rangeDays', { days: funnel.value?.range_days ?? 0 }),
      icon: 'userPlus',
      iconBg: 'bg-blue-100 dark:bg-blue-900/30',
      iconColor: 'text-blue-600 dark:text-blue-400',
    },
    {
      key: 'active',
      label: t('admin.operations.activeUsers'),
      value: formatNumber(steps.active?.count ?? 0),
      hint: `${t('admin.operations.overallConversion')} ${formatPercent(steps.active?.overall_conversion_rate ?? 0)}`,
      icon: 'chart',
      iconBg: 'bg-emerald-100 dark:bg-emerald-900/30',
      iconColor: 'text-emerald-600 dark:text-emerald-400',
    },
    {
      key: 'paying',
      label: t('admin.operations.payingUsers'),
      value: formatNumber(steps.paying?.count ?? 0),
      hint: `${t('admin.operations.overallConversion')} ${formatPercent(steps.paying?.overall_conversion_rate ?? 0)}`,
      icon: 'creditCard',
      iconBg: 'bg-purple-100 dark:bg-purple-900/30',
      iconColor: 'text-purple-600 dark:text-purple-400',
    },
    {
      key: 'revenue',
      label: t('admin.operations.totalRevenue'),
      value: formatMoney(revenue?.total_revenue ?? 0),
      hint: `${formatNumber(revenue?.paid_orders ?? 0)} ${t('admin.operations.paidOrders')}`,
      icon: 'dollar',
      iconBg: 'bg-amber-100 dark:bg-amber-900/30',
      iconColor: 'text-amber-600 dark:text-amber-400',
    },
  ]
})

const revenueItems = computed(() => {
  const revenue = funnel.value?.revenue
  return [
    { key: 'total', label: t('admin.operations.totalRevenue'), value: formatMoney(revenue?.total_revenue ?? 0) },
    { key: 'paidOrders', label: t('admin.operations.paidOrders'), value: formatNumber(revenue?.paid_orders ?? 0) },
    { key: 'payingUsers', label: t('admin.operations.payingUsersInPeriod'), value: formatNumber(revenue?.paying_users ?? 0) },
    { key: 'avg', label: t('admin.operations.averageOrderAmount'), value: formatMoney(revenue?.average_order_amount ?? 0) },
    { key: 'balance', label: t('admin.operations.balanceRevenue'), value: formatMoney(revenue?.balance_revenue ?? 0) },
    { key: 'balanceOrders', label: t('admin.operations.balanceOrders'), value: formatNumber(revenue?.balance_orders ?? 0) },
    { key: 'subscription', label: t('admin.operations.subscriptionRevenue'), value: formatMoney(revenue?.subscription_revenue ?? 0) },
    { key: 'subscriptionOrders', label: t('admin.operations.subscriptionOrders'), value: formatNumber(revenue?.subscription_orders ?? 0) },
  ]
})

const signalItems = computed(() => {
  const signals = funnel.value?.signals
  return [
    { key: 'pending', status: 'PENDING', label: t('admin.operations.pendingOrders'), value: signals?.pending_orders ?? 0, dotClass: 'bg-yellow-500' },
    { key: 'failed', status: 'FAILED', label: t('admin.operations.failedOrders'), value: signals?.failed_orders ?? 0, dotClass: 'bg-red-500' },
    { key: 'refund', status: 'REFUND_REQUESTED', label: t('admin.operations.refundRequestedOrders'), value: signals?.refund_requested_orders ?? 0, dotClass: 'bg-purple-500' },
    { key: 'expired', status: 'EXPIRED', label: t('admin.operations.expiredOrders'), value: signals?.expired_orders ?? 0, dotClass: 'bg-gray-400' },
    { key: 'cancelled', status: 'CANCELLED', label: t('admin.operations.cancelledOrders'), value: signals?.cancelled_orders ?? 0, dotClass: 'bg-slate-500' },
  ]
})

function stepMap(steps: OperationsFunnelStep[]) {
  return Object.fromEntries(steps.map((step) => [step.key, step])) as Record<string, OperationsFunnelStep | undefined>
}

function applyShortcut(key: ShortcutKey) {
  activeShortcut.value = key
  if (key !== 'custom') {
    const days = Number.parseInt(key, 10)
    startDate.value = daysAgo(days)
    endDate.value = fmtDate(new Date())
  }
  loadFunnel()
}

function onCustomRange() {
  activeShortcut.value = 'custom'
  loadFunnel()
}

function ordersLink(status: string, dateField: DateField = 'created_at') {
  return {
    path: '/admin/orders',
    query: {
      status,
      date_field: dateField,
      start_date: startDate.value,
      end_date: endDate.value,
    },
  }
}

function formatNumber(value: number): string {
  return Number(value || 0).toLocaleString()
}

function formatPercent(value: number): string {
  return `${Number(value || 0).toFixed(2)}%`
}

function formatMoney(value: number): string {
  return `¥${Number(value || 0).toFixed(2)}`
}

function formatDateTime(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

async function loadFunnel() {
  loading.value = true
  try {
    funnel.value = await adminAPI.dashboard.getOperationsFunnel({
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    })
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.operations.errors', t('admin.operations.loadFailed')))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadFunnel()
})
</script>
