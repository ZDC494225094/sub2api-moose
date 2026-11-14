import { invoiceAdminAPI, type InvoiceApplication } from './api'

export const TAX_MAKEOUT_URL = 'https://dppt.jiangsu.chinatax.gov.cn:8443/blue-invoice-makeout'
const CHANNEL = 'sub2api-invoice-assistant-v1'

function assistantRequest(action: 'ping' | 'open', application?: InvoiceApplication): Promise<void> {
  // Some private-network installations use plain HTTP. Do not depend on
  // randomUUID(), which is exposed only in secure browser contexts.
  const requestId = Array.from(crypto.getRandomValues(new Uint32Array(4)), (part) => part.toString(16).padStart(8, '0')).join('')
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      cleanup()
      reject(new Error('未检测到开票填表助手，请先下载并在当前浏览器安装，然后刷新本页。'))
    }, action === 'ping' ? 1200 : 10000)
    function cleanup() {
      clearTimeout(timeout)
      window.removeEventListener('message', listener)
    }
    function listener(event: MessageEvent) {
      if (event.source !== window || event.origin !== location.origin) return
      const data = event.data
      if (data?.channel !== CHANNEL || data.requestId !== requestId || data.type !== 'response') return
      cleanup()
      if (data.error) reject(new Error(String(data.error)))
      else resolve()
    }
    window.addEventListener('message', listener)
    // Send only the invoice snapshot needed by the tax form, never auth tokens,
    // user accounts, admin notes, order histories or internal payment data.
    window.postMessage({ channel: CHANNEL, type: 'request', requestId, action, payload: application ? {
      id: application.id, invoice_type: application.invoice_type, title_type: application.title_type,
      title_name: application.title_name, tax_no: application.tax_no, address: application.address,
      phone: application.phone, bank_name: application.bank_name, bank_account: application.bank_account,
      email: application.email, item_name: application.item_name, amount: application.amount, remark: application.remark,
    } : undefined }, location.origin)
  })
}

export async function goMakeout(app: InvoiceApplication): Promise<InvoiceApplication> {
  // Verify the assistant before reserving the application, so a missing install
  // does not unexpectedly remove the applicant's ability to cancel.
  await assistantRequest('ping')
  const updated = await invoiceAdminAPI.prepareMakeout(app.id)
  await assistantRequest('open', updated)
  return updated
}

export async function downloadMakeoutAssistant(): Promise<void> {
  const { default: JSZip } = await import('jszip')
  const sources = import.meta.glob('./tax-assistant/*', { eager: true, query: '?raw', import: 'default' }) as Record<string, string>
  const zip = new JSZip()
  for (const [path, content] of Object.entries(sources)) zip.file(path.split('/').pop()!, content)
  const url = URL.createObjectURL(await zip.generateAsync({ type: 'blob' }))
  const link = document.createElement('a')
  link.href = url
  link.download = '开票填表助手.zip'
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
