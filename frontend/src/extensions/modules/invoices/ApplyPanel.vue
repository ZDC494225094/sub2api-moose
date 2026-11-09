<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { invoiceAPI, type InvoicePublicSettings, type InvoiceTitle, type InvoiceType, type InvoiceableOrder } from './api'
import {
  centsToAmount, errorMessage, formatMoney, formatTime, invoiceTypeLabels, orderTypeLabels, paymentTypeLabels, sumCents, titleTypeLabels,
} from './format'

const props = defineProps<{ settings: InvoicePublicSettings }>()
const emit = defineEmits<{ applied: []; manageTitles: [] }>()

const appStore = useAppStore()
const orders = ref<InvoiceableOrder[]>([])
const truncated = ref(false)
const titles = ref<InvoiceTitle[]>([])
const loading = ref(true)
const error = ref('')
const selected = ref<Set<number>>(new Set())
const invoiceType = ref<InvoiceType>('normal')
const titleId = ref<number | null>(null)
const remark = ref('')
const submitting = ref(false)

const MAX_ORDERS = 200

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [list, savedTitles] = await Promise.all([invoiceAPI.orders(), invoiceAPI.titles()])
    orders.value = list.items
    truncated.value = list.truncated
    titles.value = savedTitles
    selected.value = new Set([...selected.value].filter((id) => list.items.some((o) => o.id === id)))
    if (!titles.value.some((t) => t.id === titleId.value)) {
      titleId.value = (titles.value.find((t) => t.is_default) ?? titles.value[0])?.id ?? null
    }
  } catch (e) {
    error.value = errorMessage(e, '加载可开票订单失败')
  } finally {
    loading.value = false
  }
}

const selectedOrders = computed(() => orders.value.filter((o) => selected.value.has(o.id)))
const selectedCents = computed(() => sumCents(selectedOrders.value.map((o) => o.amount)))
const thresholdCents = computed(() => sumCents([props.settings.min_amount]))
const shortfallCents = computed(() => Math.max(0, thresholdCents.value - selectedCents.value))
const progress = computed(() => (thresholdCents.value <= 0 ? 100 : Math.min(100, Math.round((selectedCents.value / thresholdCents.value) * 100))))
const allSelected = computed(() => orders.value.length > 0 && orders.value.slice(0, MAX_ORDERS).every((o) => selected.value.has(o.id)))
const selectedTitle = computed(() => titles.value.find((t) => t.id === titleId.value) ?? null)

const specialProblem = computed(() => {
  if (invoiceType.value !== 'special') return ''
  const t = selectedTitle.value
  if (!t) return ''
  if (t.title_type !== 'company') return '增值税专用发票只能开给企业抬头'
  if (!t.address || !t.phone || !t.bank_name || !t.bank_account) return '该抬头缺少注册地址、电话、开户银行或银行账号，请先补全'
  return ''
})

const blocker = computed(() => {
  if (selected.value.size === 0) return '请选择要开票的订单'
  if (selected.value.size > MAX_ORDERS) return `一次最多选择 ${MAX_ORDERS} 笔订单`
  if (shortfallCents.value > 0) return `还差 ${formatMoney(centsToAmount(shortfallCents.value))} 达到开票门槛`
  if (!selectedTitle.value) return '请选择发票抬头'
  return specialProblem.value
})

watch(() => props.settings.allow_special, (allowed) => {
  if (!allowed) invoiceType.value = 'normal'
})

function toggle(id: number) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set(orders.value.slice(0, MAX_ORDERS).map((o) => o.id))
}

async function submit() {
  if (blocker.value || !selectedTitle.value) return
  submitting.value = true
  try {
    await invoiceAPI.apply({
      order_ids: [...selected.value], title_id: selectedTitle.value.id, invoice_type: invoiceType.value, remark: remark.value.trim(),
    })
    appStore.showSuccess('开票申请已提交，我们会尽快处理')
    selected.value = new Set()
    remark.value = ''
    emit('applied')
    await load()
  } catch (e) {
    appStore.showError(errorMessage(e, '提交失败，请稍后重试'))
    await load()
  } finally {
    submitting.value = false
  }
}

defineExpose({ reload: load })
onMounted(load)
</script>

