<template>
  <div class="card p-4">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('payment.admin.dailyUsers') }}
      </h3>
      <div class="flex rounded-md border border-gray-200 dark:border-dark-600">
        <button
          v-for="mode in viewModes"
          :key="mode.key"
          type="button"
          class="px-2.5 py-1 text-xs font-medium transition-colors first:rounded-l-md last:rounded-r-md"
          :class="viewMode === mode.key
            ? 'bg-primary-600 text-white'
            : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
          @click="viewMode = mode.key"
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
        <Bar :data="chartData" :options="chartOptions" />
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
  BarElement,
  Tooltip,
  Legend
} from 'chart.js'
import { Bar } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { DailyStatPoint } from '@/types/payment'

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip, Legend)

const { t } = useI18n()

const props = defineProps<{
  data: DailyStatPoint[]
  loading?: boolean
}>()

type ViewMode = 'count' | 'amount'
const viewMode = ref<ViewMode>('count')

const viewModes = computed(() => [
  { key: 'count' as ViewMode, label: t('payment.admin.userCount') },
  { key: 'amount' as ViewMode, label: t('payment.admin.rechargeAmount') },
])

const labels = computed(() => props.data.map(d => d.date))

const chartData = computed(() => {
  if (viewMode.value === 'count') {
    return {
      labels: labels.value,
      datasets: [
        {
          label: t('payment.admin.newUser'),
          data: props.data.map(d => d.new_user_count),
          backgroundColor: 'rgba(16, 185, 129, 0.75)',
          stack: 'users',
        },
        {
          label: t('payment.admin.returningUser'),
          data: props.data.map(d => d.returning_user_count),
          backgroundColor: 'rgba(59, 130, 246, 0.75)',
          stack: 'users',
        }
      ]
    }
  }
  return {
    labels: labels.value,
    datasets: [
      {
        label: t('payment.admin.newUser'),
        data: props.data.map(d => d.new_user_amount),
        backgroundColor: 'rgba(16, 185, 129, 0.75)',
        stack: 'amounts',
      },
      {
        label: t('payment.admin.returningUser'),
        data: props.data.map(d => d.returning_user_amount),
        backgroundColor: 'rgba(59, 130, 246, 0.75)',
        stack: 'amounts',
      }
    ]
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    x: { stacked: true },
    y: {
      stacked: true,
      title: {
        display: true,
        text: viewMode.value === 'count'
          ? t('payment.admin.userCount')
          : t('payment.admin.rechargeAmount')
      }
    }
  },
  plugins: { legend: { position: 'top' as const } }
}))
</script>
