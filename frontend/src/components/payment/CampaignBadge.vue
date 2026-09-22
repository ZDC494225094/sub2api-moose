<template>
  <span class="inline-flex max-w-full items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-semibold leading-4"
    :class="campaign.kind === 'discount' ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-800 dark:bg-rose-950 dark:text-rose-300' : 'border-teal-200 bg-teal-50 text-teal-700 dark:border-teal-800 dark:bg-teal-950 dark:text-teal-300'">
    <Icon name="gift" size="xs" class="shrink-0" />
    <span>活动中 · {{ campaign.kind === 'discount' ? `${Number((campaign.percent / 10).toFixed(2))}折` : `赠${campaign.percent}%` }}</span>
  </span>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import type { RechargeCampaign } from '@/api/rechargeCampaigns'
defineProps<{ campaign: RechargeCampaign }>()
</script>
