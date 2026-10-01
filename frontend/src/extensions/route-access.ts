import { extensionOwnerForPath } from './catalog'
import { marketingHistoricalPaths } from './modules/marketing/route-access'

// An explicit, owner-bound allowlist, not a prefix or user-controlled route meta.
// Newly added write-only pages must never inherit historical access implicitly.
const historicalPages: Readonly<Record<string, readonly string[]>> = {
  'marketing-tools': marketingHistoricalPaths,
}

export function extensionHistoricalPath(path: string): boolean {
  const pathname = path.split(/[?#]/, 1)[0].replace(/\/$/, '') || '/'
  const owner = extensionOwnerForPath(pathname)
  return !!owner && (historicalPages[owner]?.includes(pathname) ?? false)
}
