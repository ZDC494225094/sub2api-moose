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

  it('orders strictly by actual publish time regardless of read state', () => {
    const mixed: UserAnnouncement[] = [
      { ...announcements[0], id: 21, title: 'Unread but old', created_at: '2026-06-01T00:00:00.000Z' },
      { ...announcements[2], id: 22, title: 'Read and newest', created_at: '2026-07-20T00:00:00.000Z' },
      // Created early, scheduled to go live later: sorted and dated by the start.
      { ...announcements[1], id: 23, title: 'Scheduled', created_at: '2026-05-01T00:00:00.000Z', starts_at: '2026-07-10T00:00:00.000Z' },
    ]
    const wrapper = mount(AnnouncementPanel, {
      props: { announcements: mixed, open: true },
      global: { stubs: { Icon: true, Teleport: true } },
    })

    const items = wrapper.findAll('.notice-item')
    expect(items.map((item) => item.find('h3').text())).toEqual(['Read and newest', 'Scheduled', 'Unread but old'])
    expect(items[1].find('.notice-date').text()).toContain('2026-07-10')
  })

  it('highlights requested notices without opening their full text', () => {
    const wrapper = mount(AnnouncementPanel, {
      props: { announcements, open: true, highlightIds: [2] },
      global: { stubs: { Icon: true, Teleport: true } },
    })

    const highlighted = wrapper.findAll('.notice-item.is-highlighted')
    expect(highlighted).toHaveLength(1)
    expect(highlighted[0].text()).toContain('Silent announcement')
    expect(highlighted[0].text()).toContain('待阅读')
    expect(wrapper.find('.notice-reader.is-open').exists()).toBe(false)
  })
})
