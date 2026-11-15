// @vitest-environment node
import { afterEach, describe, expect, it } from 'vitest'
import { createServer, type Server, type RequestListener } from 'node:http'
import type { AddressInfo } from 'node:net'
import { homepageGateMiddleware } from '../../dev/homepage-gate'

const servers: Server[] = []
async function listen(handler: RequestListener) {
  const server = createServer(handler)
  servers.push(server)
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`
}
afterEach(async () => {
  await Promise.all(servers.splice(0).map(server => new Promise<void>(resolve => {
    server.closeAllConnections()
    server.close(() => resolve())
  })))
})

async function devServer(backend: string) {
  const gate = homepageGateMiddleware(backend)
  return listen((req, res) => gate(req, res, () => {
    // Vite's HTML sender overwrites Cache-Control and honors conditional GETs.
    res.setHeader('Cache-Control', 'no-cache')
    if (req.headers['if-none-match']) { res.statusCode = 304; res.end(); return }
    res.setHeader('Content-Type', 'text/html')
    res.end('<script type="module" src="/src/main.ts"></script>')
  }))
}

describe('homepage verification before Vite', () => {
  it('serves only the backend challenge and forwards cookie, host and origin for verification', async () => {
    let seenHost: string | undefined
    let seenOrigin: string | undefined
    let seenBody = ''
    const backend = await listen(async (req, res) => {
      seenHost = req.headers.host
      if (req.method === 'POST') {
        seenOrigin = req.headers.origin
        for await (const part of req) seenBody += part
        expect(req.headers.cookie).toBe('challenge=browser')
        res.writeHead(303, { Location: '/?aff=keep', 'Set-Cookie': ['clearance=verified; HttpOnly; Path=/'] })
        res.end()
      } else {
        expect(req.headers['x-homepage-verification-check']).toBe('1')
        res.writeHead(200, { 'X-Homepage-Verification': 'required', 'Content-Security-Policy': "default-src 'none'", 'Set-Cookie': ['challenge=browser; HttpOnly; Path=/'] })
        res.end('<form action="/_homepage/verify">captcha</form>')
      }
    })
    const dev = await devServer(backend)
    const first = await fetch(dev + '/')
    expect(await first.text()).not.toContain('/src/main.ts')
    expect(first.headers.get('set-cookie')).toContain('challenge=browser')
    const posted = await fetch(dev + '/_homepage/verify', { method: 'POST', redirect: 'manual', headers: {
      Origin: dev, Cookie: 'challenge=browser', 'Content-Type': 'application/x-www-form-urlencoded',
    }, body: 'answer=123456' })
    expect(posted.status).toBe(303)
    expect(posted.headers.get('set-cookie')).toContain('clearance=verified')
    expect(posted.headers.get('location')).toBe('/?aff=keep')
    expect(seenHost).toBe(new URL(dev).host)
    expect(seenOrigin).toBe(dev)
    expect(seenBody).toBe('answer=123456')
  })

  it.each(['passed', 'disabled'])('loads Vite only after an explicit %s decision', async decision => {
    const backend = await listen((_req, res) => {
      res.writeHead(204, { 'X-Homepage-Verification': decision, 'Content-Security-Policy': "default-src 'none'" })
      res.end()
    })
    const dev = await devServer(backend)
    const response = await fetch(dev + '/home?aff=keep', { headers: { 'If-None-Match': 'old-document' } })
    expect(response.status).toBe(200)
    expect(await response.text()).toContain('/src/main.ts')
    expect(response.headers.get('content-security-policy')).toBeNull()
    expect(response.headers.get('cache-control')).toContain('no-store')
  })

  it('does not turn an old backend or failed verification into permission to load the SPA', async () => {
    const backend = await listen((_req, res) => { res.statusCode = 404; res.end('old backend') })
    const dev = await devServer(backend)
    const response = await fetch(dev + '/')
    expect(response.status).toBe(503)
    expect(await response.text()).not.toContain('/src/main.ts')
  })

  it('leaves non-home pages and module requests to Vite without contacting the backend', async () => {
    let calls = 0
    const backend = await listen((_req, res) => { calls++; res.end() })
    const dev = await devServer(backend)
    for (const path of ['/login', '/admin/settings', '/src/main.ts', '/@vite/client']) {
      expect(await (await fetch(dev + path)).text()).toContain('/src/main.ts')
    }
    expect(calls).toBe(0)
  })
})
