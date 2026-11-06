import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import AnnouncementPopup from '../AnnouncementPopup.vue'
import { useAnnouncementStore, useAuthStore } from '@/stores'
import type { Announcement, UserAnnouncement } from '@/types'

const announcementMarkdownStyles = readFileSync(
  resolve(process.cwd(), 'src/styles/announcement-markdown.css'),
  'utf8',
)

const announcement: UserAnnouncement = {
  id: 1,
  title: 'Announcement',
  content: '## Content',
  notify_mode: 'popup',
  created_at: '2026-07-13T00:00:00.000Z',
  updated_at: '2026-07-13T00:00:00.000Z',
}

const previewAnnouncement: Announcement = {
  id: 2,
  title: 'Preview announcement',
  content: '## Preview heading\n\n<div>HTML content</div><script>window.__xss = true</script>',
  status: 'draft',
  notify_mode: 'popup',
  targeting: { any_of: [] },
  created_at: '2026-07-24T07:30:00Z',
  updated_at: '2026-07-24T07:30:00Z',
}

describe('AnnouncementPopup', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.body.style.overflow = ''
  })

  afterEach(() => {
    document.body.innerHTML = ''
    document.body.style.overflow = ''
  })

  it('suppresses an automatic popup after this user closed announcements for today', async () => {
    const authStore = useAuthStore()
    authStore.user = { id: 7 } as any
    const store = useAnnouncementStore()
    localStorage.setItem(
      'console-announcement-dismiss-date:7',
      new Date().toISOString().slice(0, 10),
    )

    const wrapper = mount(AnnouncementPopup)
    store.currentPopup = announcement
    await nextTick()

    expect(store.currentPopup).toBeNull()
    wrapper.unmount()
  })

  it('renders and sanitizes mixed Markdown and HTML in the shared panel', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = {
      ...announcement,
      content: [
        '## Markdown heading',
        '',
        '<div><h3>HTML heading</h3><ul><li>HTML list item</li></ul></div>',
        '',
        '<table><thead><tr><th>Status</th></tr></thead><tbody><tr><td>OK</td></tr></tbody></table>',
        '<script>window.__announcementXss = true</script>',
      ].join('\n'),
    }
    store.announcements = [store.currentPopup]

    const wrapper = mount(AnnouncementPopup)
    await nextTick()

    // The automatic popup only opens the list; the full text renders on selection.
    document.body.querySelector<HTMLButtonElement>('.notice-item')?.click()
    await nextTick()

    const content = document.body.querySelector('.markdown-body')
    expect(content?.querySelector('h2')?.textContent).toBe('Markdown heading')
    expect(content?.querySelector('h3')?.textContent).toBe('HTML heading')
    expect(content?.querySelector('li')?.textContent).toBe('HTML list item')
    expect(content?.querySelector('table td')?.textContent).toBe('OK')
    expect(content?.querySelector('script')).toBeNull()
    wrapper.unmount()
  })

  it.each(['h2', 'h3', 'ul', 'li', 'blockquote', 'table', 'th', 'td', 'code'])(
    'loads a shared style rule for mixed-content <%s> elements',
    (element) => {
      expect(announcementMarkdownStyles).toContain(`.markdown-body ${element}`)
    },
  )

  it('previews an admin announcement without dismissing or marking user state', async () => {
    const store = useAnnouncementStore()
    const dismissPopup = vi.spyOn(store, 'dismissPopup')
    const markAsRead = vi.spyOn(store, 'markAsRead')
    const wrapper = mount(AnnouncementPopup, {
      props: {
        announcement: previewAnnouncement,
        preview: true,
      },
    })
    await nextTick()

    expect(document.body.textContent).toContain('Preview announcement')
    expect(document.body.querySelector('.markdown-body h2')?.textContent).toBe('Preview heading')
    expect(document.body.querySelector('.markdown-body script')).toBeNull()

    document.body.querySelector<HTMLButtonElement>('[data-testid="announcement-popup-dismiss"]')?.click()
    await nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(dismissPopup).not.toHaveBeenCalled()
    expect(markAsRead).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps the existing user popup dismissal behavior', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = announcement
    const dismissPopup = vi.spyOn(store, 'dismissPopup')
    const wrapper = mount(AnnouncementPopup)
    await nextTick()

    document.body.querySelector<HTMLButtonElement>('.notice-close')?.click()
    await nextTick()

    expect(dismissPopup).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('opens only the announcement list and highlights the popup notices', async () => {
    const store = useAnnouncementStore()
    store.announcements = [
      { ...announcement, id: 11, title: 'Older notice', notify_mode: 'silent', created_at: '2026-07-01T00:00:00.000Z' },
      { ...announcement, id: 12, title: 'Popup notice', created_at: '2026-07-20T00:00:00.000Z' },
    ]
    store.popupHighlightIds = [12]
    store.currentPopup = store.announcements[1]
    const wrapper = mount(AnnouncementPopup)
    await nextTick()

    expect(document.body.querySelector('.notice-card.is-open')).not.toBeNull()
    expect(document.body.querySelector('.notice-reader.is-open')).toBeNull()
    const highlighted = document.body.querySelectorAll('.notice-item.is-highlighted')
    expect(highlighted).toHaveLength(1)
    expect(highlighted[0].textContent).toContain('Popup notice')
    expect(highlighted[0].getAttribute('aria-current')).toBe('true')

    // Reading one notice and closing its text returns to the list, not out of the popup.
    ;(highlighted[0] as HTMLButtonElement).click()
    await nextTick()
    expect(document.body.querySelector('.notice-reader.is-open')).not.toBeNull()
    document.body.querySelector<HTMLButtonElement>('[data-testid="announcement-popup-dismiss"]')?.click()
    await nextTick()
    expect(document.body.querySelector('.notice-reader.is-open')).toBeNull()
    expect(store.currentPopup).not.toBeNull()
    wrapper.unmount()
  })
})
