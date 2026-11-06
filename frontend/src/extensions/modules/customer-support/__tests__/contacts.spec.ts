import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ContactEntriesEditor from '../ContactEntriesEditor.vue'
import { MAX_CONTACT_ENTRIES, parseContactEntries, parseContactLinks, serializeContactEntries } from '../contacts'

describe('contact entry storage format', () => {
  it('keeps a legacy single value as one unnamed entry', () => {
    expect(parseContactEntries('QQ群 987654')).toEqual([{ label: '', value: 'QQ群 987654' }])
    expect(parseContactEntries('')).toEqual([])
  })

  it('round-trips several named entries one per line', () => {
    const raw = serializeContactEntries([
      { label: 'QQ 一群', value: '123' },
      { label: '', value: '456' },
      { label: 'empty', value: '   ' },
    ])
    expect(raw).toBe('QQ 一群 | 123\n456')
    expect(parseContactEntries(raw)).toEqual([{ label: 'QQ 一群', value: '123' }, { label: '', value: '456' }])
  })

  it('never splits a URL on a bare pipe and neutralises pipes in names', () => {
    expect(parseContactEntries('https://x.example/a|b')).toEqual([{ label: '', value: 'https://x.example/a|b' }])
    expect(parseContactEntries(serializeContactEntries([{ label: 'a | b', value: 'v' }]))).toEqual([{ label: 'a / b', value: 'v' }])
  })

  it('drops unsafe links and caps the number of entries', () => {
    expect(parseContactLinks('企业微信 | https://work.example/kf\nbad | javascript:alert(1)')).toEqual([
      { label: '企业微信', url: 'https://work.example/kf' },
    ])
    const many = Array.from({ length: MAX_CONTACT_ENTRIES + 5 }, (_, i) => `g${i}`).join('\n')
    expect(parseContactEntries(many)).toHaveLength(MAX_CONTACT_ENTRIES)
  })
})

describe('ContactEntriesEditor', () => {
  const i18n = () => createI18n({ legacy: false, locale: 'zh', missingWarn: false, fallbackWarn: false, messages: { zh: {} } })

  it('edits several entries and writes them back in the stored format', async () => {
    const wrapper = mount(ContactEntriesEditor, {
      props: { modelValue: 'QQ 一群 | 123', kind: 'group', 'onUpdate:modelValue': (v: string) => wrapper.setProps({ modelValue: v }) },
      global: { plugins: [i18n()], stubs: { Icon: true } },
    })
    expect(wrapper.findAll('[data-test="contact-entry"]')).toHaveLength(1)
    await wrapper.get('[data-test="contact-entry-add"]').trigger('click')
    const rows = wrapper.findAll('[data-test="contact-entry"]')
    expect(rows).toHaveLength(2)
    const [label, value] = rows[1].findAll('input')
    await label.setValue('QQ 二群')
    await value.setValue('456')
    expect(wrapper.props('modelValue')).toBe('QQ 一群 | 123\nQQ 二群 | 456')
    await rows[0].get('button').trigger('click')
    expect(wrapper.props('modelValue')).toBe('QQ 二群 | 456')
  })

  it('flags links that are not http(s)', async () => {
    const wrapper = mount(ContactEntriesEditor, {
      props: { modelValue: 'javascript:alert(1)', kind: 'link' },
      global: { plugins: [i18n()], stubs: { Icon: true } },
    })
    expect(wrapper.find('.input-error').exists()).toBe(true)
  })
})
