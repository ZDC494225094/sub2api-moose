import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref, nextTick } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import LotteryView from './LotteryView.vue'
import { marketingAPI } from './api'
const state = vi.hoisted(() => ({ allowed: null as unknown }))
vi.mock('./admission', () => ({ useMarketingAdmission: () => state.allowed }))
vi.mock('./api', () => ({ marketingAPI: { getActiveLottery: vi.fn(), getMyDrawRecords: vi.fn(), drawLottery: vi.fn() } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh-CN' } }) }))
let wrapper: VueWrapper | undefined
let allowed = ref(true)
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  allowed = ref(true)
  state.allowed = allowed
  vi.mocked(marketingAPI.getMyDrawRecords).mockResolvedValue({ data: { items: [] } } as never)
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.clearAllTimers(); vi.useRealTimers() })
async function render(chances: number) {
  vi.mocked(marketingAPI.getActiveLottery).mockResolvedValue({ data: {
    activity: { id: 11, name: 'Historical activity', wallet_cost_per_draw: 5, prizes: [] },
    user_state: { available_draw_times: chances, default_granted: true },
    draw_eligibility: { consume_threshold_met: true, wallet_draw_enabled: true },
    recent_winners: [],
  } } as never)
  wrapper = mount(LotteryView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Transition: true } } })
  await flushPromises()
  return wrapper
}
function handlers() {
  return (wrapper!.vm.$ as unknown as { setupState: { handleDrawClick(): void; confirmDraw(): void; executeDraw(wallet: boolean): Promise<void> } }).setupState
}
describe('lottery read-only admission', () => {
  it('keeps all historical draws reachable when no active activity exists', async () => {
    allowed.value = false
    vi.mocked(marketingAPI.getActiveLottery).mockResolvedValue({ data: {} } as never)
    wrapper = mount(LotteryView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('userLottery.noActiveActivity')
    const history = wrapper.findAll('button').find(b => b.text().includes('userLottery.historyBtn'))!
    await history.trigger('click'); await flushPromises()
    expect(marketingAPI.getMyDrawRecords).toHaveBeenCalledWith({ activity_id: undefined, page_size: 50 })
    expect(marketingAPI.drawLottery).not.toHaveBeenCalled()
  })
  it('keeps the overview and history readable while all draw entry points reject', async () => {
    allowed.value = false
    const view = await render(3)
    expect(view.get('[data-testid="marketing-readonly"]').text()).toContain('历史次数')
    expect(view.text()).toContain('Historical activity')
    handlers().handleDrawClick()
    handlers().confirmDraw()
    await handlers().executeDraw(false)
    expect(marketingAPI.drawLottery).not.toHaveBeenCalled()
    const history = view.findAll('button').find(b => b.text().includes('userLottery.historyBtn'))!
    await history.trigger('click')
    await flushPromises()
    expect(marketingAPI.getMyDrawRecords).toHaveBeenCalledWith({ activity_id: 11, page_size: 50 })
    expect(marketingAPI.getActiveLottery).toHaveBeenCalledTimes(1)
  })
  it('closes a pending wallet confirmation and rejects a stale confirm callback', async () => {
    const view = await render(0)
    handlers().handleDrawClick()
    await nextTick()
    expect(view.text()).toContain('userLottery.confirmWalletDraw')
    allowed.value = false
    handlers().confirmDraw()
    await nextTick()
    expect(view.text()).not.toContain('userLottery.confirmWalletDraw')
    expect(marketingAPI.drawLottery).not.toHaveBeenCalled()
    allowed.value = true
    await nextTick()
    expect(view.find('[data-testid="marketing-readonly"]').exists()).toBe(false)
    expect(view.text()).not.toContain('userLottery.confirmWalletDraw')
    expect(marketingAPI.drawLottery).not.toHaveBeenCalled()
  })
  it('still dispatches an enabled draw without duplicating it when admission changes', async () => {
    vi.mocked(marketingAPI.drawLottery).mockReturnValue(new Promise(() => {}))
    const view = await render(3)
    handlers().handleDrawClick()
    expect(marketingAPI.drawLottery).toHaveBeenCalledWith({ activity_id: 11, use_wallet: false })
    allowed.value = false
    await nextTick()
    expect(view.text()).toContain('Historical activity')
    handlers().handleDrawClick()
    expect(marketingAPI.drawLottery).toHaveBeenCalledTimes(1)
  })
})

