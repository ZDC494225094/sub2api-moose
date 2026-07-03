<template>
  <div class="card p-4">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('payment.admin.dailyRevenue') }}
      </h3>
      <div class="flex rounded-md border border-gray-200 dark:border-dark-600">
        <button
          v-for="mode in chartModes"
          :key="mode.key"
          type="button"
          class="px-2.5 py-1 text-xs font-medium transition-colors first:rounded-l-md last:rounded-r-md"
          :class="chartMode === mode.key
            ? 'bg-primary-600 text-white'
            : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
          @click="chartMode = mode.key"
        >
          {{ mode.label }}
        </button>
      </div>
    </div>
    <div class="h-64">
      <div v-if="loading" class="flex h-full items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <template v-else-if="data && data.length">
        <Bar v-if="chartMode === 'bar-type'" :data="barTypeData!" :options="barTypeOptions" />
        <Bar v-else-if="chartMode === 'bar'" :data="barData!" :options="barOptions" />
        <Line v-else :data="lineData!" :options="lineOptions" />
      </template>
      <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('payment.admin.noData') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line, Bar } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { DailyStatPoint } from '@/types/payment'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend, Filler)

const { t } = useI18n()

const props = defineProps<{
  data: DailyStatPoint[]
  loading?: boolean
}>()

type ChartMode = 'line' | 'bar' | 'bar-type'

const chartMode = ref<ChartMode>('line')

const chartModes = computed(() => [
  { key: 'line' as ChartMode, label: t('payment.admin.chartLine') },
  { key: 'bar' as ChartMode, label: t('payment.admin.chartBar') },
  { key: 'bar-type' as ChartMode, label: t('payment.admin.chartBarType') },
])

const labels = computed(() => props.data.map(d => d.date))

// Line chart: revenue + order count
const lineData = computed(() => ({
  labels: labels.value,
  datasets: [
    {
      label: t('payment.admin.revenue'),
      data: props.data.map(d => d.amount),
      borderColor: 'rgb(59, 130, 246)',
      backgroundColor: 'rgba(59, 130, 246, 0.1)',
      fill: true,
      tension: 0.3,
      pointRadius: 3,
      pointHoverRadius: 5,
    },
    {
      label: t('payment.admin.orderCount'),
      data: props.data.map(d => d.count),
      borderColor: 'rgb(16, 185, 129)',
      backgroundColor: 'rgba(16, 185, 129, 0.1)',
      fill: false,
      tension: 0.3,
      pointRadius: 3,
      pointHoverRadius: 5,
      yAxisID: 'y1',
    }
  ]
}))

const lineOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    y: {
      type: 'linear' as const,
      display: true,
      position: 'left' as const,
      title: { display: true, text: t('payment.admin.revenue') },
    },
    y1: {
      type: 'linear' as const,
      display: true,
      position: 'right' as const,
      title: { display: true, text: t('payment.admin.orderCount') },
      grid: { drawOnChartArea: false },
    }
  },
  plugins: { legend: { position: 'top' as const } }
}

// Bar chart: stacked daily orders (count)
const barData = computed(() => ({
  labels: labels.value,
  datasets: [
    {
      label: t('payment.admin.balanceOrder'),
      data: props.data.map(d => d.balance_count),
      backgroundColor: 'rgba(59, 130, 246, 0.75)',
      stack: 'orders',
    },
    {
      label: t('payment.admin.subscriptionOrder'),
      data: props.data.map(d => d.subscription_count),
      backgroundColor: 'rgba(168, 85, 247, 0.75)',
      stack: 'orders',
    }
  ]
}))

const barOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    x: { stacked: true },
    y: { stacked: true, title: { display: true, text: t('payment.admin.orderCount') } }
  },
  plugins: { legend: { position: 'top' as const } }
}

// Bar chart: stacked daily revenue by order type
const barTypeData = computed(() => ({
  labels: labels.value,
  datasets: [
    {
      label: t('payment.admin.balanceOrder'),
      data: props.data.map(d => d.balance_amount),
      backgroundColor: 'rgba(59, 130, 246, 0.75)',
      stack: 'revenue',
    },
    {
      label: t('payment.admin.subscriptionOrder'),
      data: props.data.map(d => d.subscription_amount),
      backgroundColor: 'rgba(168, 85, 247, 0.75)',
      stack: 'revenue',
    }
  ]
}))

const barTypeOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    x: { stacked: true },
    y: { stacked: true, title: { display: true, text: t('payment.admin.revenue') } }
  },
  plugins: { legend: { position: 'top' as const } }
}
</script>
