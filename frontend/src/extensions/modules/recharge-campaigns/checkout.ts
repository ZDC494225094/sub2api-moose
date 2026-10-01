import { computed, inject, onScopeDispose, provide, ref, watch, type InjectionKey } from 'vue'
import { useIntervalFn, useNow } from '@vueuse/core'
import { getAffiliateDetail } from '@/api/user'
import type { CheckoutContribution, CheckoutExtensionContext } from '../../checkout'
import { useExtensionStore } from '../../store'
import { automaticCampaign, campaignAmounts, campaignAPI, campaignStatus, type RechargeCampaign } from './api'

function createController(context: CheckoutExtensionContext) {
  const store = useExtensionStore()
  const enabled = computed(() => store.enabled('recharge-campaigns'))
  const { now, pause: pauseClock, resume: resumeClock } = useNow({ interval: 1000, controls: true })
  const campaigns = ref<RechargeCampaign[]>([])
  const activeCampaigns = computed(() => enabled.value
    ? campaigns.value.filter(item => campaignStatus(item, now.value.getTime()) === '进行中') : [])
  const selectedCampaign = computed(() => enabled.value
    ? automaticCampaign(campaigns.value, context.amount.value, now.value.getTime()) : null)
  const sharingCampaign = ref<RechargeCampaign | null>(null)
  const campaignError = ref('')
  const loadFailed = ref(false)
  const loaded = ref(false)
  const pendingInitial = computed(() => enabled.value && !loaded.value && !loadFailed.value)
  const canSubmit = computed(() => !enabled.value || (loaded.value && !loadFailed.value))
  const affiliateCode = ref('')
  const sharer = computed(context.sharer)
  let revision = 0
  let disposed = false
  let inFlight: Promise<void> | undefined

  function clear() {
    revision++
    inFlight = undefined
    campaigns.value = []
    sharingCampaign.value = null
    affiliateCode.value = ''
    loadFailed.value = false
    loaded.value = false
    campaignError.value = ''
  }
  function loadCampaigns(): Promise<void> {
    if (disposed) return Promise.resolve()
    if (!enabled.value) { clear(); return Promise.resolve() }
    if (inFlight) return inFlight
    const requestRevision = ++revision
    const current = () => !disposed && enabled.value && requestRevision === revision
    inFlight = (async () => {
      try {
        const result = (await campaignAPI.publicList()).data
        if (!current()) return
        campaigns.value = result
        loaded.value = true
        loadFailed.value = false
        campaignError.value = ''
      } catch {
        if (!current()) return
        loadFailed.value = true
        campaignError.value = '活动加载失败，请刷新后重试。'
      } finally {
        if (requestRevision === revision) inFlight = undefined
      }
    })()
    return inFlight
  }
  // Synchronous invalidation also covers off -> on before a previous request
  // completes, not just the simpler late-result-while-disabled case.
  watch(enabled, () => { void loadCampaigns() }, { flush: 'sync' })
  const { pause: pauseRefresh, resume: resumeRefresh } = useIntervalFn(() => {
    if (context.mayRefresh()) void loadCampaigns()
  }, 60000, { immediate: false })
  watch(enabled, value => {
    if (value) { resumeClock(); resumeRefresh() }
    else { pauseClock(); pauseRefresh() }
  }, { immediate: true, flush: 'sync' })
  onScopeDispose(() => { disposed = true; clear() })

  function quickCampaign(amount: number) {
    return enabled.value ? automaticCampaign(campaigns.value, amount, now.value.getTime()) : null
  }
  function quickCampaignLabel(amount: number) {
    const activity = quickCampaign(amount)
    const quote = campaignAmounts(activity, amount, context.balanceMultiplier())
    const fee = context.feeRate() > 0 ? Math.ceil(quote.principal * context.feeRate()) / 100 : 0
    return activity?.kind === 'discount'
      ? `实付 ${context.formatPaymentAmount(Math.round((quote.principal + fee) * 100) / 100)}`
      : `到账 $${quote.credited.toFixed(2)}`
  }
  async function shareCampaign() {
    const selected = selectedCampaign.value
    if (!selected) return
    if (!selected.reward_percent) {
      affiliateCode.value = ''
      sharingCampaign.value = selected
      return
    }
    const requestRevision = revision
    const current = () => !disposed && enabled.value && requestRevision === revision
      && selectedCampaign.value?.id === selected.id && selectedCampaign.value?.revision === selected.revision
    try {
      const detail = await getAffiliateDetail()
      if (!current()) return
      affiliateCode.value = detail.aff_code
      sharingCampaign.value = selected
    } catch {
      if (current()) campaignError.value = '无法获取邀请码，请稍后重试。'
    }
  }
  return { selectedCampaign, activeCampaigns, sharingCampaign, campaignError, affiliateCode, sharer,
    canSubmit, pendingInitial, loadCampaigns, quickCampaign, quickCampaignLabel, shareCampaign }
}

type CampaignCheckout = ReturnType<typeof createController>
const campaignCheckoutKey: InjectionKey<CampaignCheckout> = Symbol('recharge-campaign-checkout')

export function useCampaignCheckout(): CampaignCheckout {
  const controller = inject(campaignCheckoutKey)
  if (!controller) throw new Error('Campaign checkout widget requires the checkout extension host')
  return controller
}

export function createCampaignCheckout(context: CheckoutExtensionContext): CheckoutContribution {
  const controller = createController(context)
  provide(campaignCheckoutKey, controller)
  const { selectedCampaign, activeCampaigns } = controller
  return {
    id: 'recharge-campaigns',
    quote: computed(() => selectedCampaign.value
      ? campaignAmounts(selectedCampaign.value, context.amount.value, context.balanceMultiplier()) : null),
    adjustsAmountLimits: computed(() => activeCampaigns.value.length > 0),
    excludesCoupon: computed(() => !!selectedCampaign.value),
    canSubmit: controller.canSubmit,
    initialize: controller.loadCampaigns,
    prepareOrder(payload) {
      if (payload.order_type !== 'balance') return
      const selected = controller.quickCampaign(payload.amount)
      if (!selected) return
      payload.campaign_id = selected.id
      payload.campaign_revision = selected.revision
    },
  }
}
