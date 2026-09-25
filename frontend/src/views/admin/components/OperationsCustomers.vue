<template>
  <section ref="section" class="operations-customers">
    <div class="customers-heading mb-4">
      <h2 class="text-sm font-semibold">用户经营与留存</h2>
      <label class="customers-period text-xs text-gray-500">流失观察周期
        <select v-model.number="churnDays" class="input w-24" @change="reload"><option v-for="days in [7, 30, 60, 90]" :key="days" :value="days">{{ days }} 天</option></select>
      </label>
    </div>
    <div v-if="result" class="customers-metrics mb-5">
      <button v-for="metric in metrics" :key="metric.label" class="min-w-0 text-left" @click="open(metric.segment)">
        <span class="text-xs text-gray-500">{{ metric.label }} <Icon name="chevronRight" size="xs" class="inline" /></span>
        <strong class="mt-1 block text-xl tabular-nums" :class="metric.segment === 'churned' ? 'text-amber-600' : ''">{{ metric.value }}</strong>
      </button>
    </div>
    <div class="customers-toolbar mb-3">
      <label class="customers-segment text-xs text-gray-500">用户群
        <select v-model="segment" class="input w-36" @change="reload"><option v-for="item in segments" :key="item.key" :value="item.key">{{ item.label }}</option></select>
      </label>
      <form class="customers-search" @submit.prevent="applySearch">
        <input v-model="searchInput" type="search" placeholder="邮箱或用户名" aria-label="搜索用户" class="input">
        <button class="btn btn-secondary" type="submit">搜索</button>
      </form>
      <button class="customers-refresh btn btn-secondary" title="刷新用户数据" aria-label="刷新用户数据" :disabled="loading" @click="load"><Icon name="refresh" size="sm" /></button>
    </div>
    <p v-if="error" role="alert" class="py-4 text-sm text-red-600">{{ error }}</p>
    <p v-else-if="loading" role="status" class="py-8 text-center text-sm text-gray-500">正在加载用户数据…</p>
    <template v-else-if="result">
      <div class="customers-table-scroll" tabindex="0" role="region" aria-label="用户经营明细">
        <table class="w-full whitespace-nowrap text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr>
            <th class="p-3 text-left">用户</th><th v-for="column in columns" :key="column.key" class="p-3 text-right">{{ column.label }}</th><th class="p-3 text-left">最后使用</th><th class="p-3 text-left">最后付款</th><th class="p-3 text-right">明细</th>
          </tr></thead>
          <tbody><tr v-for="user in result.items" :key="user.id" class="border-b border-gray-100 dark:border-dark-700">
            <td class="p-3"><span class="block max-w-56 truncate" :title="user.email">{{ user.email }}</span><span class="text-xs text-gray-500">{{ user.username || `#${user.id}` }}</span></td>
            <td v-for="column in columns" :key="column.key" class="p-3 text-right tabular-nums" :class="column.key === 'profit' && user.profit < 0 ? 'text-red-600' : ''">{{ column.key === 'margin' ? percent(user.margin) : column.key === 'period_orders' || column.key === 'total_orders' ? user[column.key] : money(user[column.key]) }}</td>
            <td class="p-3 text-xs">{{ date(user.last_used_at) }}</td><td class="p-3 text-xs">{{ date(user.last_paid_at) }}</td>
            <td class="p-3 text-right"><button class="text-primary-600" @click="emit('usage', { id: user.id, email: user.email })">消耗明细</button></td>
          </tr><tr v-if="!result.items.length"><td colspan="12" class="p-8 text-center text-gray-500">暂无符合条件的用户</td></tr></tbody>
        </table>
      </div>
      <div class="mt-3 flex items-center justify-end gap-3 text-xs text-gray-500"><span>共 {{ result.total }} 人 · {{ page }} / {{ totalPages }}</span><button class="btn btn-secondary" :disabled="page <= 1" aria-label="上一页用户" @click="changePage(-1)"><Icon name="chevronLeft" size="sm" /></button><button class="btn btn-secondary" :disabled="page >= totalPages" aria-label="下一页用户" @click="changePage(1)"><Icon name="chevronRight" size="sm" /></button></div>
      <p class="mt-4 text-xs leading-6 text-gray-500">截至 {{ date(result.as_of) }}。回购用户：期间有付款且累计至少两次付款；回购率 = 回购用户 ÷ 期间付款用户。首购用户可能在同一期间再次购买。流失用户：前 {{ churnDays * 2 }} 至 {{ churnDays }} 天有使用，最近 {{ churnDays }} 天无使用；流失率分母为前一观察周期活跃用户。活跃使用排除无计费、无 Token 的失败占位请求。累计付款及最后活动截至统计时点，余额为当前存量。</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getOperationsCustomers, type CustomersReport, type CustomerSegment } from '@/api/admin/operationsFinance'

