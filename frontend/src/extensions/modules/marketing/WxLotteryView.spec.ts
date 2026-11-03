import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import WxLotteryView from './WxLotteryView.vue'
import { marketingAPI, wechatAPI } from './api'
import { useExtensionStore } from '../../store'

const route = vi.hoisted(() => ({ query: {} as Record<string, string> }))

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('../../api', () => ({ extensionAPI: { publicState: vi.fn() } }))
vi.mock('./api', () => ({
  marketingAPI: { getWechatActiveLottery: vi.fn(), drawLottery: vi.fn() },
  wechatAPI: { getBinding: vi.fn(), bindOpenID: vi.fn(), getOAuthURL: vi.fn(), notifyDrawResult: vi.fn() },
}))

beforeEach(() => { setActivePinia(createPinia()); vi.resetAllMocks(); route.query = {} })
describe('official account lottery admission', () => {
  it('keeps existing data readable but allows draws only with the shared marketing flag', async () => {
    vi.mocked(wechatAPI.getBinding).mockResolvedValue({ data: { openid: 'bound-owner' } } as never)
    vi.mocked(marketingAPI.getWechatActiveLottery).mockResolvedValue({ data: {
      activity: { id: 7, name: 'Saved OA activity', prizes: [] },
      user_state: { available_draw_times: 2, default_granted: true },
    } } as never)
    vi.mocked(marketingAPI.drawLottery).mockResolvedValue({ data: { record: { result_code: 'lose' } } } as never)
    const wrapper = mount(WxLotteryView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('Saved OA activity')
    expect(wrapper.find('[data-testid="marketing-readonly"]').exists()).toBe(true)
    const draw = wrapper.get('button')
    expect(draw.attributes('disabled')).toBeDefined()
    await draw.trigger('click')
    expect(marketingAPI.drawLottery).not.toHaveBeenCalled()
    useExtensionStore().flags = { 'marketing-tools': true }
    await flushPromises()
    expect(draw.attributes('disabled')).toBeUndefined()
    await draw.trigger('click')
    await flushPromises()
    expect(marketingAPI.drawLottery).toHaveBeenCalledTimes(1)
    expect(marketingAPI.drawLottery).toHaveBeenCalledWith({ activity_id: 7, use_wallet: false })
    useExtensionStore().flags = {}
    await flushPromises()
    expect(draw.attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Saved OA activity')
    wrapper.unmount()
  })

  it('binds with the signed OAuth ticket and never a bare openid', async () => {
    route.query = { openid: 'oa-openid-1', wechat_ticket: 'signed-ticket' }
    window.history.replaceState({}, '', '/wx-lottery?openid=oa-openid-1&wechat_ticket=signed-ticket')
    vi.mocked(wechatAPI.getBinding).mockResolvedValue({ data: { openid: '' } } as never)
    vi.mocked(wechatAPI.bindOpenID).mockResolvedValue({ data: { openid: 'oa-openid-1' } } as never)
    vi.mocked(marketingAPI.getWechatActiveLottery).mockResolvedValue({ data: {} } as never)
    const wrapper = mount(WxLotteryView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } })
    await flushPromises()
    expect(window.location.search).not.toContain('wechat_ticket')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wechatAPI.bindOpenID).toHaveBeenCalledWith('signed-ticket')
    expect(wechatAPI.getOAuthURL).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('ignores a callback openid that arrives without a ticket', async () => {
    route.query = { openid: 'oa-openid-1' }
    vi.mocked(wechatAPI.getBinding).mockResolvedValue({ data: { openid: '' } } as never)
    vi.mocked(wechatAPI.getOAuthURL).mockResolvedValue({ data: { url: '' } } as never)
    const wrapper = mount(WxLotteryView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } })
    await flushPromises()
    expect(wechatAPI.getOAuthURL).toHaveBeenCalledTimes(1)
    expect(wechatAPI.bindOpenID).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
