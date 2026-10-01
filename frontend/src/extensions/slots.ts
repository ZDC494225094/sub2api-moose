import { defineAsyncComponent, markRaw, type Component } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import type { ExtensionId, pendingExtensionPaths } from './catalog'

export type ExtensionSlotName = 'application-overlay' | 'layout-overlay' | 'header-bottom' | 'checkout-summary' | 'checkout-amount' | 'checkout-overlay' | 'checkout-discount-selection' | 'checkout-discount-summary'
interface ExtensionSlotBase {
  key: string
  slot: ExtensionSlotName
  component: Component
  visible?: (route: RouteLocationNormalizedLoaded) => boolean
}

// Explicitly distinguish module ownership from supported runtime toggles.
export type ExtensionSlotEntry = ExtensionSlotBase & (
  { extension: ExtensionId; readiness?: 'managed' }
  | { extension: keyof typeof pendingExtensionPaths; readiness: 'pending' }
)

const customerSupport = markRaw(defineAsyncComponent(() => import('./modules/customer-support/SupportWidget.vue')))
const rechargeTicker = markRaw(defineAsyncComponent(() => import('./modules/recharge-campaigns/RechargeCampaignTicker.vue')))

// Adding another visual extension only changes this owned registry, never the
// upstream HomeView, AppHeader or AppLayout implementations.
export const extensionSlots: readonly ExtensionSlotEntry[] = [
  { key: 'coupon-selection', extension: 'marketing-tools', slot: 'checkout-discount-selection', component: markRaw(defineAsyncComponent(() => import('./modules/marketing/CouponSelection.vue'))) },
  { key: 'coupon-summary', extension: 'marketing-tools', slot: 'checkout-discount-summary', component: markRaw(defineAsyncComponent(() => import('./modules/marketing/CouponSummary.vue'))) },
  { key: 'home-support', extension: 'customer-support', slot: 'application-overlay', component: customerSupport, visible: route => route.name === 'Home' },
  { key: 'layout-support', extension: 'customer-support', slot: 'layout-overlay', component: customerSupport },
  { key: 'recharge-checkout-summary', extension: 'recharge-campaigns', slot: 'checkout-summary', component: markRaw(defineAsyncComponent(() => import('./modules/recharge-campaigns/CheckoutSummary.vue'))) },
  { key: 'recharge-checkout-amount', extension: 'recharge-campaigns', slot: 'checkout-amount', component: markRaw(defineAsyncComponent(() => import('./modules/recharge-campaigns/CheckoutAmount.vue'))) },
  { key: 'recharge-checkout-overlay', extension: 'recharge-campaigns', slot: 'checkout-overlay', component: markRaw(defineAsyncComponent(() => import('./modules/recharge-campaigns/CheckoutOverlay.vue'))) },
  { key: 'recharge-ticker', extension: 'recharge-campaigns', slot: 'header-bottom', component: rechargeTicker },
]
