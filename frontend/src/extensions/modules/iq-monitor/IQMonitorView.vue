<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DrawingWall from './DrawingWall.vue'
import ModelCardItem from './ModelCardItem.vue'
import ResultDialog from './ResultDialog.vue'
import SubmitBox from './SubmitBox.vue'
import { iqMonitorAPI, type Overview } from './api'
import { errorMessage, formatRate, formatTime, healthClasses, healthLabels, statusLabels } from './format'

const overview = ref<Overview | null>(null)
const loading = ref(true)
const error = ref('')
const forbidden = ref(false)
const openId = ref<number | null>(null)
const now = ref(Date.now())
// Server and browser clocks may disagree; elapsed timers use the server's.
let skew = 0
let ticker: ReturnType<typeof setInterval> | undefined
let poller: ReturnType<typeof setTimeout> | undefined

const summary = computed(() => overview.value?.summary)
const running = computed(() => !!summary.value?.running)

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const ov = await iqMonitorAPI.overview()
    overview.value = ov
    skew = new Date(ov.now).getTime() - Date.now()
    error.value = ''
  } catch (e) {
    // The API client rejects with { status, code, message }.
    if ((e as { status?: number } | null)?.status === 403) forbidden.value = true
    else error.value = errorMessage(e, '降智监测暂时不可用，请稍后再试')
  } finally {
    loading.value = false
    schedule()
  }
}

function schedule() {
  clearTimeout(poller)
  if (forbidden.value) return
  poller = setTimeout(() => {
    if (document.visibilityState === 'visible') void load(true)
    else schedule()
  }, running.value ? 5000 : 60000)
}

onMounted(() => {
  void load()
  ticker = setInterval(() => { now.value = Date.now() + skew }, 1000)
})
onBeforeUnmount(() => {
  clearInterval(ticker)
  clearTimeout(poller)
})
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6">
      <header class="flex min-w-0 items-start gap-4">
        <div class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-800/40">
          <Icon name="beaker" size="lg" />
        </div>
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">降智监测</h1>
          <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-500 dark:text-dark-400">
            每轮让所有模型回答同一道逻辑题、画同一个主题，按最近 24 小时通过率判断模型是否被降智。
            <template v-if="overview">每 {{ overview.interval_minutes }} 分钟一轮。</template>
          </p>
        </div>
      </header>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>

      <div v-else-if="forbidden" class="card empty-state">
        <Icon name="lock" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">降智监测暂未对你的账户开放</p>
      </div>

      <template v-else-if="overview && summary">
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-5">
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">监测模型</p>
            <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ summary.models }}</p>
            <p class="mt-1 text-xs text-gray-400">24h 内 {{ summary.rounds_24h }} 轮</p>
          </div>
          <div v-if="overview.logic_enabled" class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">逻辑通过率</p>
            <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatRate(summary.logic.rate, summary.logic.total) }}</p>
            <p class="mt-1 text-xs text-gray-400">{{ summary.logic.passed }}/{{ summary.logic.total }} 次</p>
          </div>
          <div v-if="overview.drawing_enabled" class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">画图通过率</p>
            <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ formatRate(summary.drawing.rate, summary.drawing.total) }}</p>
            <p class="mt-1 text-xs text-gray-400">{{ summary.drawing.passed }}/{{ summary.drawing.total }} 次</p>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">模型状态</p>
            <div class="mt-2 flex flex-wrap gap-1.5 text-xs">
              <span class="rounded-full px-2 py-0.5 ring-1" :class="healthClasses.normal">{{ healthLabels.normal }} {{ summary.normal }}</span>
              <span class="rounded-full px-2 py-0.5 ring-1" :class="healthClasses.unstable">{{ healthLabels.unstable }} {{ summary.unstable }}</span>
              <span class="rounded-full px-2 py-0.5 ring-1" :class="healthClasses.degraded">{{ healthLabels.degraded }} {{ summary.degraded }}</span>
            </div>
          </div>
          <div class="card p-4">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ running ? '本轮进行中' : '下一轮' }}</p>
            <p class="mt-1 text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ running ? '测试中' : formatTime(summary.next_round_at) }}</p>
            <p class="mt-1 text-xs text-gray-400">上一轮 {{ formatTime(summary.last_round_at) }}</p>
          </div>
        </div>

        <p class="text-xs text-gray-500 dark:text-dark-400">
          判定依据：{{ overview.primary_kind === 'logic' ? '逻辑测试' : '画图测试' }} 24h 通过率 ≥ {{ overview.thresholds.normal }}% 为「正常」，≥ {{ overview.thresholds.unstable }}% 为「偶有失败」，更低为「疑似降智」。
          时间线中 <span class="inline-block h-2.5 w-2.5 rounded-sm bg-emerald-500 align-middle"></span> {{ statusLabels.pass }}
          <span class="inline-block h-2.5 w-2.5 rounded-sm bg-lime-400 align-middle"></span> {{ statusLabels.retry_pass }}
          <span class="inline-block h-2.5 w-2.5 rounded-sm bg-rose-500 align-middle"></span> {{ statusLabels.fail }}
          <span class="inline-block h-2.5 w-2.5 rounded-sm bg-orange-400 align-middle"></span> {{ statusLabels.error }}
          <span class="inline-block h-2.5 w-2.5 rounded-sm bg-gray-300 align-middle dark:bg-dark-600"></span> {{ statusLabels.interrupted }}，点击色块查看详情。
        </p>

        <DrawingWall v-if="overview.drawing_enabled && overview.wall" :wall="overview.wall" :now="now" @open="openId = $event" />

        <section>
          <h2 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">模型状态</h2>
          <div v-if="overview.models.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <ModelCardItem
              v-for="card in overview.models"
              :key="card.target_id"
              :card="card"
              :logic-enabled="overview.logic_enabled"
              :drawing-enabled="overview.drawing_enabled"
              @open="openId = $event"
            />
          </div>
          <div v-else class="card p-6 text-sm text-gray-500">还没有配置监测模型。</div>
        </section>

        <div class="grid gap-4 lg:grid-cols-2">
          <SubmitBox v-if="overview.drawing_enabled" />
          <section v-if="overview.drawing_enabled" class="card p-5">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">排队中的投稿</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">共 {{ summary.queued_topics }} 条已通过审核，按通过顺序使用。</p>
            <ol v-if="overview.queue.length" class="mt-3 space-y-2 text-sm">
              <li v-for="(q, i) in overview.queue" :key="i" class="flex items-start gap-2">
                <span class="mt-0.5 w-5 shrink-0 text-right text-xs tabular-nums text-gray-400">{{ i + 1 }}</span>
                <span class="min-w-0 flex-1 break-words text-gray-900 dark:text-white">{{ q.topic }}</span>
                <span class="shrink-0 text-xs text-gray-400">{{ q.submitter }}</span>
              </li>
            </ol>
            <p v-else class="mt-3 text-sm text-gray-400">暂无排队主题，下一轮使用管理员题库中的随机主题。</p>
          </section>
        </div>
      </template>
    </div>
    <ResultDialog :result-id="openId" @close="openId = null" />
  </AppLayout>
</template>
