<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import type { Cell, KindView, ModelCard } from './api'
import {
  cellClasses, formatClock, formatMs, formatMultiplier, formatRate, formatTime,
  healthDots, healthLabels, healthOfRate, healthText, statusLabels,
} from './format'

const props = defineProps<{
  card: ModelCard
  logicEnabled: boolean
  drawingEnabled: boolean
  thresholds: { normal: number; unstable: number }
  intervalMinutes: number
  now: number
}>()
const emit = defineEmits<{ open: [id: number] }>()

// Short histories are padded on the left so the newest cell sits at "now".
const SLOTS = 48
const blank: Cell = { result_id: 0, round_id: 0, status: 'skipped', created_at: '' }
function padded(view: KindView): Cell[] {
  const cells = view.timeline.slice(-SLOTS)
  return [...Array.from({ length: SLOTS - cells.length }, () => blank), ...cells]
}

const rows = computed(() => {
  const out: Array<{ key: 'logic' | 'drawing'; label: string; unit: string; view: KindView; cells: Cell[]; errors: number }> = []
  const since = props.now - 24 * 3600 * 1000
  const add = (key: 'logic' | 'drawing', label: string, unit: string, view?: KindView) => {
    if (!view) return
    const errors = view.timeline.filter(c => c.status === 'error' && new Date(c.created_at).getTime() >= since).length
    out.push({ key, label, unit, view, cells: padded(view), errors })
  }
  if (props.logicEnabled) add('logic', '逻辑题测试', '答对', props.card.logic)
  if (props.drawingEnabled) add('drawing', '绘图测试', '画出', props.card.drawing)
  return out
})

// The left end of the axis is the oldest cell of a full row, or where a
// full row would start when the history is still short.
const axisStart = computed(() => {
  const full = rows.value.find(r => r.view.timeline.length >= SLOTS)
  if (full) return full.view.timeline.slice(-SLOTS)[0].created_at
  return new Date(props.now - SLOTS * props.intervalMinutes * 60000).toISOString()
})

const effortText = computed(() => {
  const c = props.card
  if (!c.reasoning_effort) return ''
  const sent = c.sent_effort && c.sent_effort !== c.reasoning_effort ? ` → ${c.sent_effort}` : ''
  return `${c.reasoning_effort}${sent}`
})

const hover = ref<{ row: string; index: number; cell: Cell } | null>(null)

function clickable(cell: Cell) {
  return cell.result_id > 0 && cell.status !== 'pending' && cell.status !== 'running' && cell.status !== 'skipped'
}

function cellLabel(cell: Cell) {
  const parts = [formatTime(cell.created_at), statusLabels[cell.status]]
  if (cell.reason) parts.push(cell.reason)
  if (cell.total_ms) parts.push(formatMs(cell.total_ms))
  return parts.join(' · ')
}

function tooltipStyle(index: number) {
  const pct = ((index + 0.5) / SLOTS) * 100
  if (pct < 12) return { left: '0' }
  if (pct > 88) return { right: '0' }
  return { left: `${pct}%`, transform: 'translateX(-50%)' }
}
</script>

