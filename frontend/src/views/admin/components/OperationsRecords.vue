<template>
  <section v-if="selection" tabindex="-1" class="records-panel rounded-2xl border border-primary-200 bg-white p-5 shadow-sm dark:border-primary-900 dark:bg-dark-800" aria-label="页内分析明细">
    <div class="mb-5 flex flex-wrap items-start justify-between gap-3">
      <div><p class="mb-1 text-xs font-medium text-primary-600">运营分析 / {{ selection.mode === 'orders' ? '成功充值' : '消耗追踪' }}</p><h2 class="text-base font-semibold">{{ selection.title }}</h2><p class="mt-2 text-xs text-gray-500">{{ selection.start_date }} — {{ selection.end_date }} · {{ timezone }} · {{ selection.mode === 'orders' ? '订单创建时间 · 仅成功支付状态 · 付款与到账分别展示' : '请求时间 · 当前筛选下的逐笔计费记录' }}</p></div>
      <button v-if="!embedded" class="btn btn-secondary" aria-label="关闭页内明细" @click="close"><Icon name="x" size="sm" /></button>
    </div>
    <div v-if="error" role="alert" class="py-6 text-sm text-red-600">{{ error }} <button class="underline" @click="load">重试</button></div>
    <p v-else-if="loading" role="status" class="py-10 text-center text-sm text-gray-500">正在加载明细…</p>
    <template v-else>
      <div class="overflow-x-auto">
        <table v-if="selection.mode === 'orders'" class="w-full whitespace-nowrap text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900"><tr><th class="p-3 text-left">订单号 / 用户</th><th class="p-3 text-left">充值类型</th><th class="p-3 text-left">支付方式</th><th class="p-3 text-right">实际付款</th><th class="p-3 text-right">实际到账 USD</th><th class="p-3 text-right">待到账 USD</th><th class="p-3 text-left">当前状态</th><th class="p-3 text-left">创建时间</th><th class="p-3 text-left">支付时间</th></tr></thead>
          <tbody><tr v-for="order in orders" :key="order.id" class="border-b border-gray-100 dark:border-dark-700"><td class="p-3"><span class="font-mono text-xs">{{ order.out_trade_no }}</span><span class="mt-1 block text-xs text-gray-500">用户 #{{ order.user_id }}</span></td><td class="p-3">{{ order.order_type === 'subscription' ? '订阅充值' : '余额充值' }}</td><td class="p-3">{{ order.payment_type }}</td><td class="p-3 text-right font-medium tabular-nums">{{ cashMoney(order.pay_amount, order.currency || 'CNY') }}</td><td class="p-3 text-right tabular-nums">{{ order.order_type === 'balance' ? money(order.status === 'COMPLETED' ? order.amount : 0) : '不适用' }}</td><td class="p-3 text-right tabular-nums">{{ order.order_type === 'balance' ? money(order.status !== 'COMPLETED' ? order.amount : 0) : '不适用' }}</td><td class="p-3"><span class="rounded-full bg-emerald-50 px-2 py-1 text-xs text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">{{ statuses[order.status] || order.status }}</span></td><td class="p-3 text-xs">{{ date(order.created_at) }}</td><td class="p-3 text-xs">{{ date(order.paid_at) }}</td></tr></tbody>
        </table>
        <table v-else class="w-full whitespace-nowrap text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900"><tr><th class="p-3 text-left">时间 / 请求</th><th class="p-3 text-left">用户</th><th class="p-3 text-left">账号 / 模型</th><th class="p-3 text-right">用户消耗</th><th class="p-3 text-right">上游成本</th><th class="p-3 text-right">计费毛利</th><th class="p-3 text-right">响应时长</th></tr></thead>
          <tbody><tr v-for="row in usage" :key="row.id" class="border-b border-gray-100 dark:border-dark-700"><td class="p-3 text-xs">{{ date(row.created_at) }}<span class="mt-1 block max-w-48 truncate font-mono text-gray-400" :title="row.request_id">{{ row.request_id }}</span></td><td class="max-w-48 truncate p-3" :title="row.user?.email">{{ row.user?.email || `#${row.user_id}` }}</td><td class="p-3"><span>{{ row.model }}</span><span class="mt-1 block text-xs text-gray-500">{{ row.account?.name || `#${row.account_id ?? '—'}` }}</span></td><td class="p-3 text-right tabular-nums">{{ money(row.actual_cost) }}</td><td class="p-3 text-right tabular-nums">{{ money(cost(row)) }}</td><td class="p-3 text-right tabular-nums" :class="row.actual_cost < cost(row) ? 'text-red-600' : 'text-emerald-600'">{{ money(row.actual_cost - cost(row)) }}</td><td class="p-3 text-right">{{ row.duration_ms == null ? '—' : `${row.duration_ms} ms` }}</td></tr></tbody>
        </table>
      </div>
      <p v-if="total === 0" class="py-10 text-center text-sm text-gray-500">当前筛选下暂无记录</p>
      <div class="mt-4 flex items-center justify-between gap-3 text-xs text-gray-500"><span>共 {{ total.toLocaleString() }} 条 · 第 {{ page }} / {{ pages }} 页</span><div class="flex gap-2"><button class="btn btn-secondary" :disabled="page <= 1" aria-label="上一页明细" @click="changePage(-1)"><Icon name="chevronLeft" size="sm" /></button><button class="btn btn-secondary" :disabled="page >= pages" aria-label="下一页明细" @click="changePage(1)"><Icon name="chevronRight" size="sm" /></button></div></div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { cashMoney } from '../operationsFinanceMetrics'
