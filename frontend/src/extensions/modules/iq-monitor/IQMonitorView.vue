<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import DrawingWall from './DrawingWall.vue'
import ModelCardItem from './ModelCardItem.vue'
import ResultDialog from './ResultDialog.vue'
import SubmitBox from './SubmitBox.vue'
import { iqMonitorAPI, type Overview, type WallView } from './api'
import {
  cellClasses, errorMessage, formatClock, formatCountdown, formatElapsed, formatRate,
  healthDots, healthLabels, healthOfRate, healthText, statusLabels,
} from './format'

const overview = ref<Overview | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const error = ref('')
const forbidden = ref(false)
const openId = ref<number | null>(null)
const showRules = ref(false)
const showAllRecent = ref(false)
const now = ref(Date.now())
// A past round picked with the wall arrows or the recent list; null follows
// the newest round.
const pinnedWall = ref<WallView | null>(null)
const switching = ref(false)
const wallError = ref('')
// Server and browser clocks may disagree; timers use the server's.
let skew = 0
let ticker: ReturnType<typeof setInterval> | undefined
let poller: ReturnType<typeof setTimeout> | undefined
let wallSeq = 0

const summary = computed(() => overview.value?.summary)
const running = computed(() => !!summary.value?.running)
const wall = computed(() => pinnedWall.value ?? overview.value?.wall ?? null)
const onLatest = computed(() => !pinnedWall.value || pinnedWall.value.round_id === overview.value?.wall?.round_id)

// The headline rate follows the kind that decides health.
const headline = computed(() => {
  const ov = overview.value
  if (!ov) return null
  const kind = ov.primary_kind
  const stat = kind === 'logic' ? ov.summary.logic : ov.summary.drawing
  return { label: kind === 'logic' ? '逻辑题通过率' : '画图通过率', stat, health: healthOfRate(stat.rate, stat.total, ov.thresholds) }
})
const secondary = computed(() => {
  const ov = overview.value
  if (!ov || !ov.logic_enabled || !ov.drawing_enabled) return null
  const stat = ov.primary_kind === 'logic' ? ov.summary.drawing : ov.summary.logic
  return { label: ov.primary_kind === 'logic' ? '画图通过率' : '逻辑题通过率', stat, health: healthOfRate(stat.rate, stat.total, ov.thresholds) }
})

const countdown = computed(() => {
  const ov = overview.value
  if (!ov?.summary.next_round_at) return null
  const left = new Date(ov.summary.next_round_at).getTime() - now.value
  const span = ov.interval_minutes * 60000
  return { text: formatCountdown(left), progress: Math.min(100, Math.max(0, (1 - left / span) * 100)) }
})

const recent = computed(() => {
  const list = overview.value?.recent ?? []
  return showAllRecent.value ? list : list.slice(0, 10)
})

const legend = [
  { status: 'pass', label: statusLabels.pass },
  { status: 'retry_pass', label: statusLabels.retry_pass },
  { status: 'fail', label: statusLabels.fail },
  { status: 'error', label: '请求失败' },
  { status: 'running', label: '检测中' },
  { status: 'interrupted', label: statusLabels.interrupted },
] as const

async function load(silent = false) {
  if (!silent) loading.value = true
  try {
    const ov = await iqMonitorAPI.overview()
    overview.value = ov
    skew = new Date(ov.now).getTime() - Date.now()
    error.value = ''
    // A pinned round that is still running keeps refreshing too.
    if (pinnedWall.value?.status === 'running') await fetchWall(pinnedWall.value.round_id, true)
  } catch (e) {
    // The API client rejects with { status, code, message }.
    if ((e as { status?: number } | null)?.status === 403) forbidden.value = true
    else if (!silent || !overview.value) error.value = errorMessage(e, '降智监测暂时不可用，请稍后再试')
  } finally {
    loading.value = false
    schedule()
  }
}

async function refresh() {
  refreshing.value = true
  try {
    await load(true)
  } finally {
    refreshing.value = false
  }
}

