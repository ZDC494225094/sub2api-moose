<template>
  <main class="campaign-landing min-h-screen bg-[#f4f8f5] text-[#182e29]">
    <header class="mx-auto flex max-w-5xl items-center justify-between px-5 py-6 sm:px-8">
      <RouterLink to="/" class="flex min-w-0 items-center gap-3"><img :src="logo" alt="站点 Logo" class="h-10 w-10 rounded-xl object-contain" /><span class="truncate text-lg font-semibold tracking-tight">{{ app.siteName }}</span></RouterLink>
      <RouterLink to="/" class="flex shrink-0 items-center gap-1 text-sm text-[#62776d]">了解平台<Icon name="arrowRight" size="xs" /></RouterLink>
    </header>
    <div class="mx-auto max-w-5xl px-4 pb-12 sm:px-8">
      <section class="overflow-hidden rounded-[28px] border border-[#dce8df] bg-white shadow-[0_24px_80px_-48px_#346955]">
        <div class="grid items-center bg-[#edf5ef] px-6 pt-10 sm:px-10 md:grid-cols-2 md:pt-6">
          <div class="relative z-10">
            <p class="text-[11px] font-semibold tracking-[0.2em] text-[#488368]">ONE API. MORE POSSIBILITIES.</p>
            <h1 class="mt-5 text-3xl font-semibold leading-[1.4] tracking-tight sm:text-4xl">让顶尖 AI，<br />成为你的<span class="text-[#138568]">创造力。</span></h1>
            <p class="mt-5 text-sm leading-7 text-[#637a6e]">一站式 AI API 聚合平台<br />统一接入多款模型，让灵感更快成为作品。</p>
          </div>
          <CampaignGlobe :logo="logo" />
        </div>
        <div class="grid grid-cols-3 gap-y-5 border-b border-[#edf1ee] px-6 py-6 sm:grid-cols-6 sm:px-10">
          <div v-for="provider in providers" :key="provider.model" class="flex items-center justify-center gap-2 text-xs text-[#65766c]"><ModelIcon :model="provider.model" size="22px" />{{ provider.name }}</div>
        </div>
        <div v-if="loading" class="px-6 py-16 text-center text-[#637a6e]" role="status">正在为你加载充值礼遇…</div>
        <div v-else-if="campaign" class="p-6 sm:p-10">
          <div class="flex flex-wrap items-center gap-3"><span class="rounded-full bg-[#edf6f0] px-3 py-1 text-xs font-medium text-[#258166]">{{ campaignStatus(campaign, now.getTime()) }}</span><span class="text-xs tracking-widest text-[#728479]">充值专享礼遇</span></div>
          <h2 class="mt-5 break-words text-xl font-medium sm:text-2xl">{{ campaign.name }}</h2>
          <p class="mt-4 text-4xl font-semibold tracking-tight sm:text-5xl" :class="campaign.kind === 'discount' ? 'text-rose-600' : 'text-[#0b8468]'">{{ campaignHeadline(campaign) }}</p>
          <p class="mt-5 whitespace-pre-line break-words text-sm leading-7 text-[#62776d]">{{ campaign.description || '为灵感补充能量，让每一次创造更从容。' }}</p>
          <div v-if="route.query.aff" class="mt-6 flex items-center gap-3 rounded-2xl bg-[#f3f8f4] p-4 text-sm text-[#3e6e58]"><Icon name="gift" size="sm" /><span>好友为你分享了一份充值福利，新用户注册时自动带入邀请码。</span></div>
          <dl class="mt-8 grid gap-6 border-y border-[#e8eee9] py-6 sm:grid-cols-2">
            <div><dt class="text-xs text-[#7d8c82]">参与门槛</dt><dd class="mt-2 text-sm font-medium">{{ campaign.min_amount ? `单笔充值满 ${campaign.min_amount}` : '不限充值门槛' }} · 自动享受</dd></div>
            <div><dt class="text-xs text-[#7d8c82]">活动时间</dt><dd class="mt-2 text-sm leading-6">{{ date(campaign.starts_at) }}<br />至 {{ date(campaign.ends_at) }}</dd></div>
          </dl>
          <details class="mt-6 text-sm text-[#62776d]"><summary class="cursor-pointer font-medium">活动与邀请奖励说明</summary><p class="mt-3 leading-7">仅限余额充值，不与优惠券叠加；多个活动按充值页面规则自动择优，到账金额及手续费以结算页为准。<template v-if="campaign.reward_percent"><br />邀请人享 {{ campaign.reward_percent }}% 奖励，每单最高 ${{ campaign.reward_cap }}，冻结 {{ campaign.freeze_hours }} 小时；{{ campaign.new_invitees_only ? '仅限活动期内新绑定的好友。' : '已绑定的好友也可参与。' }}退款按比例追回奖励。</template></p></details>
          <RouterLink :to="destination" class="mt-8 flex w-full items-center justify-center gap-3 rounded-2xl bg-[#147d62] px-6 py-4 text-sm font-medium text-white transition hover:bg-[#0b6650] sm:w-fit">{{ campaignStatus(campaign, now.getTime()) === '进行中' ? (auth.isAuthenticated ? '立即充值，享受礼遇' : '登录 / 注册，参与礼遇') : '前往充值页查看可用优惠' }}<Icon name="arrowRight" size="sm" /></RouterLink>
          <p class="mt-3 text-xs text-[#8b978f]">满足活动门槛自动生效，无需手动选择</p>
        </div>
        <div v-else class="p-10 text-center"><h2 class="text-xl font-medium">{{ error || '这场礼遇已结束，新的可能仍在继续' }}</h2><RouterLink to="/purchase" class="mt-6 inline-flex items-center gap-2 text-[#138568]">查看当前充值福利<Icon name="arrowRight" size="sm" /></RouterLink></div>
      </section>
      <footer class="pt-7 text-center text-xs text-[#8b978f]">{{ app.siteName }} · 为下一次创造，连接 AI</footer>
    </div>
  </main>
