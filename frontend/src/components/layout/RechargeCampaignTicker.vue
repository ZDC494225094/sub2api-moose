<template>
  <div v-if="active.length" class="flex h-10 min-w-0 items-center gap-3 border-t border-teal-100 bg-teal-50/80 px-4 text-teal-900 dark:border-teal-900 dark:bg-teal-950/60 dark:text-teal-100 md:px-6" @mouseenter="paused = true" @mouseleave="paused = false" @focusin="paused = true" @focusout="paused = false">
    <Icon name="gift" size="sm" class="shrink-0 text-teal-600 dark:text-teal-400" />
    <div class="relative h-6 min-w-0 flex-1 overflow-hidden">
      <Transition name="campaign-scroll">
        <RouterLink v-if="current" :key="current.id" to="/purchase" class="absolute inset-0 flex min-w-0 items-center gap-2 text-xs sm:text-sm" :title="headline(current)">
          <span class="shrink-0 font-semibold">{{ campaignHeadline(current) }}</span>
          <span class="truncate text-teal-700 dark:text-teal-300">{{ current.name }}<span v-if="current.min_amount"> · 满 {{ current.min_amount }} 可享</span></span>
        </RouterLink>
      </Transition>
    </div>
    <RouterLink to="/purchase" class="flex shrink-0 items-center gap-1 text-xs font-semibold">去充值<Icon name="arrowRight" size="xs" /></RouterLink>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useIntervalFn, useNow, usePreferredReducedMotion } from '@vueuse/core'
import { campaignAPI, campaignHeadline, campaignStatus, type RechargeCampaign } from '@/api/rechargeCampaigns'
import Icon from '@/components/icons/Icon.vue'

const campaigns = ref<RechargeCampaign[]>([])
const now = useNow({ interval: 1000 })
const active = computed(() => campaigns.value.filter(a => campaignStatus(a, now.value.getTime()) === '进行中'))
const index = ref(0)
const paused = ref(false)
const reducedMotion = usePreferredReducedMotion()
const current = computed(() => active.value[index.value % active.value.length])
const headline = (a: RechargeCampaign) => `${a.name} · ${campaignHeadline(a)}${a.min_amount ? ` · 满 ${a.min_amount} 可享` : ''}`
async function refresh() {
  try { campaigns.value = (await campaignAPI.publicList()).data } catch { /* Preserve known campaigns until their expiry. */ }
}
onMounted(refresh)
useIntervalFn(refresh, 60000)
useIntervalFn(() => { if (!paused.value && reducedMotion.value !== 'reduce') index.value++ }, 5000)
</script>

<style scoped>
.campaign-scroll-enter-active, .campaign-scroll-leave-active { transition: transform .4s ease, opacity .4s ease; }
.campaign-scroll-enter-from { transform: translateY(100%); opacity: 0; }
.campaign-scroll-leave-to { transform: translateY(-100%); opacity: 0; }
@media (prefers-reduced-motion: reduce) {
  .campaign-scroll-enter-active, .campaign-scroll-leave-active { transition: none; }
}
</style>
