<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ApplyPanel from './ApplyPanel.vue'
import RecordsPanel from './RecordsPanel.vue'
import TitlesPanel from './TitlesPanel.vue'
import { invoiceAPI, type InvoiceOverview } from './api'
import { errorMessage, formatMoney, invoiceTypeLabels } from './format'

type Tab = 'apply' | 'records' | 'titles'

const overview = ref<InvoiceOverview | null>(null)
const loading = ref(true)
const error = ref('')
const tab = ref<Tab>('apply')
const applyPanel = ref<InstanceType<typeof ApplyPanel> | null>(null)

const settings = computed(() => overview.value?.settings ?? null)
const tabs: Array<{ value: Tab; label: string; icon: 'document' | 'clock' | 'user' }> = [
  { value: 'apply', label: '申请开票', icon: 'document' },
  { value: 'records', label: '开票记录', icon: 'clock' },
  { value: 'titles', label: '发票抬头', icon: 'user' },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    overview.value = await invoiceAPI.overview()
  } catch (e) {
    error.value = errorMessage(e, '开票服务暂时不可用，请稍后再试')
  } finally {
    loading.value = false
  }
}

function onApplied() {
  tab.value = 'records'
}

function onTitlesChanged() {
  void applyPanel.value?.reload()
}

onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-6xl space-y-6">
      <header class="flex min-w-0 items-start gap-4">
        <div class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-800/40">
          <Icon name="document" size="lg" />
        </div>
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">发票管理</h1>
          <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-500 dark:text-dark-400">
            选择站内支付的充值或套餐订单申请开具发票，多笔订单可以合并开一张。
          </p>
        </div>
      </header>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>

      <div v-else-if="overview && !overview.allowed" class="card empty-state">
        <Icon name="lock" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">开票功能暂未对你的账户开放</p>
        <p class="empty-state-description">如需开具发票，请联系客服开通。</p>
      </div>

      <template v-else-if="settings">
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">开票门槛</p>
            <p class="mt-1 text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ Number(settings.min_amount) > 0 ? `满 ${formatMoney(settings.min_amount)}` : '不限金额' }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">可开票订单</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ settings.order_window_days > 0 ? `近 ${settings.order_window_days} 天` : '不限时间' }}</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">发票类型</p>
            <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ settings.allow_special ? '普票 / 专票' : invoiceTypeLabels.normal }}</p>
          </div>
        </div>

        <div v-if="settings.notice" class="flex gap-3 rounded-2xl border border-primary-100 bg-primary-50/60 px-4 py-3 text-sm leading-6 text-gray-700 dark:border-primary-900/40 dark:bg-primary-900/10 dark:text-gray-300">
          <Icon name="infoCircle" size="sm" class="mt-1 shrink-0 text-primary-600 dark:text-primary-400" />
          <p class="min-w-0 whitespace-pre-line break-words">{{ settings.notice }}</p>
        </div>

        <div class="tabs inline-flex flex-wrap" role="tablist" aria-label="发票管理">
          <button
            v-for="item in tabs"
            :key="item.value"
            type="button"
            role="tab"
            class="tab inline-flex items-center gap-1.5"
            :class="{ 'tab-active': tab === item.value }"
            :aria-selected="tab === item.value"
            @click="tab = item.value"
          >
            <Icon :name="item.icon" size="sm" />{{ item.label }}
          </button>
        </div>

        <ApplyPanel v-if="tab === 'apply'" ref="applyPanel" :settings="settings" @applied="onApplied" @manage-titles="tab = 'titles'" />
        <RecordsPanel v-else-if="tab === 'records'" />
        <TitlesPanel v-else :allow-special="settings.allow_special" @changed="onTitlesChanged" />
      </template>
    </div>
  </AppLayout>
</template>
