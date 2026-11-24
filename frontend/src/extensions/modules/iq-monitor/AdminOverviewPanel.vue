<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import { iqMonitorAdminAPI, type ConfigView, type Estimate, type Options, type RuntimeStatus, type Target } from './api'
import { errorMessage, formatTime, formatTokens, formatUSD } from './format'

const props = defineProps<{ config: ConfigView; options: Options | null }>()
const emit = defineEmits<{ goto: [tab: 'targets' | 'logic' | 'drawing' | 'settings'] }>()

const appStore = useAppStore()
const status = ref<RuntimeStatus | null>(null)
const estimate = ref<Estimate | null>(null)
const targets = ref<Target[]>([])
const error = ref('')
const triggering = ref(false)
const clearKey = ref('')
const confirmClear = ref<'one' | 'all' | null>(null)
const clearing = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

const groupName = (id: number) => props.options?.groups.find(g => g.id === id)?.name ?? `#${id}`

// History is kept per group and model, so one entry clears every effort level.
const clearChoices = computed(() => {
  const seen = new Map<string, { group_id: number; model: string; label: string }>()
  for (const t of targets.value) {
    const key = `${t.group_id}\u0000${t.model}`
    if (!seen.has(key)) seen.set(key, { group_id: t.group_id, model: t.model, label: `${groupName(t.group_id)} / ${t.model}` })
  }
  return [...seen.entries()].map(([key, v]) => ({ key, ...v }))
})
const clearTarget = computed(() => clearChoices.value.find(c => c.key === clearKey.value))

const problems = computed(() => {
  const out: Array<{ text: string; tab: 'targets' | 'logic' | 'drawing' | 'settings' }> = []
  if (!props.config.probe_user_id) out.push({ text: '还没有选择探测账号', tab: 'settings' })
  if (!props.config.logic.enabled && !props.config.drawing.enabled) out.push({ text: '逻辑测试和画图测试都已关闭，不会运行', tab: 'logic' })
  if (!targets.value.some(t => t.enabled)) out.push({ text: '没有启用的监测目标', tab: 'targets' })
  return out
})

async function loadStatus() {
  clearTimeout(timer)
  try {
    status.value = await iqMonitorAdminAPI.status()
  } catch (e) {
    error.value = errorMessage(e, '读取运行状态失败')
  }
  timer = setTimeout(loadStatus, status.value?.running ? 5000 : 30000)
}

async function loadRest() {
  try {
    const [est, t] = await Promise.all([iqMonitorAdminAPI.estimate(), iqMonitorAdminAPI.targets()])
    estimate.value = est
    targets.value = t.items
  } catch (e) {
    error.value = errorMessage(e, '读取费用估算失败')
  }
}

async function trigger() {
  triggering.value = true
  try {
    const id = await iqMonitorAdminAPI.trigger()
    appStore.showSuccess(`已开始第 ${id} 轮测试`)
    await loadStatus()
  } catch (e) {
    appStore.showError(errorMessage(e, '启动失败'))
  } finally {
    triggering.value = false
  }
}

async function doClear() {
  const mode = confirmClear.value
  if (!mode) return
  clearing.value = true
  try {
    const n = mode === 'all'
      ? await iqMonitorAdminAPI.clearHistory({ all: true })
      : await iqMonitorAdminAPI.clearHistory({ all: false, group_id: clearTarget.value!.group_id, model: clearTarget.value!.model })
    appStore.showSuccess(`已删除 ${n} 条记录`)
    confirmClear.value = null
    await loadRest()
  } catch (e) {
    appStore.showError(errorMessage(e, '清空失败'))
  } finally {
    clearing.value = false
  }
}

