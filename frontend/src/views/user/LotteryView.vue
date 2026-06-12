<template>
  <AppLayout>
    <div v-if="activity" class="mx-auto max-w-6xl">
      <!-- Header -->
      <div class="mb-5 flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ activity.name }}</h1>
          <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{{ t('userLottery.description') }}</p>
        </div>
        <button
          type="button"
          class="flex items-center gap-1.5 rounded-xl border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-200 dark:hover:bg-dark-700"
          @click="showHistory = true; loadHistory()"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          {{ t('userLottery.historyBtn') }}
        </button>
      </div>

      <div class="grid gap-5 xl:grid-cols-[1fr,280px]">
        <!-- Lottery Board -->
        <section class="relative overflow-hidden rounded-3xl border border-blue-200/60 bg-gradient-to-br from-blue-50 via-white to-sky-50 p-5 shadow-xl dark:border-blue-900/30 dark:from-blue-950/30 dark:via-dark-900 dark:to-sky-950/20">
          <!-- ambient glow -->
          <div class="pointer-events-none absolute inset-0 overflow-hidden rounded-3xl">
            <div class="absolute -top-8 left-1/2 h-32 w-72 -translate-x-1/2 rounded-full bg-blue-300/25 blur-3xl dark:bg-blue-500/10" />
          </div>

          <div class="relative grid grid-cols-3 gap-3">
            <div
              v-for="(slot, index) in boardSlots"
              :key="slot.key"
              class="relative overflow-hidden rounded-2xl transition-all duration-100"
              :class="slotClass(slot, index)"
              style="aspect-ratio: 1;"
            >
              <!-- Center draw button -->
              <template v-if="slot.kind === 'center'">
                <button
                  type="button"
                  class="flex h-full w-full flex-col items-center justify-center gap-1 p-2"
                  :disabled="drawing"
                  @click="handleDrawClick"
                >
                  <div v-if="drawing" class="absolute inset-2 animate-spin rounded-full border-2 border-transparent border-t-white/80" />
                  <span class="relative z-10 text-base font-extrabold leading-tight text-white">
                    {{ drawing ? t('common.processing') : t('userLottery.startDraw') }}
                  </span>
                  <span class="relative z-10 text-xl font-black leading-none text-white">{{ middleValue }}</span>
                  <span class="relative z-10 px-1 text-center text-[10px] leading-tight text-white/75">{{ middleHint }}</span>
                </button>
              </template>

              <!-- Prize slot -->
              <template v-else>
                <div v-if="highlightedIndex === index" class="absolute inset-0 rounded-2xl bg-blue-300/30 dark:bg-blue-500/20" />
                <div class="flex h-full flex-col items-center justify-center gap-1 p-3 text-center">
                  <span class="text-[11px] font-medium leading-tight text-blue-500 dark:text-blue-400">{{ slot.badge }}</span>
                  <p class="text-sm font-bold leading-snug text-gray-800 dark:text-white">{{ slot.label }}</p>
                </div>
              </template>
            </div>
          </div>

          <!-- Result toast -->
          <Transition
            enter-active-class="transition-all duration-500 ease-out"
            enter-from-class="opacity-0 translate-y-4 scale-95"
            enter-to-class="opacity-100 translate-y-0 scale-100"
            leave-active-class="transition-all duration-200 ease-in"
            leave-from-class="opacity-100"
            leave-to-class="opacity-0"
          >
            <div
              v-if="resultMessage"
              class="absolute inset-x-4 bottom-4 rounded-2xl border border-blue-200 bg-white/95 px-4 py-3 shadow-xl backdrop-blur-sm dark:border-blue-800/40 dark:bg-dark-800/95"
            >
              <p class="text-sm font-bold text-gray-900 dark:text-white">{{ resultMessage }}</p>
              <p v-if="resultSub" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ resultSub }}</p>
            </div>
          </Transition>
        </section>

        <!-- Right: recent winners -->
        <aside class="flex flex-col overflow-hidden rounded-2xl border border-gray-100 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-900">
          <div class="flex shrink-0 items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('userLottery.recentWinners') }}</p>
            <span class="rounded-full bg-blue-50 px-2.5 py-0.5 text-[11px] font-medium text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
              {{ prizePoolLabel }}
            </span>
          </div>
          <div class="min-h-0 flex-1 overflow-y-auto px-3 py-2">
            <div v-if="recentWinners.length === 0" class="py-10 text-center text-sm text-gray-400 dark:text-gray-500">
              {{ t('userLottery.noRecentWinners') }}
            </div>
            <TransitionGroup
              v-else
              tag="div"
              class="space-y-2"
              enter-active-class="transition-all duration-400 ease-out"
              enter-from-class="opacity-0 -translate-y-2"
              enter-to-class="opacity-100 translate-y-0"
            >
              <div
                v-for="item in recentWinners"
                :key="item.id"
                class="rounded-xl border border-gray-100 bg-gray-50/80 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-800/70"
              >
                <div class="flex items-center justify-between gap-2">
                  <p class="truncate text-xs font-medium text-gray-800 dark:text-white">{{ maskedIdentity(item.user_name, item.user_email) }}</p>
                  <span class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold" :class="badgeClass(item.prize_type)">
                    {{ prizeTypeLabel(item.prize_type) }}
                  </span>
                </div>
                <p class="mt-0.5 truncate text-[11px] text-gray-500 dark:text-gray-400">{{ item.prize_name }}</p>
              </div>
            </TransitionGroup>
          </div>
        </aside>
      </div>
    </div>

    <div v-else class="card p-12 text-center text-gray-500 dark:text-gray-400">
      {{ t('userLottery.noActiveActivity') }}
    </div>

    <!-- Wallet confirm dialog -->
    <Transition
      enter-active-class="transition-opacity duration-200"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div v-if="showWalletConfirm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm" @click.self="showWalletConfirm = false">
        <div class="mx-4 w-full max-w-sm rounded-2xl bg-white p-6 shadow-2xl dark:bg-dark-900">
          <p class="text-base font-bold text-gray-900 dark:text-white">{{ t('userLottery.confirmDraw') }}</p>
          <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">
            {{ t('userLottery.confirmWalletDraw', { amount: activity?.wallet_cost_per_draw.toFixed(2) || '0.00' }) }}
          </p>
          <div class="mt-5 flex gap-3">
            <button type="button" class="flex-1 rounded-xl border border-gray-200 py-2.5 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-200 dark:hover:bg-dark-700" @click="showWalletConfirm = false">
              {{ t('userLottery.cancelDraw') }}
            </button>
            <button type="button" class="flex-1 rounded-xl bg-blue-500 py-2.5 text-sm font-semibold text-white hover:bg-blue-600" @click="confirmDraw">
              {{ t('userLottery.confirmDraw') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- History drawer -->
    <Transition
      enter-active-class="transition-opacity duration-200"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-150"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div v-if="showHistory" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 backdrop-blur-sm sm:items-center" @click.self="showHistory = false">
        <div class="mx-0 h-[75vh] w-full max-w-lg overflow-hidden rounded-t-3xl bg-white shadow-2xl dark:bg-dark-900 sm:mx-4 sm:h-auto sm:max-h-[70vh] sm:rounded-2xl">
          <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <p class="text-base font-bold text-gray-900 dark:text-white">{{ t('userLottery.historyTitle') }}</p>
            <button type="button" class="rounded-lg p-1 hover:bg-gray-100 dark:hover:bg-dark-700" @click="showHistory = false">
              <svg class="h-5 w-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
            </button>
          </div>
          <div class="overflow-y-auto px-5 py-3" style="max-height: calc(75vh - 72px);">
            <div v-if="historyLoading" class="py-10 text-center text-sm text-gray-400">...</div>
            <div v-else-if="historyItems.length === 0" class="py-10 text-center text-sm text-gray-400 dark:text-gray-500">
              {{ t('userLottery.historyEmpty') }}
            </div>
            <div v-else class="space-y-2">
              <div
                v-for="item in historyItems"
                :key="item.id"
                class="flex items-center gap-3 rounded-xl border border-gray-100 bg-gray-50/80 px-4 py-3 dark:border-dark-700 dark:bg-dark-800/70"
              >
                <div class="min-w-0 flex-1">
                  <p class="text-sm font-semibold text-gray-800 dark:text-white">{{ item.prize_name }}</p>
                  <p v-if="item.prize_type === 'balance_redeem' && item.reward_reference" class="mt-0.5 font-mono text-xs text-emerald-600 dark:text-emerald-400">{{ item.reward_reference }}</p>
                  <p class="mt-0.5 text-[11px] text-gray-400">{{ item.created_at.replace('T', ' ').slice(0, 16) }}</p>
                </div>
                <div class="flex shrink-0 flex-col items-end gap-1">
                  <span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" :class="badgeClass(item.prize_type)">
                    {{ prizeTypeLabel(item.prize_type) }}
                  </span>
                  <span class="text-[10px] text-gray-400">
                    {{ item.chance_source === 'wallet' ? t('userLottery.historyWallet') : t('userLottery.historyDefault') }}
                    {{ item.chance_source === 'wallet' && item.wallet_amount ? `¥${item.wallet_amount.toFixed(2)}` : '' }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { paymentAPI } from '@/api/payment'
import { useAppStore } from '@/stores/app'
import type { LotteryActivity, LotteryOverview, LotteryPrize, LotteryDrawResult, LotteryDrawRecord } from '@/types/payment'

const { t } = useI18n()
const appStore = useAppStore()

const overview = ref<LotteryOverview | null>(null)
const activity = computed<LotteryActivity | null>(() => overview.value?.activity || null)
const recentWinners = computed(() => overview.value?.recent_winners || [])
const drawing = ref(false)
const highlightedIndex = ref<number>(-1)
const resultMessage = ref('')
const resultSub = ref('')
const showWalletConfirm = ref(false)
const showHistory = ref(false)
const historyLoading = ref(false)
const historyItems = ref<LotteryDrawRecord[]>([])

type SlotPrize = { key: string; label: string; badge: string; prizeType: LotteryPrize['prize_type']; kind: 'prize' }
type CenterSlot = { key: string; label: string; badge: string; prizeType: 'thanks'; kind: 'center' }
type BoardSlot = SlotPrize | CenterSlot

const availableChances = computed(() => overview.value?.user_state?.available_draw_times ?? 0)
const middleValue = computed(() => availableChances.value > 0 ? `${availableChances.value}` : `¥${activity.value?.wallet_cost_per_draw.toFixed(2) || '0.00'}`)
const middleHint = computed(() => availableChances.value > 0 ? t('userLottery.availableChances') : t('userLottery.drawWithWallet', { amount: activity.value?.wallet_cost_per_draw.toFixed(2) || '0.00' }))
const prizePoolLabel = computed(() => `${rawPrizes.value.length} ${t('userLottery.prizePoolUnits')}`)
const rawPrizes = computed(() => ((activity.value as (LotteryActivity & { prizes?: LotteryPrize[] }) | null)?.prizes || []).filter(p => p.status === 'active'))

const distributedPrizes = computed<SlotPrize[]>(() => {
  const source = rawPrizes.value
  if (source.length === 0) {
    return Array.from({ length: 8 }, (_, i) => ({ key: `ph-${i}`, label: t('userLottery.placeholderPrize'), badge: prizeTypeLabel('thanks'), prizeType: 'thanks' as const, kind: 'prize' as const }))
  }
  let remainder = 8 % source.length
  const pool: SlotPrize[] = []
  source.forEach((prize) => {
    const count = Math.floor(8 / source.length) + (remainder-- > 0 ? 1 : 0)
    for (let i = 0; i < count; i++) pool.push({ key: `p-${prize.id}-${i}`, label: prize.name, badge: prizeTypeLabel(prize.prize_type), prizeType: prize.prize_type, kind: 'prize' })
  })
  while (pool.length < 8) { const prize = source[pool.length % source.length]; pool.push({ key: `pf-${pool.length}`, label: prize.name, badge: prizeTypeLabel(prize.prize_type), prizeType: prize.prize_type, kind: 'prize' }) }
  // reorder so prizes spread evenly around the ring: top-row L→R, right col top→bot, bottom-row R→L, left col bot→top
  return [0, 3, 6, 1, 4, 7, 2, 5].map((pos, i) => pool[pos] || pool[i] || pool[0])
})

const boardSlots = computed<BoardSlot[]>(() => {
  const p = distributedPrizes.value
  return [p[0], p[1], p[2], p[3], { key: 'center', label: '', badge: '', prizeType: 'thanks', kind: 'center' }, p[4], p[5], p[6], p[7]]
})

// Clockwise ring order for 3x3 grid: top-left→top-mid→top-right→mid-right→bot-right→bot-mid→bot-left→mid-left
const candidateIndices = [0, 1, 2, 5, 8, 7, 6, 3]

function slotClass(slot: BoardSlot, index: number) {
  if (slot.kind === 'center') {
    return drawing.value
      ? 'bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg cursor-not-allowed'
      : 'bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg cursor-pointer hover:from-blue-600 hover:to-blue-700 active:scale-95'
  }
  if (highlightedIndex.value === index)
    return 'border-2 border-blue-400 bg-blue-50 shadow-lg shadow-blue-200/60 scale-105 dark:bg-blue-900/30 dark:border-blue-400 dark:shadow-blue-900/40'
  return 'border border-gray-200 bg-white/90 dark:border-dark-600 dark:bg-dark-800/80'
}

function prizeTypeLabel(type: LotteryPrize['prize_type'] | 'thanks') {
  if (type === 'balance_redeem') return t('userLottery.prizeBalance')
  if (type === 'coupon') return t('userLottery.prizeCoupon')
  return t('userLottery.prizeThanks')
}

function badgeClass(type: LotteryPrize['prize_type'] | 'thanks') {
  if (type === 'balance_redeem') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
  if (type === 'coupon') return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

function maskedIdentity(userName?: string, userEmail?: string) {
  if (userEmail) {
    const [n, d] = userEmail.split('@')
    return `${n.length <= 2 ? `${n[0] || '*'}*` : `${n.slice(0, 2)}***`}@${d || '***'}`
  }
  if (userName) return userName.length <= 2 ? `${userName[0] || '*'}*` : `${userName.slice(0, 2)}***`
  return t('userLottery.anonymousUser')
}

function handleDrawClick() {
  if (!activity.value || drawing.value) return
  // need wallet payment — show confirm
  if (availableChances.value <= 0) {
    showWalletConfirm.value = true
    return
  }
  executeDraw(false)
}

function confirmDraw() {
  showWalletConfirm.value = false
  executeDraw(true)
}

async function loadOverview() {
  try {
    const r = await paymentAPI.getActiveLottery()
    overview.value = r.data && 'activity' in r.data ? r.data as LotteryOverview : null
  } catch (e) {
    console.error(e)
    appStore.showError(t('userLottery.failedToLoad'))
  }
}

async function loadHistory() {
  if (historyLoading.value) return
  historyLoading.value = true
  try {
    const r = await paymentAPI.getMyDrawRecords({ activity_id: activity.value?.id, page_size: 50 })
    historyItems.value = (r.data?.items || []).filter(Boolean) as LotteryDrawRecord[]
  } catch (e) {
    console.error(e)
  } finally {
    historyLoading.value = false
  }
}

async function executeDraw(useWallet: boolean) {
  if (!activity.value || drawing.value) return
  drawing.value = true
  highlightedIndex.value = -1
  resultMessage.value = ''

  const totalLaps = 4 + Math.floor(Math.random() * 3)
  const totalFastSteps = totalLaps * candidateIndices.length
  let step = 0
  let currentRingPos = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let drawResult: LotteryDrawResult | null = null
  let drawError: unknown = null
  let apiDone = false

  const drawPromise = paymentAPI.drawLottery({ activity_id: activity.value.id, use_wallet: useWallet })
    .then(r => { drawResult = r.data })
    .catch(e => { drawError = e })
    .finally(() => { apiDone = true })

  function advance() {
    // If API errored, stop immediately
    if (drawError) {
      if (timer) clearTimeout(timer)
      drawing.value = false
      highlightedIndex.value = -1
      const err = drawError as any
      appStore.showError(err?.response?.data?.detail || err?.response?.data?.message || t('userLottery.drawFailed'))
      return
    }

    currentRingPos = (currentRingPos + 1) % candidateIndices.length
    highlightedIndex.value = candidateIndices[currentRingPos]
    step++

    // Fast phase: fixed 80ms
    if (step <= totalFastSteps) {
      timer = setTimeout(advance, 80)
      return
    }

    // Slowdown phase: wait for API result, then land on target
    if (apiDone && drawResult) {
      finalize(drawResult)
      return
    }

    // API not done yet, keep spinning slowly
    const extra = step - totalFastSteps
    const delay = Math.min(80 + extra * 20, 350)
    timer = setTimeout(advance, delay)
  }

  function finalize(result: LotteryDrawResult) {
    const prizeName = result?.record?.prize_name
    const prizeType = result?.record?.prize_type || 'thanks'
    const isThanks = result?.record?.result_code === 'thanks'

    const targetSlotIdx = boardSlots.value.findIndex(s => s.kind === 'prize' && s.label === prizeName)
    const targetRingPos = targetSlotIdx >= 0 ? candidateIndices.indexOf(targetSlotIdx) : -1

    if (targetRingPos < 0) {
      drawing.value = false
      updateState(result)
      showResult(prizeName || '', prizeType, isThanks)
      return
    }

    // Decelerate into target position
    const stepsToTarget = (targetRingPos - currentRingPos + candidateIndices.length) % candidateIndices.length || candidateIndices.length
    let landStep = 0
    function landTick() {
      currentRingPos = (currentRingPos + 1) % candidateIndices.length
      highlightedIndex.value = candidateIndices[currentRingPos]
      landStep++
      if (landStep < stepsToTarget) {
        const d = 200 + (landStep / stepsToTarget) * 400
        timer = setTimeout(landTick, d)
      } else {
        drawing.value = false
        updateState(result)
        showResult(prizeName || '', prizeType, isThanks)
      }
    }
    landTick()
  }

  advance()
  await drawPromise

  // If error arrived after advance() started but before apiDone check
  if (drawError && drawing.value) {
    if (timer) clearTimeout(timer)
    drawing.value = false
    highlightedIndex.value = -1
    const err = drawError as any
    appStore.showError(err?.response?.data?.detail || err?.response?.data?.message || t('userLottery.drawFailed'))
  }
}

function updateState(result: LotteryDrawResult) {
  if (!overview.value) return
  overview.value.user_state = result.user_state || overview.value.user_state
  if (result.record) {
    const existing = overview.value.recent_winners || []
    overview.value.recent_winners = [result.record as NonNullable<LotteryOverview['recent_winners']>[number], ...existing].slice(0, 20)
    // prepend to history cache if open
    if (showHistory.value) {
      historyItems.value = [result.record as LotteryDrawRecord, ...historyItems.value]
    }
  }
}

function showResult(prizeName: string, _prizeType: string, isThanks: boolean) {
  resultMessage.value = isThanks ? t('userLottery.prizeThanks') : `${t('userLottery.drawSuccess')}：${prizeName}`
  resultSub.value = isThanks ? '' : prizeName
  setTimeout(() => { resultMessage.value = ''; resultSub.value = '' }, 4000)
}

onMounted(loadOverview)
</script>
