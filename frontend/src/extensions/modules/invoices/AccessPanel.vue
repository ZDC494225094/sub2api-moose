<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { invoiceAdminAPI, type AccessEntry, type AccessMode } from './api'
import { errorMessage, formatTime } from './format'

defineProps<{ mode: AccessMode | null }>()
const emit = defineEmits<{ switchToSettings: [] }>()

const appStore = useAppStore()
const entries = ref<AccessEntry[]>([])
const loading = ref(true)
const error = ref('')
const users = ref('')
const note = ref('')
const adding = ref(false)
const missing = ref<string[]>([])
const removing = ref<AccessEntry | null>(null)

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

async function add() {
  const refs = users.value.split(/[\s,，;；]+/).map((v) => v.trim()).filter(Boolean)
  if (refs.length === 0) return
  adding.value = true
  missing.value = []
  try {
    const result = await invoiceAdminAPI.addAccess(refs, note.value.trim())
    missing.value = result.missing
    if (result.added > 0) appStore.showSuccess(`已开放给 ${result.added} 个用户`)
    users.value = result.missing.join('\n')
    if (result.missing.length === 0) note.value = ''
    await load()
  } catch (e) {
    appStore.showError(errorMessage(e, '添加失败'))
  } finally {
    adding.value = false
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
      <span class="min-w-0 flex-1">当前开放范围是「全部用户」，名单暂不生效。改为「指定用户」后只有名单内的用户能看到开票入口。</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('switchToSettings')">去设置</button>
    </div>

    <div class="grid gap-6 lg:grid-cols-[360px_minmax(0,1fr)] lg:items-start">
      <form class="card space-y-4 p-5" @submit.prevent="add">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">添加用户</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">每行一个用户邮箱或用户 ID，也可以用逗号分隔，一次最多 200 个。</p>
        </div>
        <textarea v-model="users" class="input min-h-[140px] font-mono text-sm" placeholder="alice@example.com&#10;10086"></textarea>
        <label class="block">
          <span class="input-label">备注</span>
          <input v-model="note" class="input" maxlength="200" placeholder="例如：企业客户 · 月结" />
        </label>
        <p v-if="missing.length" class="rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">
          以下用户不存在或已注销，未添加：{{ missing.join('、') }}
        </p>
        <button type="submit" class="btn btn-primary w-full" :disabled="adding || !users.trim()">{{ adding ? '添加中…' : '开放开票功能' }}</button>
      </form>

      <div class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">开放名单</h2>
          <span class="text-sm text-gray-500 dark:text-dark-400">{{ entries.length }} 人</span>
        </div>
        <p v-if="error" role="alert" class="m-5 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
        <div v-if="loading" class="p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
        <div v-else-if="entries.length === 0" class="empty-state">
          <Icon name="users" size="xl" class="empty-state-icon" />
          <p class="empty-state-title">名单为空</p>
          <p class="empty-state-description">在左侧添加用户后，这些用户的控制台会出现「发票管理」入口。</p>
        </div>
        <div v-else class="overflow-x-auto">
          <table class="table">
            <thead><tr><th>用户</th><th class="hidden md:table-cell">备注</th><th class="hidden sm:table-cell">加入时间</th><th class="!text-right">操作</th></tr></thead>
            <tbody>
              <tr v-for="entry in entries" :key="entry.user_id">
                <td class="max-w-[260px]">
                  <span class="block truncate">{{ entry.email || '（用户已删除）' }}</span>
                  <span class="block text-xs text-gray-500 dark:text-dark-400">ID {{ entry.user_id }}{{ entry.username ? ` · ${entry.username}` : '' }}</span>
                </td>
                <td class="hidden max-w-[220px] truncate md:table-cell">{{ entry.note || '—' }}</td>
                <td class="hidden whitespace-nowrap sm:table-cell">{{ formatTime(entry.created_at) }}</td>
                <td class="text-right">
                  <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="removing = entry">移出</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="!!removing"
      :title="removing ? `将 ${removing.email || `ID ${removing.user_id}`} 移出名单？` : ''"
      message="移出后该用户不能再提交新的开票申请；已提交的申请和已开具的发票仍会保留，可在审核列表中继续处理。"
      confirm-text="移出"
      danger
      @confirm="confirmRemove"
      @cancel="removing = null"
    />
  </section>
</template>
