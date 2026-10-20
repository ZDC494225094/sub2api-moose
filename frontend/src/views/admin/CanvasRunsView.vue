<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <form class="flex flex-wrap items-center gap-3" @submit.prevent="searchRuns">
          <h1 class="mr-auto text-lg font-semibold">画布任务</h1>
          <input v-model="search" class="input w-72" placeholder="任务编号、用户、模型或上游任务编号" />
          <select v-model="status" class="input w-36" @change="searchRuns">
            <option value="">全部状态</option>
            <option value="reserved">执行中 / 已预扣</option>
            <option value="succeeded">成功</option>
            <option value="failed">失败 / 已退款</option>
          </select>
          <button class="btn btn-secondary" :disabled="loading">查询 / 刷新</button>
        </form>
        <p v-if="error" role="alert" class="mt-3 text-sm text-red-500">{{ error }}</p>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-id="{ row }">
            <div class="max-w-xs break-all font-mono text-xs">{{ row.id }}</div>
            <div v-if="row.upstream_task_id" class="mt-1 max-w-xs break-all text-xs text-gray-500">上游：{{ row.upstream_task_id }}</div>
            <button class="mt-1 text-primary-600 hover:underline" @click="copyID(row.id)">{{ copied === row.id ? '已复制' : '复制编号' }}</button>
          </template>
          <template #cell-user_email="{ row }">
            <div>{{ row.user_email || '用户已删除' }}</div>
            <div class="text-xs text-gray-500">#{{ row.user_id }}</div>
          </template>
          <template #cell-model="{ row }">
            <div class="max-w-xs break-all">{{ row.model }}</div>
            <div class="text-xs text-gray-500">{{ modes[row.mode] || row.mode }}</div>
          </template>
          <template #cell-status="{ value }">
            <span :class="value === 'failed' ? 'text-red-500' : value === 'succeeded' ? 'text-green-600' : ''">{{ statuses[value] || value }}</span>
          </template>
          <template #cell-phase="{ value }">{{ phases[value] || value || '未记录' }}</template>
          <template #cell-error="{ row }">
            <div class="min-w-64 max-w-xl whitespace-pre-wrap break-words text-sm" :class="row.error ? 'text-red-500' : 'text-gray-500'">{{ row.error || (row.status === 'failed' ? '历史任务未记录失败详情' : '—') }}</div>
          </template>
          <template #cell-created_at="{ row }">
            <div class="whitespace-nowrap">{{ formatDateTime(row.created_at) }}</div>
            <div class="whitespace-nowrap text-xs text-gray-500">更新：{{ formatDateTime(row.updated_at) }}</div>
          </template>
        </DataTable>
      </template>
      <template #pagination>
        <Pagination :total="total" :page="page" :page-size="pageSize" @update:page="changePage" @update:page-size="changePageSize" />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/api/client'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import type { Column } from '@/components/common/types'
import { formatDateTime } from '@/utils/format'

interface CanvasRunRecord {
  id: string
  user_id: number
  user_email: string
  mode: string
  model: string
  status: string
  phase: string
  upstream_task_id: string
  error: string
  created_at: string
  updated_at: string
}
const statuses: Record<string, string> = { reserved: '执行中 / 已预扣', succeeded: '成功', failed: '失败 / 已退款' }
const phases: Record<string, string> = { queued: '等待执行', task_persist: '任务保存', reference_upload: '参考素材转存', upstream_submit: '提交上游', upstream_poll: '等待上游结果', result_store: '保存结果', completed: '已完成' }
const modes: Record<string, string> = { video: '视频', image: '图片', audio: '音频', chat: '对话' }
const columns: Column[] = [
  { key: 'id', label: '任务编号' },
  { key: 'user_email', label: '用户' },
  { key: 'model', label: '模型' },
  { key: 'status', label: '状态' },
  { key: 'phase', label: '执行阶段' },
  { key: 'error', label: '错误详情' },
  { key: 'created_at', label: '创建时间' }
]
const items = ref<CanvasRunRecord[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const search = ref('')
const status = ref('')
const loading = ref(false)
const error = ref('')
const copied = ref('')
let sequence = 0

async function loadRuns() {
  const current = ++sequence
  loading.value = true
  error.value = ''
  try {
    const { data } = await apiClient.get<{ items: CanvasRunRecord[]; total: number }>('/admin/canvas-runs', { params: { page: page.value, page_size: pageSize.value, search: search.value.trim(), status: status.value } })
    if (current !== sequence) return
    items.value = data.items
    total.value = data.total
  } catch {
    if (current === sequence) error.value = '读取画布任务失败，请检查数据库迁移和服务端日志'
  } finally {
    if (current === sequence) loading.value = false
  }
}
function searchRuns() { page.value = 1; void loadRuns() }
function changePage(value: number) { page.value = value; void loadRuns() }
function changePageSize(value: number) { pageSize.value = value; searchRuns() }
async function copyID(value: string) {
  try { await navigator.clipboard.writeText(value); copied.value = value } catch { error.value = '复制失败，请手动复制任务编号' }
}
onMounted(loadRuns)
</script>