const emit = defineEmits<{ usage: [user: { id: number; email: string }] }>()
const props = defineProps<{ startDate: string; endDate: string; timezone: string }>()
const section = ref<HTMLElement>(), result = ref<CustomersReport | null>(null)
const segment = ref<CustomerSegment>('all'), churnDays = ref(30), page = ref(1)
const searchInput = ref(''), search = ref(''), loading = ref(false), error = ref('')
let controller: AbortController | undefined
const segments: { key: CustomerSegment; label: string }[] = [
  { key: 'all', label: '全部用户' }, { key: 'balance', label: '有余额用户' }, { key: 'paying', label: '期间付款用户' },
  { key: 'repeat', label: '回购用户' }, { key: 'new_paying', label: '首购用户' }, { key: 'active', label: '期间活跃用户' }, { key: 'churned', label: '流失用户' }
]
const columns = [
  { key: 'balance', label: '当前余额' }, { key: 'period_orders', label: '期间付款次数' }, { key: 'total_orders', label: '累计付款次数' },
  { key: 'consumption', label: '期间消耗' }, { key: 'cost', label: '成本' },
  { key: 'profit', label: '计费毛利' }, { key: 'margin', label: '毛利率' }
] as const
const money = (n: number) => '$' + n.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 })
const percent = (n: number | null) => n == null ? '—' : n.toFixed(2) + '%'
const date = (value: string | null) => value ? new Date(value).toLocaleString('zh-CN', { timeZone: props.timezone }) : '—'
const totalPages = computed(() => Math.max(1, Math.ceil((result.value?.total ?? 0) / 20)))
const metrics = computed<{ label: string; value: string; segment: CustomerSegment }[]>(() => {
  if (!result.value) return []
  const s = result.value.summary
  return [
    { label: '期间付款用户', value: s.paying.toLocaleString(), segment: 'paying' },
    { label: '期间首购用户', value: s.new_paying.toLocaleString(), segment: 'new_paying' },
    { label: '回购用户', value: s.repeat.toLocaleString(), segment: 'repeat' },
    { label: '回购率', value: percent(s.repeat_rate), segment: 'repeat' },
    { label: '期间活跃用户', value: s.active.toLocaleString(), segment: 'active' },
    { label: '流失用户', value: s.churned.toLocaleString(), segment: 'churned' },
    { label: '流失率', value: percent(s.churn_rate), segment: 'churned' },
    { label: '有余额用户', value: s.balance_users.toLocaleString(), segment: 'balance' }
  ]
})
async function load() {
  controller?.abort()
  const request = new AbortController(); controller = request
  loading.value = true; error.value = ''; result.value = null
  try {
    const data = await getOperationsCustomers({ start_date: props.startDate, end_date: props.endDate, timezone: props.timezone, segment: segment.value, churn_days: churnDays.value, page: page.value, search: search.value }, request.signal)
    if (controller === request) result.value = data
  } catch { if (!request.signal.aborted) error.value = '用户经营数据加载失败，请重试。' }
  finally { if (controller === request) loading.value = false }
}
function reload() { page.value = 1; void load() }
function applySearch() { search.value = searchInput.value.trim(); reload() }
function changePage(delta: number) { page.value += delta; void load() }
function open(value: CustomerSegment) { segment.value = value; search.value = ''; searchInput.value = ''; reload(); section.value?.scrollIntoView({ behavior: 'smooth', block: 'start' }) }
watch(() => [props.startDate, props.endDate, props.timezone], reload, { immediate: true })
onBeforeUnmount(() => controller?.abort())
defineExpose({ open })
</script>

<style scoped>
.operations-customers {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  contain: inline-size;
  container-type: inline-size;
}
.customers-heading, .customers-period, .customers-segment {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.customers-heading { justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.customers-period, .customers-segment { white-space: nowrap; }
.customers-period select { width: 96px; flex: 0 0 96px; }
.customers-segment select { width: 144px; min-width: 0; }
.customers-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 20px 24px; }
.customers-metrics > button { min-height: 64px; overflow-wrap: anywhere; }
.customers-toolbar { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; }
.customers-search { display: flex; align-items: center; flex: 0 1 340px; min-width: 0; gap: 8px; }
.customers-search .input { width: 0; min-width: 0; flex: 1 1 0%; }
.customers-search button { flex: 0 0 auto; white-space: nowrap; }
.customers-refresh { flex: 0 0 40px; width: 40px; height: 40px; padding: 0; }
.customers-table-scroll { width: 100%; max-width: 100%; min-width: 0; overflow-x: auto; }
@container (max-width: 720px) {
  .customers-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .customers-toolbar { display: grid; grid-template-columns: minmax(0, 1fr) 40px; }
  .customers-segment { grid-column: 1 / -1; }
  .customers-search { width: 100%; }
}
</style>