onMounted(() => {
  void loadStatus()
  void loadRest()
})
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="space-y-6">
    <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>

    <div v-if="problems.length" class="rounded-2xl bg-amber-50 p-4 text-sm text-amber-800 ring-1 ring-amber-200 dark:bg-amber-900/20 dark:text-amber-200 dark:ring-amber-800/50">
      <p v-for="p in problems" :key="p.text">
        {{ p.text }}，<button type="button" class="font-medium underline" @click="emit('goto', p.tab)">去设置</button>
      </p>
    </div>

    <section class="card p-5">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">运行状态</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">每 {{ config.interval_minutes }} 分钟一轮；多节点部署时每轮只由一个节点执行。</p>
        </div>
        <button type="button" class="btn btn-primary" :disabled="triggering || !!status?.running" @click="trigger">
          {{ status?.running ? '本轮进行中' : triggering ? '启动中…' : '立即运行一轮' }}
        </button>
      </div>
      <dl v-if="status" class="mt-4 grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">当前轮次</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">
            <template v-if="status.running">第 {{ status.running.id }} 轮，{{ formatTime(status.running.started_at) }} 开始</template>
            <template v-else>空闲</template>
          </dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">上一轮</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">
            <template v-if="status.latest">#{{ status.latest.id }} · {{ formatTime(status.latest.started_at) }} · {{ status.latest.status === 'done' ? '完成' : status.latest.status === 'interrupted' ? '中断' : '进行中' }}</template>
            <template v-else>—</template>
          </dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">下一轮</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">{{ formatTime(status.next_round_at) }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">投稿</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">待审核 {{ status.pending_topics }} · 排队 {{ status.queued_topics }}</dd>
        </div>
      </dl>
      <p v-if="status?.latest?.topic || status?.latest?.question" class="mt-3 text-xs text-gray-500 dark:text-dark-400">
        <template v-if="status.latest.question">最近题目：{{ status.latest.question.slice(0, 80) }}{{ status.latest.question.length > 80 ? '…' : '' }}</template>
        <template v-if="status.latest.topic"><br />最近主题：{{ status.latest.topic }}</template>
      </p>
      <p v-if="status?.last_error" class="mt-3 rounded-xl bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/30 dark:text-red-300">本节点（{{ status.node_id }}）最近错误：{{ status.last_error }}</p>
    </section>

    <section class="card p-5">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">每日费用估算</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
        按最近 {{ estimate?.window_days ?? 7 }} 天每个目标的平均 token 用量和探测令牌的实际扣费单价推算，每天约 {{ estimate ? Number(estimate.rounds_per_day.toFixed(1)) : '—' }} 轮。
      </p>
      <template v-if="estimate">
        <div class="mt-4 flex flex-wrap gap-6">
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">预计每日费用</p>
            <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatUSD(estimate.cost_daily) }}<span v-if="!estimate.complete" class="ml-1 text-sm font-normal text-amber-600">起</span></p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">预计每日 tokens</p>
            <p class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatTokens(estimate.tokens_daily) }}</p>
          </div>
        </div>
        <p v-if="!estimate.complete" class="mt-2 text-xs text-amber-600">部分目标还没有运行记录或扣费记录，未计入费用。</p>
        <div v-if="estimate.targets.length" class="mt-4 overflow-x-auto">
          <table class="w-full min-w-[640px] text-sm">
            <thead class="text-left text-xs text-gray-500 dark:text-dark-400">
              <tr>
                <th class="py-2 pr-4 font-medium">目标</th>
                <th class="py-2 pr-4 font-medium">逻辑（平均输入/输出）</th>
                <th class="py-2 pr-4 font-medium">画图（平均输入/输出）</th>
                <th class="py-2 pr-4 text-right font-medium">每日 tokens</th>
                <th class="py-2 text-right font-medium">每日费用</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="t in estimate.targets" :key="t.target_id">
                <td class="py-2 pr-4 text-gray-900 dark:text-white">{{ t.display_name || t.model }}</td>
                <td class="py-2 pr-4 tabular-nums text-gray-600 dark:text-dark-300">
                  <template v-if="t.logic?.samples">{{ formatTokens(t.logic.avg_input_tokens) }} / {{ formatTokens(t.logic.avg_output_tokens) }} · {{ formatUSD(t.logic.cost_per_run) }}/次</template>
                  <template v-else>—</template>
                </td>
                <td class="py-2 pr-4 tabular-nums text-gray-600 dark:text-dark-300">
                  <template v-if="t.drawing?.samples">{{ formatTokens(t.drawing.avg_input_tokens) }} / {{ formatTokens(t.drawing.avg_output_tokens) }} · {{ formatUSD(t.drawing.cost_per_run) }}/次</template>
                  <template v-else>—</template>
                </td>
                <td class="py-2 pr-4 text-right tabular-nums">{{ formatTokens(t.tokens_daily) }}</td>
                <td class="py-2 text-right tabular-nums">{{ formatUSD(t.cost_daily) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </section>

    <section class="card p-5">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">清空历史</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">删除测试轮次、结果和模型回复，重新统计通过率。用户投稿不受影响，正在进行的一轮会保留。</p>
      <div class="mt-4 flex flex-col gap-3 sm:flex-row sm:items-end">
        <label class="flex-1">
          <span class="input-label">分组 / 模型（包含所有思考深度）</span>
          <select v-model="clearKey" class="input">
            <option value="">请选择</option>
            <option v-for="c in clearChoices" :key="c.key" :value="c.key">{{ c.label }}</option>
          </select>
        </label>
        <button type="button" class="btn btn-secondary" :disabled="!clearTarget" @click="confirmClear = 'one'">清空所选</button>
        <button type="button" class="btn btn-danger" @click="confirmClear = 'all'">清空全部</button>
      </div>
    </section>

    <ConfirmDialog
      :show="!!confirmClear"
      :title="confirmClear === 'all' ? '清空全部监测历史？' : `清空「${clearTarget?.label ?? ''}」的历史？`"
      message="删除后无法恢复，用户投稿会保留。"
      :confirm-text="clearing ? '清空中…' : '清空'"
      danger
      @confirm="doClear"
      @cancel="confirmClear = null"
    />
  </div>
</template>