import { adminPaymentAPI } from '@/api/admin/payment'
import { list as listUsage } from '@/api/admin/usage'
import type { PaymentOrder } from '@/types/payment'
import type { AdminUsageLog } from '@/types'

const emit = defineEmits<{ close: [] }>()
const props = defineProps<{ startDate: string; endDate: string; timezone: string; embedded?: boolean }>()
type Selection = { mode: 'orders' | 'usage'; title: string; start_date: string; end_date: string; account_id?: number; user_id?: number; model?: string }
const selection = ref<Selection | null>(null)
const orders = ref<PaymentOrder[]>([]), usage = ref<AdminUsageLog[]>([])
const loading = ref(false), error = ref(''), page = ref(1), total = ref(0)
const pages = computed(() => Math.max(1, Math.ceil(total.value / 20)))
const statuses: Record<string, string> = { PAID: '已支付', RECHARGING: '充值中', COMPLETED: '已完成' }
let controller: AbortController | undefined
const money = (value: number) => '$' + value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 4 })
const date = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', { timeZone: props.timezone }) : '—'
const cost = (row: AdminUsageLog) => (row.account_stats_cost ?? row.total_cost) * (row.account_rate_multiplier ?? 1)
async function load() {
  if (!selection.value) return
  controller?.abort()
  const request = new AbortController(); controller = request
  loading.value = true; error.value = ''; orders.value = []; usage.value = []; total.value = 0
  const { mode, start_date, end_date, account_id, user_id, model } = selection.value
  try {
    if (mode === 'orders') {
      const { data } = await adminPaymentAPI.getOrders({ finance_only: true, date_field: 'created_at', start_date, end_date, page: page.value, page_size: 20 }, request.signal)
      if (controller !== request || request.signal.aborted) return
      orders.value = data.items; total.value = data.total
    } else {
      const data = await listUsage({ start_date, end_date, timezone: props.timezone, account_id, user_id, model, page: page.value, page_size: 20, exact_total: true }, { signal: request.signal })
      if (controller !== request || request.signal.aborted) return
      usage.value = data.items; total.value = data.total
    }
  } catch { if (controller === request && !request.signal.aborted) error.value = '明细加载失败，请重试。' }
  finally { if (controller === request) loading.value = false }
}
async function open(value: Omit<Selection, 'start_date' | 'end_date'> & Partial<Pick<Selection, 'start_date' | 'end_date'>>) {
  selection.value = { start_date: props.startDate, end_date: props.endDate, ...value }; page.value = 1
  void load()
}
function reset() { controller?.abort(); selection.value = null }
function close() { reset(); emit('close') }
function changePage(delta: number) { page.value += delta; void load() }
watch(() => [props.startDate, props.endDate, props.timezone], reset)
onBeforeUnmount(() => controller?.abort())
defineExpose({ open })
</script>
