import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BalanceHistoryModal from '../BalanceHistoryModal.vue'

const { getHistory, createChart, destroyChart } = vi.hoisted(() => ({
  getHistory: vi.fn(),
  createChart: vi.fn(),
  destroyChart: vi.fn()
}))

vi.mock('@/api/user', () => ({ getBalanceHistory: getHistory }))
vi.mock('chart.js/auto', () => ({
  Chart: class {
    destroy = destroyChart
    constructor(...args: unknown[]) { createChart(...args) }
  }
}))
vi.mock('vue-i18n', async () => {
  const { ref } = await import('vue')
  return { useI18n: () => ({ t: (key: string) => key, locale: ref('en') }) }
})

enableAutoUnmount(afterEach)
beforeEach(() => {
  getHistory.mockReset()
  createChart.mockClear()
  destroyChart.mockClear()
})

function open() {
  return mount(BalanceHistoryModal, {
    props: { show: true, currentBalance: 12.5 },
    global: {
      stubs: {
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        LoadingSpinner: { template: '<div data-testid="loading" />' }
      }
    }
  })
}

function deferred() {
  let resolve!: (points: { period: string; balance: number; realtime?: boolean }[]) => void
  let reject!: (error: Error) => void
  const promise = new Promise<{ period: string; balance: number; realtime?: boolean }[]>((onResolve, onReject) => {
    resolve = onResolve
    reject = onReject
  })
  return { promise, resolve, reject }
}

describe('BalanceHistoryModal', () => {
  it('loads twelve months by default and displays loading and current balance', async () => {
    const pending = deferred()
    getHistory.mockReturnValueOnce(pending.promise)
    const wrapper = open()
    expect(getHistory).toHaveBeenCalledWith(12)
    expect(wrapper.text()).toContain('$12.50')
    expect(wrapper.find('[data-testid="loading"]').exists()).toBe(true)
    pending.resolve([])
    await flushPromises()
    expect(wrapper.text()).toContain('balanceHistory.empty')
    expect(createChart).not.toHaveBeenCalled()
  })

  it('marks the live point and connecting segment without filling missing months', async () => {
    getHistory.mockResolvedValueOnce([
      { period: '2026-08', balance: 20 },
      { period: '2026-10', balance: 12.5, realtime: true }
    ])
    const wrapper = open()
    await flushPromises()
    expect(wrapper.text()).toContain('balanceHistory.realtimeNote')
    expect(wrapper.text()).not.toContain('balanceHistory.noHistory')
    const config = createChart.mock.calls.at(-1)![1]
    expect(config.data.labels).toEqual(['2026-08', '2026-10'])
    expect(config.data.datasets[0].pointStyle).toEqual(['circle', 'rectRot'])
    expect(config.data.datasets[0].segment.borderDash({ p0DataIndex: 0, p1DataIndex: 1 })).toEqual([6, 4])
  })

  it('shows the no-history state with a live point', async () => {
    getHistory.mockResolvedValueOnce([{ period: '2026-10', balance: 0, realtime: true }])
    const wrapper = open()
    await flushPromises()
    expect(wrapper.text()).toContain('balanceHistory.noHistory')
    expect(wrapper.find('canvas').exists()).toBe(true)
  })

  it('supports three and six month ranges', async () => {
    getHistory.mockResolvedValue([])
    const wrapper = open()
    await flushPromises()
    await wrapper.findAll('[aria-pressed]')[0].trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(3)
    await wrapper.findAll('[aria-pressed]')[1].trigger('click')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(6)
    expect(wrapper.findAll('[aria-pressed]')[1].attributes('aria-pressed')).toBe('true')
  })

  it('ignores responses from an older range request', async () => {
    const old = deferred()
    const current = deferred()
    getHistory.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = open()
    await wrapper.findAll('[aria-pressed]')[0].trigger('click')
    old.resolve([{ period: '2025-11', balance: 99 }])
    await flushPromises()
    expect(wrapper.find('[data-testid="loading"]').exists()).toBe(true)
    current.resolve([{ period: '2026-10', balance: 12.5, realtime: true }])
    await flushPromises()
    expect(createChart.mock.calls.at(-1)![1].data.labels).toEqual(['2026-10'])
  })

  it('reports errors and allows retry', async () => {
    getHistory.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce([])
    const wrapper = open()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('balanceHistory.loadFailed')
    await wrapper.find('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('balanceHistory.empty')
  })

  it('destroys the chart on close and reloads on reopening', async () => {
    getHistory.mockResolvedValue([{ period: '2026-10', balance: 12.5, realtime: true }])
    const wrapper = open()
    await flushPromises()
    const destroysBeforeClosing = destroyChart.mock.calls.length
    await wrapper.setProps({ show: false })
    expect(destroyChart.mock.calls.length).toBeGreaterThan(destroysBeforeClosing)
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(getHistory).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    expect(destroyChart.mock.calls.length).toBeGreaterThan(destroysBeforeClosing + 1)
  })

  it('ignores a rejected request after closing', async () => {
    const pending = deferred()
    getHistory.mockReturnValueOnce(pending.promise)
    const wrapper = open()
    await wrapper.setProps({ show: false })
    pending.reject(new Error('obsolete'))
    await flushPromises()
    expect(createChart).not.toHaveBeenCalled()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
})
