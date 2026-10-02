import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import App from '@/App.vue'

const mocks = vi.hoisted(() => ({
  app: {} as Record<string, any>, route: {} as Record<string, any>,
  fetch: vi.fn(), replace: vi.fn(), setup: vi.fn(),
}))
vi.mock('vue-router', () => ({
  RouterView: { template: '<div data-page>page</div>' },
  useRoute: () => mocks.route,
  useRouter: () => ({ replace: mocks.replace, afterEach: vi.fn() }),
}))
vi.mock('@/extensions/runtime', () => ({ useCustomExtensionRuntime: vi.fn() }))
vi.mock('@/extensions/modules/site-customization', () => ({ useSiteCustomizationAdmission: () => ({ value: false }) }))
vi.mock('@/stores', () => ({
  useAppStore: () => mocks.app,
  useAuthStore: () => ({ isAuthenticated: false, isAdmin: false }),
  useSubscriptionStore: () => ({ clear: vi.fn() }),
  useAnnouncementStore: () => ({ reset: vi.fn() }),
  useAdminComplianceStore: () => ({ reset: vi.fn() }),
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))
vi.mock('@/api/setup', () => ({ getSetupStatus: mocks.setup }))
vi.mock('@/utils/branding', () => ({ updateFavicon: vi.fn() }))
vi.mock('@/utils/siteBillingMode', () => ({ resolveSiteBillingMode: () => 'balance' }))
vi.mock('@/router/title', () => ({ resolveRouteDocumentTitle: () => 'test' }))
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: { subscription: 'subscription' }, isFeatureFlagEnabled: () => false }))
vi.mock('@/components/common/Toast.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/NavigationProgress.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/AdminComplianceDialog.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/AnnouncementPopup.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/extensions/components/ExtensionSlot.vue', () => ({ default: { template: '<div />' } }))

describe('App request-specific access policy boundary', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.route = reactive({ path: '/dashboard', fullPath: '/dashboard', meta: {} })
    mocks.app = reactive({ cachedPublicSettings: { mainland_china_access_restriction_enabled: false }, fetchPublicSettings: mocks.fetch })
    mocks.setup.mockResolvedValue({ needs_setup: false })
    mocks.replace.mockResolvedValue(undefined)
  })
  it('never mounts from stale cache after a failed fresh request, and allows retry', async () => {
    mocks.fetch.mockResolvedValueOnce(null)
    const wrapper = mount(App)
    expect(wrapper.find('[data-page]').exists()).toBe(false)
    await flushPromises()
    expect(mocks.fetch).toHaveBeenCalledWith(true)
    expect(wrapper.find('[data-page]').exists()).toBe(false)
    expect(wrapper.text()).toContain('暂时无法确认')
    mocks.fetch.mockResolvedValueOnce(mocks.app.cachedPublicSettings)
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-page]').exists()).toBe(true)
    wrapper.unmount()
  })
  it('keeps the protected page unmounted while a restricted redirect is pending', async () => {
    let finish!: () => void
    mocks.replace.mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    mocks.fetch.mockImplementation(async () => {
      mocks.app.cachedPublicSettings = { mainland_china_access_restriction_enabled: true, mainland_china_access_restricted: true }
      return mocks.app.cachedPublicSettings
    })
    const wrapper = mount(App)
    await flushPromises()
    expect(mocks.replace).toHaveBeenCalledWith('/access-restricted')
    expect(wrapper.find('[data-page]').exists()).toBe(false)
    mocks.route.path = '/access-restricted'
    finish()
    await flushPromises()
    expect(wrapper.find('[data-page]').exists()).toBe(true)
    wrapper.unmount()
  })
  it('allows native setup without requiring public settings', async () => {
    mocks.route.path = '/setup'
    mocks.fetch.mockRejectedValue(new Error('not initialized'))
    const wrapper = mount(App)
    await flushPromises()
    expect(wrapper.find('[data-page]').exists()).toBe(true)
    wrapper.unmount()
  })
})