<template>
  <section class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-start">
    <div class="card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">可开票订单</h2>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
            仅显示站内支付完成、未退款的人民币订单<template v-if="settings.order_window_days > 0">（近 {{ settings.order_window_days }} 天）</template>，可多笔合并开一张发票。
          </p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />刷新</button>
      </div>

      <p v-if="error" role="alert" class="m-5 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
      <div v-else-if="orders.length === 0" class="empty-state">
        <Icon name="inbox" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">暂无可开票订单</p>
        <p class="empty-state-description">充值或购买套餐完成后，订单会出现在这里。已申请开票、已退款的订单不会重复出现。</p>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead>
            <tr>
              <th class="w-10">
                <input type="checkbox" class="h-4 w-4 rounded" :checked="allSelected" aria-label="全选订单" @change="toggleAll" />
              </th>
              <th>订单号</th>
              <th>类型</th>
              <th class="hidden sm:table-cell">支付方式</th>
              <th class="hidden md:table-cell">支付时间</th>
              <th class="!text-right">金额</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="o in orders" :key="o.id" class="cursor-pointer" :class="{ 'bg-primary-50/60 dark:bg-primary-900/10': selected.has(o.id) }" @click="toggle(o.id)">
              <td @click.stop>
                <input type="checkbox" class="h-4 w-4 rounded" :checked="selected.has(o.id)" :aria-label="`选择订单 ${o.out_trade_no}`" @change="toggle(o.id)" />
              </td>
              <td class="font-mono text-xs">{{ o.out_trade_no || `#${o.id}` }}</td>
              <td>{{ orderTypeLabels[o.order_type] || o.order_type }}</td>
              <td class="hidden sm:table-cell">{{ paymentTypeLabels[o.payment_type] || o.payment_type }}</td>
              <td class="hidden whitespace-nowrap md:table-cell">{{ formatTime(o.paid_at) }}</td>
              <td class="whitespace-nowrap text-right font-medium tabular-nums">{{ formatMoney(o.amount) }}</td>
            </tr>
          </tbody>
        </table>
        <p v-if="truncated" class="border-t border-gray-100 px-5 py-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
          仅显示最近 500 笔订单。开票后剩余订单会继续显示。
        </p>
      </div>
    </div>

    <aside class="card space-y-5 p-5 lg:sticky lg:top-24">
      <div>
        <p class="text-sm text-gray-500 dark:text-dark-400">已选 {{ selected.size }} 笔订单</p>
        <p class="mt-1 text-3xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatMoney(centsToAmount(selectedCents)) }}</p>
        <template v-if="thresholdCents > 0">
          <div class="progress mt-3 h-1.5" role="progressbar" :aria-valuenow="progress" aria-valuemin="0" aria-valuemax="100" aria-label="开票门槛进度">
            <div class="progress-bar h-full" :style="{ width: `${progress}%` }"></div>
          </div>
          <p class="mt-2 text-xs" :class="shortfallCents > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'">
            {{ shortfallCents > 0 ? `开票门槛 ${formatMoney(settings.min_amount)}，还差 ${formatMoney(centsToAmount(shortfallCents))}` : `已达到开票门槛 ${formatMoney(settings.min_amount)}` }}
          </p>
        </template>
      </div>

      <div v-if="settings.allow_special">
        <span class="input-label">发票类型</span>
        <div class="tabs flex" role="radiogroup" aria-label="发票类型">
          <button
            v-for="kind in (['normal', 'special'] as const)"
            :key="kind"
            type="button"
            role="radio"
            class="tab flex-1"
            :class="{ 'tab-active': invoiceType === kind }"
            :aria-checked="invoiceType === kind"
            @click="invoiceType = kind"
          >
            {{ kind === 'normal' ? '普通发票' : '专用发票' }}
          </button>
        </div>
      </div>
      <p v-else class="text-sm text-gray-600 dark:text-dark-300">发票类型：{{ invoiceTypeLabels.normal }}</p>

      <div>
        <div class="mb-1.5 flex items-center justify-between">
          <span class="input-label !mb-0">发票抬头</span>
          <button type="button" class="text-xs font-medium text-primary-600 hover:underline dark:text-primary-400" @click="emit('manageTitles')">管理抬头</button>
        </div>
        <div v-if="titles.length === 0" class="rounded-xl border border-dashed border-gray-200 p-4 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
          还没有抬头，<button type="button" class="font-medium text-primary-600 hover:underline dark:text-primary-400" @click="emit('manageTitles')">去添加</button>
        </div>
        <div v-else class="space-y-2" role="radiogroup" aria-label="发票抬头">
          <label
            v-for="t in titles"
            :key="t.id"
            class="flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors"
            :class="titleId === t.id ? 'border-primary-400 bg-primary-50/50 dark:border-primary-600 dark:bg-primary-900/10' : 'border-gray-100 hover:border-gray-200 dark:border-dark-700 dark:hover:border-dark-600'"
          >
            <input v-model="titleId" type="radio" class="mt-1 h-4 w-4" :value="t.id" name="invoice-title" />
            <span class="min-w-0">
              <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">{{ t.name }}</span>
              <span class="block truncate text-xs text-gray-500 dark:text-dark-400">{{ titleTypeLabels[t.title_type] }}{{ t.tax_no ? ` · ${t.tax_no}` : '' }}</span>
            </span>
          </label>
        </div>
        <p v-if="specialProblem" class="mt-2 text-xs text-amber-600 dark:text-amber-400">{{ specialProblem }}</p>
      </div>

      <label class="block">
        <span class="input-label">备注</span>
        <textarea v-model="remark" class="input min-h-[72px]" maxlength="200" placeholder="选填，例如需要备注的项目名称"></textarea>
      </label>

      <div class="space-y-2">
        <button type="button" class="btn btn-primary w-full" :disabled="!!blocker || submitting" @click="submit">
          {{ submitting ? '提交中…' : '提交开票申请' }}
        </button>
        <p v-if="blocker && selected.size > 0" class="text-center text-xs text-gray-500 dark:text-dark-400">{{ blocker }}</p>
        <p v-if="settings.item_name" class="text-center text-xs text-gray-500 dark:text-dark-400">开票内容：{{ settings.item_name }}</p>
      </div>
    </aside>
  </section>
</template>
