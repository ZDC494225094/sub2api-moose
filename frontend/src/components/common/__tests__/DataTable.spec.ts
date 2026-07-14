import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DataTable from '../DataTable.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const stubMatchMedia = (matches = true) => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  })
}

describe('DataTable', () => {
  beforeEach(() => {
    stubMatchMedia()
    localStorage.clear()
  })

  it('renders paired sort arrows and highlights the active direction', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name', sortable: true },
          { key: 'created_at', label: 'Created', sortable: true }
        ],
        data: [
          { id: 1, name: 'Beta', created_at: '2026-01-02T00:00:00Z' },
          { id: 2, name: 'Alpha', created_at: '2026-01-01T00:00:00Z' }
        ],
        defaultSortKey: 'name',
        defaultSortOrder: 'asc'
      }
    })

    await wrapper.vm.$nextTick()

    const nameHeader = wrapper.findAll('th')[0]
    expect(nameHeader.attributes('aria-sort')).toBe('ascending')
    expect(nameHeader.findAll('svg')).toHaveLength(2)
    expect(nameHeader.findAll('svg')[0].classes()).toContain('text-primary-600')
    expect(nameHeader.findAll('svg')[1].classes()).toContain('text-gray-300')

    await nameHeader.trigger('click')
    await wrapper.vm.$nextTick()

    expect(nameHeader.attributes('aria-sort')).toBe('descending')
    expect(nameHeader.findAll('svg')[0].classes()).toContain('text-gray-300')
    expect(nameHeader.findAll('svg')[1].classes()).toContain('text-primary-600')
  })

  it('renders all grouped rows and one header per adjacent group', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: [
          { id: 1, name: 'A', platform: 'openai' },
          { id: 2, name: 'B', platform: 'openai' },
          { id: 3, name: 'C', platform: 'anthropic' }
        ],
        rowGroup: (row: { platform: string }) => row.platform
      },
      slots: {
        'group-header': '<template #group-header="{ groupKey }"><div data-test="group-header">{{ groupKey }}</div></template>'
      }
    })

    await wrapper.vm.$nextTick()

    expect(wrapper.findAll('tr[data-row-id]')).toHaveLength(3)
    expect(wrapper.findAll('[data-test="group-header"]').map(header => header.text())).toEqual([
      'openai',
      'anthropic'
    ])
  })

  it('keeps desktop group headers visible while collapsed rows are removed', async () => {
    const expandedGroups = new Set(['openai'])
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: [
          { id: 1, name: 'A', platform: 'openai' },
          { id: 2, name: 'B', platform: 'openai' },
          { id: 3, name: 'C', platform: 'anthropic' }
        ],
        rowGroup: (row: { platform: string }) => row.platform,
        rowGroupExpanded: (groupKey: string | number) => expandedGroups.has(String(groupKey))
      },
      slots: {
        'group-header': '<template #group-header="{ groupKey, expanded }"><div data-test="group-header" :data-expanded="String(expanded)">{{ groupKey }}</div></template>'
      }
    })

    await wrapper.vm.$nextTick()

    expect(wrapper.findAll('[data-test="group-header"]')).toHaveLength(2)
    expect(wrapper.findAll('tr[data-row-id]').map(row => row.attributes('data-row-id'))).toEqual(['1', '2'])
    expect(wrapper.findAll('[data-test="group-header"]').map(header => header.attributes('data-expanded'))).toEqual([
      'true',
      'false'
    ])
  })

  it('collapses grouped cards on mobile without hiding their headers', async () => {
    stubMatchMedia(false)
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: [
          { id: 1, name: 'A', platform: 'openai' },
          { id: 2, name: 'B', platform: 'anthropic' }
        ],
        rowGroup: (row: { platform: string }) => row.platform,
        rowGroupExpanded: () => false
      },
      slots: {
        'group-header': '<template #group-header="{ groupKey }"><div data-test="group-header">{{ groupKey }}</div></template>',
        'cell-name': '<template #cell-name="{ row }"><span :data-test="\'mobile-row-\' + row.id">{{ row.name }}</span></template>'
      }
    })

    await wrapper.vm.$nextTick()

    expect(wrapper.findAll('[data-test="group-header"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-test^="mobile-row-"]')).toHaveLength(0)
  })

  it('emits row drag events after a row is armed', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: [
          { id: 1, name: 'A' },
          { id: 2, name: 'B' }
        ],
        rowDraggable: true
      }
    })
    await wrapper.vm.$nextTick()

    const rows = wrapper.findAll('tr[data-row-id]')
    const dataTransfer = {
      effectAllowed: '',
      dropEffect: '',
      setData: vi.fn()
    }
    await rows[0].trigger('pointerdown')
    await rows[0].trigger('dragstart', { dataTransfer })
    await rows[1].trigger('dragover', { dataTransfer })
    await rows[1].trigger('drop', { dataTransfer })
    await rows[0].trigger('dragend', { dataTransfer })

    expect(wrapper.emitted('rowDragStart')?.[0]?.[0]).toEqual(expect.objectContaining({ id: 1 }))
    expect(wrapper.emitted('rowDrop')?.[0]?.[0]).toEqual(expect.objectContaining({ id: 2 }))
    expect(wrapper.emitted('rowDragEnd')).toHaveLength(1)
  })
})
