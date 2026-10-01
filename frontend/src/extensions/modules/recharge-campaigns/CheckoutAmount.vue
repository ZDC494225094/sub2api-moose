<script setup lang="ts">
import { computed } from 'vue'
import CampaignBadge from './CampaignBadge.vue'
import { useCampaignCheckout } from './checkout'
const props = defineProps<{ amount: number }>()
const { activeCampaigns, quickCampaign, quickCampaignLabel } = useCampaignCheckout()
const campaign = computed(() => quickCampaign(props.amount))
</script>

<template>
  <span v-if="campaign" class="mt-1 flex min-h-[3.25rem] flex-col items-center justify-center gap-1.5">
    <CampaignBadge :campaign="campaign" />
    <span class="break-all text-xs font-medium" :class="campaign.kind === 'discount' ? 'text-rose-600 dark:text-rose-300' : 'text-teal-700 dark:text-teal-300'">{{ quickCampaignLabel(amount) }}</span>
  </span>
  <span v-else-if="activeCampaigns.length" class="mt-1 flex min-h-[3.25rem] items-center justify-center text-xs font-normal text-gray-400">标准充值</span>
</template>
