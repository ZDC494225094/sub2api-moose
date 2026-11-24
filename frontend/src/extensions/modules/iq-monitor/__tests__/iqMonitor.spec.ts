import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import ModelCardItem from '../ModelCardItem.vue'
import DrawingWall from '../DrawingWall.vue'
import SubmitBox from '../SubmitBox.vue'
import ResultDialog from '../ResultDialog.vue'
import TargetDialog from '../TargetDialog.vue'
import { iqMonitorAPI, iqMonitorAdminAPI, type ModelCard, type Options, type WallView } from '../api'
import { formatElapsed, formatMs, formatRate } from '../format'

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    iqMonitorAPI: { access: vi.fn(), overview: vi.fn(), result: vi.fn(), submissions: vi.fn(), submit: vi.fn() },
    iqMonitorAdminAPI: { result: vi.fn(), models: vi.fn(), efforts: vi.fn(), saveTarget: vi.fn() },
  }
})

const stubs = {
  ModelIcon: true,
  Icon: true,
  BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" data-dialog><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>' },
  DrawingFrame: { props: ['src', 'title'], template: '<iframe data-frame :src="src" sandbox="allow-scripts"></iframe>' },
}

function card(overrides: Partial<ModelCard> = {}): ModelCard {
  return {
    target_id: 1, display_name: 'Sonnet', model: 'claude-sonnet-4-5', group_id: 2, group_name: 'Claude', platform: 'anthropic',
    rate_multiplier: 1.5, reasoning_effort: 'xhigh', sent_effort: 'max', health: 'degraded', health_rate: { passed: 1, total: 2, rate: 50 },
    logic: {
      stat: { passed: 1, total: 2, rate: 50 },
      timeline: [
        { result_id: 11, round_id: 1, status: 'pass', created_at: '2026-11-24T01:00:00Z' },
        { result_id: 12, round_id: 2, status: 'fail', reason: '答案不对', created_at: '2026-11-24T01:30:00Z' },
        { result_id: 13, round_id: 3, status: 'running', created_at: '2026-11-24T02:00:00Z' },
      ],
    },
    drawing: { stat: { passed: 0, total: 0, rate: 0 }, timeline: [] },
    avg_first_token_ms: 1200, avg_total_ms: 8000, ...overrides,
  }
}

describe('iq-monitor formatting', () => {
  it('formats rates, durations and elapsed timers', () => {
    expect(formatRate(0, 0)).toBe('—')
    expect(formatRate(100, 5)).toBe('100%')
    expect(formatRate(83.333, 6)).toBe('83.3%')
    expect(formatMs(850)).toBe('850 ms')
    expect(formatMs(12_340)).toBe('12.3 s')
    expect(formatElapsed('2026-11-24T00:00:00Z', Date.parse('2026-11-24T00:01:05Z'))).toBe('1:05')
  })
})

describe('model card', () => {
  it('pads the timeline, shows the sent effort and opens finished cells only', async () => {
    const wrapper = mount(ModelCardItem, {
      props: { card: card(), logicEnabled: true, drawingEnabled: false, thresholds: { normal: 95, unstable: 80 }, intervalMinutes: 30, now: Date.parse('2026-11-24T02:00:00Z') },
      global: { stubs },
    })
    expect(wrapper.text()).toContain('疑似降智')
    expect(wrapper.text()).toContain('推理 xhigh → max')
    expect(wrapper.text()).toContain('1/2 答对')
    expect(wrapper.text()).not.toContain('绘图测试')
    expect(wrapper.findAll('[role="listitem"]')).toHaveLength(48)
    const buttons = wrapper.findAll('button[role="listitem"]')
    expect(buttons).toHaveLength(2)
    // Newest cells sit at the right end, next to "现在".
    const cells = wrapper.findAll('[role="listitem"]')
    expect(cells[47].classes().join(' ')).toContain('animate-pulse')
    await buttons[1].trigger('mouseenter')
    expect(wrapper.text()).toContain('答案不对')
    await buttons[1].trigger('click')
    expect(wrapper.emitted('open')).toEqual([[12]])
  })
})

