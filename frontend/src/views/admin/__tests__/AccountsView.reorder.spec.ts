import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  listUpstreamGroups,
  renameUpstreamGroup,
  updateUpstreamGroupSortOrders,
  updateSortOrder,
  getBatchTodayStats,
  showError,
  showSuccess
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  listUpstreamGroups: vi.fn(),
  renameUpstreamGroup: vi.fn(),
  updateUpstreamGroupSortOrders: vi.fn(),
  updateSortOrder: vi.fn(),
  getBatchTodayStats: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      listUpstreamGroups,
      renameUpstreamGroup,
      updateUpstreamGroupSortOrders,
      updateSortOrder,
      getBatchTodayStats
    },
    proxies: { getAll: vi.fn().mockResolvedValue([]) },
    groups: { getAll: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess, showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const DataTableStub = {
  props: ['data', 'rowGroup', 'rowGroupExpanded', 'rowDraggable'],
  emits: ['rowDragStart', 'rowDrop', 'rowDragEnd'],
  methods: {
    groupKey(row: any) {
      return this.rowGroup ? this.rowGroup(row) : ''
    },
    isGroupStart(row: any, index: number) {
      return Boolean(this.rowGroup) && (index === 0 || this.groupKey(row) !== this.groupKey(this.data[index - 1]))
    },
    rowsInGroup(row: any) {
      return this.data.filter((item: any) => this.groupKey(item) === this.groupKey(row))
    },
    isExpanded(row: any) {
      return !this.rowGroup || !this.rowGroupExpanded || this.rowGroupExpanded(this.groupKey(row))
    }
  },
  template: `
    <div data-test="data-table" :data-draggable="String(Boolean(rowDraggable))">
      <template v-for="(row, index) in data" :key="row.id">
        <slot
          v-if="isGroupStart(row, index)"
          name="group-header"
          :group-key="groupKey(row)"
          :rows="rowsInGroup(row)"
          :expanded="isExpanded(row)"
        />
        <div v-if="isExpanded(row)" :data-test="'row-' + row.id">
          <span :data-test="'group-' + row.id">{{ rowGroup ? rowGroup(row) : '' }}</span>
          <button :data-test="'drag-' + row.id" @click="$emit('rowDragStart', row, index, {})">start</button>
          <button :data-test="'drop-' + row.id" @click="$emit('rowDrop', row, index, {})">drop</button>
          <slot name="cell-select" :row="row" />
          <slot name="cell-sort_order" :row="row" />
        </div>
      </template>
    </div>
  `
}

const accounts = [
  {
    id: 1,
    name: 'OpenAI A',
    platform: 'openai',
    type: 'apikey',
    upstream_group: 'hi-code',
    status: 'active',
    schedulable: true,
    concurrency: 2,
    priority: 10,
    sort_order: 10,
    credentials: { base_url: 'https://key-owner@api.hi-code.example:443/v1' },
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  },
  {
    id: 2,
    name: 'OpenAI B',
    platform: 'openai',
    type: 'oauth',
    upstream_group: 'hi-code',
    status: 'error',
    schedulable: true,
    concurrency: 3,
    priority: 20,
    sort_order: 20,
    credentials: { base_url: 'https://edge-2.different-domain.example/v1' },
    error_message: 'failed',
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  },
  {
    id: 3,
    name: 'Claude A',
    platform: 'anthropic',
    type: 'oauth',
    upstream_group: 'Anthropic official',
    status: 'active',
    schedulable: true,
    concurrency: 1,
    priority: 30,
    sort_order: 30,
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  }
]

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        AccountTableActions: {
          emits: ['create'],
          template: '<div><button data-test="create-account" @click="$emit(\'create\')">create</button><slot name="after" /></div>'
        },
        AccountTableFilters: true,
        AccountBulkActionsBar: {
          emits: ['set-upstream-group'],
          template: '<button data-test="bulk-set-upstream-group" @click="$emit(\'set-upstream-group\')">set group</button>'
        },
        Pagination: true,
        ConfirmDialog: true,
        HelpTooltip: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: {
          props: ['show', 'upstreamGroups'],
          template: '<div v-if="show" data-test="create-upstream-groups">{{ upstreamGroups.map(group => group.name).join(\',\') }}</div>'
        },
        EditAccountModal: true,
        BulkEditAccountModal: true,
        BulkSetUpstreamGroupModal: {
          props: ['show', 'accountIds', 'upstreamGroups'],
          emits: ['close', 'updated'],
          template: '<div v-if="show" data-test="bulk-set-upstream-group-modal">{{ accountIds.join(\',\') }}:{{ upstreamGroups.map(group => group.name).join(\',\') }}</div>'
        },
        BaseDialog: {
          props: ['show'],
          emits: ['close'],
          template: '<div v-if="show" data-test="base-dialog"><slot /><slot name="footer" /></div>'
        },
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

describe('admin AccountsView upstream grouping and reorder', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    listAccounts.mockResolvedValue({ items: accounts, total: 3, page: 1, page_size: 20, pages: 1 })
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    listUpstreamGroups.mockResolvedValue([
      { id: 2, key: 'hi-code', name: 'hi-code', sort_order: 10, account_count: 2 },
      { id: 1, key: 'anthropic-official', name: 'Anthropic official', sort_order: 20, account_count: 1 }
    ])
    renameUpstreamGroup.mockResolvedValue({ id: 2, key: 'hi-code', name: 'hi-code', sort_order: 10, account_count: 2 })
    updateUpstreamGroupSortOrders.mockResolvedValue({ message: 'ok' })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    updateSortOrder.mockResolvedValue({ message: 'ok' })
  })

  it('requests upstream ordering and keeps explicit upstream groups collapsed by default', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()

    expect(listAccounts.mock.calls.at(-1)?.[2]).toEqual(expect.objectContaining({
      sort_by: 'upstream',
      sort_order: 'asc'
    }))
    expect(wrapper.findAll('[data-test^="row-"]')).toHaveLength(0)

    const hiCodeHeader = wrapper.findAll('button[aria-expanded="false"]')
      .find(button => button.text().includes('hi-code'))
    expect(hiCodeHeader).toBeTruthy()
    await hiCodeHeader!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="group-1"]').text()).toBe('group:2')
    expect(wrapper.get('[data-test="group-2"]').text()).toBe('group:2')
    expect(wrapper.find('[data-test="row-3"]').exists()).toBe(false)
    expect(hiCodeHeader!.attributes('aria-expanded')).toBe('true')

    await hiCodeHeader!.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-test^="row-"]')).toHaveLength(0)
  })

  it('loads all existing upstream groups when opening the create dialog', async () => {
    listUpstreamGroups.mockResolvedValueOnce([
      { id: 2, key: 'hi-code', name: 'hi-code', sort_order: 10, account_count: 4 }
    ])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="create-account"]').trigger('click')
    await flushPromises()

    expect(listUpstreamGroups).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="create-upstream-groups"]').text()).toBe('hi-code')
  })

  it('keeps the server directory order instead of sorting upstream names locally', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()

    const headers = wrapper.findAll('button[aria-expanded="false"]')
    expect(headers.map(header => header.text())).toEqual([
      expect.stringContaining('hi-code'),
      expect.stringContaining('Anthropic official')
    ])
  })

  it('renames and reorders directory-backed upstream groups from their headers', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="rename-upstream-group-2"]').trigger('click')
    await wrapper.get('#rename-upstream-group-name').setValue('hi-code backup')
    await wrapper.get('#rename-upstream-group-form').trigger('submit')
    await flushPromises()

    expect(renameUpstreamGroup).toHaveBeenCalledWith(2, 'hi-code backup')
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.renameUpstreamGroupSuccess')

    await wrapper.get('[data-testid="move-upstream-group-down-2"]').trigger('click')
    await flushPromises()

    expect(updateUpstreamGroupSortOrders).toHaveBeenCalledWith([
      { id: 1, sort_order: 10 },
      { id: 2, sort_order: 20 }
    ])
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.reorderUpstreamGroupsSuccess')
  })

  it('normalizes duplicate sort values when moving a newly created group', async () => {
    listUpstreamGroups.mockResolvedValueOnce([
      { id: 2, key: 'hi-code', name: 'hi-code', sort_order: 0, account_count: 2 },
      { id: 1, key: 'anthropic-official', name: 'Anthropic official', sort_order: 0, account_count: 1 }
    ])
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="move-upstream-group-up-2"]').trigger('click')
    await flushPromises()

    expect(updateUpstreamGroupSortOrders).toHaveBeenCalledWith([
      { id: 2, sort_order: 100 },
      { id: 1, sort_order: 200 }
    ])
  })

  it('opens the batch upstream-group dialog for selected accounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="row-1"] input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-test="bulk-set-upstream-group"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="bulk-set-upstream-group-modal"]').text()).toBe(
      '1:hi-code,Anthropic official'
    )
  })

  it('persists reordered rows using the existing sort slots', async () => {
    const wrapper = mountView()
    await flushPromises()

    const reorderButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.dragSort'))
    expect(reorderButton).toBeTruthy()
    await reorderButton!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="drag-1"]').trigger('click')
    await wrapper.get('[data-test="drop-2"]').trigger('click')
    await flushPromises()

    expect(updateSortOrder).toHaveBeenCalledWith([
      { id: 2, sort_order: 10 },
      { id: 1, sort_order: 20 },
      { id: 3, sort_order: 30 }
    ])
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.reorderSuccess')
  })

  it('blocks dragging an account into a different upstream site', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()
    for (const header of wrapper.findAll('button[aria-expanded="false"]')) {
      await header.trigger('click')
    }
    await flushPromises()
    const reorderButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.dragSort'))
    await reorderButton!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="drag-1"]').trigger('click')
    await wrapper.get('[data-test="drop-3"]').trigger('click')
    await flushPromises()

    expect(updateSortOrder).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.crossUpstreamDragBlocked')
  })

  it('moves by keyboard only within the current upstream site', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button[aria-pressed="false"]').trigger('click')
    await flushPromises()
    const hiCodeHeader = wrapper.findAll('button[aria-expanded="false"]')
      .find(button => button.text().includes('hi-code'))
    await hiCodeHeader!.trigger('click')
    await flushPromises()
    const reorderButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.dragSort'))
    await reorderButton!.trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="row-1"] .account-drag-handle').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()

    expect(updateSortOrder).toHaveBeenCalledWith([
      { id: 2, sort_order: 10 },
      { id: 1, sort_order: 20 }
    ])

    updateSortOrder.mockClear()
    await wrapper.get('[data-test="row-1"] .account-drag-handle').trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    expect(updateSortOrder).not.toHaveBeenCalled()
  })

  it('restores the previous order when persistence fails', async () => {
    updateSortOrder.mockRejectedValueOnce(new Error('network'))
    const wrapper = mountView()
    await flushPromises()

    const reorderButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.dragSort'))
    await reorderButton!.trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="drag-1"]').trigger('click')
    await wrapper.get('[data-test="drop-2"]').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('[data-test^="row-"]').map(row => row.attributes('data-test'))).toEqual([
      'row-1', 'row-2', 'row-3'
    ])
    expect(showError).toHaveBeenCalledWith('admin.accounts.reorderFailed')
  })
})
