<template>
  <AppLayout>
    <div class="finance space-y-6">
      <header class="finance-hero">
        <div class="flex items-center gap-4"><div class="hero-icon"><Icon name="chartBar" size="lg" /></div><div><p class="mb-1 text-[10px] font-semibold tracking-[0.24em] text-indigo-500 dark:text-indigo-300">OPERATIONS INTELLIGENCE</p><h1 class="text-2xl font-semibold tracking-tight">运营分析</h1><p class="mt-2 text-xs text-gray-500 dark:text-gray-400">从充值、消耗到利润，追踪每一笔经营变化</p></div></div>
        <button v-if="report" type="button" class="btn btn-secondary flex items-center gap-2" @click="openCustomers('repeat')"><Icon name="users" size="sm" /> 用户经营与留存 <Icon name="arrowRight" size="sm" /></button>
      </header>
      <form class="finance-panel finance-filter" @submit.prevent="load">
        <div class="mr-auto"><p class="mb-2 text-xs text-gray-500">统计周期 <span class="ml-2 text-gray-400">{{ timezone }} · 最多 90 天</span></p><div class="date-presets" role="group" aria-label="日期快捷选择"><button v-for="preset in presets" :key="preset.label" type="button" :aria-pressed="activePreset === preset.label" :class="{ selected: activePreset === preset.label }" @click="setRange(preset.days, preset.label)">{{ preset.label }}</button></div></div>
        <label class="text-xs text-gray-500">开始日期<input v-model="start" type="date" required class="input mt-1" :max="end" @input="activePreset = ''"></label>
        <label class="text-xs text-gray-500">结束日期<input v-model="end" type="date" required class="input mt-1" :min="start" @input="activePreset = ''"></label>
        <div class="filter-actions"><button class="btn btn-primary" type="submit" :disabled="loading">查询</button>
        <button class="btn btn-secondary" type="button" title="刷新数据" aria-label="刷新数据" :disabled="loading" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" /></button></div>
      </form>
      <div v-if="error" role="alert" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/20">{{ error }}</div>
      <div v-if="loading" role="status" class="finance-panel py-16 text-center text-sm text-gray-500"><Icon name="refresh" class="mx-auto mb-3 animate-spin text-indigo-500" />正在汇总经营数据…</div>
      <template v-else-if="report">
        <div class="section-heading"><div><span class="section-index">01</span><h2>经营概览</h2><span class="hidden text-xs text-gray-400 sm:inline">{{ report.start_date }} — {{ report.end_date }}</span></div><span class="text-xs text-gray-400">更新于 {{ new Date(report.generated_at).toLocaleString('zh-CN', { timeZone: timezone }) }}</span></div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <button v-for="(metric, index) in metrics" :key="metric.label" type="button" class="metric-card" :class="{ 'metric-featured': index === 0 }" @click="metric.action()">
            <span class="flex items-center justify-between gap-2"><span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ metric.label }}</span><span class="metric-icon"><Icon :name="metric.icon" size="sm" /></span></span>
            <strong class="my-3 block break-words text-[26px] font-semibold leading-tight tracking-tight tabular-nums" :class="metric.color">{{ metric.value }}</strong>
            <span class="flex items-center justify-between gap-2 text-[11px] text-gray-500 dark:text-gray-400">{{ metric.hint }}<Icon name="chevronRight" size="xs" class="shrink-0" /></span>
          </button>
        </div>
        <div class="insight-strip"><button v-for="item in insights" :key="item.label" class="insight-item" @click="item.action()"><span class="flex items-center gap-2 text-xs text-gray-500"><Icon :name="item.icon" size="sm" />{{ item.label }}</span><strong class="mt-2 block text-lg tabular-nums">{{ item.value }}</strong><span class="mt-1 block text-[11px] text-gray-500">{{ item.hint }}</span></button></div>
        <div class="section-heading"><div><span class="section-index">02</span><h2>趋势与结构</h2></div><div class="flex items-center gap-2"><span class="hidden text-xs text-gray-400 sm:inline">粒度 · 周从周一开始</span><div class="date-presets" aria-label="趋势统计粒度"><button v-for="item in ([{ key: 'day', label: '日' }, { key: 'week', label: '周' }, { key: 'month', label: '月' }] as const)" :key="item.key" :aria-pressed="granularity === item.key" :class="{ selected: granularity === item.key }" @click="granularity = item.key">{{ item.label }}</button></div></div></div>
        <div class="grid min-w-0 gap-4 xl:grid-cols-3">
          <section class="finance-panel min-w-0 xl:col-span-2"><div class="chart-heading"><div><h3>消耗、成本与毛利</h3><p>识别收入与成本的变化 · 点击数据点查看请求</p></div><span class="unit-badge">USD</span></div><div class="h-72"><Line :data="trendData" :options="lineOptions" /></div></section>
          <section class="finance-panel min-w-0"><div class="chart-heading"><div><h3>上游成本分布</h3><p>Top 10 · 点击柱形下探账号</p></div><Icon name="server" size="sm" class="text-indigo-400" /></div><div v-if="upstreams.length" class="h-72"><Bar :data="upstreamData" :options="barOptions" /></div><div v-else class="py-20 text-center text-sm text-gray-500">暂无上游消耗</div></section>
        </div>
        <div class="grid min-w-0 gap-4 xl:grid-cols-2">
          <section class="finance-panel min-w-0"><div class="chart-heading"><div><h3>实际付款结构</h3><p>充值与订阅付款分开统计 · 点击柱形查看构成</p></div><select v-model="cashCurrency" class="input !w-auto !py-1" aria-label="付款币种"><option v-for="currency in cashCurrencies.length ? cashCurrencies : ['CNY']" :key="currency" :value="currency">{{ currency }}</option></select></div><div class="h-56"><Bar :data="rechargeData" :options="rechargeOptions" /></div><p class="mt-3 border-t border-gray-100 pt-3 text-[11px] leading-5 text-gray-500 dark:border-dark-700">未计入 {{ (report.summary.total_orders - report.summary.paid_orders).toLocaleString() }} 笔：待支付、取消、过期、失败及退款相关状态。点击柱形核对成功订单。</p></section>
          <section class="finance-panel min-w-0"><div class="chart-heading"><div><h3>计费毛利率</h3><p>按选定粒度重新加权汇总，不对每日百分比取平均</p></div><span class="unit-badge">毛利 / 消耗</span></div><div class="h-56"><Line :data="marginData" :options="marginOptions" /></div><p class="mt-3 border-t border-gray-100 pt-3 text-[11px] leading-5 text-gray-500 dark:border-dark-700">无计费消耗时不计算毛利率，以断点显示；计费毛利不是现金净利润。</p></section>
        </div>
        <section class="finance-panel">
          <div class="section-heading"><div><span class="section-index">03</span><h2>多维经营分析</h2></div><span class="text-xs text-gray-400">原地打开 · 逐层分析 · 关闭后保留当前位置</span></div>
          <div class="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4"><button v-for="item in dimensions" :key="item.key" class="analysis-entry" @click="showDetails(item.key)"><Icon :name="item.key === 'upstream' ? 'server' : item.key === 'account' ? 'users' : item.key === 'model' ? 'cube' : 'chartBar'" size="sm" /><span>{{ item.label }}</span><Icon name="chevronRight" size="xs" class="ml-auto" /></button></div>
        </section>
        <BaseDialog :show="modal !== null" :title="modalTitle" width="extra-wide" @close="closeModal">
          <div ref="modalContent" class="finance space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500"><button v-if="modalHistory.length" class="btn btn-secondary flex items-center gap-1" @click="backModal"><Icon name="chevronLeft" size="sm" />返回上一级</button><span>{{ report.start_date }} — {{ report.end_date }} · {{ timezone }}</span><span>原地分析 · Esc 关闭</span></div>
        <section v-show="modal === 'dimensions'" class="finance-panel min-w-0 ">
          <div class="section-heading mb-5"><div><span class="section-index">03</span><h2>多维经营分析</h2></div><span class="text-xs text-gray-400">上游 → 账号 → 请求 · 原地弹窗下探</span></div>
          <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div class="dimension-tabs" role="tablist" aria-label="分析维度"><button v-for="item in dimensions" :key="item.key" role="tab" :class="{ selected: dimension === item.key }" :aria-selected="dimension === item.key" @click="selectDimension(item.key)">{{ item.label }}</button></div>
            <div class="flex flex-wrap items-center gap-3"><label class="flex items-center gap-1 text-xs text-gray-500"><input v-model="lossOnly" type="checkbox" class="rounded">仅看亏损</label><input v-model="search" type="search" class="input !w-44 max-w-full" placeholder="搜索名称" aria-label="搜索明细"><button class="btn btn-secondary" title="导出当前维度 CSV" aria-label="导出当前维度 CSV" @click="exportRows"><Icon name="download" size="sm" /></button></div>
          </div>
          <div v-if="upstreamFilter" class="mb-4 flex items-center gap-2 rounded-lg bg-indigo-50 px-3 py-2 text-xs text-indigo-600 dark:bg-indigo-950/40 dark:text-indigo-300"><button class="underline" @click="selectDimension('upstream')">全部上游</button><Icon name="chevronRight" size="xs" /><span>上游：{{ upstreamFilter }}</span><button class="ml-auto" @click="upstreamFilter = ''; page = 1">清除筛选</button></div>
          <div class="mb-4 flex flex-wrap gap-x-6 gap-y-2 rounded-xl bg-gray-50 p-3 text-xs text-gray-500 dark:bg-dark-900"><span>筛选后 {{ filteredRows.length }} 项</span><span>消耗 <b class="text-gray-900 dark:text-gray-100">{{ money(detailTotals.consumption) }}</b></span><span>成本 <b class="text-gray-900 dark:text-gray-100">{{ money(detailTotals.cost) }}</b></span><span>毛利 <b :class="detailTotals.profit < 0 ? 'text-red-600' : 'text-emerald-600'">{{ money(detailTotals.profit) }}</b></span><span>加权毛利率 <b>{{ percent(detailTotals.margin) }}</b></span></div>
          <div v-if="dimension !== 'day' && rankedRows.length" class="mb-5"><div class="mb-3 flex flex-wrap items-center justify-between gap-2"><p class="text-xs text-gray-500">{{ dimensions.find(d => d.key === dimension)?.label }} · Top 12 · 点击柱形继续下探</p><label class="flex items-center gap-2 text-xs text-gray-500">排序指标<select v-model="rankingMetric" class="input !w-auto !py-1" aria-label="排行指标"><option v-for="(label, key) in rankingLabels" :key="key" :value="key">{{ label }}</option></select></label></div><div class="h-56"><Bar :data="rankingData" :options="rankingOptions" /></div></div>
          <div class="overflow-x-auto"><table class="w-full whitespace-nowrap text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900"><tr><th class="p-3 text-left">{{ dimensions.find(d => d.key === dimension)?.label }}</th><th v-if="dimension === 'account'" class="p-3 text-left">上游</th><th v-for="column in columns" :key="column.key" class="p-3 text-right" :aria-sort="sortKey === column.key ? (descending ? 'descending' : 'ascending') : 'none'"><button @click="sortBy(column.key)">{{ column.label }} {{ sortKey === column.key ? (descending ? '↓' : '↑') : '' }}</button></th><th class="p-3 text-right">页内明细</th></tr></thead>
            <tbody><tr v-for="row in pageRows" :key="row.key" class="border-b border-gray-100 hover:bg-gray-50/70 dark:border-dark-700 dark:hover:bg-dark-700/40"><td class="max-w-64 truncate p-3 font-medium" :title="row.label">{{ row.label }}</td><td v-if="dimension === 'account'" class="max-w-48 truncate p-3 text-gray-500">{{ row.upstream }}</td><td v-for="column in columns" :key="column.key" class="p-3 text-right tabular-nums" :class="column.key === 'profit' ? (row.profit < 0 ? 'text-red-600' : 'text-emerald-600') : ''">{{ column.key === 'requests' ? row.requests.toLocaleString() : column.key === 'margin' ? percent(row.margin) : money(row[column.key]) }}</td><td class="p-3 text-right"><button class="text-primary-600" @click="drill(row)">{{ dimension === 'upstream' ? '账号明细' : '请求明细' }}</button><button v-if="dimension === 'day'" class="ml-3 text-primary-600" @click="orders(row.key)">订单</button></td></tr><tr v-if="!filteredRows.length"><td :colspan="columns.length + 3" class="p-10 text-center text-gray-500">暂无符合条件的数据</td></tr></tbody>
          </table></div>
          <div class="mt-4 flex items-center justify-end gap-3 text-xs text-gray-500"><span>共 {{ filteredRows.length }} 项 · {{ page }} / {{ totalPages }}</span><button class="btn btn-secondary" :disabled="page <= 1" aria-label="上一页" @click="page--"><Icon name="chevronLeft" size="sm" /></button><button class="btn btn-secondary" :disabled="page >= totalPages" aria-label="下一页" @click="page++"><Icon name="chevronRight" size="sm" /></button></div>
        </section>

            <template v-if="modal === 'balance'">
              <div class="grid gap-3 md:grid-cols-3" aria-label="当前余额构成">
                <div class="inventory-card"><span>用户余额 · USD</span><strong>{{ money(report.current_balance) }}</strong><p>含赠送的当前账面余额，不受日期筛选影响。</p></div>
                <div class="inventory-card"><span>订阅可用额度 · USD</span><strong>{{ report.inventory ? money(report.inventory.subscription_remaining) : '—' }}</strong><p>{{ report.inventory?.limited_subscriptions ?? 0 }} 份有限额订阅的当前可用量；另有 {{ report.inventory?.unlimited_subscriptions ?? 0 }} 份无限额订阅。</p></div>
                <div class="inventory-card"><span>剩余赠送额度 · USD</span><strong>{{ report.inventory?.gift_remaining == null ? '未单独核算' : money(report.inventory.gift_remaining) }}</strong><p>赠送已合并进余额，未独立追踪消耗；无法准确拆分，不按 0 展示。</p></div>
              </div>
              <p class="rounded-xl bg-gray-50 p-4 text-xs leading-6 text-gray-500 dark:bg-dark-900">订阅额度按每份有效订阅的日 / 周 / 月剩余限额取最小值后汇总，已按实际重置规则刷新。无限额订阅单独计数，不将多个周期相加，也不与余额相加。冻结余额：{{ report.inventory ? money(report.inventory.frozen_balance) : '—' }}。</p>
              <button class="btn btn-secondary" @click="openCustomers('balance')">查看有余额用户明细</button>
            </template>
            <OperationsCustomers v-if="modal === 'customers' || modalHistory.includes('customers')" v-show="modal === 'customers'" ref="customers" :start-date="report.start_date" :end-date="report.end_date" :timezone="timezone" @usage="(user) => usage({ user_id: user.id }, `${user.email} · 请求消耗明细`)" />
            <OperationsPaymentBreakdown v-if="modal === 'orders'" :payments="report.payments ?? []" :start="paymentRange.start" :end="paymentRange.end" />
            <OperationsRecords v-if="modal === 'orders' || modal === 'usage'" ref="records" embedded :start-date="report.start_date" :end-date="report.end_date" :timezone="timezone" @close="closeModal" />
          </div>
        </BaseDialog>
        <details class="finance-panel text-xs leading-6 text-gray-500"><summary class="cursor-pointer font-medium text-gray-700 dark:text-gray-300">统计口径与数据说明</summary><div class="mt-3 grid gap-3 md:grid-cols-2"><p><b>付款与到账：</b>按订单创建日期、服务器业务时区归属；实际付款取 pay_amount，按币种分别汇总余额充值与订阅付款，仅含 PAID / RECHARGING / COMPLETED。实际到账仅取 COMPLETED 余额订单的 amount（USD），其余成功余额订单单列待到账。退款相关状态整单排除；此处不是按付款日期统计的现金流水报表。</p><p><b>计费利润：</b>消耗、成本与毛利均为 USD 计费口径。毛利 = 实际计费额 − 账号统计价 × 账号倍率；毛利率 = 汇总毛利 ÷ 汇总消耗。零计费但产生成本的请求仍计入；此处不包含固定运营费用。</p><p><b>维度与时间：</b>日 / 周 / 月只在选定日期范围内汇总，首尾可能是不完整周期。上游归属为账号当前分组，不代表历史归属；当前用户余额为存量，包含赠送，不受日期筛选影响。</p><p><b>下探与核对：</b>卡片、趋势与明细共享筛选口径；CSV 导出当前维度全部筛选结果，而非当前页。明细为打开时的实时查询，支付状态变化可能导致与上次汇总有差异，可刷新重新核对。</p></div></details>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line, Bar } from 'vue-chartjs'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import OperationsPaymentBreakdown from './components/OperationsPaymentBreakdown.vue'
