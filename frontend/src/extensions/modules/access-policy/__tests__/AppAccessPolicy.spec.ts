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

// Plugin isolation must not introduce a new loading/error screen around native routes.
function expectNativePage(wrapper: ReturnType<typeof mount>) {
  expect(wrapper.find('[data-page]').exists()).toBe(true)
  expect(wrapper.find('[role="status"]').exists()).toBe(false)
  expect(wrapper.find('button').exists()).toBe(false)
  expect(wrapper.text()).not.toMatch(/访问策略|access policy/i)
}

describe('App preserves pre-plugin refresh behavior', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.route = reactive({ path: '/dashboard', fullPath: '/dashboard', meta: {} })
    mocks.app = reactive({ cachedPublicSettings: { mainland_china_access_restriction_enabled: false }, fetchPublicSettings: mocks.fetch })
    mocks.setup.mockResolvedValue({ needs_setup: false })
    mocks.replace.mockResolvedValue(undefined)
  })

  it('mounts the native page immediately even while setup status is pending', async () => {
    let finish!: (value: { needs_setup: boolean }) => void
    mocks.setup.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    mocks.fetch.mockResolvedValue(mocks.app.cachedPublicSettings)
    const wrapper = mount(App)
    expectNativePage(wrapper)
    expect(mocks.fetch).not.toHaveBeenCalled()
    finish({ needs_setup: false })
    await flushPromises()
    expectNativePage(wrapper)
    wrapper.unmount()
  })

  it.each([false, true, undefined])('does not block rendering while public settings are pending (cached restriction=%s)', async enabled => {
    mocks.app.cachedPublicSettings = enabled === undefined ? null : { mainland_china_access_restriction_enabled: enabled }
    let finish!: (value: unknown) => void
    mocks.fetch.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = mount(App)
    expectNativePage(wrapper)
    await flushPromises()
    expect(mocks.fetch).toHaveBeenCalledWith(true)
    expectNativePage(wrapper)
    finish({ mainland_china_access_restriction_enabled: false })
    await flushPromises()
    expectNativePage(wrapper)
    wrapper.unmount()
  })

  it('keeps the native page on public-settings failure without adding an error screen or retry policy', async () => {
    // The real app store normalizes API failures to null.
    mocks.fetch.mockResolvedValue(null)
    const wrapper = mount(App)
    expectNativePage(wrapper)
    await flushPromises()
    expectNativePage(wrapper)
    expect(mocks.replace).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('preserves the existing regional redirect without adding a render boundary', async () => {
    let finish!: () => void
    mocks.replace.mockImplementation(() => new Promise<void>(resolve => { finish = resolve }))
    mocks.fetch.mockImplementation(async () => {
      mocks.app.cachedPublicSettings = { mainland_china_access_restriction_enabled: true, mainland_china_access_restricted: true }
      return mocks.app.cachedPublicSettings
    })
    const wrapper = mount(App)
    expectNativePage(wrapper)
    await flushPromises()
    expect(mocks.replace).toHaveBeenCalledWith('/access-restricted')
    expectNativePage(wrapper)
    mocks.route.path = '/access-restricted'
    finish()
    await flushPromises()
    expectNativePage(wrapper)
    wrapper.unmount()
  })

  it('preserves the native setup redirect without fetching public settings', async () => {
    mocks.setup.mockResolvedValue({ needs_setup: true })
    const wrapper = mount(App)
    expectNativePage(wrapper)
    await flushPromises()
    expect(mocks.replace).toHaveBeenCalledWith('/setup')
    expect(mocks.fetch).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps the native page when setup status fails', async () => {
    mocks.setup.mockRejectedValue(new Error('setup unavailable'))
    mocks.fetch.mockResolvedValue(null)
    const wrapper = mount(App)
    await flushPromises()
    expectNativePage(wrapper)
    expect(mocks.fetch).toHaveBeenCalledWith(true)
    wrapper.unmount()
  })

  it('keeps native setup visible when public settings are unavailable', async () => {
    mocks.route.path = '/setup'
    mocks.fetch.mockResolvedValue(null)
    const wrapper = mount(App)
    await flushPromises()
    expectNativePage(wrapper)
    wrapper.unmount()
  })
})
