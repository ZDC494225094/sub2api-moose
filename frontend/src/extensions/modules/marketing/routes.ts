import type { RouteRecordRaw } from 'vue-router'

// Managed admission gates writes; these authenticated pages retain historical reads.
export const marketingRoutes: RouteRecordRaw[] = [
  {
    path: '/lottery',
    name: 'Lottery',
    component: () => import('./LotteryView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Lottery',
      titleKey: 'nav.lottery',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders/coupons',
    name: 'AdminPaymentCoupons',
    component: () => import('./AdminCouponTemplatesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Coupon Templates',
      titleKey: 'nav.couponTemplates',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders/lottery',
    name: 'AdminPaymentLottery',
    component: () => import('./AdminLotteryView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Lottery',
      titleKey: 'nav.marketingLottery',
      requiresPayment: true
    }
  },
]
