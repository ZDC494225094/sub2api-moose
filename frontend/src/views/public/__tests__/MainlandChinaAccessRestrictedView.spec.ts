import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import MainlandChinaAccessRestrictedView from '../MainlandChinaAccessRestrictedView.vue'

describe('MainlandChinaAccessRestrictedView', () => {
  it('renders the fixed HTTP 451 legal notice and official rule link', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useAppStore().siteLogo = '/brand-logo.svg'
    const wrapper = mount(MainlandChinaAccessRestrictedView, {
      global: { plugins: [pinia] },
    })

    expect(wrapper.get('h1').text()).toBe('依法限制访问')
    expect(wrapper.text()).toContain('本站点不向中国大陆地区提供服务')
    expect(wrapper.text()).toContain(
      'In accordance with the Interim Measures for the Administration of Generative Artificial Intelligence Services and related laws and regulations, this service is not available to users in mainland China.',
    )
    expect(wrapper.get('#english-notice-title').text()).toBe('Error 451 · Unavailable For Legal Reasons')
    expect(wrapper.text()).toContain('Access Restricted')
    expect(wrapper.get('.access-restricted-logo img').attributes('src')).toBe('/brand-logo.svg')

    const lawLink = wrapper.get('a')
    expect(lawLink.attributes('href')).toBe(
      'https://www.gov.cn/zhengce/zhengceku/202307/content_6891752.htm',
    )
    expect(lawLink.attributes('target')).toBe('_blank')
    expect(lawLink.attributes('rel')).toBe('noopener noreferrer')
  })
})
