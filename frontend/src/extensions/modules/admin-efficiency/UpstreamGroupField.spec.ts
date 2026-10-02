import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UpstreamGroupField from './UpstreamGroupField.vue'
import LegacyField from '@/components/account/UpstreamGroupField.vue'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('upstream group field compatibility', () => {
  it('keeps the host component identity and v-model contract', async () => {
    expect(LegacyField).toBe(UpstreamGroupField)
    const wrapper = mount(UpstreamGroupField, { props: { modelValue: 'Old' } })
    const input = wrapper.get('input')
    expect((input.element as HTMLInputElement).value).toBe('Old')
    await input.setValue('New')
    expect(wrapper.emitted('update:modelValue')).toEqual([['New']])
    expect(input.attributes('maxlength')).toBe('100')
  })
  it('retains empty groups as choices and gives simultaneous fields unique ids', () => {
    const props = { groups: [{ id: 1, key: 'edge', name: 'Edge', account_count: 0, sort_order: 0 }], loading: true }
    const wrapper = mount({ components: { UpstreamGroupField }, setup: () => ({ props }), template: '<div><UpstreamGroupField v-bind="props" /><UpstreamGroupField v-bind="props" /></div>' })
    const inputs = wrapper.findAll('input')
    expect(inputs[0].attributes('id')).not.toBe(inputs[1].attributes('id'))
    for (const input of inputs) {
      expect(wrapper.get('#' + input.attributes('list') + ' option').attributes('value')).toBe('Edge')
      expect(wrapper.get('#' + input.attributes('aria-describedby')).text()).toBe('admin.accounts.upstreamGroupsLoading')
    }
  })
})
