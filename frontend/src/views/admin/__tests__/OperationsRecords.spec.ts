import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OperationsRecords from '../components/OperationsRecords.vue'
const { getOrders, listUsage } = vi.hoisted(() => ({ getOrders: vi.fn(), listUsage: vi.fn() }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getOrders } }))
vi.mock('@/api/admin/usage', () => ({ list: listUsage }))
const create = () => mount(OperationsRecords, { props: { startDate: '2026-09-01', endDate: '2026-09-24', timezone: 'Asia/Shanghai' } })
describe('Operations in-page records', () => {
  beforeEach(() => {
    vi.clearAllMocks(); Element.prototype.scrollIntoView = vi.fn()
    getOrders.mockResolvedValue({ data: { items: [], total: 21 } })
    listUsage.mockResolvedValue({ items: [], total: 0 })
  })
  it('loads only finance-eligible orders and preserves scope across pagination', async () => {
    const wrapper = create()
    await wrapper.vm.open({ mode: 'orders', title: '充值订单' }); await flushPromises()
    expect(getOrders.mock.lastCall?.[0]).toEqual({ finance_only: true, date_field: 'created_at', start_date: '2026-09-01', end_date: '2026-09-24', page: 1, page_size: 20 })
    await wrapper.get('[aria-label="下一页明细"]').trigger('click'); await flushPromises()
    expect(getOrders.mock.lastCall?.[0]).toMatchObject({ finance_only: true, page: 2 })
    await wrapper.vm.open({ mode: 'orders', title: '当日订单', start_date: '2026-09-10', end_date: '2026-09-10' }); await flushPromises()
    expect(getOrders.mock.lastCall?.[0]).toMatchObject({ start_date: '2026-09-10', end_date: '2026-09-10', page: 1 })
    wrapper.unmount()
  })
  it('preserves account, model, user, timezone and dates, closes when report changes', async () => {
    const wrapper = create()
    await wrapper.vm.open({ mode: 'usage', title: '账号请求', account_id: 9, model: 'test-model', user_id: 4 }); await flushPromises()
    expect(listUsage.mock.lastCall?.[0]).toMatchObject({ account_id: 9, model: 'test-model', user_id: 4, timezone: 'Asia/Shanghai', start_date: '2026-09-01', end_date: '2026-09-24', exact_total: true })
    expect(wrapper.text()).toContain('暂无记录')
    await wrapper.setProps({ endDate: '2026-09-25' })
    expect(wrapper.find('section').exists()).toBe(false)
    wrapper.unmount()
  })
  it('ignores superseded responses and closes without reopening', async () => {
    let resolveOld!: (value: unknown) => void
    getOrders.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = create()
    await wrapper.vm.open({ mode: 'orders', title: '旧请求' })
    const signal = getOrders.mock.lastCall?.[1] as AbortSignal
    await wrapper.vm.open({ mode: 'usage', title: '新请求', account_id: 2 }); await flushPromises()
    expect(signal.aborted).toBe(true)
    resolveOld({ data: { items: [], total: 999 } }); await flushPromises()
    expect(wrapper.text()).not.toContain('999')
    await wrapper.get('[aria-label="关闭页内明细"]').trigger('click')
    expect(wrapper.find('section').exists()).toBe(false)
    wrapper.unmount()
  })
  it('does not show stale rows on errors and supports retry', async () => {
    getOrders.mockRejectedValueOnce(new Error('offline'))
    const wrapper = create()
    await wrapper.vm.open({ mode: 'orders', title: '订单' }); await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('加载失败')
    expect(wrapper.find('table').exists()).toBe(false)
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('uses the finance cost null fallback without treating zero as missing', async () => {
    listUsage.mockResolvedValue({ total: 2, items: [
      { id: 1, user_id: 1, model: 'one', actual_cost: 10, total_cost: 12, account_stats_cost: 0, account_rate_multiplier: 4, created_at: '2026-09-01T01:00:00Z' },
      { id: 2, user_id: 1, model: 'two', actual_cost: 2, total_cost: 8, account_stats_cost: null, account_rate_multiplier: 2, created_at: '2026-09-01T02:00:00Z' }
    ] })
    const wrapper = create()
    await wrapper.vm.open({ mode: 'usage', title: '请求' }); await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('$0.00')
    expect(rows[1].text()).toContain('$16.00')
    expect(rows[1].text()).toContain('$-14.00')
    wrapper.unmount()
  })
})
