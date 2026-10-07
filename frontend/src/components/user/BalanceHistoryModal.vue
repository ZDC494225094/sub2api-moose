<template>
  <BaseDialog :show="show" :title="t('balanceHistory.title')" width="wide" @close="emit('close')">
    <div class="space-y-5">
      <div class="rounded-xl bg-primary-50 p-4 dark:bg-primary-900/20">
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('balanceHistory.currentBalance') }}</p>
        <p class="mt-1 text-2xl font-semibold text-primary-700 dark:text-primary-300">
          {{ formatBalance(currentBalance) }}
        </p>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <span class="text-sm text-gray-500 dark:text-dark-400">{{ t('balanceHistory.range') }}</span>
        <div class="flex gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-700" role="group" :aria-label="t('balanceHistory.range')">
          <button
            v-for="range in ranges"
            :key="range"
            type="button"
            class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
            :class="months === range
              ? 'bg-white text-primary-600 shadow-sm dark:bg-dark-600 dark:text-primary-400'
              : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
            :aria-pressed="months === range"
            @click="months = range"
          >
            {{ t('balanceHistory.months', { count: range }) }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="flex h-72 flex-col items-center justify-center gap-3" aria-live="polite">
        <LoadingSpinner size="lg" />
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('common.loading') }}</p>
      </div>
      <div v-else-if="error" class="flex h-72 flex-col items-center justify-center gap-3" role="alert">
        <p class="text-sm text-red-600 dark:text-red-400">{{ t('balanceHistory.loadFailed') }}</p>
        <button type="button" class="btn btn-secondary" @click="loadHistory">{{ t('balanceHistory.retry') }}</button>
      </div>
      <div v-else-if="history.length === 0" class="flex h-72 items-center justify-center text-sm text-gray-500 dark:text-dark-400">
        {{ t('balanceHistory.empty') }}
      </div>
      <template v-else>
        <p v-if="!hasSnapshots" class="text-sm text-gray-500 dark:text-dark-400" role="status">
          {{ t('balanceHistory.noHistory') }}
        </p>
        <div class="relative h-72">
          <canvas ref="canvasRef" role="img" :aria-label="t('balanceHistory.chartLabel')"></canvas>
        </div>
        <ul class="sr-only">
          <li v-for="point in history" :key="point.period">
            {{ point.period }}: {{ formatBalance(point.balance) }}
            {{ point.realtime ? t('balanceHistory.realtime') : '' }}
          </li>
        </ul>
        <p v-if="hasRealtime" class="flex items-center gap-2 text-sm text-gray-500 dark:text-dark-400">
          <span class="h-2.5 w-2.5 rotate-45 bg-amber-500" aria-hidden="true"></span>
          {{ t('balanceHistory.realtimeNote') }}
        </p>
        <p class="text-xs text-gray-400 dark:text-dark-500">{{ t('balanceHistory.snapshotNote') }}</p>
      </template>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart } from 'chart.js/auto'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { getBalanceHistory, type BalanceHistoryMonths, type BalanceHistoryPoint } from '@/api/user'

const props = defineProps<{ show: boolean; currentBalance: number }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t, locale } = useI18n()
const ranges: BalanceHistoryMonths[] = [3, 6, 12]
const months = ref<BalanceHistoryMonths>(12)
const history = ref<BalanceHistoryPoint[]>([])
const loading = ref(false)
const error = ref(false)
const canvasRef = ref<HTMLCanvasElement | null>(null)
const hasSnapshots = computed(() => history.value.some(point => !point.realtime))
const hasRealtime = computed(() => history.value.some(point => point.realtime))
let chart: Chart<'line'> | null = null
let requestID = 0

function formatBalance(balance: number): string {
  return new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD' }).format(balance)
}

function destroyChart() {
  chart?.destroy()
  chart = null
}

function renderChart() {
  destroyChart()
  if (!canvasRef.value || !props.show || loading.value || error.value || history.value.length === 0) return
  const points = history.value
  chart = new Chart(canvasRef.value, {
    type: 'line',
    data: {
      labels: points.map(point => point.period),
      datasets: [{
        label: t('balanceHistory.balanceAxis'),
        data: points.map(point => point.balance),
        borderColor: '#6366f1',
        backgroundColor: '#6366f1',
        borderWidth: 2,
        pointRadius: points.map(point => point.realtime ? 6 : 4),
        pointHoverRadius: 7,
        pointStyle: points.map(point => point.realtime ? 'rectRot' : 'circle'),
        pointBackgroundColor: points.map(point => point.realtime ? '#f59e0b' : '#6366f1'),
        pointBorderColor: points.map(point => point.realtime ? '#f59e0b' : '#6366f1'),
        segment: {
          borderDash: segment => points[segment.p0DataIndex]?.realtime || points[segment.p1DataIndex]?.realtime ? [6, 4] : [],
          borderColor: segment => points[segment.p1DataIndex]?.realtime ? '#f59e0b' : '#6366f1'
        }
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: context => `${formatBalance(Number(context.parsed.y))}${points[context.dataIndex]?.realtime ? ` (${t('balanceHistory.realtime')})` : ''}`
          }
        }
      },
      scales: {
        x: { title: { display: true, text: t('balanceHistory.monthAxis') }, grid: { display: false } },
        y: {
          title: { display: true, text: t('balanceHistory.balanceAxis') },
          ticks: { callback: value => formatBalance(Number(value)) }
        }
      }
    }
  })
}

async function loadHistory() {
  const activeRequest = ++requestID
  destroyChart()
  loading.value = true
  error.value = false
  history.value = []
  try {
    const points = await getBalanceHistory(months.value)
    if (activeRequest === requestID && props.show) history.value = points
  } catch {
    if (activeRequest === requestID && props.show) error.value = true
  } finally {
    if (activeRequest === requestID) loading.value = false
  }
}

watch([() => props.show, months], ([show]) => {
  if (show) {
    void loadHistory()
  } else {
    requestID++
    loading.value = false
    destroyChart()
  }
}, { immediate: true })

watch([canvasRef, history, locale, loading], renderChart, { flush: 'post' })

onBeforeUnmount(() => {
  requestID++
  destroyChart()
})
</script>
