import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { nextTick, type Component } from 'vue'
import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { useAppStore } from '@/stores/app'
import { useExtensionStore } from '@/extensions/store'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import PremiumHomeView from '@/features/premium-home/runtime/PremiumHomeView.vue'
import type { PublicSettings } from '@/types'

const { auth, adminMenus } = vi.hoisted(() => ({
  auth: { isAdmin: false, isSimpleMode: false, isAuthenticated: false, user: null, checkAuth: vi.fn() },
  adminMenus: [{ id: 'admin-guide', label: 'Admin custom guide', url: 'https://example.com/admin', icon_svg: '', visibility: 'admin', sort_order: 0 }],
}))

vi.mock('@/stores', async () => ({
  useAppStore: (await import('@/stores/app')).useAppStore,
  useAuthStore: () => auth,
  useOnboardingStore: () => ({ isCurrentStep: () => false }),
  useAdminSettingsStore: () => ({ customMenuItems: adminMenus, fetch: async () => {}, opsMonitoringEnabled: true, paymentEnabled: true }),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))
vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn() }))
vi.mock('@/composables/useBatchImageAccess', async () => {
  const { ref } = await import('vue')
  return { useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: async () => false }) }
})
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/features/premium-home/runtime/api', () => ({ getPublicPlans: async () => [], getPublicAnnouncements: async () => [] }))
vi.mock('@/features/premium-home/runtime/useHomeMotion', () => ({ useHomeMotion: () => {} }))
vi.mock('@/features/premium-home/runtime/ProviderNetwork.vue', () => ({ default: { template: '<div />' } }))

const wrappers: VueWrapper[] = []
beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  auth.isAdmin = false
  const app = useAppStore()
  app.publicSettingsLoaded = true
  app.cachedPublicSettings = {
    custom_menu_items: [{ id: 'user-guide', label: 'User custom guide', url: 'https://example.com/user', icon_svg: '', visibility: 'user', sort_order: 0 }],
    footer_friend_links: [{ label: 'Custom friend', url: 'https://example.com/friend' }],
    subscription_enabled: false,
  } as PublicSettings
})
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  document.documentElement.classList.remove('dark')
})

async function render(component: Component) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/dashboard')
  const wrapper = shallowMount(component, { global: { plugins: [router], stubs: {
    RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
  } } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('site customization display admission', () => {
  it.each([false, true])('gates cached custom menus for admin=%s and preserves native navigation', async isAdmin => {
    auth.isAdmin = isAdmin
    const store = useExtensionStore()
    const wrapper = await render(AppSidebar)
    expect(wrapper.text()).not.toContain('User custom guide')
    expect(wrapper.text()).not.toContain('Admin custom guide')
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)

    store.flags['site-customization'] = true; await nextTick()
    expect(wrapper.text()).toContain('User custom guide')
    expect(wrapper.text().includes('Admin custom guide')).toBe(isAdmin)
    for (const flags of [{ 'site-customization': false }, {}]) {
      store.flags = flags; await nextTick()
      expect(wrapper.text()).not.toContain('User custom guide')
      expect(wrapper.text()).not.toContain('Admin custom guide')
      expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    }
    store.flags['site-customization'] = true; await nextTick()
    expect(wrapper.text()).toContain('User custom guide')
    expect(useAppStore().cachedPublicSettings?.custom_menu_items).toHaveLength(1)
  })

  it('gates cached footer friend links while keeping native footer navigation and content', async () => {
    const store = useExtensionStore()
    const wrapper = await render(PremiumHomeView)
    expect(wrapper.find('.footer-friends').exists()).toBe(false)
    expect(wrapper.find('footer a[href="#top"]').exists()).toBe(true)
    expect(wrapper.find('.footer-bottom').exists()).toBe(true)

    store.flags['site-customization'] = true; await nextTick()
    expect(wrapper.get('.footer-friends').text()).toContain('Custom friend')
    for (const flags of [{ 'site-customization': false }, {}]) {
      store.flags = flags; await nextTick()
      expect(wrapper.find('.footer-friends').exists()).toBe(false)
      expect(wrapper.find('footer a[href="#top"]').exists()).toBe(true)
      expect(wrapper.find('.footer-bottom').exists()).toBe(true)
    }
    store.flags['site-customization'] = true; await nextTick()
    expect(wrapper.get('.footer-friends').text()).toContain('Custom friend')
    expect(useAppStore().cachedPublicSettings?.footer_friend_links).toHaveLength(1)
  })
})
