<template>
  <AppLayout>
    <div class="mx-auto max-w-lg px-4 py-6">
      <!-- Step indicator -->
      <div class="mb-6 flex items-center justify-center gap-2 text-xs">
        <span :class="stepBadge(1)">{{ t('wechatLottery.stepAuth') }}</span>
        <span class="text-gray-300">›</span>
        <span :class="stepBadge(2)">{{ t('wechatLottery.stepBind') }}</span>
        <span class="text-gray-300">›</span>
        <span :class="stepBadge(3)">{{ t('wechatLottery.stepDraw') }}</span>
      </div>

      <!-- Loading / error -->
      <div v-if="fatalError" class="rounded-2xl border border-red-200 bg-red-50 p-5 text-center dark:border-red-900/40 dark:bg-red-950/20">
        <p class="text-sm font-medium text-red-700 dark:text-red-300">{{ fatalError }}</p>
      </div>
      <div v-else-if="booting" class="py-16 text-center text-sm text-gray-400">
        {{ t('common.loading') }}...
      </div>

      <template v-else>
        <!-- Bind prompt -->
        <div v-if="step === 2" class="rounded-2xl border border-amber-200 bg-amber-50 p-5 text-center dark:border-amber-900/40 dark:bg-amber-950/20">
          <p class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ t('wechatLottery.bindPrompt') }}</p>
          <button class="btn btn-primary mt-4 w-full" :disabled="binding" @click="doBind">
            {{ binding ? t('common.processing') : t('wechatLottery.bindNow') }}
          </button>
        </div>

        <!-- Lottery -->
        <div v-if="step === 3">
          <div v-if="!activity" class="rounded-2xl border border-gray-200 bg-white p-8 text-center dark:border-dark-700 dark:bg-dark-900">
            <p class="text-sm text-gray-500">{{ t('wechatLottery.noActivity') }}</p>
          </div>
          <div v-else>
            <div class="overflow-hidden rounded-3xl border border-emerald-200/60 bg-gradient-to-br from-emerald-50 via-white to-teal-50 p-6 text-center shadow-xl dark:border-emerald-900/30 dark:from-emerald-950/30 dark:via-dark-900 dark:to-teal-950/20">
              <p class="text-xs font-medium text-emerald-600 dark:text-emerald-400">{{ t('wechatLottery.oaExclusive') }}</p>
              <h1 class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ activity.name }}</h1>
              <p v-if="activity.description" class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ activity.description }}</p>

              <div class="mt-4 flex items-center justify-center gap-2 text-sm">
                <span class="rounded-full bg-white/80 px-3 py-1 font-semibold text-gray-700 shadow-sm dark:bg-dark-800 dark:text-gray-200">
                  {{ t('wechatLottery.chances', { n: availableChances }) }}
                </span>
              </div>

              <button
                class="mt-5 w-full rounded-2xl bg-gradient-to-br from-emerald-500 to-teal-600 py-4 text-lg font-extrabold text-white shadow-lg transition active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
                :disabled="drawing || availableChances <= 0"
                @click="doDraw"
              >
                {{ drawing ? t('common.processing') : t('wechatLottery.drawNow') }}
              </button>
              <p v-if="availableChances <= 0" class="mt-2 text-xs text-gray-400">{{ t('wechatLottery.noChances') }}</p>
            </div>

            <!-- Result -->
            <Transition enter-active-class="transition-all duration-300" enter-from-class="opacity-0 translate-y-3" enter-to-class="opacity-100 translate-y-0">
              <div v-if="resultTitle" class="mt-4 rounded-2xl border p-5 text-center" :class="resultWon ? 'border-amber-300 bg-amber-50 dark:border-amber-800/40 dark:bg-amber-950/20' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900'">
                <p class="text-lg font-extrabold" :class="resultWon ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-200'">{{ resultTitle }}</p>
                <p v-if="resultSub" class="mt-1 text-sm text-gray-500">{{ resultSub }}</p>
                <p v-if="notifySent" class="mt-2 text-xs text-emerald-600 dark:text-emerald-400">✓ {{ t('wechatLottery.pushedToOA') }}</p>
              </div>
            </Transition>

            <!-- Prizes -->
            <div v-if="prizes.length" class="mt-5 rounded-2xl border border-gray-100 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
              <p class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('wechatLottery.prizePool') }}</p>
              <div class="space-y-2">
                <div v-for="p in prizes" :key="p.id" class="flex items-center justify-between rounded-xl bg-gray-50 px-3 py-2 dark:bg-dark-800">
                  <span class="text-sm font-medium text-gray-800 dark:text-gray-100">{{ p.name }}</span>
                  <span class="text-xs text-gray-400">× {{ p.remaining_stock }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { marketingAPI, wechatAPI } from './api'
import { useAppStore } from '@/stores/app'
import { availableLotteryChances } from '@/extensions/modules/marketing/lottery-state'
import type { LotteryActivity, LotteryOverview, LotteryDrawResult } from '@/types/payment'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const booting = ref(true)
const fatalError = ref('')
const step = ref(1)
const openid = ref('')
const boundOpenid = ref('')
const binding = ref(false)
const overview = ref<LotteryOverview | null>(null)
const activity = computed<LotteryActivity | null>(() => overview.value?.activity || null)
const prizes = computed(() => ((activity.value as any)?.prizes || []).filter((p: any) => p.status === 'active'))
const availableChances = computed(() => availableLotteryChances(overview.value))
const drawing = ref(false)
const resultTitle = ref('')
const resultSub = ref('')
const resultWon = ref(false)
const notifySent = ref(false)

function stepBadge(n: number) {
  const base = 'rounded-full px-3 py-1 font-medium'
  return step.value >= n
    ? `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300`
    : `${base} bg-gray-100 text-gray-400 dark:bg-dark-800 dark:text-gray-500`
}

function errorMessage(e: any): string {
  return e?.response?.data?.detail || e?.response?.data?.message || e?.message || t('wechatLottery.failed')
}

onMounted(async () => {
  try {
    // 1. openid: from OAuth callback (?openid=) or existing binding
    const q = route.query.openid as string | undefined
    if (q) {
      openid.value = q
      // clean the url
      const url = new URL(window.location.href)
      url.searchParams.delete('openid')
      window.history.replaceState({}, '', url.toString())
    }
    if (!openid.value) {
      // try existing binding first (returning user)
      try {
        const b = await wechatAPI.getBinding()
        if (b.data?.openid) {
          openid.value = b.data.openid
          boundOpenid.value = b.data.openid
        }
      } catch { /* ignore, will go OAuth */ }
    }
    if (!openid.value) {
      // 2. redirect to WeChat webpage authorization
      step.value = 1
      try {
        const r = await wechatAPI.getOAuthURL('/wx-lottery')
        const authUrl = r.data?.url
        if (!authUrl) throw new Error(t('wechatLottery.oauthNotConfigured'))
        window.location.href = authUrl
        return
      } catch (e: any) {
        // Not in WeChat or not configured: allow binding-less preview? No — fail clearly.
        fatalError.value = errorMessage(e)
        return
      }
    }
    // 3. ensure binding
    if (!boundOpenid.value) {
      try {
        const b = await wechatAPI.getBinding()
        boundOpenid.value = b.data?.openid || ''
      } catch { /* ignore */ }
    }
    if (boundOpenid.value && boundOpenid.value !== openid.value) {
      // bound to a different openid (e.g. changed WeChat account) — rebind
      boundOpenid.value = ''
    }
    if (!boundOpenid.value) {
      step.value = 2
      return
    }
    // 4. load activity
    await loadActivity()
    step.value = 3
  } finally {
    booting.value = false
  }
})

async function doBind() {
  binding.value = true
  try {
    const r = await wechatAPI.bindOpenID(openid.value)
    boundOpenid.value = r.data.openid
    await loadActivity()
    step.value = 3
  } catch (e: any) {
    appStore.showError(errorMessage(e))
  } finally {
    binding.value = false
  }
}

async function loadActivity() {
  const r = await marketingAPI.getWechatActiveLottery()
  overview.value = r.data && 'activity' in r.data ? (r.data as LotteryOverview) : null
}

async function doDraw() {
  if (!activity.value || drawing.value || availableChances.value <= 0) return
  drawing.value = true
  resultTitle.value = ''
  notifySent.value = false
  try {
    const r = await marketingAPI.drawLottery({
      activity_id: activity.value.id,
      use_wallet: false,
      wechat_openid: openid.value,
    })
    const result = r.data as LotteryDrawResult
    const rec: any = (result as any).record
    resultWon.value = rec?.result_code === 'win'
    resultTitle.value = rec?.prize_name || t('wechatLottery.drawDone')
    resultSub.value = resultWon.value ? t('wechatLottery.youWon') : t('wechatLottery.thanksForJoining')
    // push result to OA conversation (best effort)
    try {
      await wechatAPI.notifyDrawResult(activity.value.id)
      notifySent.value = true
    } catch (e) {
      console.error('notify failed', e)
    }
    await loadActivity()
  } catch (e: any) {
    appStore.showError(errorMessage(e))
  } finally {
    drawing.value = false
  }
}
</script>
