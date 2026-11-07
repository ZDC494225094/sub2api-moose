import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const getPublic = vi.fn()

vi.mock('../api', () => ({
  guideAPI: { getPublic: () => getPublic() },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { site_name: '麋鹿云', site_logo: '' },
    siteName: '麋鹿云',
    siteLogo: '',
    fetchPublicSettings: vi.fn().mockResolvedValue(undefined),
  }),
  useAuthStore: () => ({ isAuthenticated: false }),
}))

import GuideView from '../GuideView.vue'

const content = ['## 一、注册账号', '正文', '### 细节', '正文', '## 二、充值', '正文'].join('\n\n')

class FakeObserver {
  static instances: FakeObserver[] = []
  observed: Element[] = []
  constructor(public callback: IntersectionObserverCallback) {
    FakeObserver.instances.push(this)
  }
  observe(element: Element) { this.observed.push(element) }
  disconnect() { this.observed = [] }
  unobserve() {}
  takeRecords() { return [] }
}

function mountView() {
  return mount(GuideView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
}

describe('docs centre view', () => {
  beforeEach(() => {
    FakeObserver.instances = []
    vi.stubGlobal('IntersectionObserver', FakeObserver)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    getPublic.mockReset()
  })

  it('shows the update date only for an edited manual', async () => {
    getPublic.mockResolvedValue({ title: '使用手册', content_md: content, updated_at: '2026-11-05T08:00:00Z', is_default: false })
    const edited = mountView()
    await flushPromises()
    expect(edited.find('.guide-eyebrow').text()).toBe('文档中心')
    expect(edited.find('.guide-meta').text()).toContain('2026')
    edited.unmount()

    getPublic.mockResolvedValue({ title: '使用手册', content_md: content, updated_at: null, is_default: true })
    const fallback = mountView()
    await flushPromises()
    expect(fallback.find('.guide-meta').exists()).toBe(false)
    fallback.unmount()
  })

  it('marks the table-of-contents entry of the heading in view', async () => {
    getPublic.mockResolvedValue({ title: '使用手册', content_md: content, updated_at: null, is_default: true })
    const wrapper = mountView()
    await flushPromises()

    const links = wrapper.findAll('.guide-toc-card a')
    expect(links).toHaveLength(3)
    expect(links[0].classes()).toContain('is-active')
    expect(links[0].attributes('aria-current')).toBe('location')

    const observer = FakeObserver.instances.at(-1)!
    expect(observer.observed).toHaveLength(3)
    observer.callback([{ target: observer.observed[2], isIntersecting: true } as unknown as IntersectionObserverEntry], observer as unknown as IntersectionObserver)
    await wrapper.vm.$nextTick()
    expect(links[2].classes()).toContain('is-active')
    expect(links[0].classes()).not.toContain('is-active')
    wrapper.unmount()
    expect(observer.observed).toHaveLength(0)
  })
})