describe('drawing wall', () => {
  it('shows elapsed time for drawings in progress and previews for finished ones', () => {
    const wall: WallView = {
      round_id: 3, topic: '一只猫追蝴蝶', topic_source: 'user', submitter: 'h**g', status: 'running', started_at: '2026-11-24T00:00:00Z', finished_at: null,
      prev_round_id: 2, next_round_id: null,
      cards: [
        { result_id: 1, target_id: 1, display_name: 'A', model: 'gpt-5', group_name: 'OpenAI', platform: 'openai', rate_multiplier: 1, reasoning_effort: '', status: 'running', started_at: '2026-11-24T00:00:00Z', total_ms: null, output_tokens: 0 },
        { result_id: 2, target_id: 2, display_name: 'B', model: 'gemini-3-pro', group_name: 'Gemini', platform: 'gemini', rate_multiplier: 0.8, reasoning_effort: '', status: 'pass', started_at: '2026-11-24T00:00:00Z', total_ms: 30_000, output_tokens: 900, preview_url: '/api/v1/iq-monitor/preview/2?exp=1&sig=x' },
      ],
    }
    const wrapper = mount(DrawingWall, { props: { wall, now: Date.parse('2026-11-24T00:00:42Z'), latest: true, switching: false }, global: { stubs } })
    expect(wrapper.text()).toContain('用户投稿')
    expect(wrapper.text()).toContain('h**g')
    expect(wrapper.text()).not.toContain('回到最新')
    expect(wrapper.text()).toContain('42 秒')
    const frames = wrapper.findAll('[data-frame]')
    expect(frames).toHaveLength(1)
    expect(frames[0].attributes('src')).toContain('/preview/2?')
    expect(wrapper.text()).toContain('0.8x')
  })

  it('switches between rounds with the arrows and returns to the latest', async () => {
    const wall: WallView = {
      round_id: 5, topic: '宇航员放风筝', topic_source: 'admin', status: 'done', started_at: '2026-11-24T00:00:00Z', finished_at: '2026-11-24T00:05:00Z',
      prev_round_id: 4, next_round_id: 6, cards: [],
    }
    const wrapper = mount(DrawingWall, { props: { wall, now: 0, latest: false, switching: false }, global: { stubs } })
    await wrapper.find('button[aria-label="上一轮"]').trigger('click')
    await wrapper.find('button[aria-label="下一轮"]').trigger('click')
    const back = wrapper.findAll('button').find(b => b.text() === '回到最新')!
    await back.trigger('click')
    expect(wrapper.emitted('navigate')).toEqual([[4], [6], [null]])

    await wrapper.setProps({ wall: { ...wall, next_round_id: null }, latest: true })
    expect(wrapper.find('button[aria-label="下一轮"]').attributes('disabled')).toBeDefined()
  })
})

