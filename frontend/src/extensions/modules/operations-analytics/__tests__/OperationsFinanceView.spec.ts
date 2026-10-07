import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n, type MessageContext } from 'vue-i18n'
import type { ChartConfiguration, ScriptableLineSegmentContext } from 'chart.js'
import en from '@/i18n/locales/en/custom'
import zh from '@/i18n/locales/zh/custom'
import type { BalanceMonthlyStats } from '../api'
import OperationsFinanceView from '../OperationsFinanceView.vue'

const { getReport, getHistory, createChart, push, replace, openRecords } = vi.hoisted(() => ({
  getReport: vi.fn(), getHistory: vi.fn(), push: vi.fn(), replace: vi.fn(), openRecords: vi.fn(),
  createChart: vi.fn(function (_canvas: HTMLCanvasElement, _config: ChartConfiguration<'line'>) { return { destroy: vi.fn() } })
}))
vi.mock('@/extensions/modules/operations-analytics/api', () => ({ getOperationsFinance: getReport, getOperationsBalanceHistory: getHistory }))
vi.mock('chart.js/auto', () => ({ Chart: createChart }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: { props: ['show', 'title'], emits: ['close'], template: '<div v-if="show" role="dialog"><h2>{{ title }}</h2><button aria-label="关闭弹窗" @click="$emit(\'close\')">关闭</button><slot /></div>' } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push, replace }), useRoute: () => ({ query: {} }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<canvas />' }, Bar: { name: 'Bar', props: ['data', 'options'], template: '<canvas />' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('../components/OperationsCustomers.vue', () => ({ default: { template: '<section />', methods: { open() {} } } }))
vi.mock('../components/OperationsRecords.vue', () => ({ default: { template: '<section />', methods: { open: openRecords } } }))
const row = { total_orders: 5, paid_orders: 2, excluded_recharge: 45, key: '7', label: 'Account A', upstream: 'Vendor A', requests: 3, consumption: 10, cost: 12, list_cost: 20, recharge: 30, subscription: 0, profit: -2, margin: -20 }
const payments = [
 { day: '2026-09-01', currency: 'CNY', recharge_paid: 70, subscription_paid: 30, credited: 120, pending_credit: 50 },
 { day: '2026-09-02', currency: 'CNY', recharge_paid: 20, subscription_paid: 10, credited: 40, pending_credit: 0 },
 { day: '2026-09-01', currency: 'USD', recharge_paid: 4, subscription_paid: 0, credited: 5, pending_credit: 0 }
]
const data = { payments, inventory: { frozen_balance: 3, subscription_remaining: 75, limited_subscriptions: 2, unlimited_subscriptions: 1, gift_remaining: null }, start_date: '2026-09-01', end_date: '2026-09-14', generated_at: '2026-09-14T10:00:00Z', current_balance: 50, summary: row, rows: [{ ...row, dimension: 'day', key: '2026-09-01' }, { ...row, dimension: 'upstream', key: 'Vendor A', label: 'Vendor A' }, { ...row, dimension: 'account' }] }
const history: BalanceMonthlyStats[] = [
  { period: '2026-07', total_balance: 30, total_subscription_remaining: 60 },
  { period: '2026-08', total_balance: 40, total_subscription_remaining: 70 },
  { period: '2026-09', total_balance: 50, total_subscription_remaining: 75, realtime: true }
]
const balanceMessages = (source: typeof en) => ({ admin: { operations: {
  balanceTrend: Object.fromEntries(Object.entries(source.admin.operations.balanceTrend).map(([key, message]) => [
    key, (context: MessageContext) => message.replace('{months}', String(context.named('months')))
  ]))
} } })
const create = (locale = 'zh') => mount(OperationsFinanceView, { global: {
  plugins: [createI18n({ legacy: false, locale, messages: { en: balanceMessages(en), zh: balanceMessages(zh) } })],
  stubs: { RouterLink: { template: '<a><slot /></a>' } }
} })
describe('Operations finance', () => {
  beforeEach(() => { vi.clearAllMocks(); Element.prototype.scrollIntoView = vi.fn(); getReport.mockResolvedValue(structuredClone(data)); getHistory.mockReset().mockResolvedValue(structuredClone(history)) })
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
    await wrapper.findAll('button').find(b => b.text() === '上游汇总')!.trigger('click')
    await wrapper.find('tbody button').trigger('click')
    expect(wrapper.text()).toContain('上游：Vendor A')
    await wrapper.find('tbody button').trigger('click')
    await flushPromises()
    expect(openRecords).toHaveBeenCalledWith({ mode: 'usage', title: 'Account A · 请求消耗明细', account_id: 7 })
    expect(push).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('opens successful recharge orders in place', async () => {
    const wrapper = create(); await flushPromises()
    const rechargeMetric = wrapper.findAll('button').find(button => button.text().includes('期间实际付款'))
    expect(rechargeMetric).toBeDefined()
    await rechargeMetric!.trigger('click')
    await flushPromises()
    expect(openRecords).toHaveBeenCalledWith({ mode: 'orders', title: '期间成功付款订单' })
    expect(push).not.toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').text()).toContain('充值付款')
    expect(wrapper.text()).toContain('¥130.00 元')
    expect(wrapper.text()).toContain('USD 4.00')
    expect(wrapper.text()).toContain('$165.00')
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled()
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
    const recharge = wrapper.findAllComponents({ name: 'Bar' }).find(b => b.props('data').datasets[0].label === '充值付款')!
    expect(recharge.props('data').datasets.map((d: { data: number[] }) => d.data)).toEqual([[90], [40]])
    recharge.props('options').onClick({}, [{ index: 0 }]); await flushPromises()
    await flushPromises()
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
  it('opens balance inventory in a modal and never invents a remaining gift balance', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text().includes('当前用户余额'))!.trigger('click')
    const dialog = wrapper.get('[role="dialog"]')
    expect(dialog.text()).toContain('$75.00')
    expect(dialog.text()).toContain('1 份无限额订阅')
    expect(dialog.text()).toContain('未单独核算')
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
    await dialog.get('[aria-label="关闭弹窗"]').trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('loads balance history only on demand and styles live points and segments distinctly', async () => {
    const wrapper = create(); await flushPromises()
    expect(getHistory).not.toHaveBeenCalled()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenCalledWith({ months: 3 }, expect.any(AbortSignal))
    const config = createChart.mock.lastCall![1]
    expect(config.type).toBe('line')
    expect(config.options).toMatchObject({ responsive: true, maintainAspectRatio: false })
    expect(config.data.labels).toEqual(history.map(point => point.period))
    expect(config.data.datasets.map(series => series.label)).toEqual(['余额剩余总额', '订阅余额剩余总额'])
    expect(config.data.datasets.map(series => series.borderColor)).toEqual(['#6366f1', '#10b981'])
    expect(config.data.datasets.map(series => series.data)).toEqual([[30, 40, 50], [60, 70, 75]])
    for (const series of config.data.datasets) {
      expect(series.pointStyle).toEqual(['circle', 'circle', 'rectRot'])
      expect(series.pointBackgroundColor).toEqual([series.borderColor, series.borderColor, '#f59e0b'])
      const borderDash = series.segment!.borderDash
      const borderColor = series.segment!.borderColor
      expect(typeof borderDash).toBe('function')
      expect(typeof borderColor).toBe('function')
      const historicalSegment = { p0DataIndex: 0, p1DataIndex: 1 } as ScriptableLineSegmentContext
      const liveSegment = { p0DataIndex: 1, p1DataIndex: 2 } as ScriptableLineSegmentContext
      if (typeof borderDash === 'function' && typeof borderColor === 'function') {
        expect(borderDash(historicalSegment, {})).toEqual([])
        expect(borderDash(liveSegment, {})).toEqual([6, 4])
        expect(borderColor(historicalSegment, {})).toBe(series.borderColor)
        expect(borderColor(liveSegment, {})).toBe('#f59e0b')
      }
    }
    expect(wrapper.get('canvas[role="img"]').element.parentElement!.classList.contains('h-72')).toBe(true)
    expect(wrapper.get('[role="dialog"]').text()).toContain('本月实时数据，非月末快照')
    wrapper.unmount()
    expect(createChart.mock.results[0].value.destroy).toHaveBeenCalledOnce()
  })
  it('switches all supported month ranges and destroys replaced and closed charts', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    for (const months of [6, 12]) {
      const previousChart = createChart.mock.results[createChart.mock.results.length - 1].value
      const button = wrapper.findAll('button').find(item => item.text() === `${months} 个月`)!
      await button.trigger('click'); await flushPromises()
      expect(getHistory.mock.lastCall![0]).toEqual({ months })
      expect(button.attributes('aria-pressed')).toBe('true')
      expect(previousChart.destroy).toHaveBeenCalledOnce()
    }
    const lastChart = createChart.mock.results[createChart.mock.results.length - 1].value
    await wrapper.get('[aria-label="关闭弹窗"]').trigger('click')
    expect(lastChart.destroy).toHaveBeenCalledOnce()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    expect(getHistory.mock.lastCall![0]).toEqual({ months: 12 })
    wrapper.unmount()
    expect(createChart.mock.results[createChart.mock.results.length - 1].value.destroy).toHaveBeenCalledOnce()
  })
  it('shows loading and errors, then retries without changing the selected range', async () => {
    let rejectHistory!: (error: Error) => void
    getHistory.mockImplementationOnce(() => new Promise<BalanceMonthlyStats[]>((_resolve, reject) => { rejectHistory = reject }))
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    expect(wrapper.get('[role="dialog"] [role="status"]').text()).toContain('正在加载余额趋势')
    expect(createChart).not.toHaveBeenCalled()
    rejectHistory(new Error('unavailable')); await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toContain('余额趋势加载失败')
    expect(wrapper.text()).toContain('$75.00')
    await wrapper.findAll('button').find(button => button.text() === '重试')!.trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenCalledTimes(2)
    expect(getHistory.mock.lastCall![0]).toEqual({ months: 3 })
    expect(wrapper.find('[role="dialog"] [role="alert"]').exists()).toBe(false)
    expect(createChart).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
  it('renders an empty state rather than a chart with invented zeroes', async () => {
    getHistory.mockResolvedValue([])
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="status"]').text()).toContain('暂无余额历史数据')
    expect(createChart).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('本月实时数据，非月末快照')
    wrapper.unmount()
  })
  it('renders real zero balances and does not mark historical snapshots as live data', async () => {
    getHistory.mockResolvedValue([{ period: '2026-08', total_balance: 0, total_subscription_remaining: 0 }])
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    expect(createChart.mock.lastCall![1].data.datasets.map(series => series.data)).toEqual([[0], [0]])
    expect(createChart.mock.lastCall![1].data.datasets[0].pointStyle).toEqual(['circle'])
    expect(wrapper.text()).not.toContain('本月实时数据，非月末快照')
    wrapper.unmount()
  })
  it('ignores stale responses after range changes and aborts on close and unmount', async () => {
    const pending: { signal: AbortSignal; resolve: (points: BalanceMonthlyStats[]) => void }[] = []
    getHistory.mockImplementation((_params, signal: AbortSignal) => new Promise<BalanceMonthlyStats[]>(resolve => { pending.push({ signal, resolve }) }))
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '6 个月')!.trigger('click')
    expect(pending[0].signal.aborted).toBe(true)
    pending[1].resolve(history); await flushPromises()
    pending[0].resolve([]); await flushPromises()
    expect(createChart).toHaveBeenCalledOnce()
    expect(wrapper.find('canvas[role="img"]').exists()).toBe(true)
    await wrapper.findAll('button').find(button => button.text() === '12 个月')!.trigger('click')
    await wrapper.get('[aria-label="关闭弹窗"]').trigger('click')
    expect(pending[2].signal.aborted).toBe(true)
    pending[2].resolve(history); await flushPromises()
    expect(createChart).toHaveBeenCalledOnce()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    wrapper.unmount()
    expect(pending[3].signal.aborted).toBe(true)
    pending[3].resolve(history); await flushPromises()
    expect(createChart).toHaveBeenCalledOnce()
  })
  it('destroys the balance chart when drilling into customers and recreates it on return', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    const chart = createChart.mock.results[0].value
    await wrapper.findAll('button').find(button => button.text() === '查看有余额用户明细')!.trigger('click')
    expect(chart.destroy).toHaveBeenCalledOnce()
    await wrapper.findAll('button').find(button => button.text() === '返回上一级')!.trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenCalledTimes(2)
    expect(createChart).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('uses English messages for the chart and updates them when the locale changes', async () => {
    const wrapper = create('en'); await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('当前用户余额'))!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"]').text()).toContain('Monthly Balance Trend')
    expect(wrapper.get('[role="dialog"]').text()).toContain('12 months')
    expect(createChart.mock.lastCall![1].data.datasets[0].label).toBe('Total Remaining Balance')
    expect(wrapper.text()).toContain('not a month-end snapshot')
    const chart = createChart.mock.results[0].value
    wrapper.vm.$i18n.locale = 'zh'
    await flushPromises()
    expect(chart.destroy).toHaveBeenCalledOnce()
    expect(createChart.mock.lastCall![1].data.datasets[0].label).toBe('余额剩余总额')
    expect(getHistory).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
  it('keeps dimension filters when returning from a request drilldown', async () => {
    const wrapper = create(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '上游汇总')!.trigger('click')
    await wrapper.find('tbody button').trigger('click')
    await wrapper.find('tbody button').trigger('click'); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '返回上一级')!.trigger('click')
    expect(wrapper.get('[role="dialog"]').text()).toContain('上游：Vendor A')
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled()
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
