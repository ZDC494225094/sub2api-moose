import { sanitizeUrl } from '@/utils/url'

// After-sales groups and one-click contact links keep their single string
// settings (after_sales_group / customer_service_link) so older values and the
// API stay unchanged. Several entries are stored one per line as
// "名称 | 值"; a line without " | " is a value without a name.
export interface ContactEntry {
  label: string
  value: string
}

export interface ContactLink {
  label: string
  url: string
}

export const MAX_CONTACT_ENTRIES = 10
const LABEL_MAX = 32
const VALUE_MAX = 500
const LINE_PATTERN = /^(.{1,40}?)\s+\|\s+(.+)$/

function clean(value: string, max: number): string {
  return value.replace(/[\r\n]+/g, ' ').trim().slice(0, max)
}

export function parseContactEntries(raw: string | null | undefined): ContactEntry[] {
  const out: ContactEntry[] = []
  for (const line of (raw || '').split(/\r?\n/)) {
    const text = line.trim()
    if (!text) continue
    const match = LINE_PATTERN.exec(text)
    const entry = match
      ? { label: clean(match[1], LABEL_MAX), value: clean(match[2], VALUE_MAX) }
      : { label: '', value: clean(text, VALUE_MAX) }
    if (entry.value) out.push(entry)
    if (out.length >= MAX_CONTACT_ENTRIES) break
  }
  return out
}

export function serializeContactEntries(entries: readonly ContactEntry[]): string {
  return entries
    .map(e => ({ label: clean(e.label, LABEL_MAX).replace(/\|/g, '/'), value: clean(e.value, VALUE_MAX) }))
    .filter(e => e.value)
    .slice(0, MAX_CONTACT_ENTRIES)
    .map(e => (e.label ? `${e.label} | ${e.value}` : e.value))
    .join('\n')
}

/** Only links that pass URL sanitizing; others are dropped instead of rendered. */
export function parseContactLinks(raw: string | null | undefined): ContactLink[] {
  return parseContactEntries(raw)
    .map(e => ({ label: e.label, url: sanitizeUrl(e.value) }))
    .filter(link => !!link.url)
}
