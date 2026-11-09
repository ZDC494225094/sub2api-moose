import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ApplyPanel from '../ApplyPanel.vue'
import ReviewDialog from '../ReviewDialog.vue'
import { invoiceAPI, invoiceAdminAPI, type InvoiceApplication, type InvoicePublicSettings, type InvoiceTitle } from '../api'
import { centsToAmount, formatMoney, sumCents } from '../format'

const showSuccess = vi.fn()
const showError = vi.fn()
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    invoiceAPI: { overview: vi.fn(), orders: vi.fn(), titles: vi.fn(), apply: vi.fn() },
    invoiceAdminAPI: { process: vi.fn(), issue: vi.fn(), reject: vi.fn(), void: vi.fn(), uploadFile: vi.fn(), download: vi.fn() },
  }
})

const stubs = {
  Icon: true,
  BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" data-dialog><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>' },
}

const settings: InvoicePublicSettings = { min_amount: '100.00', allow_special: true, order_window_days: 365, item_name: '信息技术服务费', notice: '' }

function title(overrides: Partial<InvoiceTitle> = {}): InvoiceTitle {
  return {
    id: 1, title_type: 'personal', name: '张三', tax_no: '', address: '', phone: '', bank_name: '', bank_account: '', email: '',
    is_default: true, created_at: '', updated_at: '', ...overrides,
  }
}

function application(overrides: Partial<InvoiceApplication> = {}): InvoiceApplication {
  return {
    id: 9, user_id: 7, user_email: 'alice@example.com', invoice_type: 'normal', title_type: 'company', title_name: '麋鹿云科技有限公司',
    tax_no: '91310000MA1FL1XX2A', address: '', phone: '', bank_name: '', bank_account: '', email: '', item_name: '', amount: '110.50',
    order_count: 2, remark: '', status: 'pending', invoice_no: '', reject_reason: '', admin_note: '', has_file: false,
    issued_at: null, closed_at: null, created_at: '2026-11-07T10:00:00Z', updated_at: '2026-11-07T10:00:00Z', orders: [], ...overrides,
  }
}

describe('invoice amounts', () => {
  it('sums in cents without float noise and formats CNY', () => {
    expect(centsToAmount(sumCents(['60.10', '50.40', '0.01']))).toBe('110.51')
    expect(centsToAmount(sumCents(['0.1', '0.2']))).toBe('0.30')
    expect(formatMoney('1234.5')).toBe('¥1,234.50')
  })
})

describe('invoice application panel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(invoiceAPI.orders).mockResolvedValue({
      items: [
        { id: 1, out_trade_no: 'T1', order_type: 'balance', payment_type: 'alipay', amount: '60.00', paid_at: '2026-11-06T10:00:00Z' },
        { id: 2, out_trade_no: 'T2', order_type: 'subscription', payment_type: 'wxpay', amount: '50.50', paid_at: '2026-11-05T10:00:00Z' },
      ],
      total_amount: '110.50',
      truncated: false,
    })
    vi.mocked(invoiceAPI.titles).mockResolvedValue([
      title(),
      title({ id: 2, title_type: 'company', name: '麋鹿云科技有限公司', tax_no: '91310000MA1FL1XX2A', is_default: false }),
    ])
    vi.mocked(invoiceAPI.apply).mockResolvedValue(application())
  })

  it('merges orders until the threshold is met and submits them under the default title', async () => {
    const wrapper = mount(ApplyPanel, { props: { settings }, global: { stubs } })
    await flushPromises()
    const submit = wrapper.findAll('button').find((b) => b.text().includes('提交开票申请'))!
    expect(submit.attributes('disabled')).toBeDefined()

    await wrapper.findAll('tbody tr')[0].trigger('click')
    expect(wrapper.text()).toContain('¥60.00')
    expect(wrapper.text()).toContain('还差 ¥40.00')
    expect(submit.attributes('disabled')).toBeDefined()

    await wrapper.findAll('tbody tr')[1].trigger('click')
    expect(wrapper.text()).toContain('¥110.50')
    expect(wrapper.text()).toContain('已达到开票门槛')
    expect(submit.attributes('disabled')).toBeUndefined()

    await wrapper.find('textarea').setValue(' 项目 A ')
    await submit.trigger('click')
    await flushPromises()
    expect(invoiceAPI.apply).toHaveBeenCalledWith({ order_ids: [1, 2], title_id: 1, invoice_type: 'normal', remark: '项目 A' })
    expect(wrapper.emitted('applied')).toHaveLength(1)
    expect(showSuccess).toHaveBeenCalled()
  })

  it('blocks special invoices for titles without complete company details', async () => {
    const wrapper = mount(ApplyPanel, { props: { settings: { ...settings, min_amount: '0.00' } }, global: { stubs } })
    await flushPromises()
    await wrapper.find('thead input[type="checkbox"]').trigger('change')
    const special = wrapper.findAll('[role="radio"]').find((b) => b.text().includes('专用发票'))!
    await special.trigger('click')
    expect(wrapper.text()).toContain('增值税专用发票只能开给企业抬头')

    await wrapper.findAll('input[name="invoice-title"]')[1].setValue(true)
    expect(wrapper.text()).toContain('请先补全')
    const submit = wrapper.findAll('button').find((b) => b.text().includes('提交开票申请'))!
    expect(submit.attributes('disabled')).toBeDefined()
  })

  it('offers only ordinary invoices when special invoices are switched off', async () => {
    const wrapper = mount(ApplyPanel, { props: { settings: { ...settings, allow_special: false } }, global: { stubs } })
    await flushPromises()
    expect(wrapper.findAll('[role="radio"]').some((b) => b.text().includes('专用发票'))).toBe(false)
    expect(wrapper.text()).toContain('电子普通发票')
  })
})

describe('invoice review dialog', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('requires an invoice number before issuing and shows refund alerts', async () => {
    vi.mocked(invoiceAdminAPI.issue).mockResolvedValue(application({ status: 'issued', invoice_no: 'NO-1' }))
    const wrapper = mount(ReviewDialog, { props: { app: null }, global: { stubs } })
    await wrapper.setProps({ app: application({ refund_alert: true }) })
    expect(wrapper.text()).toContain('含已退款订单')

    await wrapper.findAll('button').find((b) => b.text() === '登记开具')!.trigger('click')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.text()).toContain('请填写发票号码')
    expect(invoiceAdminAPI.issue).not.toHaveBeenCalled()

    await wrapper.find('input.font-mono').setValue(' NO-1 ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(invoiceAdminAPI.issue).toHaveBeenCalledWith(9, 'NO-1', '', null)
    expect(wrapper.emitted('changed')?.[0][0]).toMatchObject({ status: 'issued' })
  })

  it('offers void and PDF actions only for issued invoices', async () => {
    const wrapper = mount(ReviewDialog, { props: { app: application({ status: 'issued', invoice_no: 'NO-1', has_file: true }) }, global: { stubs } })
    const labels = wrapper.findAll('footer button').map((b) => b.text())
    expect(labels).toEqual(expect.arrayContaining(['下载 PDF', '替换 PDF', '作废']))
    expect(labels).not.toContain('登记开具')
    expect(labels).not.toContain('驳回')
  })
})
