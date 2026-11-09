<script setup lang="ts">
import { onMounted, ref } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import ApplicationDetail from './ApplicationDetail.vue'
import { invoiceAPI, type ApplicationStatus, type InvoiceApplication } from './api'
import { errorMessage, formatMoney, formatTime, invoiceTypeLabels, statusBadge, statusLabels } from './format'

const appStore = useAppStore()
const items = ref<InvoiceApplication[]>([])
const total = ref(0)
const page = ref(1)
const pages = ref(1)
const status = ref<ApplicationStatus | ''>('')
const loading = ref(true)
const error = ref('')
const detail = ref<InvoiceApplication | null>(null)
const cancelling = ref<InvoiceApplication | null>(null)
const downloading = ref<number | null>(null)

const statusOptions: Array<{ value: ApplicationStatus | ''; label: string }> = [
  { value: '', label: '全部' },
  { value: 'pending', label: statusLabels.pending },
  { value: 'processing', label: statusLabels.processing },
  { value: 'issued', label: statusLabels.issued },
  { value: 'rejected', label: statusLabels.rejected },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await invoiceAPI.applications({ page: page.value, page_size: 10, status: status.value })
    items.value = result.items
    total.value = result.total
    pages.value = Math.max(1, result.pages)
  } catch (e) {
    error.value = errorMessage(e, '加载开票记录失败')
  } finally {
    loading.value = false
  }
}

function setStatus(value: ApplicationStatus | '') {
  status.value = value
  page.value = 1
  void load()
}

function go(next: number) {
  page.value = Math.min(Math.max(1, next), pages.value)
  void load()
}

async function openDetail(app: InvoiceApplication) {
  try {
    detail.value = await invoiceAPI.application(app.id)
  } catch (e) {
    appStore.showError(errorMessage(e, '加载申请详情失败'))
  }
}

async function confirmCancel() {
  const target = cancelling.value
  cancelling.value = null
  if (!target) return
  try {
    await invoiceAPI.cancel(target.id)
    appStore.showSuccess('申请已撤销，订单已恢复为可开票')
    if (detail.value?.id === target.id) detail.value = null
    await load()
  } catch (e) {
    appStore.showError(errorMessage(e, '撤销失败'))
    await load()
  }
}

async function download(app: InvoiceApplication) {
  downloading.value = app.id
  try {
    await invoiceAPI.download(app)
  } catch (e) {
    appStore.showError(errorMessage(e, '下载失败，请稍后重试'))
  } finally {
    downloading.value = null
  }
}

defineExpose({ reload: load })
onMounted(load)
</script>

<template>
  <section class="space-y-4">
    <div class="tabs inline-flex flex-wrap" role="tablist" aria-label="按状态筛选">
      <button
        v-for="option in statusOptions"
        :key="option.value || 'all'"
        type="button"
        role="tab"
        class="tab"
        :class="{ 'tab-active': status === option.value }"
        :aria-selected="status === option.value"
        @click="setStatus(option.value)"
      >
        {{ option.label }}
      </button>
    </div>

    <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <div class="card overflow-hidden">
      <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
      <div v-else-if="items.length === 0" class="empty-state">
        <Icon name="document" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">暂无开票记录</p>
        <p class="empty-state-description">在「申请开票」中选择订单并提交后，可以在这里查看进度和下载发票。</p>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead>
            <tr>
              <th>申请时间</th>
              <th>抬头</th>
              <th class="hidden md:table-cell">发票类型</th>
              <th class="!text-right">金额</th>
              <th>状态</th>
              <th class="!text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="app in items" :key="app.id">
              <td class="whitespace-nowrap">
                {{ formatTime(app.created_at) }}
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ app.order_count }} 笔订单</span>
              </td>
              <td class="max-w-[220px] truncate" :title="app.title_name">{{ app.title_name }}</td>
              <td class="hidden md:table-cell">{{ invoiceTypeLabels[app.invoice_type] }}</td>
              <td class="whitespace-nowrap text-right font-medium tabular-nums">{{ formatMoney(app.amount) }}</td>
              <td>
                <span :class="statusBadge[app.status]">{{ statusLabels[app.status] }}</span>
                <span v-if="app.invoice_no" class="mt-1 block font-mono text-xs text-gray-500 dark:text-dark-400">{{ app.invoice_no }}</span>
              </td>
              <td class="whitespace-nowrap text-right">
                <div class="flex justify-end gap-1">
                  <button v-if="app.has_file" type="button" class="btn btn-secondary btn-sm" :disabled="downloading === app.id" @click="download(app)">
                    <Icon name="download" size="sm" />下载
                  </button>
                  <button type="button" class="btn btn-ghost btn-sm" @click="openDetail(app)">详情</button>
                  <button v-if="app.status === 'pending'" type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="cancelling = app">撤销</button>
                </div>
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

    <BaseDialog :show="!!detail" :title="detail ? `开票申请 #${detail.id}` : ''" width="wide" @close="detail = null">
      <ApplicationDetail v-if="detail" :app="detail" />
      <template #footer>
        <div class="flex justify-end gap-2">
          <button v-if="detail?.status === 'pending'" type="button" class="btn btn-secondary text-red-600 dark:text-red-400" @click="cancelling = detail">撤销申请</button>
          <button v-if="detail?.has_file" type="button" class="btn btn-primary" @click="detail && download(detail)"><Icon name="download" size="sm" />下载发票</button>
          <button type="button" class="btn btn-secondary" @click="detail = null">关闭</button>
        </div>
      </template>
    </BaseDialog>
    <ConfirmDialog
      :show="!!cancelling"
      title="撤销这条开票申请？"
      :message="cancelling ? `撤销后其中 ${cancelling.order_count} 笔订单会恢复为可开票，可以重新申请。` : ''"
      confirm-text="撤销申请"
      danger
      @confirm="confirmCancel"
      @cancel="cancelling = null"
    />
  </section>
</template>
