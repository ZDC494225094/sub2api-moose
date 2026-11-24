<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import TargetDialog from './TargetDialog.vue'
import { iqMonitorAdminAPI, type ConfigView, type Options, type Target } from './api'
import { errorMessage, formatMultiplier } from './format'

const props = defineProps<{ config: ConfigView; options: Options | null }>()
const emit = defineEmits<{ goto: [tab: 'settings'] }>()

const appStore = useAppStore()
const items = ref<Target[]>([])
const max = ref(30)
const loading = ref(true)
const error = ref('')
const dialogOpen = ref(false)
const editing = ref<Target | null>(null)
const deleting = ref<Target | null>(null)
const busy = ref(false)

const groupOf = (id: number) => props.options?.groups.find(g => g.id === id)
const keyName = (id: number) => {
  const k = props.options?.keys.find(k => k.id === id)
  return k ? k.name || `#${k.id}` : `#${id}（不属于探测账号）`
}
const full = computed(() => items.value.length >= max.value)

async function load() {
  loading.value = true
  try {
    const res = await iqMonitorAdminAPI.targets()
    items.value = res.items
    max.value = res.max
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e, '读取监测目标失败')
  } finally {
    loading.value = false
  }
}

function openEdit(t: Target | null) {
  editing.value = t
  dialogOpen.value = true
}

async function onSaved() {
  dialogOpen.value = false
  appStore.showSuccess('已保存')
  await load()
}

async function move(index: number, delta: number) {
  const next = [...items.value]
  const [item] = next.splice(index, 1)
  next.splice(index + delta, 0, item)
  const before = items.value
  items.value = next
  try {
    await iqMonitorAdminAPI.reorderTargets(next.map(t => t.id))
  } catch (e) {
    items.value = before
    appStore.showError(errorMessage(e, '调整顺序失败'))
  }
}

async function setEnabled(t: Target, enabled: boolean) {
  busy.value = true
  try {
    await iqMonitorAdminAPI.saveTarget({ group_id: t.group_id, model: t.model, display_name: t.display_name, reasoning_effort: t.reasoning_effort, api_key_id: t.api_key_id, enabled }, t.id)
    t.enabled = enabled
  } catch (e) {
    appStore.showError(errorMessage(e, '保存失败'))
  } finally {
    busy.value = false
  }
}

async function confirmDelete() {
  const t = deleting.value
  if (!t) return
  try {
    await iqMonitorAdminAPI.deleteTarget(t.id)
    deleting.value = null
    await load()
  } catch (e) {
    appStore.showError(errorMessage(e, '删除失败'))
  }
}

onMounted(load)
</script>

<template>
  <section class="card">
    <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">监测目标</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
          每个目标是「分组 + 模型 + 思考深度」，用户页面按这里的顺序展示。最多 {{ max }} 个，当前 {{ items.length }} 个。
        </p>
      </div>
      <button type="button" class="btn btn-primary" :disabled="full || !options?.probe_user" @click="openEdit(null)">添加目标</button>
    </div>
    <p v-if="!options?.probe_user" class="mx-5 mt-4 rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">
      还没有选择探测账号，<button type="button" class="font-medium underline" @click="emit('goto', 'settings')">去设置</button>。
    </p>
    <p v-if="error" role="alert" class="m-5 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <div v-if="loading" class="p-5 text-sm text-gray-500">正在加载…</div>
    <p v-else-if="!items.length" class="p-5 text-sm text-gray-500">还没有监测目标。</p>
    <ul v-else class="divide-y divide-gray-100 dark:divide-dark-700">
      <li v-for="(t, i) in items" :key="t.id" class="flex flex-wrap items-center gap-3 px-5 py-3" :class="t.enabled ? '' : 'opacity-60'">
        <div class="flex flex-col">
          <button type="button" class="rounded px-1 text-gray-400 hover:text-gray-700 disabled:opacity-30 dark:hover:text-white" :disabled="i === 0" aria-label="上移" @click="move(i, -1)">▲</button>
          <button type="button" class="rounded px-1 text-gray-400 hover:text-gray-700 disabled:opacity-30 dark:hover:text-white" :disabled="i === items.length - 1" aria-label="下移" @click="move(i, 1)">▼</button>
        </div>
        <ModelIcon :model="t.model" size="22px" />
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
            {{ t.display_name || t.model }}
            <span v-if="t.display_name" class="ml-1 font-mono text-xs font-normal text-gray-400">{{ t.model }}</span>
          </p>
          <p class="mt-0.5 flex flex-wrap gap-x-3 text-xs text-gray-500 dark:text-dark-400">
            <span>分组 {{ groupOf(t.group_id)?.name ?? `#${t.group_id}` }}<template v-if="groupOf(t.group_id)"> · {{ formatMultiplier(groupOf(t.group_id)!.rate_multiplier) }}</template></span>
            <span>令牌 {{ keyName(t.api_key_id) }}</span>
            <span>
              思考 {{ t.reasoning_effort || '不发送' }}
              <template v-if="t.effort_denied"><span class="text-rose-600">（分组策略不允许）</span></template>
              <template v-else-if="t.sent_effort && t.sent_effort !== t.reasoning_effort">（实际发送 {{ t.sent_effort }}）</template>
            </span>
          </p>
        </div>
        <Toggle :model-value="t.enabled" :aria-label="`启用 ${t.display_name || t.model}`" @update:model-value="setEnabled(t, $event)" />
        <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="openEdit(t)">编辑</button>
        <button type="button" class="btn btn-secondary btn-sm text-rose-600" @click="deleting = t">删除</button>
      </li>
    </ul>

    <TargetDialog :show="dialogOpen" :target="editing" :options="options" @close="dialogOpen = false" @saved="onSaved" />
    <ConfirmDialog
      :show="!!deleting"
      :title="deleting ? `删除「${deleting.display_name || deleting.model}」？` : ''"
      message="已有的测试记录会保留，可在运行概览中按分组和模型清空。"
      confirm-text="删除"
      danger
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </section>
</template>
