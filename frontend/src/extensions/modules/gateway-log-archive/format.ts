export function formatBytes(n: number | null | undefined): string {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 2)} ${units[i]}`
}

export function formatTime(v: string | null | undefined): string {
  if (!v) return '—'
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? v : d.toLocaleString('zh-CN', { hour12: false })
}

export function errorMessage(e: unknown, fallback: string): string {
  const m = (e as { message?: string })?.message
  return m && typeof m === 'string' ? m : fallback
}

export const providerLabels: Record<string, string> = {
  aws: 'Amazon S3',
  's3-compatible': 'S3 兼容',
}