async function fetchWall(roundId: number, silent = false) {
  const seq = ++wallSeq
  if (!silent) switching.value = true
  try {
    const w = await iqMonitorAPI.wall(roundId)
    if (seq !== wallSeq) return
    pinnedWall.value = w
    wallError.value = ''
  } catch (e) {
    if (seq === wallSeq && !silent) wallError.value = errorMessage(e, '读取这一轮的画作失败')
  } finally {
    if (seq === wallSeq) switching.value = false
  }
}

function navigate(roundId: number | null) {
  wallError.value = ''
  if (roundId == null || roundId === overview.value?.wall?.round_id) {
    wallSeq++
    pinnedWall.value = null
    switching.value = false
    return
  }
  void fetchWall(roundId)
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
    <div class="mx-auto w-full max-w-[1400px] space-y-5">
      <header class="flex flex-wrap items-center gap-3">
        <div class="min-w-0 flex-1">
          <h1 class="text-xl font-semibold tracking-tight text-gray-900 dark:text-white">降智监测</h1>
          <p class="mt-0.5 text-sm text-gray-500 dark:text-dark-400">
            每轮让所有模型回答同一道逻辑题、画同一个主题，用最近 24 小时的通过率判断是否被降智<template v-if="overview">，每 {{ overview.interval_minutes }} 分钟一轮</template>。
          </p>
        </div>
        <div v-if="overview" class="flex items-center gap-2">
          <button
            type="button"
            class="grid h-9 w-9 place-items-center rounded-lg text-gray-500 ring-1 ring-gray-200 transition hover:bg-gray-50 hover:text-gray-700 dark:text-dark-400 dark:ring-dark-700 dark:hover:bg-dark-800"
            :class="showRules ? 'bg-gray-100 text-gray-800 dark:bg-dark-800 dark:text-white' : ''"
            :aria-expanded="showRules"
            aria-label="判定规则"
            title="判定规则"
            @click="showRules = !showRules"
          >
            <Icon name="infoCircle" size="sm" />
          </button>
          <button
            type="button"
            class="grid h-9 w-9 place-items-center rounded-lg text-gray-500 ring-1 ring-gray-200 transition hover:bg-gray-50 hover:text-gray-700 disabled:opacity-50 dark:text-dark-400 dark:ring-dark-700 dark:hover:bg-dark-800"
            :disabled="refreshing"
            aria-label="刷新"
            title="刷新"
            @click="refresh"
          >
            <Icon name="refresh" size="sm" :class="refreshing ? 'animate-spin' : ''" />
          </button>
        </div>
      </header>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>

      <div v-else-if="forbidden" class="card empty-state">
        <Icon name="lock" size="xl" class="empty-state-icon" />
        <p class="empty-state-title">降智监测暂未对你的账户开放</p>
      </div>

      <template v-else-if="overview && summary">
        <div v-if="showRules" class="card space-y-1.5 p-4 text-sm leading-6 text-gray-600 dark:text-dark-300">
          <p>按{{ overview.primary_kind === 'logic' ? '逻辑题' : '画图' }}测试最近 24 小时的通过率判定：≥ {{ overview.thresholds.normal }}% 为「{{ healthLabels.normal }}」，≥ {{ overview.thresholds.unstable }}% 为「{{ healthLabels.unstable }}」，更低为「{{ healthLabels.degraded }}」。</p>
          <p>逻辑题看模型最后给出的答案是否正确；画图看回复中是否包含可渲染的 SVG。请求失败（超时、上游报错）计入通过率，中断的轮次不计入。</p>
          <p>所有请求都经本站网关转发，与普通用户走同一条线路。点击时间线色块或画作可查看完整回复。</p>
        </div>

        <div class="grid items-start gap-5" :class="overview.drawing_enabled ? 'lg:grid-cols-[minmax(0,1fr)_320px]' : ''">
          <div class="min-w-0 space-y-5">
            <div class="card grid grid-cols-2 divide-gray-100 dark:divide-dark-700 md:grid-cols-4 md:divide-x">
              <div v-if="headline" class="p-4 md:px-5">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ headline.label }} · 最近 24 小时</p>
                <p class="mt-1.5 flex items-baseline gap-2">
                  <span class="text-[28px] font-semibold leading-none tabular-nums" :class="healthText[headline.health]">{{ formatRate(headline.stat.rate, headline.stat.total) }}</span>
                  <span class="font-mono text-xs tabular-nums text-gray-400">{{ headline.stat.passed }}/{{ headline.stat.total }}</span>
                </p>
              </div>
              <div class="p-4 md:px-5">
                <template v-if="secondary">
                  <p class="text-xs text-gray-500 dark:text-dark-400">{{ secondary.label }}</p>
                  <p class="mt-1.5 flex items-baseline gap-2">
                    <span class="text-[28px] font-semibold leading-none tabular-nums" :class="healthText[secondary.health]">{{ formatRate(secondary.stat.rate, secondary.stat.total) }}</span>
                    <span class="font-mono text-xs tabular-nums text-gray-400">{{ secondary.stat.passed }}/{{ secondary.stat.total }}</span>
                  </p>
                </template>
                <template v-else>
                  <p class="text-xs text-gray-500 dark:text-dark-400">24 小时轮次</p>
                  <p class="mt-1.5 text-[28px] font-semibold leading-none tabular-nums text-gray-900 dark:text-white">{{ summary.rounds_24h }}</p>
                </template>
              </div>
              <div class="border-t border-gray-100 p-4 dark:border-dark-700 md:border-t-0 md:px-5">
                <p class="text-xs text-gray-500 dark:text-dark-400">监测模型</p>
                <p class="mt-1.5 flex flex-wrap items-baseline gap-x-3 gap-y-1">
                  <span class="text-[28px] font-semibold leading-none tabular-nums text-gray-900 dark:text-white">{{ summary.models }}</span>
                  <span class="flex flex-wrap gap-x-2.5 text-xs">
                    <span v-for="h in (['normal', 'unstable', 'degraded'] as const)" v-show="summary[h] > 0 || h === 'normal'" :key="h" class="inline-flex items-center gap-1" :class="healthText[h]">
                      <span class="h-1.5 w-1.5 rounded-full" :class="healthDots[h]"></span>{{ summary[h] }} {{ healthLabels[h] }}
                    </span>
                  </span>
                </p>
              </div>
              <div class="border-t border-gray-100 p-4 dark:border-dark-700 md:border-t-0 md:px-5">
                <template v-if="running">
                  <p class="text-xs text-gray-500 dark:text-dark-400">本轮进行中</p>
                  <p class="mt-1.5 flex items-baseline gap-2">
                    <span class="font-mono text-[28px] font-semibold leading-none tabular-nums text-sky-600 dark:text-sky-400">{{ formatElapsed(summary.last_round_at, now) }}</span>
                    <span class="text-xs text-gray-400">已用时</span>
                  </p>
                  <div class="mt-2.5 h-1 overflow-hidden rounded-full bg-sky-100 dark:bg-sky-900/40"><div class="h-full w-1/3 animate-pulse rounded-full bg-sky-500"></div></div>
                </template>
                <template v-else>
                  <p class="text-xs text-gray-500 dark:text-dark-400">下一轮</p>
                  <p class="mt-1.5 flex items-baseline gap-2">
                    <span class="font-mono text-[28px] font-semibold leading-none tabular-nums text-gray-900 dark:text-white">{{ countdown?.text ?? '—' }}</span>
                    <span v-if="overview.drawing_enabled" class="text-xs text-gray-400">{{ summary.queued_topics ? '投稿主题' : '随机主题' }}</span>
                  </p>
                  <div class="mt-2.5 h-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                    <div class="h-full rounded-full bg-primary-500 transition-[width] duration-1000 ease-linear" :style="{ width: `${countdown?.progress ?? 0}%` }"></div>
                  </div>
                </template>
              </div>
            </div>

            <template v-if="overview.drawing_enabled && wall">
              <DrawingWall :wall="wall" :now="now" :latest="onLatest" :switching="switching" @open="openId = $event" @navigate="navigate" />
              <p v-if="wallError" role="alert" class="-mt-3 text-xs text-rose-600">{{ wallError }}</p>
            </template>

            <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-1 text-xs text-gray-500 dark:text-dark-400">
              <span v-for="item in legend" :key="item.status" class="inline-flex items-center gap-1.5">
                <span class="h-2.5 w-2.5 rounded-[3px]" :class="cellClasses[item.status]"></span>{{ item.label }}
              </span>
              <span class="inline-flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-[3px] bg-gray-200/70 dark:bg-dark-700/70"></span>暂无数据</span>
              <span class="ml-auto hidden text-gray-400 sm:inline">点击色块查看该轮回复</span>
            </div>

            <div v-if="overview.models.length" class="space-y-3">
              <ModelCardItem
                v-for="card in overview.models"
                :key="card.target_id"
                :card="card"
                :logic-enabled="overview.logic_enabled"
                :drawing-enabled="overview.drawing_enabled"
                :thresholds="overview.thresholds"
                :interval-minutes="overview.interval_minutes"
                :now="now"
                @open="openId = $event"
              />
            </div>
            <div v-else class="card p-6 text-sm text-gray-500 dark:text-dark-400">还没有配置监测模型。</div>
          </div>
          <aside v-if="overview.drawing_enabled" class="space-y-4 lg:sticky lg:top-20">
            <SubmitBox />

            <section class="card p-4">
              <div class="flex items-center justify-between">
                <h2 class="text-sm font-semibold text-gray-900 dark:text-white">排队中</h2>
                <span class="font-mono text-xs tabular-nums text-gray-400">{{ summary.queued_topics }}</span>
              </div>
              <ol v-if="overview.queue.length" class="mt-3 space-y-2.5">
                <li v-for="(q, i) in overview.queue" :key="i" class="flex items-start gap-2.5 text-sm">
                  <span class="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full bg-gray-100 font-mono text-[11px] tabular-nums text-gray-500 dark:bg-dark-700 dark:text-dark-300">{{ i + 1 }}</span>
                  <div class="min-w-0 flex-1">
                    <p class="break-words leading-5 text-gray-800 dark:text-dark-100">{{ q.topic }}</p>
                    <p class="text-xs text-gray-400">{{ q.submitter }}</p>
                  </div>
                </li>
              </ol>
              <p v-else class="mt-2 text-xs text-gray-500 dark:text-dark-400">队列为空，下一轮使用随机主题。</p>
            </section>

            <section class="card p-4">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">最近画过</h2>
              <ul v-if="recent.length" class="-mx-2 mt-2 max-h-[560px] space-y-0.5 overflow-y-auto">
                <li v-for="r in recent" :key="r.round_id">
                  <button
                    type="button"
                    class="flex w-full items-start gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-gray-50 dark:hover:bg-dark-800"
                    :class="wall?.round_id === r.round_id ? 'bg-primary-50 hover:bg-primary-50 dark:bg-primary-900/20 dark:hover:bg-primary-900/20' : ''"
                    :aria-current="wall?.round_id === r.round_id ? 'true' : undefined"
                    :disabled="switching"
                    @click="navigate(r.round_id)"
                  >
                    <span class="mt-0.5 w-11 shrink-0 font-mono text-[11px] tabular-nums text-gray-400">{{ formatClock(r.started_at, now) }}</span>
                    <span class="min-w-0 flex-1">
                      <span class="line-clamp-2 break-words text-sm leading-5" :class="wall?.round_id === r.round_id ? 'font-medium text-primary-700 dark:text-primary-300' : 'text-gray-800 dark:text-dark-100'">{{ r.topic }}</span>
                      <span class="block text-[11px] text-gray-400">{{ r.topic_source === 'user' ? (r.submitter || '用户投稿') : '随机主题' }}</span>
                    </span>
                  </button>
                </li>
              </ul>
              <p v-else class="mt-2 text-xs text-gray-500 dark:text-dark-400">还没有画图记录。</p>
              <button
                v-if="overview.recent.length > 10"
                type="button"
                class="mt-2 text-xs font-medium text-primary-600 hover:underline dark:text-primary-400"
                @click="showAllRecent = !showAllRecent"
              >
                {{ showAllRecent ? '收起' : `显示全部 ${overview.recent.length} 个` }}
              </button>
            </section>
          </aside>
        </div>
      </template>
    </div>
    <ResultDialog :result-id="openId" @close="openId = null" />
  </AppLayout>
</template>
