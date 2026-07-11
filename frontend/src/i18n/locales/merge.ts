export function mergeLocale<T extends Record<string, unknown>>(base: T, overrides: Record<string, unknown>): T {
  const result: Record<string, unknown> = { ...base }
  for (const [key, value] of Object.entries(overrides)) {
    const current = result[key]
    if (
      value !== null && typeof value === 'object' && !Array.isArray(value) &&
      current !== null && typeof current === 'object' && !Array.isArray(current)
    ) {
      result[key] = mergeLocale(current as Record<string, unknown>, value as Record<string, unknown>)
    } else {
      result[key] = value
    }
  }
  return result as T
}
