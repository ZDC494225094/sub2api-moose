<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import DrawingFrame from './DrawingFrame.vue'
import type { WallView, WallCard } from './api'
import { formatElapsed, formatMs, formatMultiplier, formatTime, statusLabels } from './format'

defineProps<{ wall: WallView; now: number; latest: boolean; switching: boolean }>()
const emit = defineEmits<{ open: [id: number]; navigate: [roundId: number | null] }>()

function busy(card: WallCard) {
  return card.status === 'pending' || card.status === 'running'
}

function stateTone(card: WallCard) {
  switch (card.status) {
    case 'error': return 'text-amber-600 dark:text-amber-400'
    case 'fail': return 'text-rose-600 dark:text-rose-400'
    default: return 'text-gray-500 dark:text-dark-400'
  }
}

function stateIcon(card: WallCard) {
  if (card.status === 'error') return 'exclamationTriangle' as const
  if (card.status === 'fail') return 'xCircle' as const
  return 'ban' as const
}
</script>

<template>
  <section class="card overflow-hidden">
    <header class="flex items-start gap-3 px-4 pb-3 pt-4 sm:gap-4 sm:px-5 sm:pb-4 sm:pt-5">
      <div class="min-w-0 flex-1">
        <p class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
          <span class="font-mono tabular-nums">{{ formatTime(wall.started_at) }}</span>
          <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-600"></span>
          <span>{{ wall.topic_source === 'user' ? '用户投稿' : '随机主题' }}</span>
          <template v-if="wall.topic_source === 'user' && wall.submitter">
            <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-600"></span>
            <span>{{ wall.submitter }}</span>
          </template>
          <span v-if="wall.status === 'running'" class="inline-flex items-center gap-1 rounded-full bg-sky-50 px-2 py-0.5 font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300">
            <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-sky-500"></span>进行中
          </span>
          <span v-else-if="wall.status === 'interrupted'" class="rounded-full bg-gray-100 px-2 py-0.5 text-gray-600 dark:bg-dark-700 dark:text-dark-300">已中断</span>
        </p>
        <h2 class="mt-1.5 break-words text-xl font-semibold leading-snug tracking-tight text-gray-900 dark:text-white">{{ wall.topic }}</h2>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button v-if="!latest" type="button" class="btn btn-secondary btn-sm" :disabled="switching" @click="emit('navigate', null)">回到最新</button>
        <div class="inline-flex overflow-hidden rounded-lg ring-1 ring-gray-200 dark:ring-dark-700">
          <button
            type="button"
            class="grid h-8 w-8 place-items-center text-gray-600 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-35 dark:text-dark-300 dark:hover:bg-dark-800"
            :disabled="switching || !wall.prev_round_id"
            aria-label="上一轮"
            title="上一轮"
            @click="emit('navigate', wall.prev_round_id)"
          >
            <Icon name="chevronLeft" size="sm" />
          </button>
          <button
            type="button"
            class="grid h-8 w-8 place-items-center border-l border-gray-200 text-gray-600 transition hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-35 dark:border-dark-700 dark:text-dark-300 dark:hover:bg-dark-800"
            :disabled="switching || !wall.next_round_id"
            aria-label="下一轮"
            title="下一轮"
            @click="emit('navigate', wall.next_round_id)"
          >
            <Icon name="chevronRight" size="sm" />
          </button>
        </div>
      </div>
    </header>

    <div class="grid grid-cols-2 gap-3 px-4 pb-4 transition-opacity sm:gap-4 sm:px-5 sm:pb-5 xl:grid-cols-3" :class="switching ? 'opacity-50' : ''">
      <button
        v-for="card in wall.cards"
        :key="card.result_id"
        type="button"
        class="group flex min-w-0 flex-col overflow-hidden rounded-xl bg-white text-left ring-1 ring-gray-200 transition hover:-translate-y-0.5 hover:shadow-md hover:ring-primary-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:hover:translate-y-0 disabled:hover:shadow-none dark:bg-dark-900 dark:ring-dark-700"
        :disabled="busy(card)"
        @click="emit('open', card.result_id)"
      >
        <div v-if="card.preview_url" class="relative">
          <DrawingFrame :src="card.preview_url" :title="`${card.display_name || card.model} 的画作`" />
          <span
            v-if="card.status === 'pass'"
            data-pass-badge
            class="pointer-events-none absolute right-2 top-2 inline-flex items-center gap-1 rounded-full bg-emerald-500/95 py-0.5 pl-1 pr-2 text-[11px] font-medium text-white shadow-sm ring-1 ring-white/40 backdrop-blur-sm"
          >
            <Icon name="check" size="xs" :stroke-width="3" />通过
          </span>
        </div>
        <div v-else class="grid aspect-[16/10] place-items-center bg-gray-50 dark:bg-dark-800/60">
          <span v-if="busy(card)" class="flex flex-col items-center gap-2 text-sky-600 dark:text-sky-300">
            <span class="h-6 w-6 animate-spin rounded-full border-2 border-sky-200 border-t-sky-500 dark:border-sky-900 dark:border-t-sky-400"></span>
            <span class="text-xs">绘制中 <span class="font-mono tabular-nums">{{ formatElapsed(card.started_at, now) }}</span></span>
          </span>
          <span v-else class="flex max-w-[85%] flex-col items-center gap-1.5 text-center text-xs" :class="stateTone(card)">
            <Icon :name="stateIcon(card)" size="md" />
            <span class="line-clamp-2 break-words">{{ card.reason || statusLabels[card.status] }}</span>
          </span>
        </div>
        <div class="border-t border-gray-100 px-2.5 py-2 sm:px-3.5 sm:py-3 dark:border-dark-700">
          <p class="truncate text-sm font-medium text-gray-900 group-hover:text-primary-600 dark:text-white dark:group-hover:text-primary-400" :title="card.display_name && card.display_name !== card.model ? `${card.display_name}（${card.model}）` : card.model">{{ card.display_name || card.model }}</p>
          <p class="mt-0.5 flex min-w-0 items-center gap-1.5 whitespace-nowrap text-xs text-gray-500 dark:text-dark-400">
            <span class="min-w-0 truncate">{{ card.group_name || '—' }}</span>
            <span class="shrink-0 tabular-nums text-gray-400">{{ formatMultiplier(card.rate_multiplier) }}</span>
            <span v-if="busy(card)" class="ml-auto shrink-0 text-sky-600 dark:text-sky-300">绘制中</span>
            <span v-else-if="card.status === 'pass'" class="ml-auto shrink-0 font-mono tabular-nums">{{ formatMs(card.total_ms) }}</span>
            <span v-else class="ml-auto inline-flex shrink-0 items-center gap-1" :class="stateTone(card)">
              <span class="h-1.5 w-1.5 rounded-full" :class="card.status === 'error' ? 'bg-amber-400' : card.status === 'fail' ? 'bg-rose-500' : 'bg-gray-400'"></span>{{ statusLabels[card.status] }}
            </span>
          </p>
        </div>
      </button>
    </div>
    <p v-if="!wall.cards.length" class="px-5 pb-5 text-sm text-gray-500 dark:text-dark-400">这一轮没有画图结果。</p>
  </section>
</template>
