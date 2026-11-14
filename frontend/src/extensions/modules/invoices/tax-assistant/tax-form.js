/* global chrome */
(() => {
  // Only fill the makeout route. Login/other tax pages keep their normal flow.
  if (location.origin !== 'https://dppt.jiangsu.chinatax.gov.cn:8443') return
  const definitions = [
    { key: 'title_name', label: '购买方名称', names: ['gfmc', 'gmfmc', 'gmf_mc', 'ghdwmc', 'buyerName'], labels: ['购买方名称', '购买方信息名称', '购方名称'] },
    { key: 'tax_no', label: '购买方税号', names: ['gfsbh', 'gmfnsrsbh', 'gmf_nsrsbh', 'ghdwsbh', 'buyerTaxNo'], labels: ['购买方纳税人识别号', '购买方统一社会信用代码', '购方税号'] },
    { key: 'address', label: '购买方地址', names: ['gfdz', 'gmfdz', 'gmf_dz', 'buyerAddress'], labels: ['购买方地址', '购买方注册地址', '购方地址'] },
    { key: 'phone', label: '购买方电话', names: ['gfdh', 'gmfdh', 'gmf_dh', 'buyerPhone'], labels: ['购买方电话', '购买方联系电话', '购方电话'] },
    { key: 'bank_name', label: '购买方开户银行', names: ['gfkhh', 'gmfkhh', 'gmf_khh', 'buyerBankName'], labels: ['购买方开户银行', '购买方开户行', '购方开户行'] },
    { key: 'bank_account', label: '购买方银行账号', names: ['gfyhzh', 'gmfyhzh', 'gmf_yhzh', 'buyerBankAccount'], labels: ['购买方银行账号', '购买方账号', '购方银行账号'] },
    { key: 'email', label: '接收邮箱', names: ['buyerEmail', 'gmf_dzyx'], labels: ['接收邮箱', '购买方邮箱', '发票接收邮箱'] },
    { key: 'item_name', label: '商品/服务名称', names: ['hwmc', 'spmc', 'xmmc', 'itemName'], labels: ['项目名称', '货物或应税劳务服务名称', '商品和服务名称', '商品名称', '货物或应税劳务、服务名称'] },
    // Never guess whether a bare "金额" is tax-inclusive or tax-exclusive.
    { key: 'amount', label: '含税金额（价税合计）', names: ['jshj', 'hsje', 'totalAmountWithTax'], labels: ['价税合计', '含税金额', '价税合计小写'] },
    { key: 'remark', label: '备注', names: ['bz', 'remark'], labels: ['备注'] },
  ]
  let payload
  let panel
  let status
  let mappings = {}
  let expired = false
  const applied = new WeakMap()
  const normalize = (text) => String(text || '').replace(/[\s:：*＊()（）/]/g, '').toLowerCase()
  const visible = (element) => !element.disabled && !element.readOnly && element.getAttribute('type') !== 'hidden' && element.getClientRects().length > 0 && !panel?.contains(element)
  const controls = () => [...document.querySelectorAll('input:not([type=checkbox]):not([type=radio]):not([type=button]):not([type=submit]):not([type=password]):not([type=file]), textarea')].filter(visible)

  function descriptor(element) {
    const formItem = element.closest('.el-form-item, .ant-form-item, .ivu-form-item, .form-item, td')
    const labels = [...(element.labels || [])].map((label) => label.textContent)
    const local = formItem?.querySelector('.el-form-item__label, .ant-form-item-label, .ivu-form-item-label, label')
    if (local) labels.push(local.textContent)
    labels.push(element.getAttribute('aria-label'), element.getAttribute('placeholder'))
    const scope = element.closest('[data-section], fieldset, .buyer-info, .gmf-info, .purchase-info')
    const heading = scope?.querySelector('legend, h2, h3, h4, .title')?.textContent || ''
    const buyer = /购买方|购方/.test(heading)
    return { labels: labels.filter(Boolean).map(normalize), buyer, seller: /销售方|销方/.test(heading) }
  }

  function matches(element, definition) {
    const info = descriptor(element)
    if (info.seller) return false
    const identifiers = [element.id, element.getAttribute('name'), element.getAttribute('data-field')].map(normalize)
    if (definition.names.some((name) => identifiers.includes(normalize(name)))) return true
    return info.labels.some((label) => definition.labels.some((name) => normalize(name) === label || (info.buyer && normalize(name).replace(/购买方|购方/g, '') === label)))
  }

  function setValue(element, value) {
    const prototype = element.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype
    const setter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set
    if (!setter) return false
    setter.call(element, value)
    element.dispatchEvent(new Event('input', { bubbles: true }))
    element.dispatchEvent(new Event('change', { bubbles: true }))
    element.dispatchEvent(new Event('blur', { bubbles: true }))
    return element.value === value
  }

  function candidates(definition) {
    if (mappings[definition.key]) {
      try {
        const elements = [...document.querySelectorAll(mappings[definition.key])].filter(visible)
        if (elements.length === 1 && controls().includes(elements[0])) return elements
      } catch { /* Invalid/stale selectors fall back to explicit labels. */ }
    }
    return controls().filter((element) => matches(element, definition))
  }

  function fill() {
    if (!payload || expired || !location.pathname.startsWith('/blue-invoice-makeout')) return
    const unresolved = []
    let count = 0
    for (const definition of definitions) {
      const value = payload[definition.key]
      if (!value) continue
      const elements = candidates(definition)
      if (elements.length !== 1) { unresolved.push(definition.label); continue }
      const element = elements[0]
      // Respect tax-site lookups and anything the operator has typed, including
      // deliberate corrections made after the assistant initially filled it.
      if (applied.has(element)) { count++; continue }
      if (element.value && element.value !== value) { unresolved.push(`${definition.label}（已有值）`); continue }
      if (element.value === value || setValue(element, value)) {
        applied.set(element, value)
        count++
      } else unresolved.push(definition.label)
    }
    status.textContent = `已识别 ${count} 项。${unresolved.length ? `请补填/核对：${unresolved.join('、')}。` : ''}请核对发票类型、自然人选项、含税口径、商品税收编码和税率后再开具。`
  }

  function stableSelector(element) {
    if (element.id) return `#${CSS.escape(element.id)}`
    if (element.name) {
      const selector = `${element.tagName.toLowerCase()}[name="${CSS.escape(element.name)}"]`
      if (document.querySelectorAll(selector).length === 1) return selector
    }
    return null
  }

  function button(text, action) {
    const node = document.createElement('button')
    node.textContent = text
    node.type = 'button'
    node.style.cssText = 'padding:6px 10px;border:1px solid #cbd5e1;border-radius:6px;background:white;color:#0f172a;cursor:pointer;margin:4px 4px 4px 0'
    node.addEventListener('click', action)
    return node
  }

  function createPanel() {
    panel = document.createElement('aside')
    panel.setAttribute('aria-label', '开票填表助手')
    panel.style.cssText = 'position:fixed;right:16px;top:80px;width:320px;max-width:calc(100vw - 32px);max-height:75vh;overflow:auto;z-index:2147483647;background:#f8fafc;color:#0f172a;border:1px solid #cbd5e1;border-radius:10px;padding:16px;font:13px/1.6 system-ui;box-shadow:0 8px 30px #0002'
    const heading = document.createElement('strong')
    heading.textContent = `开票资料 #${payload.id} · ${payload.invoice_type === 'special' ? '专票' : '普票'} · ${payload.title_type === 'personal' ? '个人' : '企业'}`
    panel.append(heading)
    for (const field of definitions) {
      if (!payload[field.key]) continue
      const line = document.createElement('div')
      line.style.cssText = 'margin-top:6px;overflow-wrap:anywhere;user-select:text'
      line.textContent = `${field.label}：${payload[field.key]}`
      panel.append(line)
    }
    status = document.createElement('p')
    status.setAttribute('role', 'status')
    panel.append(status)
    panel.append(button('重新尝试填入', fill))
    const select = document.createElement('select')
    select.setAttribute('aria-label', '要定位的开票字段')
    select.style.cssText = 'max-width:100%;padding:6px;border:1px solid #cbd5e1;background:white;color:#0f172a'
    for (const field of definitions.filter((field) => payload[field.key])) {
      const option = document.createElement('option')
      option.value = field.key
      option.textContent = field.label
      select.append(option)
    }
    panel.append(select)
    panel.append(button('定位后填入', () => {
      if (!payload || expired) return
      const key = select.value
      status.textContent = '请点击税务表单中要填入的输入框（Esc 取消）。'
      const cancel = () => { document.removeEventListener('click', pick, true); document.removeEventListener('keydown', escape, true) }
      const escape = (event) => { if (event.key === 'Escape') { cancel(); status.textContent = '已取消定位。' } }
      const pick = async (event) => {
        if (!payload || expired) { cancel(); return }
        const target = event.target
        if (!controls().includes(target)) return
        event.preventDefault()
        event.stopPropagation()
        cancel()
        if (target.value && target.value !== payload[key] && !window.confirm('输入框已有内容，确认替换为本次申请信息？')) return
        if (setValue(target, payload[key])) applied.set(target, payload[key])
        const selector = stableSelector(target)
        if (selector) {
          mappings[key] = selector
          await chrome.storage.local.set({ invoiceFieldMappings: mappings })
        }
        status.textContent = selector ? '已填入并记住此字段。请核对。' : '已填入。此输入框没有稳定标识，下次需要重新定位。'
      }
      setTimeout(() => { document.addEventListener('click', pick, true); document.addEventListener('keydown', escape, true) }, 0)
    }))
    panel.append(button('清除字段映射', async () => {
      mappings = {}
      await chrome.storage.local.remove('invoiceFieldMappings')
      status.textContent = '已清除字段映射。'
    }))
    panel.append(button('清除本次资料', async () => {
      await chrome.runtime.sendMessage({ type: 'clear' })
      expired = true
      payload = null
      panel.remove()
    }))
    document.body.append(panel)
  }

  async function start() {
    const result = await chrome.runtime.sendMessage({ type: 'get' })
    if (!result?.payload || result.error) return
    payload = result.payload
    mappings = (await chrome.storage.local.get('invoiceFieldMappings')).invoiceFieldMappings || {}
    let timer
    const observer = new MutationObserver((changes) => {
      if (changes.every((change) => panel?.contains(change.target))) return
      clearTimeout(timer)
      timer = setTimeout(checkRoute, 400)
    })
    function checkRoute() {
      const makeout = location.pathname.startsWith('/blue-invoice-makeout')
      if (makeout && !panel) createPanel()
      if (panel) panel.hidden = !makeout
      if (makeout) fill()
    }
    observer.observe(document.body, { childList: true, subtree: true })
    checkRoute()
    // Also handle client-side routing with no relevant DOM mutation.
    const interval = setInterval(checkRoute, 2000)
    setTimeout(async () => {
      expired = true
      clearTimeout(timer)
      clearInterval(interval)
      observer.disconnect()
      payload = null
      panel?.remove()
      await chrome.runtime.sendMessage({ type: 'clear' })
    }, result.expiresIn || 30 * 60 * 1000)
  }
  void start()
})()
