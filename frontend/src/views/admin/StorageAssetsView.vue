<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <form class="flex flex-wrap items-center gap-3" @submit.prevent="searchAssets">
          <h1 class="mr-auto text-lg font-semibold">存储素材</h1>
          <input v-model="search" class="input w-72" placeholder="用户邮箱、用户 ID 或对象路径" />
          <button class="btn btn-secondary" :disabled="loading">查询 / 刷新</button>
        </form>
        <p v-if="error" role="alert" class="mt-3 text-sm text-red-500">{{ error }}</p>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-user_email="{ row }">
            <div>{{ row.user_email || (row.user_id ? '用户已删除' : '未知用户') }}</div>
            <div v-if="row.user_id" class="text-xs text-gray-500">#{{ row.user_id }}</div>
          </template>
          <template #cell-object_key="{ row }">
            <div class="max-w-xs truncate" :title="row.object_key">{{ row.object_key }}</div>
            <div class="text-xs text-gray-500">{{ row.bucket }} · {{ row.content_type }} · {{ formatBytes(row.bytes) }}</div>
          </template>
          <template #cell-uploaded_at="{ value }">{{ formatDateTime(value) }}</template>
          <template #cell-url="{ row }">
            <a v-if="safeURL(row.url)" :href="row.url" target="_blank" rel="noopener noreferrer" class="block max-w-sm truncate text-primary-600 hover:underline" :title="row.url">{{ row.url }}</a>
            <span v-else class="text-gray-500">存储桶配置已变更</span>
          </template>
          <template #cell-actions="{ row }">
            <div v-if="safeURL(row.url)" class="flex items-center gap-3 whitespace-nowrap">
              <a :href="row.url" target="_blank" rel="noopener noreferrer" class="text-primary-600 hover:underline">打开</a>
              <button class="text-primary-600 hover:underline" @click="copyURL(row)">{{ copied === row.id ? '已复制' : '复制链接' }}</button>
            </div>
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

interface StorageAsset {
  id: number
  user_id: number | null
  user_email: string
  bucket: string
  object_key: string
  content_type: string
  bytes: number
  uploaded_at: string
  url: string
}

const columns: Column[] = [
  { key: 'user_email', label: '用户' },
  { key: 'object_key', label: '素材' },
  { key: 'uploaded_at', label: '上传时间' },
  { key: 'url', label: '访问链接' },
  { key: 'actions', label: '操作' }
]
const items = ref<StorageAsset[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const search = ref('')
const loading = ref(false)
const error = ref('')
const copied = ref<number | null>(null)
let sequence = 0

function safeURL(value: string) {
  try { return ['http:', 'https:'].includes(new URL(value).protocol) } catch { return false }
}
function formatBytes(bytes: number) {
  return bytes >= 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${(bytes / 1024).toFixed(1)} KB`
}
async function loadAssets() {
  const current = ++sequence
  loading.value = true
  error.value = ''
  try {
    const { data } = await apiClient.get<{ items: StorageAsset[]; total: number }>('/admin/backups/storage-assets', { params: { page: page.value, page_size: pageSize.value, search: search.value.trim() } })
    if (current !== sequence) return
    items.value = data.items
    total.value = data.total
  } catch {
    if (current === sequence) error.value = '读取素材失败，请检查对象存储配置后重试'
  } finally {
    if (current === sequence) loading.value = false
  }
}
function searchAssets() { page.value = 1; void loadAssets() }
function changePage(value: number) { page.value = value; void loadAssets() }
function changePageSize(value: number) { pageSize.value = value; searchAssets() }
async function copyURL(row: StorageAsset) {
  try { await navigator.clipboard.writeText(row.url); copied.value = row.id } catch { error.value = '复制失败，请手动复制访问链接' }
}
onMounted(loadAssets)
</script>
