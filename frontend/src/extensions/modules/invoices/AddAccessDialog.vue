<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import usersAPI from '@/api/admin/users'
import type { AdminUser } from '@/types'
import { useAppStore } from '@/stores/app'
import { invoiceAdminAPI } from './api'
import { errorMessage } from './format'

const props = defineProps<{ show: boolean; existingIds: number[] }>()
const emit = defineEmits<{ close: []; added: [] }>()
const appStore = useAppStore()
const query = ref('')
const page = ref(1)
const pages = ref(1)
const total = ref(0)
const items = ref<AdminUser[]>([])
const selected = ref(new Map<number, string>())
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const note = ref('')
let controller: AbortController | undefined
let timer: ReturnType<typeof setTimeout> | undefined
const available = computed(() => items.value.filter((user) => !props.existingIds.includes(user.id)))
const allChecked = computed(() => available.value.length > 0 && available.value.every((user) => selected.value.has(user.id)))

async function load() {
  controller?.abort()
  const request = new AbortController()
  controller = request
  loading.value = true
  error.value = ''
  try {
    const result = await usersAPI.list(page.value, 20, { search: query.value.trim(), include_subscriptions: false }, { signal: request.signal })
    if (request.signal.aborted) return
    items.value = result.items
    pages.value = Math.max(1, result.pages)
    total.value = result.total
  } catch (e) {
    if (!request.signal.aborted) error.value = errorMessage(e, '搜索用户失败')
  } finally {
    if (!request.signal.aborted) loading.value = false
  }
}

watch(() => props.show, (show) => {
  clearTimeout(timer)
  controller?.abort()
  if (!show) return
  query.value = ''
  page.value = 1
  selected.value = new Map()
  note.value = ''
  items.value = []
  void load()
}, { immediate: true })

watch(query, () => {
  clearTimeout(timer)
  controller?.abort()
  page.value = 1
  items.value = []
  loading.value = true
  if (props.show) timer = setTimeout(() => void load(), 300)
})

function toggle(id: number, email: string) {
  if (selected.value.has(id)) selected.value.delete(id)
  else if (selected.value.size < 200) selected.value.set(id, email)
  else error.value = '一次最多添加 200 个用户'
}

function togglePage() {
  if (allChecked.value) available.value.forEach((user) => selected.value.delete(user.id))
  else for (const user of available.value) {
    if (!selected.value.has(user.id)) toggle(user.id, user.email)
  }
}

function go(next: number) {
  clearTimeout(timer)
  page.value = next
  void load()
}

async function add() {
  if (!selected.value.size) return
  saving.value = true
  error.value = ''
  try {
    const result = await invoiceAdminAPI.addAccess([...selected.value.keys()].map(String), note.value.trim())
    emit('added')
    if (result.added > 0) appStore.showSuccess(`已开放给 ${result.added} 个用户`)
    if (result.missing.length) {
      const missing = new Set(result.missing)
      selected.value = new Map([...selected.value].filter(([id]) => missing.has(String(id))))
      error.value = `以下用户已不存在或已注销：${result.missing.join('、')}`
    } else emit('close')
  } catch (e) {
    error.value = errorMessage(e, '添加失败')
  } finally {
    saving.value = false
  }
}

onUnmounted(() => { clearTimeout(timer); controller?.abort() })
</script>

<template>
  <BaseDialog :show="show" title="添加开放用户" width="wide" @close="!saving && emit('close')">
    <div class="space-y-4">
      <input v-model="query" class="input" placeholder="输入邮箱或用户名模糊搜索" aria-label="搜索用户" />
      <p class="text-xs text-gray-500">支持跨页、跨搜索保留勾选，一次最多 200 人。已在开放名单内的用户无需重复添加。</p>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <div v-if="loading" class="py-8 text-center text-sm text-gray-500">正在搜索…</div>
      <div v-else-if="!items.length" class="py-8 text-center text-sm text-gray-500">没有匹配的用户</div>
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead><tr><th><input type="checkbox" aria-label="全选当前页" :checked="allChecked" :disabled="!available.length || saving" @change="togglePage" /></th><th>邮箱</th><th>用户名 / ID</th><th>状态</th></tr></thead>
          <tbody>
            <tr v-for="user in items" :key="user.id">
              <td><input type="checkbox" :aria-label="`选择 ${user.email}`" :checked="existingIds.includes(user.id) || selected.has(user.id)" :disabled="existingIds.includes(user.id) || saving" @change="toggle(user.id, user.email)" /></td>
              <td>{{ user.email }}</td><td>{{ user.username || '—' }} <span class="text-xs text-gray-500">#{{ user.id }}</span></td>
              <td>{{ existingIds.includes(user.id) ? '已开放' : user.status === 'disabled' ? '账户已禁用' : '未开放' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="pages > 1" class="flex items-center justify-between gap-2 text-sm">
        <span>共 {{ total }} 人 · {{ page }} / {{ pages }} 页</span>
        <div class="flex gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1 || loading" @click="go(page - 1)">上一页</button><button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pages || loading" @click="go(page + 1)">下一页</button></div>
      </div>
      <div v-if="selected.size" class="flex max-h-28 flex-wrap gap-2 overflow-y-auto" aria-label="已选用户">
        <button v-for="[id, email] in selected" :key="id" type="button" class="badge badge-gray" :aria-label="`取消选择 ${email}`" :disabled="saving" @click="selected.delete(id)">{{ email }} ×</button>
      </div>
      <label class="block"><span class="input-label">备注（选填）</span><input v-model="note" class="input" maxlength="200" placeholder="例如：企业客户 · 月结" /></label>
    </div>
    <template #footer>
      <span class="mr-auto text-sm text-gray-500">已选 {{ selected.size }} 人</span>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="emit('close')">取消</button>
      <button type="button" class="btn btn-primary" :disabled="saving || !selected.size" @click="add">{{ saving ? '添加中…' : '添加所选用户' }}</button>
    </template>
  </BaseDialog>
</template>
