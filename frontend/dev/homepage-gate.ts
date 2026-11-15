import { request as httpRequest, type IncomingMessage, type ServerResponse } from 'node:http'
import { request as httpsRequest } from 'node:https'
import type { Connect, Plugin } from 'vite'

const homePaths = new Set(['/', '/home', '/home/', '/index.html'])
const hopByHopHeaders = new Set(['connection', 'keep-alive', 'transfer-encoding', 'upgrade'])

/** Ask the real backend before Vite injects its client or transforms index.html. */
export function homepageGateMiddleware(backendUrl: string): Connect.NextHandleFunction {
  const target = new URL(backendUrl)
  const request = target.protocol === 'https:' ? httpsRequest : httpRequest
  return (req: IncomingMessage, res: ServerResponse, next: Connect.NextFunction) => {
    const pathname = new URL(req.url || '/', 'http://localhost').pathname
    const homepage = homePaths.has(pathname) && (req.method === 'GET' || req.method === 'HEAD')
    if (!homepage && pathname !== '/_homepage/verify') return next()

    // Preserve the browser's Host/Origin and cookies together. Changing only
    // Host to the backend port would reject a legitimate same-origin form POST.
    const headers: Record<string, string | string[]> = {}
    for (const key of ['host', 'cookie', 'user-agent', 'content-type', 'content-length', 'origin', 'sec-fetch-site']) {
      const value = req.headers[key]
      if (value !== undefined) headers[key] = value
    }
    if (homepage) headers['x-homepage-verification-check'] = '1'

    const upstream = request({
      protocol: target.protocol,
      hostname: target.hostname.replace(/^\[|\]$/g, ''),
      port: target.port,
      path: req.url,
      method: req.method,
      headers,
    }, (response) => {
      const decision = response.headers['x-homepage-verification']
      if (homepage && response.statusCode === 204 && (decision === 'passed' || decision === 'disabled')) {
        response.resume()
        // Only the backend's explicit decision lets Vite load the application.
        // The challenge CSP must never leak onto the allowed Vite/HMR page.
        // Vite's HTML sender sets no-cache itself, so retain no-store when it
        // writes its headers and avoid reusing an older conditional response.
        const setHeader = res.setHeader
        res.setHeader = function (name, value) {
          return setHeader.call(this, name, name.toLowerCase() === 'cache-control' ? 'private, no-store' : value)
        }
        delete req.headers['if-none-match']
        delete req.headers['if-modified-since']
        res.setHeader('Cache-Control', 'private, no-store')
        res.setHeader('Vary', 'Cookie')
        res.setHeader('X-Homepage-Verification', decision)
        next()
        return
      }
      if (homepage && decision !== 'required') {
        response.resume()
        unavailable()
        return
      }
      res.statusCode = response.statusCode || 502
      for (const [key, value] of Object.entries(response.headers)) {
        if (value !== undefined && !hopByHopHeaders.has(key)) res.setHeader(key, value)
      }
      response.on('error', () => res.destroy())
      response.pipe(res)
    })
    function unavailable() {
      if (res.writableEnded || res.destroyed) return
      if (res.headersSent) { res.destroy(); return }
      res.writeHead(503, { 'Content-Type': 'text/plain; charset=utf-8', 'Cache-Control': 'no-store', 'Retry-After': '5' })
      res.end('首页验证服务暂时不可用，请确认后端已更新并启动后刷新。')
    }
    upstream.setTimeout(5000, () => upstream.destroy(new Error('Homepage verification timeout')))
    upstream.on('error', unavailable)
    req.on('aborted', () => upstream.destroy())
    res.on('close', () => { if (!res.writableEnded) upstream.destroy() })
    req.pipe(upstream)
  }
}

export function homepageGateDevServer(backendUrl: string): Plugin {
  return {
    name: 'homepage-gate-before-vite',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use(homepageGateMiddleware(backendUrl))
    },
  }
}
