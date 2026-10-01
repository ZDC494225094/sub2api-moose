// These exact pages implement read-only admission locally and keep their normal
// host authentication/admin/payment gates. This does not authorize API writes.
export const marketingHistoricalPaths = [
  '/lottery',
  '/admin/orders/coupons',
  '/admin/orders/lottery',
] as const
