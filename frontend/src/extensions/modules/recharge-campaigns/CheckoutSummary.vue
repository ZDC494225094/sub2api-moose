<script setup lang="ts">
import { campaignHeadline } from './api'
import CampaignBadge from './CampaignBadge.vue'
import { useCampaignCheckout } from './checkout'
const { selectedCampaign, campaignError, pendingInitial, shareCampaign } = useCampaignCheckout()
</script>

<template>
  <div v-if="selectedCampaign || campaignError || pendingInitial" class="space-y-2 border-l-2 border-teal-500 pl-4">
    <p v-if="pendingInitial" class="text-sm text-gray-500" role="status">正在确认充值活动，请稍候…</p>
    <div v-if="selectedCampaign" class="flex flex-wrap items-center justify-between gap-2">
      <div class="flex flex-wrap items-center gap-2"><CampaignBadge :campaign="selectedCampaign" /><span class="text-sm font-semibold">{{ selectedCampaign.name }} · {{ campaignHeadline(selectedCampaign) }}</span></div>
      <button class="btn btn-secondary btn-sm" @click="shareCampaign">分享福利</button>
    </div>
    <template v-if="selectedCampaign">
      <p class="text-sm text-gray-500">{{ selectedCampaign.description }}<span v-if="selectedCampaign.min_amount"> · 满 {{ selectedCampaign.min_amount }} 可享</span></p>
      <p class="text-xs text-gray-500">{{ new Date(selectedCampaign.ends_at).toLocaleString() }} 截止 · 已自动享受，不与优惠券叠加</p>
      <p v-if="selectedCampaign.reward_percent" class="text-sm text-teal-600">邀请好友充值得 {{ selectedCampaign.reward_percent }}% 奖励，每单最高 ${{ selectedCampaign.reward_cap }}，冻结 {{ selectedCampaign.freeze_hours }} 小时。{{ selectedCampaign.new_invitees_only ? '仅限活动期内新邀请的好友。' : '' }}</p>
    </template>
    <p v-if="campaignError" class="text-sm text-red-500" role="alert">{{ campaignError }}</p>
  </div>
</template>
