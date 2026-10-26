import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import GenerationOptions from './GenerationOptions.vue'

describe('marketing generation options', () => {
  it('emits changes without mutating parent state and reflects parent updates', async () => {
    const options = Object.freeze({ newUsersOnly: false })
    const wrapper = mount(GenerationOptions, { props: { options } })
    await wrapper.get('input').setValue(true)
    expect(wrapper.emitted('update:newUsersOnly')).toEqual([[true]])
    expect(options.newUsersOnly).toBe(false)
    await wrapper.setProps({ options: { newUsersOnly: true } })
    expect((wrapper.get('input').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('input').setValue(false)
    expect(wrapper.emitted('update:newUsersOnly')).toEqual([[true], [false]])
    wrapper.unmount()
  })
})
