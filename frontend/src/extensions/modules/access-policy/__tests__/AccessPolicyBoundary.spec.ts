import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import AccessPolicyBoundary from '../AccessPolicyBoundary.vue'

describe('page security boundary', () => {
  it('does not mount protected routes on pending/failed request, retries and releases after resolution', async () => {
    const wrapper = mount(AccessPolicyBoundary, { props: { resolved: false, loading: true, setup: false }, slots: { default: '<div data-protected>protected page</div>' } })
    expect(wrapper.find('[data-protected]').exists()).toBe(false)
    expect(wrapper.find('button').exists()).toBe(false)
    await wrapper.setProps({ loading: false })
    expect(wrapper.find('[data-protected]').exists()).toBe(false)
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    await wrapper.setProps({ resolved: true })
    expect(wrapper.find('[data-protected]').exists()).toBe(true)
    await wrapper.setProps({ resolved: false })
    expect(wrapper.find('[data-protected]').exists()).toBe(false)
  })
  it('allows the existing setup route without a configured settings database', () => {
    const wrapper = mount(AccessPolicyBoundary, { props: { resolved: false, loading: false, setup: true }, slots: { default: '<div data-setup />' } })
    expect(wrapper.find('[data-setup]').exists()).toBe(true)
  })
})
