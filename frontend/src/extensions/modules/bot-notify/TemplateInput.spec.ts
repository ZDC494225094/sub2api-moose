import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TemplateInput from './TemplateInput.vue'

describe('bot notification templates', () => {
  it('offers all placeholders with explanations and inserts at the selection', async () => {
    const wrapper = mount(TemplateInput, { props: { modelValue: 'hello world', label: '推送模板' } })
    expect(wrapper.findAll('button')).toHaveLength(5)
    expect(wrapper.text()).toContain('sub 内部公告服务')
    const input = wrapper.get('textarea').element
    input.setSelectionRange(6, 11)
    await wrapper.findAll('button')[0]!.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['hello {channel_status}'])
    expect(wrapper.findAll('button').map(button => button.text())).toEqual([
      '{channel_status}', '{channel_status_abnormal}', '{announcements}', '{site_name}', '{now}',
    ])
  })
})
