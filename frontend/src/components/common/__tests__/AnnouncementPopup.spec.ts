import { nextTick } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AnnouncementPopup from '../AnnouncementPopup.vue'
import { useAnnouncementStore, useAuthStore } from '@/stores'
import type { UserAnnouncement } from '@/types'

const announcement: UserAnnouncement = {
  id: 1,
  title: 'Announcement',
  content: 'Content',
  notify_mode: 'popup',
  created_at: '2026-07-13T00:00:00.000Z',
  updated_at: '2026-07-13T00:00:00.000Z',
}

describe('AnnouncementPopup', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    document.body.style.overflow = ''
  })

  it('suppresses an automatic popup after this user closed announcements for today', async () => {
    const authStore = useAuthStore()
    authStore.user = { id: 7 } as any
    const announcementStore = useAnnouncementStore()
    localStorage.setItem(
      'console-announcement-dismiss-date:7',
      new Date().toISOString().slice(0, 10)
    )

    const wrapper = mount(AnnouncementPopup, {
      global: {
        stubs: {
          AnnouncementPanel: true,
        },
      },
    })

    announcementStore.currentPopup = announcement
    await nextTick()

    expect(announcementStore.currentPopup).toBeNull()
    wrapper.unmount()
  })
})
