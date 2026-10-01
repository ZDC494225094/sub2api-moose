<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { extensionAPI, type ExtensionState } from '../api'
import type { ExtensionId } from '../catalog'
import { useExtensionStore } from '../store'

const store = useExtensionStore()
const items = ref<ExtensionState[]>([])
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const query = ref('')
const pending = ref<ExtensionState | null>(null)
const pendingTarget = ref(false)
const stateUnavailable = computed(() => !store.loaded || !!store.error)
const confirmationMessage = computed(() => !pending.value ? '' : pendingTarget.value
  ? '开启后恢复此扩展的入口及新业务处理；原有权限和功能自身设置仍然生效。'
  : pending.value.disable_behavior)
function askToggle(item: ExtensionState) {
  pending.value = item
  pendingTarget.value = !store.enabled(item.id as ExtensionId)
}
const managed = computed(() => items.value.filter(item => item.managed))
const unmanaged = computed(() => items.value.filter(item => !item.managed))
const visible = (list: ExtensionState[]) => list.filter(item =>
  `${item.name} ${item.id} ${item.description}`.toLowerCase().includes(query.value.trim().toLowerCase()))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const { data } = await extensionAPI.list()
    items.value = data.items
    await store.refresh(true)
  } catch {
    error.value = '加载二开插件失败，请检查管理员权限或服务连接后重试。'
  } finally { loading.value = false }
}

async function confirm() {
  const item = pending.value
  if (!item || saving.value) return
  pending.value = null
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    await store.setEnabled(item.id as ExtensionId, pendingTarget.value)
    notice.value = `${item.name}已${store.enabled(item.id as ExtensionId) ? '开启' : '关闭'}。服务端新请求立即生效，其他页面在约 15 秒内同步。`
  } catch {
    error.value = '保存失败，开关未确认变更。请刷新状态后重试。'
    await store.refresh(true)
  } finally { saving.value = false }
}

onMounted(load)
</script>

<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-200 pb-5 dark:border-dark-700">
        <div>
          <div class="mb-2 flex items-center gap-2 text-xs font-medium uppercase tracking-widest text-primary-600 dark:text-primary-400">
            <Icon name="grid" size="sm" /> Business extensions
          </div>
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">二开插件管理</h2>
          <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            集中管理独立业务扩展，不覆盖上游功能。关闭只停止新入口，不删除业务数据，也不会终止已有订单的履约。
          </p>
        </div>
        <button class="btn btn-secondary" type="button" :disabled="loading || saving" @click="load">
          <Icon name="refresh" size="sm" /> 刷新状态
        </button>
      </header>

      <div class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/20 dark:text-amber-200">
        <strong>当前是内置扩展开关，不是全部二开的可卸载插件。</strong>
        已接入的功能具备前端入口和服务端门禁；核心计费、订阅等仍在拆分清单中。
        开关不能消除已有源代码差异，升级仍需合并和回归验证。上游的 OAuth 传输插件管理保持独立。
      </div>

      <div v-if="error || store.error" role="alert" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error || store.error }}</div>
      <div v-if="notice && !error && !store.error" role="status" class="rounded-lg bg-green-50 p-4 text-sm text-green-800 dark:bg-green-950/30 dark:text-green-300">{{ notice }}</div>
      <div v-if="loading" class="py-12 text-center text-sm text-gray-500">正在读取插件状态…</div>
      <template v-else>
        <div class="flex flex-wrap items-center justify-between gap-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">
            <span class="font-semibold text-gray-900 dark:text-gray-100">{{ managed.length }}</span> 项已接入开关
            <span class="mx-2">/</span>
            <span class="font-semibold text-gray-900 dark:text-gray-100">{{ unmanaged.length }}</span> 项待解耦
          </p>
          <label class="w-full sm:w-72">
            <span class="sr-only">搜索二开功能</span>
            <input v-model="query" class="input w-full" type="search" placeholder="搜索名称或插件 ID" />
          </label>
        </div>

        <section aria-labelledby="managed-heading">
          <h3 id="managed-heading" class="mb-3 text-sm font-semibold text-gray-900 dark:text-gray-100">已接入开关</h3>
          <div class="grid gap-4 lg:grid-cols-2">
            <article v-for="item in visible(managed)" :key="item.id" class="flex flex-col rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0">
                  <h4 class="font-semibold text-gray-900 dark:text-white">{{ item.name }}</h4>
                  <code class="mt-1 block text-xs text-gray-400">{{ item.id }}</code>
                </div>
                <button
                  type="button" role="switch" :aria-checked="store.enabled(item.id as ExtensionId)"
                  :aria-label="`${item.name}开关`" :disabled="saving || stateUnavailable"
                  class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-wait disabled:opacity-50"
                  :class="store.enabled(item.id as ExtensionId) ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600'"
                  @click="askToggle(item)"
                >
                  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform" :class="store.enabled(item.id as ExtensionId) ? 'translate-x-6' : 'translate-x-1'" />
                </button>
              </div>
              <p class="mt-4 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.description }}</p>
              <p class="mt-3 flex-1 border-t border-gray-100 pt-3 text-xs leading-5 text-gray-500 dark:border-dark-700 dark:text-gray-400">关闭行为：{{ item.disable_behavior }}</p>
              <div class="mt-3 flex flex-wrap gap-2 text-xs">
                <span :class="store.enabled(item.id as ExtensionId) ? 'text-green-600 dark:text-green-400' : 'text-gray-500'">{{ stateUnavailable ? '状态未知' : store.enabled(item.id as ExtensionId) ? '● 已开启' : '○ 已关闭' }}</span>
                <span class="text-gray-400">前后端联动 · 保留数据 · 内置扩展</span>
              </div>
            </article>
          </div>
        </section>

        <section aria-labelledby="pending-heading">
          <h3 id="pending-heading" class="mb-3 text-sm font-semibold text-gray-900 dark:text-gray-100">待解耦的核心改动</h3>
          <div class="divide-y divide-gray-100 overflow-hidden rounded-xl border border-gray-200 dark:divide-dark-700 dark:border-dark-700">
            <article v-for="item in visible(unmanaged)" :key="item.id" class="p-4 sm:flex sm:items-start sm:gap-6">
              <div class="mb-2 shrink-0 sm:mb-0 sm:w-48">
                <h4 class="text-sm font-medium text-gray-900 dark:text-gray-100">{{ item.name }}</h4>
                <span class="text-xs text-amber-600 dark:text-amber-400">待解耦 · 尚不可切换</span>
              </div>
              <div class="text-xs leading-5 text-gray-500 dark:text-gray-400"><p>{{ item.description }}</p><p class="mt-1">{{ item.disable_behavior }}</p></div>
            </article>
          </div>
        </section>
      </template>
    </div>
    <ConfirmDialog
      :show="!!pending" :title="pending ? `${pendingTarget ? '开启' : '关闭'}${pending.name}？` : ''"
      :message="confirmationMessage" :danger="!!pending && !pendingTarget"
      @confirm="confirm" @cancel="pending = null"
    />
  </AppLayout>
</template>
