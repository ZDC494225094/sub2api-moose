import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OperationsFinanceView from '../OperationsFinanceView.vue'

const { getReport, push, replace, openRecords } = vi.hoisted(() => ({ getReport: vi.fn(), push: vi.fn(), replace: vi.fn(), openRecords: vi.fn() }))
vi.mock('@/api/admin/operationsFinance', () => ({ getOperationsFinance: getReport }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push, replace }), useRoute: () => ({ query: {} }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<canvas />' }, Bar: { name: 'Bar', props: ['data', 'options'], template: '<canvas />' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('../components/OperationsCustomers.vue', () => ({ default: { template: '<section />', methods: { open() {} } } }))
vi.mock('../components/OperationsRecords.vue', () => ({ default: { template: '<section />', methods: { open: openRecords } } }))
const row = { total_orders: 5, paid_orders: 2, excluded_recharge: 45, key: '7', label: 'Account A', upstream: 'Vendor A', requests: 3, consumption: 10, cost: 12, list_cost: 20, recharge: 30, subscription: 0, profit: -2, margin: -20 }
const data = { start_date: '2026-09-01', end_date: '2026-09-14', generated_at: '2026-09-14T10:00:00Z', current_balance: 50, summary: row, rows: [{ ...row, dimension: 'day', key: '2026-09-01' }, { ...row, dimension: 'upstream', key: 'Vendor A', label: 'Vendor A' }, { ...row, dimension: 'account' }] }
const create = () => mount(OperationsFinanceView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
describe('Operations finance', () => {
  beforeEach(() => { vi.clearAllMocks(); Element.prototype.scrollIntoView = vi.fn(); getReport.mockResolvedValue(structuredClone(data)) })
  it('uses the server date despite a changed local clock', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2040-01-01T00:00:00Z'))
    try {
      const wrapper = create(); await flushPromises()
      expect(getReport.mock.calls[0][0]).toEqual({ preset: 'today' })
      expect(wrapper.findAll('input[type="date"]')[0].element.value).toBe(data.start_date)
      await wrapper.findAll('button').find(button => button.text() === '昨天')!.trigger('click')
      await flushPromises()
      expect(getReport.mock.lastCall?.[0]).toEqual({ preset: 'yesterday' })
      wrapper.unmount()
    } finally { vi.useRealTimers() }
  })
  it('drills from upstream to account to date-filtered requests', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    await wrapper.find('tbody button').trigger('click')
    expect(wrapper.text()).toContain('上游：Vendor A')
    await wrapper.find('tbody button').trigger('click')
    expect(openRecords).toHaveBeenCalledWith({ mode: 'usage', title: 'Account A · 请求消耗明细', account_id: 7 })
    expect(push).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('opens successful recharge orders in place', async () => {
    const wrapper = create(); await flushPromises()
    const rechargeMetric = wrapper.findAll('button').find(button => button.text().includes('期间充值额度'))
    expect(rechargeMetric).toBeDefined()
    await rechargeMetric!.trigger('click')
    expect(openRecords).toHaveBeenCalledWith({ mode: 'orders', title: '期间成功充值订单' })
    expect(push).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('仅成功支付状态')
    expect(wrapper.text()).toContain('40.00%')
    wrapper.unmount()
  })
  it('aggregates monthly amounts and computes weighted margin instead of averaging rates', async () => {
    getReport.mockResolvedValue({ ...structuredClone(data), rows: [
      { ...row, dimension: 'day', key: '2026-09-01', consumption: 10, cost: 5, profit: 5, margin: 50, recharge: 30, subscription: 10 },
      { ...row, dimension: 'day', key: '2026-09-02', consumption: 90, cost: 85, profit: 5, margin: 5.5555, recharge: 70, subscription: 20 }
    ] })
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '月')!.trigger('click')
    const lines = wrapper.findAllComponents({ name: 'Line' })
    expect(lines[0].props('data').datasets[0].data).toEqual([100])
    expect(lines[1].props('data').datasets[0].data).toEqual([10])
    const recharge = wrapper.findAllComponents({ name: 'Bar' }).find(b => b.props('data').datasets[0].label === '余额充值')!
    expect(recharge.props('data').datasets.map((d: { data: number[] }) => d.data)).toEqual([[70], [30]])
    recharge.props('options').onClick({}, [{ index: 0 }])
    expect(openRecords).toHaveBeenCalledWith(expect.objectContaining({ mode: 'orders', start_date: '2026-09-01', end_date: '2026-09-02' }))
    expect(push).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('resets loss filters when changing dimensions', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('亏损账号'))!.trigger('click')
    expect((wrapper.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.findAll('[role="tab"]')[0].trigger('click')
    expect((wrapper.find('input[type="checkbox"]').element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.text()).toContain('加权毛利率')
    wrapper.unmount()
  })
  it('rejects ranges beyond 90 days and clears stale data', async () => {
    const wrapper = create(); await flushPromises()
    const dates = wrapper.findAll('input[type="date"]')
    await dates[0].setValue('2025-01-01'); await dates[1].setValue('2026-09-14')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(getReport).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[role="alert"]').text()).toContain('90')
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })
  it('shows failures instead of fabricated zero metrics', async () => {
    getReport.mockRejectedValue(new Error('unavailable'))
    const wrapper = create(); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.find('table').exists()).toBe(false)
    wrapper.unmount()
  })
})
