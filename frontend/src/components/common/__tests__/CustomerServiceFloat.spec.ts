import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const { appStore, copyToClipboard } = vi.hoisted(() => ({
  appStore: {
    cachedPublicSettings: null as null | Record<string, unknown>,
    contactInfo: '',
  },
  copyToClipboard: vi.fn().mockResolvedValue(true),
}))

const messages: Record<string, string> = {
  'common.customerService.title': '联系客服',
  'common.customerService.description': '需要帮助？请通过以下方式联系我们',
  'common.customerService.contact': '客服联系方式',
  'common.customerService.afterSalesGroup': '售后群号',
  'common.customerService.contactNow': '立即联系',
  'common.customerService.contactUs': '联系我们',
  'common.customerService.unavailable': '联系信息暂未提供，请稍后再试。',
  'common.copy': '复制',
  'common.close': '关闭',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

import CustomerServiceFloat from '../CustomerServiceFloat.vue'

describe('CustomerServiceFloat', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = null
    appStore.contactInfo = ''
    copyToClipboard.mockClear()
  })

  it('does not render when customer service details are empty', () => {
    const wrapper = mount(CustomerServiceFloat)

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('shows configured contact details and copies the after-sales group', async () => {
    appStore.cachedPublicSettings = {
      contact_info: 'QQ: 123456',
      after_sales_group: 'QQ群 987654',
      customer_service_link: 'https://support.example.com/contact',
    }

    const wrapper = mount(CustomerServiceFloat)
    await wrapper.find('button[aria-controls="customer-service-panel"]').trigger('click')

    expect(wrapper.text()).toContain('QQ: 123456')
    expect(wrapper.text()).toContain('QQ群 987654')

    await wrapper.find('button[aria-label="复制 售后群号"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('QQ群 987654')

    const contactLink = wrapper.get('a')
    expect(contactLink.text()).toContain('立即联系')
    expect(contactLink.attributes('href')).toBe('https://support.example.com/contact')
    expect(contactLink.attributes('target')).toBe('_blank')
    expect(contactLink.attributes('rel')).toBe('noopener noreferrer')
  })

  it('does not render an unsafe one-click link', () => {
    appStore.cachedPublicSettings = {
      customer_service_link: 'javascript:alert(1)',
    }

    const wrapper = mount(CustomerServiceFloat)
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('keeps the homepage contact entry visible without loaded settings', async () => {
    const wrapper = mount(CustomerServiceFloat, { props: { alwaysVisible: true, directLink: true } })
    await wrapper.get('button[aria-controls="customer-service-panel"]').trigger('click')
    expect(wrapper.text()).toContain('联系我们')
    expect(wrapper.text()).toContain('联系信息暂未提供')
    expect(wrapper.find('a').exists()).toBe(false)
  })

  it('opens the configured contact URL directly on the homepage', () => {
    appStore.cachedPublicSettings = { customer_service_link: 'https://support.example.com/contact' }
    const wrapper = mount(CustomerServiceFloat, { props: { alwaysVisible: true, directLink: true } })
    const link = wrapper.get('a.support-direct-link')
    expect(link.text()).toContain('联系我们')
    expect(link.attributes('href')).toBe('https://support.example.com/contact')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
  })

  it('retains a safe contact panel when a configured homepage link is invalid', async () => {
    appStore.cachedPublicSettings = { customer_service_link: 'javascript:alert(1)' }
    const wrapper = mount(CustomerServiceFloat, { props: { alwaysVisible: true, directLink: true } })
    wrapper.vm.openPanel()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('section').exists()).toBe(true)
    expect(wrapper.find('a').exists()).toBe(false)
  })
})