import { cashMoney, paymentTotals } from './operationsFinanceMetrics'
import OperationsCustomers from './components/OperationsCustomers.vue'
import OperationsRecords from './components/OperationsRecords.vue'
import { getOperationsFinance, type FinanceReport, type FinanceRow, type CustomerSegment } from '@/api/admin/operationsFinance'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend)
const router = useRouter()
const route = useRoute()
const timezone = ref('Asia/Shanghai')
const start = ref(''), end = ref(''), activePreset = ref('今天')
const report = ref<FinanceReport | null>(null), loading = ref(false), error = ref('')
let controller: AbortController | undefined
const presets = [{ label: '今天', days: 1 }, { label: '昨天', days: 0 }, { label: '近7天', days: 7 }, { label: '近30天', days: 30 }, { label: '本月', days: -1 }]
const money = (value: number) => '$' + value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 })
const percent = (value: number | null) => value == null ? '—' : `${value.toFixed(2)}%`
const presetKeys: Record<string, string> = { '今天': 'today', '昨天': 'yesterday', '近7天': '7d', '近30天': '30d', '本月': 'month' }
function setRange(_days: number, label: string) {
  activePreset.value = label
  void load()
}
async function load() {
  controller?.abort()
  const request = new AbortController(); controller = request
  closeModal(); report.value = null; error.value = ''; loading.value = false
  const days = (Date.parse(end.value) - Date.parse(start.value)) / 86400000 + 1
  if (!activePreset.value && (!Number.isFinite(days) || days < 1 || days > 90)) { error.value = '请选择有效日期，单次查询最多 90 天。'; return }
  loading.value = true
  try {
    const result = await getOperationsFinance(activePreset.value ? { preset: presetKeys[activePreset.value] } : { start_date: start.value, end_date: end.value }, request.signal)
    if (controller !== request) return
    report.value = result; page.value = 1; upstreamFilter.value = ''
    timezone.value = result.timezone || 'Asia/Shanghai'; start.value = result.start_date; end.value = result.end_date
    void router.replace({ query: activePreset.value ? { preset: presetKeys[activePreset.value] } : { start_date: result.start_date, end_date: result.end_date } })
  } catch { if (!request.signal.aborted) error.value = '经营数据加载失败，请重试。' }
  finally { if (controller === request) loading.value = false }
}
type Dimension = FinanceRow['dimension']
type NumericKey = 'requests' | 'consumption' | 'list_cost' | 'cost' | 'recharge' | 'subscription' | 'profit' | 'margin'
const dimensions: { key: Dimension; label: string }[] = [{ key: 'day', label: '每日汇总' }, { key: 'upstream', label: '上游汇总' }, { key: 'account', label: '上游账号' }, { key: 'model', label: '计费模型' }]
const dimension = ref<Dimension>('day'), upstreamFilter = ref(''), search = ref(''), page = ref(1)
const sortKey = ref<NumericKey>('cost'), descending = ref(true)
type ModalMode = 'dimensions' | 'orders' | 'usage' | 'customers' | 'balance'
const modal = ref<ModalMode | null>(null), modalHistory = ref<ModalMode[]>([])
const modalContent = ref<HTMLElement>()
function trapModalFocus(event: KeyboardEvent) {
  if (!modal.value || event.key !== 'Tab' || event.defaultPrevented) return
  const dialog = modalContent.value?.closest('[role="dialog"]')
  if (!dialog) return
  const elements = Array.from(dialog.querySelectorAll<HTMLElement>('button, a[href], input, select, textarea, [tabindex]'))
    .filter(el => el.tabIndex >= 0 && !el.hasAttribute('disabled') && el.getClientRects().length > 0)
  const first = elements[0], last = elements[elements.length - 1]
  if (!first || !last) return
  const active = document.activeElement
  if (!dialog.contains(active) || (event.shiftKey ? active === first : active === last)) {
    event.preventDefault(); (event.shiftKey ? last : first).focus({ preventScroll: true })
  }
}
const modalTitle = computed(() => ({ dimensions: '多维经营分析', orders: '充值付款与到账明细', usage: '请求消耗明细', customers: '用户经营与留存', balance: '当前余额与订阅额度' })[modal.value ?? 'dimensions'])
const paymentRange = ref<{ start?: string; end?: string }>({})
function openModal(mode: ModalMode) { if (modal.value && modal.value !== mode) modalHistory.value.push(modal.value); modal.value = mode }
function closeModal() { modal.value = null; modalHistory.value = [] }
function backModal() { modal.value = modalHistory.value.pop() ?? null }
async function openCustomers(segment: CustomerSegment) { openModal('customers'); await nextTick(); customers.value?.open(segment) }
const cashTotals = computed(() => paymentTotals(report.value?.payments ?? []))
const cashCurrency = ref('CNY')
const cashCurrencies = computed(() => cashTotals.value.currencies.map(c => c.currency))
watch(cashCurrencies, currencies => { if (!currencies.includes(cashCurrency.value)) cashCurrency.value = currencies[0] ?? 'CNY' })
const cashHeadline = computed(() => cashTotals.value.currencies.length ? cashTotals.value.currencies.map(c => cashMoney(c.recharge_paid + c.subscription_paid, c.currency)).join(' / ') : '暂无成功付款')
const customers = ref<InstanceType<typeof OperationsCustomers>>()
const records = ref<InstanceType<typeof OperationsRecords>>()
const lossOnly = ref(false)
const columns = computed<{ key: NumericKey; label: string }[]>(() => [
  ...(dimension.value === 'day' ? [{ key: 'recharge' as const, label: '余额到账 USD' }] : []),
  { key: 'requests', label: '请求数' }, { key: 'consumption', label: '用户消耗' }, { key: 'list_cost', label: '标准价消耗' }, { key: 'cost', label: '上游成本' }, { key: 'profit', label: '计费毛利' }, { key: 'margin', label: '毛利率' }
])
const filteredRows = computed(() => (report.value?.rows ?? []).filter(r => r.dimension === dimension.value && (!lossOnly.value || r.profit < 0) && (!upstreamFilter.value || r.upstream === upstreamFilter.value) && r.label.toLowerCase().includes(search.value.toLowerCase())).sort((a, b) => ((a[sortKey.value] ?? -Infinity) - (b[sortKey.value] ?? -Infinity)) * (descending.value ? -1 : 1) || a.key.localeCompare(b.key)))
const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / 20)))
const pageRows = computed(() => filteredRows.value.slice((page.value - 1) * 20, page.value * 20))
watch([search, lossOnly, upstreamFilter], () => { page.value = 1 })
function sortBy(key: NumericKey) { descending.value = sortKey.value === key ? !descending.value : true; sortKey.value = key; page.value = 1 }
function selectDimension(key: Dimension) { dimension.value = key; lossOnly.value = false; upstreamFilter.value = ''; search.value = ''; page.value = 1; if (!columns.value.some(c => c.key === sortKey.value)) sortKey.value = 'cost' }
function showDetails(key: Dimension) { selectDimension(key); openModal('dimensions') }
async function usage(extra: { start_date?: string; end_date?: string; account_id?: number; model?: string; user_id?: number } = {}, title = '请求消耗明细') {
  openModal('usage'); await nextTick(); void records.value?.open({ mode: 'usage', title, ...extra })
}
async function orders(day?: string, lastDay?: string) {
  paymentRange.value = { start: day, end: lastDay ?? day }
  openModal('orders'); await nextTick()
  void records.value?.open({ mode: 'orders', title: day ? `${day}${lastDay && lastDay !== day ? ' 至 ' + lastDay : ''} · 成功付款订单` : '期间成功付款订单', ...(day ? { start_date: day, end_date: lastDay ?? day } : {}) })
}
function drill(row: FinanceRow) {
  if (row.dimension === 'upstream') { selectDimension('account'); upstreamFilter.value = row.key; openModal('dimensions') }
  else if (row.dimension === 'day') usage({ start_date: row.key, end_date: row.key }, `${row.key} · 请求消耗明细`)
  else usage(row.dimension === 'account' ? { account_id: Number(row.key) } : { model: row.key }, `${row.label} · 请求消耗明细`)
}
const metrics = computed(() => {
  if (!report.value) return []
  const s = report.value.summary
  return [
    { label: '期间实际付款', icon: 'creditCard' as const, value: cashHeadline.value, hint: '充值 + 订阅 · 点击拆分付款与到账', color: '', action: () => orders() },
    { label: '当前用户余额', icon: 'users' as const, value: money(report.value.current_balance), hint: '当前存量 · 含赠送', color: '', action: () => openModal('balance') },
    { label: '用户消耗', icon: 'chartBar' as const, value: money(s.consumption), hint: '期间实际计费额', color: 'text-sky-600', action: () => usage() },
    { label: '上游汇总成本', icon: 'server' as const, value: money(s.cost), hint: '账号统计价 × 账号倍率', color: 'text-amber-600', action: () => showDetails('upstream') },
    { label: '计费毛利', icon: 'trendingUp' as const, value: money(s.profit), hint: '用户消耗 − 上游成本', color: s.profit < 0 ? 'text-red-600' : 'text-emerald-600', action: () => showDetails('day') },
    { label: '计费毛利率', icon: 'chart' as const, value: percent(s.margin), hint: '按汇总金额计算', color: s.profit < 0 ? 'text-red-600' : 'text-emerald-600', action: () => showDetails('account') },
    { label: '标准价消耗', icon: 'cube' as const, value: money(s.list_cost), hint: `${s.requests.toLocaleString()} 次请求`, color: '', action: () => showDetails('model') },
    { label: '请求总量', icon: 'bolt' as const, value: s.requests.toLocaleString(), hint: '含零计费请求 · 查看逐笔记录', color: '', action: () => usage() }
  ]
})
const days = computed(() => (report.value?.rows ?? []).filter(r => r.dimension === 'day').sort((a, b) => a.key.localeCompare(b.key)))
const upstreams = computed(() => (report.value?.rows ?? []).filter(r => r.dimension === 'upstream').sort((a, b) => b.cost - a.cost).slice(0, 10))
const granularity = ref<'day' | 'week' | 'month'>('day')
const periods = computed(() => {
  const buckets = new Map<string, FinanceRow & { endKey: string }>()
  for (const row of days.value) {
    const date = new Date(row.key + 'T00:00:00Z')
    if (granularity.value === 'week') date.setUTCDate(date.getUTCDate() - (date.getUTCDay() + 6) % 7)
    const key = granularity.value === 'month' ? row.key.slice(0, 7) : date.toISOString().slice(0, 10)
    const existing = buckets.get(key)
    if (!existing) buckets.set(key, { ...row, endKey: row.key })
    else {
      for (const field of ['requests', 'consumption', 'cost', 'profit', 'recharge', 'subscription', 'list_cost', 'total_orders', 'paid_orders', 'excluded_recharge'] as const) existing[field] += row[field]
      existing.endKey = row.key
      existing.margin = existing.consumption > 0 ? existing.profit / existing.consumption * 100 : null
    }
  }
  return [...buckets.values()].map(row => ({ ...row, label: row.key === row.endKey ? row.key.slice(5) : `${row.key.slice(5)} ~ ${row.endKey.slice(5)}` }))
})
const trendData = computed(() => ({ labels: periods.value.map(r => r.label), datasets: [
  { label: '用户消耗', data: periods.value.map(r => r.consumption), borderColor: '#6366f1', backgroundColor: '#6366f1', pointRadius: 2, tension: 0.25 },
  { label: '上游成本', data: periods.value.map(r => r.cost), borderColor: '#f59e0b', backgroundColor: '#f59e0b', pointRadius: 2, tension: 0.25 },
  { label: '计费毛利', data: periods.value.map(r => r.profit), borderColor: '#10b981', backgroundColor: '#10b981', pointRadius: 2, tension: 0.25 }
] }))
const upstreamData = computed(() => ({ labels: upstreams.value.map(r => r.label), datasets: [{ label: '上游成本', data: upstreams.value.map(r => r.cost), backgroundColor: '#818cf8', borderRadius: 5, maxBarThickness: 36 }] }))
const cashPeriods = computed(() => periods.value.map(period => paymentTotals(report.value?.payments ?? [], period.key, period.endKey).currencies.find(c => c.currency === cashCurrency.value)))
const rechargeData = computed(() => ({ labels: periods.value.map(r => r.label), datasets: [
  { label: '充值付款', data: cashPeriods.value.map(r => r?.recharge_paid ?? 0), backgroundColor: '#6366f1', borderRadius: 3, maxBarThickness: 32 },
  { label: '订阅付款', data: cashPeriods.value.map(r => r?.subscription_paid ?? 0), backgroundColor: '#2dd4bf', borderRadius: 3, maxBarThickness: 32 }
] }))
const marginData = computed(() => ({ labels: periods.value.map(r => r.label), datasets: [{ label: '计费毛利率 %', data: periods.value.map(r => r.margin), borderColor: '#10b981', backgroundColor: '#10b981', pointRadius: 3, tension: 0.2 }] }))
const lineOptions: ChartOptions<'line'> = { responsive: true, maintainAspectRatio: false, interaction: { mode: 'index', intersect: false }, plugins: { legend: { position: 'bottom', labels: { usePointStyle: true, boxWidth: 7, padding: 20 } } }, scales: { x: { grid: { display: false }, ticks: { maxTicksLimit: 9 } }, y: { title: { display: true, text: 'USD 计费额度' }, grid: { color: '#94a3b81a' } } }, onClick: (_event, elements) => { const row = periods.value[elements[0]?.index ?? -1]; if (row) usage({ start_date: row.key, end_date: row.endKey }, `${row.label} · 请求消耗明细`) } }
const marginOptions: ChartOptions<'line'> = { ...lineOptions, scales: { x: { grid: { display: false }, ticks: { maxTicksLimit: 9 } }, y: { title: { display: true, text: '%' }, grid: { color: '#94a3b81a' } } } }
const barOptions: ChartOptions<'bar'> = { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false } }, scales: { x: { grid: { display: false }, ticks: { maxRotation: 35 } }, y: { beginAtZero: true, grid: { color: '#94a3b81a' }, title: { display: true, text: 'USD' } } }, onClick: (_event, elements) => { const row = upstreams.value[elements[0]?.index ?? -1]; if (row) drill(row) } }
const rechargeOptions = computed<ChartOptions<'bar'>>(() => ({ ...barOptions, plugins: { legend: { position: 'bottom', labels: { usePointStyle: true, boxWidth: 7, padding: 20 } } }, scales: { x: { stacked: true, grid: { display: false }, ticks: { maxTicksLimit: 9 } }, y: { stacked: true, beginAtZero: true, grid: { color: '#94a3b81a' }, title: { display: true, text: cashCurrency.value } } }, onClick: (_event, elements) => { const row = periods.value[elements[0]?.index ?? -1]; if (row) orders(row.key, row.endKey) } } ))
const rankingMetric = ref<'cost' | 'consumption' | 'profit' | 'requests'>('cost')
const rankingLabels = { cost: '上游成本', consumption: '用户消耗', profit: '计费毛利', requests: '请求数' }
const rankedRows = computed(() => [...filteredRows.value].sort((a, b) => b[rankingMetric.value] - a[rankingMetric.value] || a.key.localeCompare(b.key)).slice(0, 12))
const rankingData = computed(() => ({ labels: rankedRows.value.map(r => r.label), datasets: [{ label: rankingLabels[rankingMetric.value], data: rankedRows.value.map(r => r[rankingMetric.value]), backgroundColor: rankedRows.value.map(r => r[rankingMetric.value] < 0 ? '#fb7185' : '#818cf8'), borderRadius: 4, maxBarThickness: 42 }] }))
const rankingOptions = computed<ChartOptions<'bar'>>(() => ({ ...barOptions, scales: { ...barOptions.scales, y: { beginAtZero: true, grid: { color: '#94a3b81a' }, title: { display: true, text: rankingMetric.value === 'requests' ? '请求数' : 'USD' } } }, onClick: (_event, elements) => { const row = rankedRows.value[elements[0]?.index ?? -1]; if (row) drill(row) } }))
const detailTotals = computed(() => {
  const sum = filteredRows.value.reduce((sum, row) => ({ requests: sum.requests + row.requests, consumption: sum.consumption + row.consumption, cost: sum.cost + row.cost, profit: sum.profit + row.profit }), { requests: 0, consumption: 0, cost: 0, profit: 0 })
  return { ...sum, margin: sum.consumption > 0 ? sum.profit / sum.consumption * 100 : null }
})
const insights = computed(() => {
  if (!report.value) return []
  const s = report.value.summary
  const lossAccounts = report.value.rows.filter(r => r.dimension === 'account' && r.profit < 0)
  const top3 = upstreams.value.slice(0, 3).reduce((sum, row) => sum + row.cost, 0)
  return [
    { label: '订单支付率', value: percent(s.total_orders > 0 ? s.paid_orders / s.total_orders * 100 : null), hint: `${s.paid_orders ?? 0} / ${s.total_orders ?? 0} 笔 · 当前成功状态占比`, icon: 'creditCard' as const, action: () => orders() },
    { label: '单次请求成本', value: s.requests ? money(s.cost / s.requests) : '—', hint: '上游成本 ÷ 期间请求数', icon: 'bolt' as const, action: () => usage() },
    { label: 'Top 3 上游成本占比', value: percent(s.cost > 0 ? top3 / s.cost * 100 : null), hint: '观察上游成本集中度', icon: 'server' as const, action: () => showDetails('upstream') },
    { label: '亏损账号', value: `${lossAccounts.length} 个`, hint: `合计毛利 ${money(lossAccounts.reduce((sum, row) => sum + row.profit, 0))}`, icon: 'chartBar' as const, action: () => { showDetails('account'); lossOnly.value = true } }
  ]
})
function exportRows() {
  const cell = (value: unknown) => '"' + String(value ?? '').replace(/^[=+@\-\t\r]/, "'$&").replace(/"/g, '""') + '"'
  const values = [['名称', '上游', ...columns.value.map(c => c.label)], ...filteredRows.value.map(r => [r.label, r.upstream, ...columns.value.map(c => r[c.key])])]
  const url = URL.createObjectURL(new Blob(['\uFEFF' + values.map(r => r.map(cell).join(',')).join('\r\n')], { type: 'text/csv;charset=utf-8;' }))
  const a = document.createElement('a'); a.href = url; a.download = `operations-${dimension.value}-${report.value!.start_date}-${report.value!.end_date}.csv`; a.click(); URL.revokeObjectURL(url)
}
if (typeof route.query.preset === 'string') activePreset.value = Object.keys(presetKeys).find(key => presetKeys[key] === route.query.preset) || '今天'
if (typeof route.query.start_date === 'string' && typeof route.query.end_date === 'string') {
  start.value = route.query.start_date; end.value = route.query.end_date; activePreset.value = ''
}
onMounted(() => { void load(); document.addEventListener('keydown', trapModalFocus) })
onBeforeUnmount(() => { controller?.abort(); document.removeEventListener('keydown', trapModalFocus) })
</script>

<style scoped>
.finance { letter-spacing: 0; --panel-border: #e5e7eb; --panel-bg: #fff; }
:global(.dark .finance) { --panel-border: #293448; --panel-bg: #141d2c; }
.analysis-entry { display:flex; align-items:center; gap:10px; border:1px solid var(--panel-border); border-radius:12px; padding:16px; font-size:13px; text-align:left; transition:background .15s; }
.analysis-entry:hover { background:#6366f10c; color:#6366f1; }
.inventory-card { padding:20px; border:1px solid var(--panel-border); border-radius:14px; background:var(--panel-bg); }
.inventory-card > span { font-size:12px; color:#64748b; }
.inventory-card strong { display:block; margin:12px 0; font-size:24px; font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }
:global(.dark .inventory-card > span), :global(.dark .inventory-card p) { color:#94a3b8; }
.inventory-card p { font-size:12px; line-height:1.8; color:#64748b; }
.finance-filter { display:flex; flex-wrap:wrap; align-items:flex-end; gap:12px; }
.filter-actions { display:flex; gap:8px; flex-shrink:0; }
.finance-hero { display:flex; flex-wrap:wrap; justify-content:space-between; align-items:center; gap:20px; padding:12px 0 8px; }
.hero-icon { display:grid; place-items:center; width:52px; height:52px; border-radius:16px; color:#6366f1; background:linear-gradient(140deg,#e0e7ff,#eef2ff); }
.finance-panel { padding:22px; border:1px solid var(--panel-border); background:var(--panel-bg); border-radius:16px; box-shadow:0 2px 5px #0f172a03; }
.section-heading { display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; gap:12px; }
.section-heading > div:first-child { display:flex; align-items:center; gap:12px; }
.section-heading h2 { font-size:15px; font-weight:650; }
.section-index { font-size:10px; font-weight:700; letter-spacing:1px; color:#818cf8; }
.metric-card { padding:18px 20px; text-align:left; border:1px solid var(--panel-border); border-radius:14px; background:var(--panel-bg); transition:border-color .15s,box-shadow .15s; }
.metric-card:hover { border-color:#a5b4fc; box-shadow:0 5px 18px #6366f110; }
.metric-featured { border-color:#a5b4fc; background:linear-gradient(125deg,#eef2ff,var(--panel-bg)); }
:global(.dark .finance .metric-featured) { background:linear-gradient(125deg,#252b50,var(--panel-bg)); border-color:#4f46e5; }
.metric-icon { padding:7px; color:#818cf8; background:#818cf80d; border-radius:9px; }
.insight-strip { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); border:1px solid var(--panel-border); border-radius:14px; background:var(--panel-bg); overflow:hidden; }
.insight-item { text-align:left; padding:17px 22px; }
.insight-item + .insight-item { border-left:1px solid var(--panel-border); }
.insight-item:hover { background:#818cf808; }
.chart-heading { display:flex; justify-content:space-between; align-items:flex-start; gap:12px; margin-bottom:20px; }
.chart-heading h3 { font-size:14px; font-weight:600; }
.chart-heading p { margin-top:6px; font-size:11px; color:#6b7280; line-height:1.6; }
.unit-badge { flex-shrink:0; font-size:10px; padding:3px 8px; border:1px solid var(--panel-border); border-radius:6px; color:#6b7280; }
.date-presets, .dimension-tabs { display:flex; flex-wrap:wrap; gap:3px; background:#94a3b80d; padding:4px; border-radius:10px; }
.date-presets button, .dimension-tabs button { padding:7px 12px; font-size:12px; border-radius:7px; color:#6b7280; }
.date-presets button.selected, .dimension-tabs button.selected { background:var(--panel-bg); color:#6366f1; box-shadow:0 1px 4px #0f172a12; font-weight:600; }
.finance :deep(button:focus-visible) { outline:2px solid #818cf8; outline-offset:3px; }
@media(max-width:767px) { .analysis-entry { display:flex; align-items:center; gap:10px; border:1px solid var(--panel-border); border-radius:12px; padding:16px; font-size:13px; text-align:left; transition:background .15s; }
.analysis-entry:hover { background:#6366f10c; color:#6366f1; }
.inventory-card { padding:20px; border:1px solid var(--panel-border); border-radius:14px; background:var(--panel-bg); }
.inventory-card > span { font-size:12px; color:#64748b; }
.inventory-card strong { display:block; margin:12px 0; font-size:24px; font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }
:global(.dark .inventory-card > span), :global(.dark .inventory-card p) { color:#94a3b8; }
.inventory-card p { font-size:12px; line-height:1.8; color:#64748b; }
.finance-filter { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1fr); } .finance-filter > div:first-child, .filter-actions { grid-column:1 / -1; } .finance-filter > label { min-width:0; } .finance-filter .input { min-width:0; width:100%; } .filter-actions .btn-primary { flex:1; } .finance-panel { padding:16px; } .insight-strip { grid-template-columns:repeat(2,minmax(0,1fr)); } .insight-item:nth-child(3) { border-left:0; } .insight-item:nth-child(n+3) { border-top:1px solid var(--panel-border); } .insight-item { padding:15px; } }
@media(prefers-reduced-motion:reduce) { .metric-card { transition:none; } }
</style>
