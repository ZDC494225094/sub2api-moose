<script setup lang="ts">
import { computed } from 'vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import type { Cell, KindView, ModelCard } from './api'
import { cellClasses, formatMs, formatMultiplier, formatRate, formatTime, healthClasses, healthLabels, statusLabels } from './format'

const props = defineProps<{ card: ModelCard; logicEnabled: boolean; drawingEnabled: boolean }>()
const emit = defineEmits<{ open: [id: number] }>()

// Short histories are padded on the left so every bar has the same width.
const SLOTS = 48
function padded(view: KindView): Cell[] {
  const cells = view.timeline.slice(-SLOTS)
  const blank: Cell = { result_id: 0, round_id: 0, status: 'skipped', created_at: '' }
  return [...Array.from({ length: SLOTS - cells.length }, () => blank), ...cells]
}

const rows = computed(() => {
  const out: Array<{ key: string; label: string; view: KindView }> = []
  if (props.logicEnabled && props.card.logic) out.push({ key: 'logic', label: '逻辑', view: props.card.logic })
  if (props.drawingEnabled && props.card.drawing) out.push({ key: 'drawing', label: '画图', view: props.card.drawing })
  return out
})

function cellTitle(cell: Cell) {
  const parts = [formatTime(cell.created_at), statusLabels[cell.status]]
  if (cell.reason) parts.push(cell.reason)
  if (cell.total_ms) parts.push(formatMs(cell.total_ms))
  return parts.join(' · ')
}

function clickable(cell: Cell) {
  return cell.result_id > 0 && cell.status !== 'pending' && cell.status !== 'running' && cell.status !== 'skipped'
}
</script>

<template>
  <article class="card flex flex-col gap-4 p-5">
    <header class="flex items-start gap-3">
      <div class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-gray-50 dark:bg-dark-800">
        <ModelIcon :model="card.model" size="24px" />
      </div>
      <div class="min-w-0 flex-1">
        <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white" :title="card.model">{{ card.display_name || card.model }}</h3>
        <p class="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
          <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-700">{{ card.group_name || '—' }}</span>
          <span class="tabular-nums">{{ formatMultiplier(card.rate_multiplier) }}</span>
        </p>
        <p v-if="card.reasoning_effort" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
          思考 {{ card.reasoning_effort }}<template v-if="card.sent_effort && card.sent_effort !== card.reasoning_effort">（实际发送 {{ card.sent_effort }}）</template>
        </p>
      </div>
      <span class="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium ring-1" :class="healthClasses[card.health]">{{ healthLabels[card.health] }}</span>
    </header>

    <div v-for="row in rows" :key="row.key" class="space-y-1.5">
      <div class="flex items-center justify-between text-xs">
        <span class="font-medium text-gray-600 dark:text-dark-300">{{ row.label }}</span>
        <span class="tabular-nums text-gray-500 dark:text-dark-400">
          24h 通过率 <span class="font-semibold text-gray-900 dark:text-white">{{ formatRate(row.view.stat.rate, row.view.stat.total) }}</span>
          <span class="ml-1 text-gray-400">({{ row.view.stat.passed }}/{{ row.view.stat.total }})</span>
        </span>
      </div>
      <div class="flex h-6 items-stretch gap-[2px]" role="list" :aria-label="`${row.label}测试时间线`">
        <template v-for="(cell, i) in padded(row.view)" :key="`${row.key}-${i}`">
          <button
            v-if="clickable(cell)"
            type="button"
            role="listitem"
            class="min-w-0 flex-1 rounded-sm transition hover:opacity-70 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
            :class="cellClasses[cell.status]"
            :title="cellTitle(cell)"
            :aria-label="cellTitle(cell)"
            @click="emit('open', cell.result_id)"
          ></button>
          <span
            v-else
            role="listitem"
            class="min-w-0 flex-1 rounded-sm"
            :class="cell.result_id > 0 ? cellClasses[cell.status] : 'bg-gray-100 dark:bg-dark-800'"
            :title="cell.result_id > 0 ? cellTitle(cell) : '无数据'"
          ></span>
        </template>
      </div>
    </div>
    <p v-if="!rows.length" class="text-xs text-gray-400">没有启用的测试。</p>

    <footer class="mt-auto flex gap-4 text-xs text-gray-500 dark:text-dark-400">
      <span>平均首字 {{ formatMs(card.avg_first_token_ms) }}</span>
      <span>平均耗时 {{ formatMs(card.avg_total_ms) }}</span>
    </footer>
  </article>
</template>
