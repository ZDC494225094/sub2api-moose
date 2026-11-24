<script setup lang="ts">
import ModelIcon from '@/components/common/ModelIcon.vue'
import DrawingFrame from './DrawingFrame.vue'
import type { WallView, WallCard } from './api'
import { formatElapsed, formatMs, formatMultiplier, formatTime, statusLabels } from './format'

defineProps<{ wall: WallView; now: number }>()
const emit = defineEmits<{ open: [id: number] }>()

function busy(card: WallCard) {
  return card.status === 'pending' || card.status === 'running'
}

function badgeClass(card: WallCard) {
  switch (card.status) {
    case 'pass': return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
    case 'fail': case 'error': return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
    case 'running': case 'pending': return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
    default: return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
  }
}
</script>

<template>
  <section class="card p-5">
    <div class="flex flex-wrap items-baseline justify-between gap-2">
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">画图墙</h2>
        <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
          本轮主题：<span class="font-medium text-gray-900 dark:text-white">{{ wall.topic }}</span>
          <span v-if="wall.topic_source === 'user' && wall.submitter" class="ml-2 text-xs text-gray-400">由 {{ wall.submitter }} 投稿</span>
        </p>
      </div>
      <span class="text-xs text-gray-400">
        {{ wall.status === 'running' ? '进行中' : wall.status === 'interrupted' ? '已中断' : '已完成' }} · {{ formatTime(wall.started_at) }}
      </span>
    </div>

    <div class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <button
        v-for="card in wall.cards"
        :key="card.result_id"
        type="button"
        class="group flex flex-col overflow-hidden rounded-2xl text-left ring-1 ring-gray-200 transition hover:ring-primary-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:ring-dark-700"
        :disabled="busy(card)"
        @click="emit('open', card.result_id)"
      >
        <div class="relative">
          <DrawingFrame v-if="card.preview_url" :src="card.preview_url" :title="`${card.display_name} 的画作`" />
          <div v-else class="grid aspect-[16/10] place-items-center bg-gray-50 text-sm text-gray-400 dark:bg-dark-900">
            <span v-if="busy(card)" class="flex flex-col items-center gap-1 text-sky-600 dark:text-sky-300">
              <span class="animate-pulse">绘制中</span>
              <span class="font-mono text-xs tabular-nums">{{ formatElapsed(card.started_at, now) }}</span>
            </span>
            <span v-else class="px-4 text-center">{{ card.reason || statusLabels[card.status] }}</span>
          </div>
        </div>
        <div class="flex items-center gap-2 border-t border-gray-100 px-3 py-2.5 dark:border-dark-700">
          <ModelIcon :model="card.model" size="20px" />
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ card.display_name || card.model }}</p>
            <p class="flex min-w-0 items-center gap-1.5 whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">
              <span class="min-w-0 truncate rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-700">{{ card.group_name || '—' }}</span>
              <span class="shrink-0 tabular-nums">{{ formatMultiplier(card.rate_multiplier) }}</span>
              <span v-if="!busy(card) && card.total_ms" class="shrink-0 tabular-nums">{{ formatMs(card.total_ms) }}</span>
            </p>
          </div>
          <span class="shrink-0 rounded-full px-2 py-0.5 text-xs" :class="badgeClass(card)">{{ busy(card) ? '绘制中' : statusLabels[card.status] }}</span>
        </div>
      </button>
    </div>
    <p v-if="!wall.cards.length" class="mt-4 text-sm text-gray-500">本轮没有画图结果。</p>
  </section>
</template>
