export const BILLING_MODE_TOKEN = 'token'
export const BILLING_MODE_PER_REQUEST = 'per_request'
export const BILLING_MODE_IMAGE = 'image'
export const BILLING_MODE_VIDEO = 'video'

export function getBillingModeLabel(mode: string | null | undefined, t: (key: string) => string): string {
  switch (mode) {
    case BILLING_MODE_PER_REQUEST: return t('admin.usage.billingModePerRequest')
    case BILLING_MODE_IMAGE: return t('admin.usage.billingModeImage')
    case BILLING_MODE_VIDEO: return t('admin.usage.billingModeVideo')
    default: return t('admin.usage.billingModeToken')
  }
}

export function getBillingModeBadgeClass(mode: string | null | undefined): string {
  switch (mode) {
    case BILLING_MODE_PER_REQUEST: return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300'
    case BILLING_MODE_IMAGE: return 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300'
    case BILLING_MODE_VIDEO: return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
    default: return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  }
}

interface MediaBillingRow {
  image_count?: number
  video_count?: number
  video_duration_seconds?: number | null
  billing_mode?: string | null
  total_cost?: number
}

export function isVideoUsage(row: Pick<MediaBillingRow, 'video_count'> | null | undefined): boolean {
  return (row?.video_count ?? 0) > 0
}

export function isImageUsage(row: Pick<MediaBillingRow, 'image_count' | 'video_count' | 'billing_mode'> | null | undefined): boolean {
  return (row?.image_count ?? 0) > 0 && !isVideoUsage(row) && row?.billing_mode !== BILLING_MODE_TOKEN && row?.billing_mode !== BILLING_MODE_VIDEO
}

export function getDisplayBillingMode(row: Pick<MediaBillingRow, 'billing_mode' | 'image_count' | 'video_count'> | null | undefined): string | null | undefined {
  if ((row?.video_count ?? 0) > 0) {
    return BILLING_MODE_VIDEO
  }
  if ((row?.image_count ?? 0) > 0 && !row?.billing_mode) {
    return BILLING_MODE_IMAGE
  }
  return row?.billing_mode
}

export function imageUnitPrice(row: Pick<MediaBillingRow, 'image_count' | 'total_cost'> | null): number {
  const imageCount = row?.image_count ?? 0
  if (!row || imageCount <= 0) return 0
  const total = row.total_cost ?? 0
  const price = total / imageCount
  return Number.isFinite(price) ? price : 0
}

export function videoUnitPrice(row: Pick<MediaBillingRow, 'video_count' | 'video_duration_seconds' | 'total_cost'> | null): number {
  const videoCount = row?.video_count ?? 0
  if (!row || videoCount <= 0) return 0
  const durationSeconds = row.video_duration_seconds && row.video_duration_seconds > 0 ? row.video_duration_seconds : 1
  const price = (row.total_cost ?? 0) / (videoCount * durationSeconds)
  return Number.isFinite(price) ? price : 0
}
