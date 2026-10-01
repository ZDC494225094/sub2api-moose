<script setup lang="ts">
import CampaignShareDialog from './CampaignShareDialog.vue'
import { useCampaignCheckout } from './checkout'
const { sharingCampaign, affiliateCode, sharer } = useCampaignCheckout()
</script>

<template>
  <CampaignShareDialog :campaign="sharingCampaign" :affiliate-code="affiliateCode" :sharer="sharer" @close="sharingCampaign = null" />
</template>
