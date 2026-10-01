import type { RouteRecordRaw } from 'vue-router'
import { marketingRoutes } from './modules/marketing/routes'

// Host route registration stays one spread, independent of the number of extensions.
export const customExtensionRoutes: RouteRecordRaw[] = [
  ...marketingRoutes,
  {
    path: '/',
    name: 'PremiumHome',
    component: () => import('@/features/premium-home/runtime/PremiumHomeView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Home'
    }
  },
  {
    path: '/docs',
    name: 'PremiumDocs',
    component: () => import('@/features/premium-home/runtime/PremiumDocsView.vue'),
    meta: {
      requiresAuth: false,
      title: '文档中心'
    }
  },
  {
    path: '/playground',
    name: 'Playground',
    component: () => import('@/views/user/PlaygroundView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Playground',
      titleKey: 'playground.title',
      descriptionKey: 'playground.description'
    }
  },
  {
    path: '/recharge-campaigns/:id',
    name: 'RechargeCampaignLanding',
    component: () => import('@/extensions/modules/recharge-campaigns/RechargeCampaignLandingView.vue'),
    meta: { requiresAuth: false, title: '充值活动' }
  },
  {
    path: '/admin/operations',
    name: 'AdminOperations',
    component: () => import('@/extensions/modules/operations-analytics/OperationsFinanceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Operations Analysis',
      titleKey: 'admin.operations.title',
      descriptionKey: 'admin.operations.description'
    }
  },
  {
    path: '/admin/operations/conversion',
    name: 'AdminOperationsConversion',
    redirect: to => ({ path: '/admin/operations', query: to.query }),
  },
  {
    path: '/admin/orders/campaigns',
    name: 'AdminRechargeCampaigns',
    component: () => import('@/extensions/modules/recharge-campaigns/AdminRechargeCampaignsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, requiresPayment: true, title: 'Recharge Campaigns', titleKey: 'nav.rechargeCampaigns' }
  },
  {
    path: '/admin/custom-extensions',
    name: 'AdminCustomExtensions',
    component: () => import('./views/CustomExtensionsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true, title: '二开插件管理' },
  },
]
