import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAnnouncementStore } from '@/stores/announcements'
import type { UserAnnouncement } from '@/types'

const mockList = vi.fn()
const mockMarkRead = vi.fn()

vi.mock('@/api', () => ({
  announcementsAPI: {
    list: (...args: unknown[]) => mockList(...args),
    markRead: (...args: unknown[]) => mockMarkRead(...args),
  },
}))

function createAnnouncement(id: number, overrides: Partial<UserAnnouncement> = {}): UserAnnouncement {
  return {
    id,
    title: `Announcement ${id}`,
    content: `Content for announcement ${id}`,
    notify_mode: 'popup',
    created_at: '2026-07-13T00:00:00.000Z',
    updated_at: '2026-07-13T00:00:00.000Z',
    ...overrides,
  }
}

describe('useAnnouncementStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('opens one announcement panel for multiple new popup announcements', async () => {
    mockList.mockResolvedValue([
      createAnnouncement(1),
      createAnnouncement(2),
      createAnnouncement(3),
    ])
    const store = useAnnouncementStore()

    await store.fetchAnnouncements()

    expect(store.currentPopup?.id).toBe(1)
    expect(store.announcements.map((announcement) => announcement.id)).toEqual([1, 2, 3])

    store.dismissPopup()

    expect(store.currentPopup).toBeNull()
    expect(mockMarkRead).not.toHaveBeenCalled()
  })

  it('keeps all displayed announcements available to the shared panel', async () => {
    mockList.mockResolvedValue([
      createAnnouncement(1),
      createAnnouncement(2, { notify_mode: 'silent' }),
      createAnnouncement(3, { read_at: '2026-07-12T00:00:00.000Z' }),
    ])
    const store = useAnnouncementStore()

    await store.fetchAnnouncements()

    expect(store.currentPopup?.id).toBe(1)
    expect(store.announcements.map((announcement) => announcement.id)).toEqual([1, 2, 3])
  })

  it('adds newly fetched popup announcements to the already open panel', async () => {
    const first = createAnnouncement(1)
    const second = createAnnouncement(2)
    const third = createAnnouncement(3)
    mockList
      .mockResolvedValueOnce([first, second])
      .mockResolvedValueOnce([first, second, third])
    const store = useAnnouncementStore()

    await store.fetchAnnouncements()
    const initialPopup = store.currentPopup

    await store.fetchAnnouncements(true)

    expect(store.currentPopup).toBe(initialPopup)
    expect(store.announcements.map((announcement) => announcement.id)).toEqual([1, 2, 3])
  })

  it('keeps announcements newest first by publish time and records popup highlights', async () => {
    mockList.mockResolvedValue([
      // API order: unread first, then by id.
      createAnnouncement(5, { created_at: '2026-07-01T00:00:00.000Z' }),
      createAnnouncement(4, { created_at: '2026-07-03T00:00:00.000Z', notify_mode: 'silent' }),
      createAnnouncement(6, { created_at: '2026-07-20T00:00:00.000Z', read_at: '2026-07-21T00:00:00.000Z' }),
      createAnnouncement(3, { created_at: '2026-07-10T00:00:00.000Z' }),
    ])
    const store = useAnnouncementStore()

    await store.fetchAnnouncements()

    expect(store.announcements.map((announcement) => announcement.id)).toEqual([6, 3, 4, 5])
    expect(store.currentPopup?.id).toBe(3)
    expect(store.popupHighlightIds).toEqual([3, 5])

    store.dismissPopup()
    expect(store.popupHighlightIds).toEqual([])
  })
})
