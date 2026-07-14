import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import AnnouncementPanel from '../AnnouncementPanel.vue'
import type { UserAnnouncement } from '@/types'

const announcements: UserAnnouncement[] = [
  {
    id: 1,
    title: 'Popup announcement',
    content: 'A popup announcement.',
    notify_mode: 'popup',
    created_at: '2026-07-13T00:00:00.000Z',
    updated_at: '2026-07-13T00:00:00.000Z',
  },
  {
    id: 2,
    title: 'Silent announcement',
    content: 'A silent announcement that is still visible in the list.',
    notify_mode: 'silent',
    created_at: '2026-07-12T00:00:00.000Z',
    updated_at: '2026-07-12T00:00:00.000Z',
  },
  {
    id: 3,
    title: 'Read announcement',
    content: 'An already read announcement that remains visible in the list.',
    notify_mode: 'popup',
    read_at: '2026-07-12T00:00:00.000Z',
    created_at: '2026-07-11T00:00:00.000Z',
    updated_at: '2026-07-11T00:00:00.000Z',
  },
]

describe('AnnouncementPanel', () => {
  it('renders every supplied announcement in one panel', async () => {
    const wrapper = mount(AnnouncementPanel, {
      props: {
        announcements,
        open: true,
      },
      global: {
        stubs: {
          Icon: true,
          Teleport: true,
        },
      },
    })

    const items = wrapper.findAll('.notice-item')
    expect(items).toHaveLength(3)
    expect(items.map((item) => item.text())).toEqual(expect.arrayContaining([
      expect.stringContaining('Popup announcement'),
      expect.stringContaining('Silent announcement'),
      expect.stringContaining('Read announcement'),
    ]))

    await items[1].trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([announcements[1]])
  })
})