</template>
<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useNow } from '@vueuse/core'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { storeAffiliateReferralCode } from '@/utils/oauthAffiliate'
import { sanitizeUrl } from '@/utils/url'
import { campaignAPI, campaignHeadline, campaignStatus, type RechargeCampaign } from '@/api/rechargeCampaigns'
import CampaignGlobe from '@/components/payment/CampaignGlobe.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Icon from '@/components/icons/Icon.vue'
const route = useRoute(), auth = useAuthStore(), app = useAppStore()
const campaign = ref<RechargeCampaign | null>(null), loading = ref(true), error = ref('')
const now = useNow({ interval: 1000 })
const logo = computed(() => sanitizeUrl(app.cachedPublicSettings?.site_logo || app.siteLogo || '/logo.svg', { allowRelative: true, allowDataUrl: true }) || '/logo.svg')
const providers = [{ name: 'OpenAI', model: 'gpt' }, { name: 'Claude', model: 'claude' }, { name: 'Gemini', model: 'gemini' }, { name: 'DeepSeek', model: 'deepseek' }, { name: 'Qwen', model: 'qwen' }, { name: 'Grok', model: 'grok' }]
const date = (v: string) => new Date(v).toLocaleString('zh-CN', { hour12: false, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
const destination = computed(() => {
  const target = `/purchase?campaign=${campaign.value?.id}`
  return auth.isAuthenticated ? target : { path: '/login', query: { redirect: target } }
})
let request = 0
async function load() {
  const current = ++request
  loading.value = true; error.value = ''
  storeAffiliateReferralCode(route.query.aff)
  try {
    const items = (await campaignAPI.publicList()).data
    if (current !== request) return
    campaign.value = items.find(a => a.id === Number(route.params.id)) || null
    if (campaign.value) { try { sessionStorage.setItem('recharge_campaign_referral', String(campaign.value.id)) } catch { /* storage unavailable */ } }
  } catch { if (current === request) { campaign.value = null; error.value = '暂时无法加载活动，请稍后重试' } }
  finally { if (current === request) loading.value = false }
}
onMounted(load)
watch(() => [route.params.id, route.query.aff], load)
</script>
<style scoped>
.campaign-landing { font-family: Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif; -webkit-font-smoothing: antialiased; }
</style>
