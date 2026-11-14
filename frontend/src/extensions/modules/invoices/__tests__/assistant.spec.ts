import { Script, createContext } from 'node:vm'
import { JSDOM } from 'jsdom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { goMakeout } from '../makeout'
import { invoiceAdminAPI, type InvoiceApplication } from '../api'
import backgroundSource from '../tax-assistant/background.js?raw'
import taxFormSource from '../tax-assistant/tax-form.js?raw'

vi.mock('../api', () => ({ invoiceAdminAPI: { prepareMakeout: vi.fn() } }))
const payload = {
  id: 9, invoice_type: 'normal', title_type: 'company', title_name: '测试企业', tax_no: '91310000MA1FL1XX2A',
  address: '南京市测试路', phone: '025-12345678', bank_name: '测试银行', bank_account: '622200001111',
  email: 'fin@example.com', item_name: '技术服务费', amount: '110.50', remark: '测试',
}

beforeEach(() => vi.clearAllMocks())
afterEach(() => vi.useRealTimers())

describe('makeout handoff', () => {
  it('checks installation before reserving an application', async () => {
    vi.useFakeTimers()
    const result = goMakeout({ id: 9 } as InvoiceApplication)
    const assertion = expect(result).rejects.toThrow('未检测到开票填表助手')
    await vi.advanceTimersByTimeAsync(1200)
    await assertion
    expect(invoiceAdminAPI.prepareMakeout).not.toHaveBeenCalled()
  })

  it('reserves server state and sends only invoice fields to the assistant', async () => {
    const messages: Array<Record<string, any>> = []
    const spy = vi.spyOn(window, 'postMessage').mockImplementation((message) => {
      messages.push(message)
      queueMicrotask(() => window.dispatchEvent(new MessageEvent('message', {
        source: window, origin: location.origin,
        data: { channel: message.channel, type: 'response', requestId: message.requestId },
      })))
    })
    vi.mocked(invoiceAdminAPI.prepareMakeout).mockResolvedValue({ ...payload, status: 'processing', admin_note: 'private', user_email: 'private@example.com' } as unknown as InvoiceApplication)
    try {
      const app = await goMakeout({ id: 9 } as InvoiceApplication)
      expect(app.status).toBe('processing')
      expect(messages.map((message) => message.action)).toEqual(['ping', 'open'])
      expect(invoiceAdminAPI.prepareMakeout).toHaveBeenCalledWith(9)
      expect(messages[1].payload).toEqual(payload)
      expect(messages[1].payload).not.toHaveProperty('admin_note')
      expect(messages[1].payload).not.toHaveProperty('user_email')
    } finally { spy.mockRestore() }
  })
})

describe('browser assistant', () => {
  it('isolates snapshots by tax tab, expires data and rejects other origins', async () => {
    const session: Record<string, any> = {}
    let listener: (message: unknown, sender: unknown, reply: (value: any) => void) => void
    let nextId = 20
    const chrome = {
      storage: { session: {
        get: async (key: string) => ({ [key]: session[key] }),
        set: async (value: object) => Object.assign(session, value),
        remove: async (key: string) => { delete session[key] },
      } },
      tabs: {
        create: vi.fn(async () => ({ id: nextId++ })), update: vi.fn(async () => {}), remove: vi.fn(async () => {}),
        onRemoved: { addListener: vi.fn() },
      },
      runtime: { onMessage: { addListener: (value: typeof listener) => { listener = value } } },
    }
    new Script(backgroundSource).runInContext(createContext({ chrome, URL, Date }))
    const send = (message: unknown, url: string, id = 1) => new Promise<any>((resolve) => listener(message, { url, frameId: 0, tab: { id } }, resolve))
    const adminURL = 'http://localhost:6200/admin/invoices'
    const taxURL = 'https://dppt.jiangsu.chinatax.gov.cn:8443/blue-invoice-makeout'
    expect(await send({ type: 'open', payload }, adminURL)).toEqual({ opened: true })
    expect(await send({ type: 'open', payload: { ...payload, id: 10, title_name: '另一个客户' } }, adminURL)).toEqual({ opened: true })
    expect(chrome.tabs.update).toHaveBeenCalledWith(20, { url: taxURL })
    expect((await send({ type: 'get' }, taxURL, 20)).payload.title_name).toBe('测试企业')
    expect((await send({ type: 'get' }, taxURL, 21)).payload.title_name).toBe('另一个客户')
    expect((await send({ type: 'get' }, 'https://example.com/blue-invoice-makeout', 20)).error).toBeDefined()
    expect((await send({ type: 'open', payload }, 'https://example.com/other')).error).toBeDefined()
    session['invoice:20'].created = Date.now() - 31 * 60 * 1000
    expect(await send({ type: 'get' }, taxURL, 20)).toEqual({})
    expect(session).not.toHaveProperty('invoice:20')
    await send({ type: 'clear' }, taxURL, 21)
    expect(session).not.toHaveProperty('invoice:21')
  })

  it('fills buyer fields, preserves seller and ambiguous amounts, and respects operator edits', async () => {
    const dom = new JSDOM(`<body>
      <input id="gfmc"><input id="gfsbh"><input id="hsje">
      <input id="seller" aria-label="销售方名称" value="销售方公司">
      <input id="amount" aria-label="金额"><input id="existing" name="gfphone" value="existing">
    </body>`, { url: 'https://dppt.jiangsu.chinatax.gov.cn:8443/blue-invoice-makeout', runScripts: 'outside-only', pretendToBeVisual: true })
    const win = dom.window
    const chrome = {
      runtime: { sendMessage: vi.fn(async (message: { type: string }) => message.type === 'get' ? { payload, expiresIn: 60000 } : {}) },
      storage: { local: { get: vi.fn(async () => ({})), set: vi.fn(async () => {}), remove: vi.fn(async () => {}) } },
    }
    Object.assign(win, { chrome })
    win.HTMLElement.prototype.getClientRects = () => [{ width: 100, height: 20 }] as unknown as DOMRectList
    const input = (id: string) => win.document.getElementById(id) as HTMLInputElement
    const event = vi.fn()
    input('gfmc').addEventListener('input', event)
    try {
      new Script(taxFormSource).runInContext(dom.getInternalVMContext())
      await flushMicrotasks()
      expect(input('gfmc').value).toBe(payload.title_name)
      expect(input('gfsbh').value).toBe(payload.tax_no)
      expect(input('hsje').value).toBe('110.50')
      expect(input('seller').value).toBe('销售方公司')
      expect(input('amount').value).toBe('')
      expect(event).toHaveBeenCalled()
      input('gfmc').value = '人工修正抬头'
      const retry = [...win.document.querySelectorAll('button')].find((button) => button.textContent === '重新尝试填入')!
      retry.click()
      expect(input('gfmc').value).toBe('人工修正抬头')
      expect(win.document.querySelector('aside')?.textContent).toContain('请核对发票类型')
    } finally { win.close() }
  })
})

async function flushMicrotasks() {
  for (let i = 0; i < 12; i++) await Promise.resolve()
}