describe('submission box', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('shows remaining quota and rejection reasons, and submits trimmed topics', async () => {
    vi.mocked(iqMonitorAPI.submissions).mockResolvedValue({
      enabled: true, quota: 2, used: 1, max_length: 40, day: '2026-11-24',
      items: [{ id: 1, user_id: 7, topic: '某明星跳舞', status: 'rejected', reject_reason: '涉及真实人物', attempts: 1, quota_day: '2026-11-24', round_id: null, created_at: '2026-11-24T01:00:00Z', reviewed_at: null, used_at: null }],
    })
    vi.mocked(iqMonitorAPI.submit).mockResolvedValue({} as never)
    const wrapper = mount(SubmitBox, { global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('今日 1/2')
    expect(wrapper.text()).toContain('原因：涉及真实人物')
    await wrapper.find('input').setValue('  一只鹈鹕骑自行车  ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(iqMonitorAPI.submit).toHaveBeenCalledWith('一只鹈鹕骑自行车')
  })

  it('blocks submission when the quota is used up', async () => {
    vi.mocked(iqMonitorAPI.submissions).mockResolvedValue({ enabled: true, quota: 2, used: 2, max_length: 40, day: '2026-11-24', items: [] })
    const wrapper = mount(SubmitBox, { global: { stubs } })
    await flushPromises()
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('明天再来')
  })
})

describe('result dialog', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows the question, accepted answers and every attempt for a logic result', async () => {
    vi.mocked(iqMonitorAPI.result).mockResolvedValue({
      result: {
        id: 5, round_id: 1, kind: 'logic', group_name: 'Claude', model: 'claude', display_name: 'Claude', reasoning_effort: '', sent_effort: '',
        status: 'retry_pass', reason: '', attempts: 2, request_count: 2, input_tokens: 100, output_tokens: 300, first_token_ms: 900, total_ms: 5000,
        truncated: false, started_at: null, finished_at: null, created_at: '2026-11-24T01:00:00Z',
      },
      question: '1+1=?', answers: ['2'],
      responses: [
        { id: 1, attempt: 1, content: '答案：3', extracted: '3', correct: false, finish_reason: 'stop', truncated: false, error: '', input_tokens: 50, output_tokens: 150, first_token_ms: 900, total_ms: 2500, created_at: '' },
        { id: 2, attempt: 2, content: '答案：2', extracted: '2', correct: true, finish_reason: 'stop', truncated: false, error: '', input_tokens: 50, output_tokens: 150, first_token_ms: 900, total_ms: 2500, created_at: '' },
      ],
    })
    const wrapper = mount(ResultDialog, { props: { resultId: 5 }, global: { stubs } })
    await flushPromises()
    expect(iqMonitorAPI.result).toHaveBeenCalledWith(5)
    const text = wrapper.text()
    expect(text).toContain('重试后正确')
    expect(text).toContain('1+1=?')
    expect(text).toContain('第 1 次回答')
    expect(text).toContain('第 2 次回答')
    expect(text).toContain('提取答案：3')
  })

  it('renders drawings from the signed preview URL in a sandboxed frame', async () => {
    vi.mocked(iqMonitorAdminAPI.result).mockResolvedValue({
      result: {
        id: 6, round_id: 1, kind: 'drawing', group_name: 'G', model: 'm', display_name: 'M', reasoning_effort: '', sent_effort: '',
        status: 'pass', reason: '', attempts: 1, request_count: 1, input_tokens: 10, output_tokens: 10, first_token_ms: null, total_ms: null,
        truncated: false, started_at: null, finished_at: null, created_at: '',
      },
      topic: '猫', responses: [], preview_url: '/api/v1/iq-monitor/preview/6?exp=1&sig=s',
    })
    const wrapper = mount(ResultDialog, { props: { resultId: 6, admin: true }, global: { stubs } })
    await flushPromises()
    expect(iqMonitorAdminAPI.result).toHaveBeenCalledWith(6)
    const frame = wrapper.find('[data-frame]')
    expect(frame.attributes('src')).toBe('/api/v1/iq-monitor/preview/6?exp=1&sig=s')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
  })
})

describe('target dialog', () => {
  beforeEach(() => vi.clearAllMocks())

  it('limits keys to the chosen group and labels the effort the gateway actually sends', async () => {
    vi.mocked(iqMonitorAdminAPI.models).mockResolvedValue({ models: ['claude-opus-4-5'] })
    vi.mocked(iqMonitorAdminAPI.efforts).mockResolvedValue({
      platform: 'anthropic',
      items: [
        { value: 'high', preview: { sent: 'high', denied: false } },
        { value: 'xhigh', preview: { sent: 'max', denied: false } },
        { value: 'max', preview: { sent: '', denied: true } },
      ],
    })
    const options: Options = {
      groups: [{ id: 2, name: 'Claude', platform: 'anthropic', rate_multiplier: 1, status: 'active' }, { id: 3, name: 'Other', platform: 'openai', rate_multiplier: 1, status: 'active' }],
      keys: [{ id: 10, name: 'claude-key', group_ids: [2], status: 'active' }, { id: 11, name: 'openai-key', group_ids: [3], status: 'active' }],
      bots: [],
      probe_user: { id: 1, email: 'probe@example.com', username: '' },
    }
    const wrapper = mount(TargetDialog, { props: { show: true, target: null, options }, global: { stubs } })
    const selects = wrapper.findAll('select')
    await selects[0].setValue('2')
    await flushPromises()
    const keyOptions = selects[1].findAll('option').map(o => o.text())
    expect(keyOptions).toContain('claude-key')
    expect(keyOptions).not.toContain('openai-key')
    expect((selects[1].element as HTMLSelectElement).value).toBe('10')
    await wrapper.find('input[list]').setValue('claude-opus-4-5')
    await flushPromises()
    expect(iqMonitorAdminAPI.efforts).toHaveBeenLastCalledWith(2, 'claude-opus-4-5')
    const effortText = selects[2].findAll('option').map(o => o.text())
    expect(effortText).toContain('xhigh（实际发送 max）')
    expect(effortText).toContain('max（分组策略不允许）')
  })
})
