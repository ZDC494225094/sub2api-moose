/* global chrome */
const taxURL = 'https://dppt.jiangsu.chinatax.gov.cn:8443/blue-invoice-makeout'
const ttl = 30 * 60 * 1000
const fields = ['invoice_type', 'title_type', 'title_name', 'tax_no', 'address', 'phone', 'bank_name', 'bank_account', 'email', 'item_name', 'amount', 'remark']

async function handle(message, sender) {
  const source = new URL(sender.url || 'about:blank')
  if (sender.frameId !== 0 || !sender.tab) throw new Error('仅支持浏览器主页面')
  if (message.type === 'get' || message.type === 'clear') {
    if (source.origin !== new URL(taxURL).origin) throw new Error('税务页面地址不匹配')
    const key = `invoice:${sender.tab.id}`
    const data = (await chrome.storage.session.get(key))[key]
    if (message.type === 'clear' || !data || Date.now() - data.created > ttl) {
      await chrome.storage.session.remove(key)
      return {}
    }
    return { payload: data.payload, expiresIn: Math.max(1, ttl - (Date.now() - data.created)) }
  }
  if (!['http:', 'https:'].includes(source.protocol) || !/^\/admin\/invoices\/?$/.test(source.pathname)) throw new Error('请从发票管理页面操作')
  if (message.type === 'ping') return { ready: true }
  if (message.type !== 'open') throw new Error('未知操作')
  const raw = message.payload
  if (!Number.isSafeInteger(raw?.id) || raw.id <= 0) throw new Error('申请编号无效')
  const payload = { id: raw.id }
  for (const key of fields) {
    if (typeof raw[key] !== 'string' || raw[key].length > 500) throw new Error('开票资料格式无效')
    payload[key] = raw[key]
  }
  if (!['normal', 'special'].includes(payload.invoice_type) || !['personal', 'company'].includes(payload.title_type) || !/^\d+(\.\d{1,2})?$/.test(payload.amount)) throw new Error('开票资料格式无效')
  // Store before navigation: each tax tab has its own snapshot, avoiding any
  // shared "latest application" race when several invoices are opened.
  const tab = await chrome.tabs.create({ url: 'about:blank' })
  const key = `invoice:${tab.id}`
  try {
    await chrome.storage.session.set({ [key]: { created: Date.now(), payload } })
    await chrome.tabs.update(tab.id, { url: taxURL })
  } catch (error) {
    await chrome.storage.session.remove(key)
    await chrome.tabs.remove(tab.id).catch(() => {})
    throw error
  }
  return { opened: true }
}

chrome.runtime.onMessage.addListener((message, sender, reply) => {
  handle(message, sender).then(reply, (error) => reply({ error: error.message }))
  return true
})
chrome.tabs.onRemoved.addListener((id) => { void chrome.storage.session.remove(`invoice:${id}`) })
