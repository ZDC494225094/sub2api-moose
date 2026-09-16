<template>
  <AppLayout>
    <div class="finance space-y-6">
      <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-600">
        <div><h1 class="text-xl font-semibold">运营分析</h1><p class="mt-1 text-xs text-gray-500">{{ timezone }} · USD 计费额度</p></div>
        <button v-if="report" type="button" class="flex items-center gap-1 text-sm text-primary-600" @click="customers?.open('repeat')">用户经营与留存 <Icon name="arrowRight" size="sm" /></button>
      </header>
      <form class="flex flex-wrap items-end gap-3" @submit.prevent="load">
        <div class="flex flex-wrap gap-1" role="group" aria-label="日期快捷选择">
          <button v-for="preset in presets" :key="preset.label" type="button" class="btn btn-secondary text-xs" :aria-pressed="activePreset === preset.label" :class="activePreset === preset.label ? '!border-primary-500 !text-primary-600' : ''" @click="setRange(preset.days, preset.label)">{{ preset.label }}</button>
        </div>
        <label class="text-xs text-gray-500">开始日期<input v-model="start" type="date" required class="input mt-1" :max="end" @input="activePreset = ''"></label>
        <label class="text-xs text-gray-500">结束日期<input v-model="end" type="date" required class="input mt-1" :min="start" @input="activePreset = ''"></label>
        <button class="btn btn-primary" type="submit" :disabled="loading">查询</button>
        <button class="btn btn-secondary" type="button" title="刷新数据" aria-label="刷新数据" :disabled="loading" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" /></button>
      </form>
      <div v-if="error" role="alert" class="border-l-4 border-red-500 bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/20">{{ error }}</div>
      <div v-if="loading" role="status" class="py-8 text-center text-sm text-gray-500">正在汇总经营数据…</div>
      <template v-else-if="report">
        <div class="flex flex-wrap justify-between gap-2 text-xs text-gray-500"><span>{{ report.start_date }} 至 {{ report.end_date }}</span><span>更新于 {{ new Date(report.generated_at).toLocaleString('zh-CN', { timeZone: timezone }) }}</span></div>
        <div class="grid grid-cols-2 gap-x-5 gap-y-6 border-b border-gray-200 pb-6 xl:grid-cols-4 dark:border-dark-600">
          <button v-for="metric in metrics" :key="metric.label" type="button" class="min-w-0 text-left" @click="metric.action()">
            <span class="text-xs text-gray-500">{{ metric.label }} <Icon name="chevronRight" size="xs" class="inline" /></span>
            <span class="mt-2 block whitespace-pre-line break-words text-2xl font-semibold tabular-nums" :class="metric.color">{{ metric.value }}</span>
            <span class="mt-1 block text-xs text-gray-500">{{ metric.hint }}</span>
          </button>
        </div>
        <div class="grid min-w-0 gap-6 xl:grid-cols-3">
          <section class="min-w-0 xl:col-span-2"><h2 class="mb-4 text-sm font-semibold">每日消耗、成本与毛利</h2><div class="h-72"><Line :data="trendData" :options="lineOptions" /></div></section>
          <section class="min-w-0"><h2 class="mb-4 text-sm font-semibold">上游成本排行 · Top 10</h2><div v-if="upstreams.length" class="h-72"><Bar :data="upstreamData" :options="barOptions" /></div><div v-else class="py-20 text-center text-sm text-gray-500">暂无上游消耗</div></section>
        </div>
        <div class="grid min-w-0 gap-6 border-t border-gray-200 pt-5 xl:grid-cols-2 dark:border-dark-600">
          <section class="min-w-0"><h2 class="mb-4 text-sm font-semibold">充值额度（含订阅）趋势</h2><div class="h-56"><Bar :data="rechargeData" :options="rechargeOptions" /></div></section>
          <section class="min-w-0"><h2 class="mb-4 text-sm font-semibold">每日计费毛利率</h2><div class="h-56"><Line :data="marginData" :options="marginOptions" /></div></section>
        </div>
        <section ref="detailsSection" class="min-w-0 border-t border-gray-200 pt-5 dark:border-dark-600">
          <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
            <div class="flex flex-wrap gap-4" role="tablist" aria-label="分析维度"><button v-for="item in dimensions" :key="item.key" role="tab" class="border-b-2 pb-2 text-sm" :class="dimension === item.key ? 'border-primary-500 text-primary-600' : 'border-transparent text-gray-500'" :aria-selected="dimension === item.key" @click="selectDimension(item.key)">{{ item.label }}</button></div>
            <div class="flex flex-wrap items-center gap-2"><label class="flex items-center gap-1 text-xs"><input v-model="lossOnly" type="checkbox">仅看亏损</label><input v-model="search" type="search" class="input max-w-48" placeholder="搜索名称" aria-label="搜索明细"><button class="btn btn-secondary" title="导出当前维度 CSV" aria-label="导出当前维度 CSV" @click="exportRows"><Icon name="download" size="sm" /></button></div>
          </div>
          <div v-if="upstreamFilter" class="mb-3 flex items-center gap-2 text-sm"><span>上游：{{ upstreamFilter }}</span><button class="text-primary-600" @click="upstreamFilter = ''; page = 1">清除筛选</button></div>
          <div class="overflow-x-auto"><table class="w-full whitespace-nowrap text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="p-3 text-left">{{ dimensions.find(d => d.key === dimension)?.label }}</th><th v-if="dimension === 'account'" class="p-3 text-left">上游</th><th v-for="column in columns" :key="column.key" class="p-3 text-right"><button @click="sortBy(column.key)">{{ column.label }} {{ sortKey === column.key ? (descending ? '↓' : '↑') : '' }}</button></th><th class="p-3 text-right">明细</th></tr></thead>
            <tbody><tr v-for="row in pageRows" :key="row.key" class="border-b border-gray-100 dark:border-dark-700"><td class="max-w-64 truncate p-3" :title="row.label">{{ row.label }}</td><td v-if="dimension === 'account'" class="max-w-48 truncate p-3">{{ row.upstream }}</td><td v-for="column in columns" :key="column.key" class="p-3 text-right tabular-nums" :class="column.key === 'profit' && row.profit < 0 ? 'text-red-600' : ''">{{ column.key === 'requests' ? row.requests.toLocaleString() : column.key === 'margin' ? percent(row.margin) : money(row[column.key]) }}</td><td class="p-3 text-right"><button class="text-primary-600" @click="drill(row)">{{ dimension === 'upstream' ? '账号明细' : '请求明细' }}</button><button v-if="dimension === 'day'" class="ml-3 text-primary-600" @click="orders(row.key)">订单</button></td></tr><tr v-if="!filteredRows.length"><td :colspan="columns.length + 3" class="p-10 text-center text-gray-500">暂无符合条件的数据</td></tr></tbody>
          </table></div>
          <div class="mt-4 flex items-center justify-end gap-3 text-xs text-gray-500"><span>共 {{ filteredRows.length }} 项 · {{ page }} / {{ totalPages }}</span><button class="btn btn-secondary" :disabled="page <= 1" aria-label="上一页" @click="page--"><Icon name="chevronLeft" size="sm" /></button><button class="btn btn-secondary" :disabled="page >= totalPages" aria-label="下一页" @click="page++"><Icon name="chevronRight" size="sm" /></button></div>
        </section>
        <footer class="border-t border-gray-200 pt-4 text-xs leading-6 text-gray-500 dark:border-dark-600">统计日期和时间范围由服务器业务时区确定。期间充值额度与订单管理保持一致：按订单创建时间、当前日期范围、余额充值和订阅充值订单、全部订单状态汇总订单 amount。消耗、成本和毛利采用 USD 计费口径；毛利 = 实际计费额 − 账号统计价 × 账号倍率，毛利率按汇总金额相除，非现金净利润。余额为当前存量；上游归属为账号当前分组。</footer>
        <OperationsCustomers ref="customers" :start-date="report.start_date" :end-date="report.end_date" :timezone="timezone" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, BarElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line, Bar } from 'vue-chartjs'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import OperationsCustomers from './components/OperationsCustomers.vue'
