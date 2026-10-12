import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import AnnouncementBell from '../AnnouncementBell.vue'
import { useAnnouncementStore } from '@/stores'
import { useAppStore } from '@/stores/app'
import type { UserAnnouncement } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const announcements: UserAnnouncement[] = [
  {
    id: 1,
    title: 'Popup announcement',
    content: 'Popup content',
    notify_mode: 'popup',
    created_at: '2026-07-13T00:00:00.000Z',
    updated_at: '2026-07-13T00:00:00.000Z',
  },
  {
    id: 2,
    title: 'Silent announcement',
    content: 'Silent content',
    notify_mode: 'silent',
    created_at: '2026-07-12T00:00:00.000Z',
    updated_at: '2026-07-12T00:00:00.000Z',
  },
]

const AnnouncementPanelStub = {
  props: ['announcements', 'open'],
  template: '<div class="home-announcement-panel" :data-open="open" :data-count="announcements.length"></div>',
}

describe('AnnouncementBell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('reports a failed read without closing the panel and retries successfully', async () => {
    const store = useAnnouncementStore()
    store.announcements = announcements.map(a => ({ ...a }))
    const markRead = vi.spyOn(store, 'markAsRead').mockResolvedValueOnce(false).mockImplementationOnce(async id => {
      store.announcements.find(a => a.id === id)!.read_at = new Date().toISOString()
      return true
    })
    const error = vi.spyOn(useAppStore(), 'showError')
    const wrapper = mount(AnnouncementBell, { global: { stubs: { AnnouncementPanel: AnnouncementPanelStub, Icon: true } } })
    await wrapper.get('button').trigger('click')
    wrapper.findComponent(AnnouncementPanelStub).vm.$emit('select', store.announcements[0])
    await flushPromises()
    expect(error).toHaveBeenCalledWith('common.unknownError')
    expect(store.announcements[0].read_at).toBeUndefined()
    expect(wrapper.get('.home-announcement-panel').attributes('data-open')).toBe('true')
    wrapper.findComponent(AnnouncementPanelStub).vm.$emit('select', store.announcements[0])
    await flushPromises()
    expect(markRead).toHaveBeenCalledTimes(2)
    expect(store.announcements[0].read_at).toBeTruthy()
    wrapper.unmount()
    vi.restoreAllMocks()
  })

  it('opens the shared home panel with every displayed announcement', async () => {
    const announcementStore = useAnnouncementStore()
    announcementStore.announcements = announcements

    const wrapper = mount(AnnouncementBell, {
      global: {
        stubs: {
          AnnouncementPanel: AnnouncementPanelStub,
          Icon: true,
        },
      },
    })

    await wrapper.get('[aria-label="announcements.title"]').trigger('click')

    const panel = wrapper.get('.home-announcement-panel')
    expect(panel.attributes('data-open')).toBe('true')
    expect(panel.attributes('data-count')).toBe('2')

    wrapper.unmount()
  })
})
