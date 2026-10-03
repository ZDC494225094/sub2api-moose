import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { extensionForPath, extensionOwnerForPath, extensionPaths, pendingExtensionPaths } from '../catalog'
import { customExtensionRoutes } from '../routes'

describe('extension ownership contract', () => {
  it('matches the backend managed catalog exactly', () => {
    const catalog = JSON.parse(readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../../../backend/internal/customize/catalog.json'), 'utf8'))
    const managed = Object.fromEntries(catalog.filter((item: { managed: boolean }) => item.managed)
      .map((item: { id: string; paths: string[] }) => [item.id, item.paths]))
    expect(extensionPaths).toEqual(managed)
  })
  it('uses path segment boundaries and leaves upstream routes alone', () => {
    expect(extensionForPath('/canvas/')).toBe('infinite-canvas')
    expect(extensionForPath('/admin/orders/campaigns?search=1')).toBe('recharge-campaigns')
    expect(extensionForPath('/admin/operations/conversion')).toBe('operations-analytics')
    expect(extensionForPath('/')).toBe('premium-home')
    for (const path of ['/home', '/docs-other', '/canvases', '/admin/ops', '/admin/plugins', '/admin/custom-extensions', '/admin/orders', '/payment/result']) {
      expect(extensionForPath(path)).toBeUndefined()
    }
  })
  it('promotes marketing route ownership without losing historical pages', () => {
    expect(pendingExtensionPaths).toEqual({ 'billing-scheduling': [] })
    for (const path of extensionPaths['marketing-tools']) {
      expect(extensionOwnerForPath(path)).toBe('marketing-tools')
      expect(extensionForPath(path)).toBe('marketing-tools')
      expect(customExtensionRoutes.filter(route => route.path === path)).toHaveLength(1)
    }
    expect(extensionOwnerForPath('/lottery-other')).toBeUndefined()
  })
  it('keeps management admin-only and every migrated route owned', () => {
    const management = customExtensionRoutes.find(route => route.path === '/admin/custom-extensions')!
    expect(management.meta).toMatchObject({ requiresAuth: true, requiresAdmin: true })
    for (const route of customExtensionRoutes) {
      if (route !== management) expect(extensionOwnerForPath(route.path), route.path).toBeDefined()
    }
  })
})
