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

const stubMobileMatchMedia = () => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
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
      },
      slots: {
        'header-name': '<span data-test="custom-name-header">Name</span>'
      }
    })

    await wrapper.vm.$nextTick()

    const nameHeader = wrapper.findAll('th')[0]
    expect(nameHeader.find('[data-test="custom-name-header"]').exists()).toBe(true)
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

  it('renders every row with no virtual padding spacer for small datasets (virtualization off)', async () => {
    const data = Array.from({ length: 8 }, (_, i) => ({ id: i + 1, name: `Row ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data
      }
    })

    await wrapper.vm.$nextTick()

    expect((wrapper.vm as any).shouldVirtualize).toBe(false)
    expect(wrapper.findAll('tbody tr[data-index]')).toHaveLength(data.length)
    expect(wrapper.findAll('tbody tr[aria-hidden="true"]')).toHaveLength(0)
  })

  it('switches to windowed rendering once row count exceeds virtualizeThreshold', async () => {
    const data = Array.from({ length: 12 }, (_, i) => ({ id: i + 1, name: `Row ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data,
        virtualizeThreshold: 3
      }
    })

    await wrapper.vm.$nextTick()

    expect((wrapper.vm as any).shouldVirtualize).toBe(true)
    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    expect(instance.options.count).toBe(data.length)
  })

  it('keys the virtualizer size cache by row identity, not index (avoids stale heights on sort/filter)', async () => {
    const data = Array.from({ length: 12 }, (_, i) => ({ id: 100 + i, name: `Row ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data,
        rowKey: 'id',
        virtualizeThreshold: 3
      }
    })

    await wrapper.vm.$nextTick()

    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    expect(instance.options.getItemKey(0)).toBe(100)
    expect(instance.options.getItemKey(5)).toBe(105)
  })

  it('clears stale row and element caches when pagination replaces the row ID set', async () => {
    const firstPage = Array.from({ length: 100 }, (_, i) => ({ id: i + 1, name: `First ${i + 1}` }))
    const secondPage = Array.from({ length: 100 }, (_, i) => ({ id: i + 101, name: `Second ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: firstPage,
        rowKey: 'id',
        virtualizeThreshold: 1
      }
    })

    await wrapper.vm.$nextTick()

    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    const firstPageIDs = firstPage.map(row => row.id)
    ;(instance as any).itemSizeCache = new Map(firstPageIDs.map(id => [id, 156]))
    instance.elementsCache.clear()
    for (const id of firstPageIDs) {
      instance.elementsCache.set(id, document.createElement('tr'))
    }
    const measureElementSpy = vi.spyOn(instance, 'measureElement')

    await wrapper.setProps({ data: secondPage })
    await wrapper.vm.$nextTick()

    const sizeCache = (instance as any).itemSizeCache as Map<number, number>
    expect(sizeCache.size).toBeLessThanOrEqual(secondPage.length)
    expect(instance.elementsCache.size).toBeLessThanOrEqual(secondPage.length)
    expect(firstPageIDs.some(id => sizeCache.has(id))).toBe(false)
    expect(firstPageIDs.some(id => instance.elementsCache.has(id))).toBe(false)
    expect(measureElementSpy.mock.calls.some(([node]) => node === null)).toBe(true)
  })

  it('clears stale caches when equal-length pages replace rows without stable keys', async () => {
    const firstPage = Array.from({ length: 12 }, (_, i) => ({ name: `First ${i + 1}` }))
    const secondPage = Array.from({ length: 12 }, (_, i) => ({ name: `Second ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: firstPage,
        virtualizeThreshold: 1
      }
    })

    await wrapper.vm.$nextTick()

    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    const measureElementSpy = vi.spyOn(instance, 'measureElement')

    await wrapper.setProps({ data: secondPage })
    await wrapper.vm.$nextTick()

    expect(measureElementSpy.mock.calls.some(([node]) => node === null)).toBe(true)
  })

  it('conservatively clears caches when duplicate row-key multiplicity changes', async () => {
    const firstPage = [
      { id: 1, name: 'First A' },
      { id: 1, name: 'First B' },
      { id: 2, name: 'First C' }
    ]
    const secondPage = [
      { id: 1, name: 'Second A' },
      { id: 2, name: 'Second B' },
      { id: 2, name: 'Second C' }
    ]
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: firstPage,
        rowKey: 'id',
        virtualizeThreshold: 1
      }
    })

    await wrapper.vm.$nextTick()

    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    const measureElementSpy = vi.spyOn(instance, 'measureElement')

    await wrapper.setProps({ data: secondPage })
    await wrapper.vm.$nextTick()

    expect(measureElementSpy.mock.calls.some(([node]) => node === null)).toBe(true)
  })

  it('preserves cache when rows without stable keys only reorder the same objects', async () => {
    const data = Array.from({ length: 12 }, (_, i) => ({ name: `Row ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data,
        virtualizeThreshold: 1
      }
    })

    await wrapper.vm.$nextTick()

    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    const measureSpy = vi.spyOn(instance, 'measure')

    await wrapper.setProps({ data: [...data].reverse() })
    await wrapper.vm.$nextTick()

    expect(measureSpy).not.toHaveBeenCalled()
  })

  it('preserves stable row height cache when the same row IDs are only reordered', async () => {
    const data = Array.from({ length: 100 }, (_, i) => ({ id: i + 1, name: `Row ${i + 1}` }))
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data,
        rowKey: 'id',
        virtualizeThreshold: 1
      }
    })

    await wrapper.vm.$nextTick()

    const exposed = (wrapper.vm as any).virtualizer
    const instance = exposed?.value ?? exposed
    ;(instance as any).itemSizeCache = new Map(data.map(row => [row.id, 156]))
    const measureSpy = vi.spyOn(instance, 'measure')

    await wrapper.setProps({ data: [...data].reverse() })
    await wrapper.vm.$nextTick()

    const sizeCache = (instance as any).itemSizeCache as Map<number, number>
    expect(measureSpy).not.toHaveBeenCalled()
    expect(sizeCache.size).toBe(100)
  })

  it('emits controlled current-page selection while preserving off-page keys', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: [
          { id: 1, name: 'One' },
          { id: 2, name: 'Two' }
        ],
        rowKey: 'id',
        selectable: true,
        selectedKeys: [99]
      }
    })

    await wrapper.get('[data-test="select-all"]').setValue(true)

    const selectedAll = wrapper.emitted('update:selectedKeys')?.at(-1)?.[0]
    expect(selectedAll).toEqual([99, 1, 2])

    await wrapper.setProps({ selectedKeys: selectedAll as number[] })
    const rowCheckboxes = wrapper.findAll<HTMLInputElement>('[data-test="select-row"]')
    expect(rowCheckboxes.every((checkbox) => checkbox.element.checked)).toBe(true)

    await rowCheckboxes[0].setValue(false)

    expect(wrapper.emitted('update:selectedKeys')?.at(-1)?.[0]).toEqual([99, 2])
    expect(wrapper.emitted('selectionChange')?.at(-1)?.[0]).toEqual([99, 2])
  })

  it('offers current-page select all in the mobile card layout', async () => {
    stubMobileMatchMedia()
    const wrapper = mount(DataTable, {
      props: {
        columns: [{ key: 'name', label: 'Name' }],
        data: [
          { id: 1, name: 'One' },
          { id: 2, name: 'Two' }
        ],
        rowKey: 'id',
        selectable: true,
        selectedKeys: [99]
      }
    })

    await wrapper.get('[data-test="select-all-mobile"]').setValue(true)

    expect(wrapper.emitted('update:selectedKeys')?.at(-1)?.[0]).toEqual([99, 1, 2])
  })
})
