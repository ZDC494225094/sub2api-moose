<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { invoiceAdminAPI, type AccessEntry, type AccessMode } from './api'
import AddAccessDialog from './AddAccessDialog.vue'
import { errorMessage, formatMoney, formatTime } from './format'

defineProps<{ mode: AccessMode | null }>()
const emit = defineEmits<{ switchToSettings: [] }>()
const appStore = useAppStore()
const entries = ref<AccessEntry[]>([])
const loading = ref(true)
const error = ref('')
const search = ref('')
const showAdd = ref(false)
const removing = ref<AccessEntry | null>(null)
const editing = ref<AccessEntry | null>(null)
const inherit = ref(true)
const amount = ref<string | number>('')
const note = ref('')
const saving = ref(false)
const editError = ref('')
const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  return entries.value.filter((entry) => !q || `${entry.user_id} ${entry.email} ${entry.username} ${entry.note}`.toLowerCase().includes(q))
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    entries.value = await invoiceAdminAPI.access()
  } catch (e) {
    error.value = errorMessage(e, '加载开放名单失败')
  } finally {
    loading.value = false
  }
}

function edit(entry: AccessEntry) {
  editing.value = entry
  inherit.value = entry.min_amount == null
  amount.value = entry.min_amount ?? ''
  note.value = entry.note
  editError.value = ''
}

async function save() {
  if (!editing.value) return
  editError.value = ''
  const raw = String(amount.value).trim()
  if (!inherit.value && (!/^\d+(\.\d{1,2})?$/.test(raw) || Number(raw) > 100000000)) {
    editError.value = '请输入 0 到 1 亿元之间的金额，最多两位小数'
    return
  }
  saving.value = true
  try {
    await invoiceAdminAPI.updateAccess(editing.value.user_id, { note: note.value.trim(), min_amount: inherit.value ? null : raw })
    editing.value = null
    appStore.showSuccess('用户开票设置已保存')
    await load()
  } catch (e) {
    editError.value = errorMessage(e, '保存失败')
  } finally {
    saving.value = false
  }
}

async function confirmRemove() {
  const target = removing.value
  removing.value = null
  if (!target) return
  try {
    await invoiceAdminAPI.removeAccess(target.user_id)
    appStore.showSuccess('已移出开放名单')
    await load()
  } catch (e) {
    appStore.showError(errorMessage(e, '移除失败'))
  }
}

onMounted(load)
</script>

<template>
  <section class="space-y-4">
    <div v-if="mode === 'all'" class="flex flex-wrap items-center gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/40 dark:bg-amber-950/30 dark:text-amber-300">
      <Icon name="exclamationTriangle" size="sm" class="shrink-0" />
      <span class="min-w-0 flex-1">当前对全部用户开放。名单中的独立门槛仍然生效；切换为「指定用户」后，仅名单内用户可以申请。</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('switchToSettings')">去设置</button>
    </div>
    <div class="card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">开放名单 <span class="ml-2 text-sm font-normal text-gray-500">{{ entries.length }} 人</span></h2>
        <div class="flex flex-wrap gap-2">
          <input v-model="search" class="input w-60" aria-label="搜索开放名单" placeholder="搜索邮箱、用户名、ID 或备注" />
          <button type="button" class="btn btn-primary" @click="showAdd = true"><Icon name="plus" size="sm" />添加用户</button>
        </div>
      </div>
      <p v-if="error" role="alert" class="m-5 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
      <div v-else-if="filtered.length === 0" class="empty-state">
        <Icon name="users" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">{{ search.trim() ? '没有匹配的用户' : '名单为空' }}</p>
        <p class="empty-state-description">点击「添加用户」搜索并批量选择需要开放开票功能的用户。</p>
      </div>
      <div v-else class="overflow-x-auto">
        <table class="table">
          <thead><tr><th>用户</th><th>开票门槛</th><th class="hidden md:table-cell">备注</th><th class="hidden lg:table-cell">加入时间</th><th class="!text-right">操作</th></tr></thead>
          <tbody>
            <tr v-for="entry in filtered" :key="entry.user_id">
              <td class="max-w-[260px]">
                <span class="block truncate">{{ entry.email || '（用户已删除）' }}</span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">ID {{ entry.user_id }}{{ entry.username ? ` · ${entry.username}` : '' }}</span>
              </td>
              <td class="whitespace-nowrap">{{ entry.min_amount == null ? '跟随全局设置' : Number(entry.min_amount) === 0 ? '不限金额' : formatMoney(entry.min_amount) }}</td>
              <td class="hidden max-w-[220px] truncate md:table-cell">{{ entry.note || '—' }}</td>
              <td class="hidden whitespace-nowrap lg:table-cell">{{ formatTime(entry.created_at) }}</td>
              <td class="whitespace-nowrap text-right">
                <button type="button" class="btn btn-secondary btn-sm mr-2" @click="edit(entry)">设置门槛</button>
                <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="removing = entry">移出</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
    <AddAccessDialog :show="showAdd" :existing-ids="entries.map((entry) => entry.user_id)" @close="showAdd = false" @added="load" />
    <BaseDialog :show="!!editing" :title="`用户开票设置 · ${editing?.email || editing?.user_id || ''}`" @close="!saving && (editing = null)">
      <form id="invoice-user-settings" class="space-y-4" @submit.prevent="save">
        <label class="flex items-center gap-2"><input v-model="inherit" type="checkbox" />跟随全局开票门槛</label>
        <label v-if="!inherit" class="block">
          <span class="input-label">独立开票门槛（元）</span>
          <input v-model="amount" class="input" type="number" min="0" max="100000000" step="0.01" required />
          <span class="input-hint">0 表示不限金额。独立门槛优先于全局设置，对该用户的每次新申请生效。</span>
        </label>
        <label class="block"><span class="input-label">备注</span><input v-model="note" class="input" maxlength="200" /></label>
        <p v-if="editError" role="alert" class="text-sm text-red-600">{{ editError }}</p>
      </form>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="editing = null">取消</button>
        <button type="submit" form="invoice-user-settings" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
      </template>
    </BaseDialog>
    <ConfirmDialog
      :show="!!removing"
      :title="removing ? `将 ${removing.email || `ID ${removing.user_id}`} 移出名单？` : ''"
      message="移出后会清除该用户的独立门槛。指定用户模式下，该用户不能再提交新申请；全部用户模式下，该用户恢复全局门槛。已有申请和发票会保留。"
      confirm-text="移出"
      danger
      @confirm="confirmRemove"
      @cancel="removing = null"
    />
  </section>
</template>
