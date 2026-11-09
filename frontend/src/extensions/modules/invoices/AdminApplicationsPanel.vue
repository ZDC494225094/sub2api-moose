<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import ReviewDialog from './ReviewDialog.vue'
import { invoiceAdminAPI, type ApplicationStatus, type InvoiceApplication } from './api'
import { errorMessage, formatMoney, formatTime, invoiceTypeLabels, statusBadge, statusLabels, titleTypeLabels } from './format'

const appStore = useAppStore()
const items = ref<InvoiceApplication[]>([])
const counts = ref<Partial<Record<ApplicationStatus, number>>>({})
const total = ref(0)
const page = ref(1)
const pages = ref(1)
// Pending first: that is the work queue.
const status = ref<ApplicationStatus | ''>('pending')
const search = ref('')
const loading = ref(true)
const error = ref('')
const reviewing = ref<InvoiceApplication | null>(null)

const statusTabs: Array<ApplicationStatus | ''> = ['pending', 'processing', 'issued', 'rejected', 'voided', 'cancelled', '']

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await invoiceAdminAPI.applications({ page: page.value, page_size: 20, status: status.value, q: search.value })
    items.value = result.items
    counts.value = result.counts ?? {}
    total.value = result.total
    pages.value = Math.max(1, result.pages)
  } catch (e) {
    error.value = errorMessage(e, '加载开票申请失败')
  } finally {
    loading.value = false
  }
}

function setStatus(value: ApplicationStatus | '') {
  status.value = value
  page.value = 1
  void load()
}

function runSearch() {
  page.value = 1
  void load()
}

function go(next: number) {
  page.value = Math.min(Math.max(1, next), pages.value)
  void load()
}

async function open(app: InvoiceApplication) {
  try {
    reviewing.value = await invoiceAdminAPI.application(app.id)
  } catch (e) {
    appStore.showError(errorMessage(e, '加载申请详情失败'))
  }
}

function onChanged(updated: InvoiceApplication) {
  reviewing.value = updated
  void load()
}

function tabLabel(value: ApplicationStatus | '') {
  if (!value) return '全部'
  const n = counts.value[value]
  return n ? `${statusLabels[value]} ${n}` : statusLabels[value]
}

onMounted(load)
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <div class="tabs inline-flex flex-wrap" role="tablist" aria-label="按状态筛选">
        <button
          v-for="value in statusTabs"
          :key="value || 'all'"
          type="button"
          role="tab"
          class="tab"
          :class="{ 'tab-active': status === value }"
          :aria-selected="status === value"
          @click="setStatus(value)"
        >
          {{ tabLabel(value) }}
        </button>
      </div>
      <form class="flex gap-2" @submit.prevent="runSearch">
        <input v-model="search" class="input w-full lg:w-72" placeholder="搜索申请号、用户邮箱、抬头、税号、发票号" />
        <button type="submit" class="btn btn-secondary"><Icon name="search" size="sm" />搜索</button>
      </form>
    </div>

    <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <div class="card overflow-hidden">
      <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
      <div v-else-if="items.length === 0" class="empty-state">
        <Icon name="inbox" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">{{ status === 'pending' ? '没有待处理的申请' : '没有符合条件的申请' }}</p>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead>
            <tr>
              <th>申请</th>
              <th>用户</th>
              <th>抬头</th>
              <th class="hidden lg:table-cell">类型</th>
              <th class="!text-right">金额</th>
              <th>状态</th>
              <th class="!text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="app in items" :key="app.id">
              <td class="whitespace-nowrap">
                <span class="font-mono text-xs text-gray-500 dark:text-dark-400">#{{ app.id }}</span>
                <span class="block">{{ formatTime(app.created_at) }}</span>
              </td>
              <td class="max-w-[200px] truncate" :title="app.user_email">{{ app.user_email || `ID ${app.user_id}` }}</td>
              <td class="max-w-[240px]">
                <span class="block truncate" :title="app.title_name">{{ app.title_name }}</span>
                <span class="block truncate text-xs text-gray-500 dark:text-dark-400">{{ titleTypeLabels[app.title_type] }}{{ app.tax_no ? ` · ${app.tax_no}` : '' }}</span>
              </td>
              <td class="hidden whitespace-nowrap lg:table-cell">{{ invoiceTypeLabels[app.invoice_type] }}</td>
              <td class="whitespace-nowrap text-right tabular-nums">
                <span class="font-medium">{{ formatMoney(app.amount) }}</span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ app.order_count }} 笔</span>
              </td>
              <td class="whitespace-nowrap">
                <span :class="statusBadge[app.status]">{{ statusLabels[app.status] }}</span>
                <span v-if="app.refund_alert" class="badge badge-danger ml-1" title="有订单在申请后发生退款">退款</span>
              </td>
              <td class="whitespace-nowrap text-right">
                <button type="button" class="btn btn-sm" :class="app.status === 'pending' || app.status === 'processing' ? 'btn-primary' : 'btn-secondary'" @click="open(app)">
                  {{ app.status === 'pending' || app.status === 'processing' ? '处理' : '查看' }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="pages > 1" class="flex items-center justify-between border-t border-gray-100 px-5 py-3 text-sm dark:border-dark-700">
        <span class="text-gray-500 dark:text-dark-400">共 {{ total }} 条</span>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="go(page - 1)">上一页</button>
          <span class="tabular-nums text-gray-600 dark:text-dark-300">{{ page }} / {{ pages }}</span>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pages" @click="go(page + 1)">下一页</button>
        </div>
      </div>
    </div>

    <ReviewDialog :app="reviewing" @close="reviewing = null" @changed="onChanged" />
  </section>
</template>
