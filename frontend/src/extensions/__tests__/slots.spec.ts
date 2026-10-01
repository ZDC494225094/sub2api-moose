import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { useExtensionStore } from '../store'
import { extensionIds, pendingExtensionPaths } from '../catalog'
import { extensionSlots } from '../slots'
import ExtensionSlot from '../components/ExtensionSlot.vue'
import ExtensionMount from '../components/ExtensionMount.vue'

const { route } = vi.hoisted(() => ({ route: { name: 'Home' } }))
vi.mock('vue-router', () => ({ useRoute: () => reactive(route) }))
vi.mock('../modules/customer-support/SupportWidget.vue', () => ({ __esModule: true, default: { template: '<aside data-support>support</aside>' } }))
vi.mock('../modules/recharge-campaigns/RechargeCampaignTicker.vue', () => ({ __esModule: true, default: { template: '<aside data-ticker>campaign</aside>' } }))

vi.mock('../modules/marketing/CouponSelection.vue', () => ({ __esModule: true, default: { template: '<aside data-coupon-selection />' } }))
vi.mock('../modules/marketing/CouponSummary.vue', () => ({ __esModule: true, default: { template: '<aside data-coupon-summary />' } }))

describe('extension UI slots', () => {
  beforeEach(() => { setActivePinia(createPinia()); route.name = 'Home' })
  it('only registers known extensions and unique mount keys', () => {
    expect(new Set(extensionSlots.map(entry => entry.key)).size).toBe(extensionSlots.length)
    for (const entry of extensionSlots) {
      if (entry.readiness === 'pending') {
        expect(pendingExtensionPaths).toHaveProperty(entry.extension)
        expect(extensionIds).not.toContain(entry.extension)
      } else expect(extensionIds).toContain(entry.extension)
    }
  })
  it('renders nothing while disabled or unknown, without replacing native content', async () => {
    const wrapper = mount(defineComponent({ components: { ExtensionSlot }, template: '<main><p data-native>upstream</p><ExtensionSlot name="application-overlay" /></main>' }))
    await flushPromises()
    expect(wrapper.get('[data-native]').text()).toBe('upstream')
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    useExtensionStore().flags = { 'customer-support': true }
    await flushPromises()
    expect(wrapper.find('[data-support]').exists()).toBe(true)
    useExtensionStore().flags = { 'customer-support': false }
    await flushPromises()
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    expect(wrapper.get('[data-native]').text()).toBe('upstream')
    wrapper.unmount()
  })
  it('does not add a second float outside the native Home route', async () => {
    route.name = 'PremiumHome'
    useExtensionStore().flags = { 'customer-support': true }
    const wrapper = mount(ExtensionSlot, { props: { name: 'application-overlay' } })
    await flushPromises()
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    await wrapper.setProps({ name: 'layout-overlay' }); await flushPromises()
    expect(wrapper.findAll('[data-support]')).toHaveLength(1)
    wrapper.unmount()
  })
  it('keeps independent widgets independently switchable', async () => {
    const store = useExtensionStore()
    store.flags = { 'customer-support': true, 'recharge-campaigns': false }
    const wrapper = mount(ExtensionSlot, { props: { name: 'header-bottom' } })
    await flushPromises()
    expect(wrapper.find('[data-ticker]').exists()).toBe(false)
    store.flags['recharge-campaigns'] = true
    await flushPromises()
    expect(wrapper.find('[data-ticker]').exists()).toBe(true)
    expect(wrapper.find('[data-support]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('mounts coupon slots only with real managed enablement', async () => {
    const store = useExtensionStore()
    const wrapper = mount(defineComponent({ components: { ExtensionSlot }, template: '<main><p data-native>upstream</p><ExtensionSlot name="checkout-discount-selection" /><ExtensionSlot name="checkout-discount-summary" /></main>' }))
    await flushPromises()
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)
    store.flags = { 'marketing-tools': true }; await flushPromises()
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(true)
    expect(wrapper.find('[data-coupon-summary]').exists()).toBe(true)
    store.flags['marketing-tools'] = false; await flushPromises()
    expect(wrapper.find('[data-coupon-selection]').exists()).toBe(false)
    expect(wrapper.find('[data-coupon-summary]').exists()).toBe(false)
    expect(wrapper.get('[data-native]').text()).toBe('upstream')
    wrapper.unmount()
  })
  it('contains optional widget failures rather than breaking the native page', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const Broken = defineComponent({ setup() { throw new Error('failed optional extension') }, template: '<div />' })
    const wrapper = mount(defineComponent({
      components: { ExtensionMount }, setup: () => ({ Broken }),
      template: '<main><p data-native>upstream still works</p><ExtensionMount :component="Broken" /></main>',
    }))
    await flushPromises()
    expect(wrapper.get('[data-native]').text()).toBe('upstream still works')
    expect(warn).toHaveBeenCalled()
    wrapper.unmount(); warn.mockRestore()
  })
})