import { getOperationsFinance, type FinanceReport, type FinanceRow } from '@/api/admin/operationsFinance'

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
  report.value = null; error.value = ''; loading.value = false
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
const detailsSection = ref<HTMLElement>()
const customers = ref<InstanceType<typeof OperationsCustomers>>()
const lossOnly = ref(false)
const columns = computed<{ key: NumericKey; label: string }[]>(() => [
  ...(dimension.value === 'day' ? [{ key: 'recharge' as const, label: '充值额度' }] : []),
  { key: 'requests', label: '请求数' }, { key: 'consumption', label: '用户消耗' }, { key: 'list_cost', label: '标准价消耗' }, { key: 'cost', label: '上游成本' }, { key: 'profit', label: '计费毛利' }, { key: 'margin', label: '毛利率' }
])
const filteredRows = computed(() => (report.value?.rows ?? []).filter(r => r.dimension === dimension.value && (!lossOnly.value || r.profit < 0) && (!upstreamFilter.value || r.upstream === upstreamFilter.value) && r.label.toLowerCase().includes(search.value.toLowerCase())).sort((a, b) => ((a[sortKey.value] ?? -Infinity) - (b[sortKey.value] ?? -Infinity)) * (descending.value ? -1 : 1) || a.key.localeCompare(b.key)))
const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / 20)))
const pageRows = computed(() => filteredRows.value.slice((page.value - 1) * 20, page.value * 20))
watch([search, lossOnly], () => { page.value = 1 })
function sortBy(key: NumericKey) { descending.value = sortKey.value === key ? !descending.value : true; sortKey.value = key; page.value = 1 }
function selectDimension(key: Dimension) { dimension.value = key; upstreamFilter.value = ''; search.value = ''; page.value = 1; if (!columns.value.some(c => c.key === sortKey.value)) sortKey.value = 'cost' }
function showDetails(key: Dimension) { selectDimension(key); detailsSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }
function usage(extra: Record<string, string> = {}) { void router.push({ path: '/admin/usage', query: { start_date: report.value!.start_date, end_date: report.value!.end_date, timezone: timezone.value, ...extra } }) }
function orders(day?: string) { void router.push({ path: '/admin/orders', query: { date_field: 'created_at', start_date: day ?? report.value!.start_date, end_date: day ?? report.value!.end_date } }) }
function drill(row: FinanceRow) {
  if (row.dimension === 'upstream') { selectDimension('account'); upstreamFilter.value = row.key }
  else if (row.dimension === 'day') usage({ start_date: row.key, end_date: row.key })
  else usage(row.dimension === 'account' ? { account_id: row.key } : { model: row.key })
}
const metrics = computed(() => {
  if (!report.value) return []
  const s = report.value.summary
  return [
    { label: '期间充值额度', value: money(s.recharge), hint: '余额充值 + 订阅充值 · 创建时间 · 全部状态', color: '', action: () => orders() },
    { label: '当前用户余额', value: money(report.value.current_balance), hint: '当前存量 · 含赠送', color: '', action: () => customers.value?.open('balance') },
    { label: '用户消耗', value: money(s.consumption), hint: '期间实际计费额', color: 'text-sky-600', action: () => usage() },
    { label: '上游汇总成本', value: money(s.cost), hint: '账号统计价 × 账号倍率', color: 'text-amber-600', action: () => showDetails('upstream') },
    { label: '计费毛利', value: money(s.profit), hint: '用户消耗 − 上游成本', color: s.profit < 0 ? 'text-red-600' : 'text-emerald-600', action: () => showDetails('day') },
    { label: '计费毛利率', value: percent(s.margin), hint: '按汇总金额计算', color: s.profit < 0 ? 'text-red-600' : 'text-emerald-600', action: () => showDetails('account') },
    { label: '标准价消耗', value: money(s.list_cost), hint: `${s.requests.toLocaleString()} 次请求`, color: '', action: () => showDetails('model') }
  ]
})
const days = computed(() => (report.value?.rows ?? []).filter(r => r.dimension === 'day').sort((a, b) => a.key.localeCompare(b.key)))
const upstreams = computed(() => (report.value?.rows ?? []).filter(r => r.dimension === 'upstream').sort((a, b) => b.cost - a.cost).slice(0, 10))
const trendData = computed(() => ({ labels: days.value.map(r => r.key), datasets: [
  { label: '用户消耗', data: days.value.map(r => r.consumption), borderColor: '#0284c7', backgroundColor: '#0284c7', pointRadius: 2 },
  { label: '上游成本', data: days.value.map(r => r.cost), borderColor: '#d97706', backgroundColor: '#d97706', pointRadius: 2 },
  { label: '计费毛利', data: days.value.map(r => r.profit), borderColor: '#059669', backgroundColor: '#059669', pointRadius: 2 }
] }))
const upstreamData = computed(() => ({ labels: upstreams.value.map(r => r.label), datasets: [{ label: '上游成本', data: upstreams.value.map(r => r.cost), backgroundColor: '#d97706' }] }))
const rechargeData = computed(() => ({ labels: days.value.map(r => r.key), datasets: [{ label: '充值额度（含订阅）', data: days.value.map(r => r.recharge), backgroundColor: '#0284c7' }] }))
const marginData = computed(() => ({ labels: days.value.map(r => r.key), datasets: [{ label: '计费毛利率', data: days.value.map(r => r.margin), borderColor: '#059669', backgroundColor: '#059669', pointRadius: 2, spanGaps: false }] }))
const marginOptions: ChartOptions<'line'> = { responsive: true, maintainAspectRatio: false, plugins: { legend: { display: false }, tooltip: { callbacks: { label: context => percent(context.parsed.y) } } }, scales: { y: { title: { display: true, text: '%' }, ticks: { callback: value => `${value}%` } } }, onClick: (_event, elements) => { const row = days.value[elements[0]?.index ?? -1]; if (row) drill(row) } }
const lineOptions: ChartOptions<'line'> = { responsive: true, maintainAspectRatio: false, interaction: { mode: 'index', intersect: false }, plugins: { legend: { position: 'bottom' } }, onClick: (_event, elements) => { const row = days.value[elements[0]?.index ?? -1]; if (row) drill(row) }, scales: { y: { title: { display: true, text: 'USD' } } } }
const barOptions: ChartOptions<'bar'> = { responsive: true, maintainAspectRatio: false, indexAxis: 'y', plugins: { legend: { display: false } }, onClick: (_event, elements) => { const row = upstreams.value[elements[0]?.index ?? -1]; if (row) { drill(row); detailsSection.value?.scrollIntoView({ behavior: 'smooth' }) } } }
const rechargeOptions: ChartOptions<'bar'> = { responsive: true, maintainAspectRatio: false, plugins: { legend: { position: 'bottom' } }, scales: { y: { title: { display: true, text: 'USD' } } }, onClick: (_event, elements) => { const row = days.value[elements[0]?.index ?? -1]; if (row) orders(row.key) } }
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
onMounted(load)
onBeforeUnmount(() => controller?.abort())
</script>

<style scoped>
.finance { letter-spacing: 0; }
.finance button { border-radius: 6px; }
</style>
