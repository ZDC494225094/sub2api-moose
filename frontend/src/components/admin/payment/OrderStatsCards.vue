<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <!-- Today Revenue -->
    <RouterLink :to="ordersLink" class="card block p-4 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30">
          <Icon name="dollar" size="md" class="text-green-600 dark:text-green-400" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('payment.admin.todayRevenue') }}</p>
          <p v-for="[currency, amount] in sortedAmounts(stats.today_amount)" :key="currency" class="text-xl font-bold text-gray-900 dark:text-white">
            {{ formatMoney(currency, amount) }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ stats.today_count }} {{ t('payment.admin.orders') }}
          </p>
        </div>
      </div>
    </RouterLink>

    <!-- Total Revenue -->
    <RouterLink :to="ordersLink" class="card block p-4 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30">
          <Icon name="creditCard" size="md" class="text-blue-600 dark:text-blue-400" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('payment.admin.totalRevenue') }}</p>
          <p v-for="[currency, amount] in sortedAmounts(stats.total_amount)" :key="currency" class="text-xl font-bold text-gray-900 dark:text-white">
            {{ formatMoney(currency, amount) }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ stats.total_count }} {{ t('payment.admin.orders') }}
          </p>
        </div>
      </div>
    </RouterLink>

    <!-- Today Orders -->
    <RouterLink :to="ordersLink" class="card block p-4 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30">
          <Icon name="chart" size="md" class="text-purple-600 dark:text-purple-400" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('payment.admin.todayOrders') }}</p>
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ stats.today_count }}</p>
        </div>
      </div>
    </RouterLink>

    <!-- Average Amount -->
    <RouterLink :to="ordersLink" class="card block p-4 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700">
      <div class="flex items-center gap-3">
        <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30">
          <Icon name="chart" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
        </div>
        <div>
          <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('payment.admin.avgAmount') }}</p>
          <p v-for="[currency, amount] in sortedAmounts(stats.avg_amount)" :key="currency" class="text-xl font-bold text-gray-900 dark:text-white">
            {{ formatMoney(currency, amount) }}
          </p>
        </div>
      </div>
    </RouterLink>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CurrencyAmounts, DashboardStats } from '@/types/payment'

const { t } = useI18n()

const props = defineProps<{
  stats: DashboardStats
  startDate?: string
  endDate?: string
}>()

const ordersLink = computed(() => {
  const query: Record<string, string> = { date_field: 'paid_at' }
  if (props.startDate) query.start_date = props.startDate
  if (props.endDate) query.end_date = props.endDate
  return { path: '/admin/orders', query }
})

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}
</script>
