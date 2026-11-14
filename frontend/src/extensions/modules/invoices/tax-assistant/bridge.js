/* global chrome */
(() => {
  if (location.pathname !== '/admin/invoices' && location.pathname !== '/admin/invoices/') return
  const channel = 'sub2api-invoice-assistant-v1'
  window.addEventListener('message', (event) => {
    if (event.source !== window || event.origin !== location.origin) return
    const msg = event.data
    if (msg?.channel !== channel || msg.type !== 'request' || !['ping', 'open'].includes(msg.action)) return
    chrome.runtime.sendMessage({ type: msg.action, payload: msg.payload }, (result) => {
      const error = chrome.runtime.lastError?.message || result?.error
      window.postMessage({ channel, type: 'response', requestId: msg.requestId, error }, location.origin)
    })
  })
})()
