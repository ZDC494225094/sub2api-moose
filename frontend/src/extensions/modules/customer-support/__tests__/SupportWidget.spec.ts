import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { useExtensionStore } from '@/extensions/store'
import SupportWidget from '../SupportWidget.vue'

const { openPanel, unmounted } = vi.hoisted(() => ({ openPanel: vi.fn(), unmounted: vi.fn() }))
vi.mock('../CustomerServiceFloat.vue', () => ({ default: {
  props: ['alwaysVisible', 'directLink'], methods: { openPanel }, unmounted,
  template: '<aside data-widget :data-always="alwaysVisible" :data-direct="directLink" />',
} }))

describe('customer support lifecycle', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks() })
  it('never mounts or opens the widget when disabled, including the premium imperative entry', async () => {
    const wrapper = mount(SupportWidget, { props: { alwaysVisible: true, directLink: true } })
    wrapper.vm.openPanel()
    expect(wrapper.find('[data-widget]').exists()).toBe(false)
    expect(openPanel).not.toHaveBeenCalled()
    const store = useExtensionStore()
    store.flags['customer-support'] = true
    await flushPromises()
    expect(wrapper.get('[data-widget]').attributes('data-always')).toBe('true')
    expect(wrapper.get('[data-widget]').attributes('data-direct')).toBe('true')
    wrapper.vm.openPanel()
    expect(openPanel).toHaveBeenCalledOnce()
    store.flags['customer-support'] = false
    await flushPromises()
    expect(wrapper.find('[data-widget]').exists()).toBe(false)
    expect(unmounted).toHaveBeenCalledOnce()
    wrapper.vm.openPanel()
    expect(openPanel).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
