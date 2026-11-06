// Announcement lists are shown strictly by when each notice actually went live.
// The user API orders unread notices first, which reads as "unsorted" once read
// and unread items mix, so every list sorts here before rendering.

interface DatedAnnouncement {
  created_at?: string | null
  starts_at?: string | null
}

function toTime(value?: string | null): number {
  if (!value) return Number.NaN
  return new Date(value).getTime()
}

/**
 * Milliseconds of the actual publish time: a scheduled start later than creation
 * wins; an earlier start means the notice was visible as soon as it was created.
 */
export function announcementPublishedAt(notice: DatedAnnouncement): number {
  const created = toTime(notice.created_at)
  const starts = toTime(notice.starts_at)
  if (Number.isFinite(created) && Number.isFinite(starts)) return Math.max(created, starts)
  if (Number.isFinite(created)) return created
  return Number.isFinite(starts) ? starts : 0
}

/** YYYY-MM-DD of the actual publish time, or '' when no valid timestamp exists. */
export function announcementPublishedDate(notice?: DatedAnnouncement | null): string {
  if (!notice) return ''
  const time = announcementPublishedAt(notice)
  return time > 0 ? new Date(time).toISOString().slice(0, 10) : ''
}

/** Published within the last seven days. */
export function isRecentAnnouncement(notice: DatedAnnouncement, now = Date.now()): boolean {
  const time = announcementPublishedAt(notice)
  return time > 0 && now - time < 7 * 86400000
}

/** Newest first by actual publish time; equal times keep their incoming order. */
export function sortAnnouncementsByPublishedAt<T extends DatedAnnouncement>(list: readonly T[]): T[] {
  return [...list].sort((a, b) => announcementPublishedAt(b) - announcementPublishedAt(a))
}