<template>
  <article class="card px-5 py-4">
    <header class="flex flex-wrap items-center gap-x-3 gap-y-1.5">
      <ModelIcon :model="card.model" size="20px" />
      <h3 class="min-w-0 truncate text-[15px] font-semibold text-gray-900 dark:text-white" :title="card.display_name || card.model">{{ card.display_name || card.model }}</h3>
      <span v-if="card.display_name && card.display_name !== card.model" class="min-w-0 truncate font-mono text-xs text-gray-500 dark:text-dark-400">{{ card.model }}</span>
      <span class="flex min-w-0 items-center gap-1.5 text-xs">
        <span class="truncate rounded-md bg-gray-100 px-1.5 py-0.5 text-gray-600 dark:bg-dark-700 dark:text-dark-300">{{ card.group_name || '—' }}</span>
        <span v-if="effortText" class="shrink-0 rounded-md bg-violet-50 px-1.5 py-0.5 font-mono text-[11px] uppercase text-violet-700 dark:bg-violet-900/30 dark:text-violet-300" :title="card.sent_effort && card.sent_effort !== card.reasoning_effort ? `设置 ${card.reasoning_effort}，网关实际发送 ${card.sent_effort}` : '思考深度'">推理 {{ effortText }}</span>
        <span class="shrink-0 font-mono tabular-nums text-gray-400">{{ formatMultiplier(card.rate_multiplier) }}</span>
      </span>
      <span class="ml-auto flex shrink-0 items-center gap-3 text-xs">
        <span class="hidden text-gray-400 md:inline">首字 {{ formatMs(card.avg_first_token_ms) }} · 耗时 {{ formatMs(card.avg_total_ms) }}</span>
        <span class="inline-flex items-center gap-1.5 font-medium" :class="healthText[card.health]">
          <span class="h-2 w-2 rounded-full" :class="healthDots[card.health]"></span>{{ healthLabels[card.health] }}
        </span>
      </span>
    </header>

    <div class="mt-4 space-y-3.5">
      <div v-for="row in rows" :key="row.key" class="grid grid-cols-1 items-center gap-x-5 gap-y-2 sm:grid-cols-[132px_minmax(0,1fr)]">
        <div class="flex items-baseline gap-2 sm:block">
          <p class="flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-dark-300">
            <Icon :name="row.key === 'logic' ? 'brain' : 'sparkles'" size="xs" />{{ row.label }}
          </p>
          <p class="flex items-baseline gap-1.5 sm:mt-0.5">
            <span class="text-lg font-semibold tabular-nums" :class="healthText[healthOfRate(row.view.stat.rate, row.view.stat.total, thresholds)]">{{ formatRate(row.view.stat.rate, row.view.stat.total) }}</span>
            <span class="text-xs tabular-nums text-gray-400">{{ row.view.stat.passed }}/{{ row.view.stat.total }} {{ row.unit }}</span>
          </p>
          <p v-if="row.errors" class="text-xs text-amber-600 dark:text-amber-400">{{ row.errors }} 次请求失败</p>
        </div>
        <div class="relative" @mouseleave="hover = null">
          <div class="flex h-7 items-stretch gap-[3px]" role="list" :aria-label="`${row.label}最近 ${SLOTS} 轮`">
            <template v-for="(cell, i) in row.cells" :key="`${row.key}-${i}`">
              <button
                v-if="clickable(cell)"
                type="button"
                role="listitem"
                class="min-w-0 flex-1 rounded-[3px] transition hover:scale-y-110 hover:brightness-110 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-1"
                :class="cellClasses[cell.status]"
                :aria-label="cellLabel(cell)"
                @mouseenter="hover = { row: row.key, index: i, cell }"
                @focus="hover = { row: row.key, index: i, cell }"
                @blur="hover = null"
                @click="emit('open', cell.result_id)"
              ></button>
              <span
                v-else
                role="listitem"
                class="min-w-0 flex-1 rounded-[3px]"
                :class="cell.result_id > 0 ? cellClasses[cell.status] : 'bg-gray-200/70 dark:bg-dark-700/70'"
                :aria-label="cell.result_id > 0 ? cellLabel(cell) : '暂无数据'"
                @mouseenter="cell.result_id > 0 ? (hover = { row: row.key, index: i, cell }) : (hover = null)"
              ></span>
            </template>
          </div>
          <div
            v-if="hover && hover.row === row.key"
            class="pointer-events-none absolute bottom-full z-10 mb-2 w-max max-w-[260px] rounded-lg bg-gray-900 px-2.5 py-1.5 text-xs text-white shadow-lg dark:bg-dark-700"
            :style="tooltipStyle(hover.index)"
          >
            <p class="font-mono tabular-nums text-gray-300">{{ formatTime(hover.cell.created_at) }}</p>
            <p class="mt-0.5 font-medium">{{ statusLabels[hover.cell.status] }}<template v-if="hover.cell.total_ms"> · {{ formatMs(hover.cell.total_ms) }}</template></p>
            <p v-if="hover.cell.reason" class="mt-0.5 break-words text-gray-300">{{ hover.cell.reason }}</p>
          </div>
        </div>
      </div>
      <div v-if="rows.length" class="grid grid-cols-1 sm:grid-cols-[132px_minmax(0,1fr)] sm:gap-x-5">
        <span class="hidden sm:block"></span>
        <div class="flex justify-between font-mono text-[11px] tabular-nums text-gray-400 dark:text-dark-500">
          <span>{{ formatClock(axisStart, now) }}</span>
          <span>现在</span>
        </div>
      </div>
      <p v-else class="text-xs text-gray-400">没有启用的测试。</p>
    </div>
  </article>
</template>
