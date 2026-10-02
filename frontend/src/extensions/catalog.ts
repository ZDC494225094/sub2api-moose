// Only fully managed extensions belong here. Unmigrated core changes must never be
// presented as disabled merely because their navigation is hidden.
export const extensionPaths = {
  'premium-home': ['/', '/docs'],
  playground: ['/playground'],
  'infinite-canvas': ['/canvas'],
  'operations-analytics': ['/admin/operations'],
  'recharge-campaigns': ['/recharge-campaigns', '/admin/orders/campaigns'],
  'customer-support': [],
  // UI/configuration admission only; published announcements remain readable.
  'site-customization': [],
  'marketing-tools': ['/lottery', '/admin/orders/coupons', '/admin/orders/lottery'],
  // Key routing ownership is per key; the switch only gates new configuration.
  'multi-group-billing': [],
  // Backend-only pricing/scheduling gates; no page interception.
  'billing-scheduling': [],
  'admin-efficiency': [],
  // Configuration admission only; persisted security rules are always enforced.
  'access-policy': [],
  // New media submissions only; historical task pages must remain accessible.
  'media-gateway': [],
  // Issuance policy only; existing subscriptions and redeem pages remain available.
  'subscription-extensions': [],
} as const

export type ExtensionId = keyof typeof extensionPaths
export const extensionIds = Object.keys(extensionPaths) as ExtensionId[]

export function extensionForPath(path: string): ExtensionId | undefined {
  const pathname = path.split(/[?#]/, 1)[0].replace(/\/$/, '') || '/'
  return extensionIds.find(id => extensionPaths[id].some(prefix =>
    pathname === prefix || (prefix !== '/' && pathname.startsWith(`${prefix}/`))))
}

export function extensionFallback(id: ExtensionId, isAdmin: boolean): string {
  if (id === 'premium-home') return '/home'
  if (id === 'recharge-campaigns' && !isAdmin) return '/purchase'
  return isAdmin ? '/admin/dashboard' : '/dashboard'
}


// Ownership is not switch readiness. These module-owned routes retain their
// existing host auth/payment gates until admission and historical drain are ready.
export const pendingExtensionPaths: Readonly<Record<string, readonly string[]>> = {}

export function extensionOwnerForPath(path: string): ExtensionId | keyof typeof pendingExtensionPaths | undefined {
  const managed = extensionForPath(path)
  if (managed) return managed
  const pathname = path.split(/[?#]/, 1)[0].replace(/\/$/, '') || '/'
  return (Object.keys(pendingExtensionPaths) as (keyof typeof pendingExtensionPaths)[])
    .find(id => pendingExtensionPaths[id]?.some(prefix => pathname === prefix || pathname.startsWith(`${prefix}/`)))
}
