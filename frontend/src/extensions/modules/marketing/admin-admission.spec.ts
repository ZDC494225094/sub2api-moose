import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { CouponTemplate, LotteryActivity, LotteryPrize } from '@/types/payment'
import AdminCouponTemplatesView from './AdminCouponTemplatesView.vue'
import AdminLotteryView from './AdminLotteryView.vue'
import { adminMarketingAPI } from './api'

const state = vi.hoisted(() => ({ allowed: null as unknown }))
vi.mock('./admission', () => ({ useMarketingAdmission: () => state.allowed }))
vi.mock('./api', () => ({ adminMarketingAPI: {
  getCouponTemplates: vi.fn(), createCouponTemplate: vi.fn(), updateCouponTemplate: vi.fn(),
  getLotteryActivities: vi.fn(), createLotteryActivity: vi.fn(), updateLotteryActivity: vi.fn(),
  deleteLotteryActivity: vi.fn(), createLotteryPrize: vi.fn(), updateLotteryPrize: vi.fn(), getLotteryDrawRecords: vi.fn(),
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh-CN' } }) }))
const writes = ['createCouponTemplate', 'updateCouponTemplate', 'createLotteryActivity', 'updateLotteryActivity', 'deleteLotteryActivity', 'createLotteryPrize', 'updateLotteryPrize'] as const
const wrappers: VueWrapper[] = []
let allowed = ref(true)
let coupon: CouponTemplate
let activity: LotteryActivity
let prize: LotteryPrize
const stubs = {
  AppLayout: { template: '<main><slot /></main>' },
  BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /><slot name="footer" /></section>' },
  Select: true,
}
interface CouponState {
  openCreate(): void; openEdit(item: CouponTemplate): void; submit(): Promise<void>; toggleStatus(item: CouponTemplate): Promise<void>
}
interface LotteryState {
  openActivityCreate(): void; openActivityEdit(item: LotteryActivity): void; submitActivity(): Promise<void>
  openPrizeCreate(): void; openPrizeEdit(item: LotteryPrize): void; submitPrize(): Promise<void>
  deleteActivity(item: LotteryActivity): Promise<void>; toggleActivityStatus(item: LotteryActivity): Promise<void>; togglePrizeStatus(item: LotteryPrize): Promise<void>
  openParticipants(item: LotteryActivity): Promise<void>
}
function setup<T>(view: VueWrapper): T { return (view.vm.$ as unknown as { setupState: T }).setupState }
function noWrites() { writes.forEach(name => expect(adminMarketingAPI[name], name).not.toHaveBeenCalled()) }
function button(view: VueWrapper, label: string) { return view.findAll('button').find(b => b.text() === label)! }
async function render(kind: 'coupon' | 'lottery') {
  const view = mount(kind === 'coupon' ? AdminCouponTemplatesView : AdminLotteryView, { global: { stubs } })
  wrappers.push(view)
  await flushPromises()
  return view
}
beforeEach(() => {
  vi.clearAllMocks()
  allowed = ref(true); state.allowed = allowed
  vi.stubGlobal('confirm', vi.fn(() => true))
  coupon = { id: 7, name: 'Historical coupon', description: 'Retained template', scope: 'universal', discount_amount: 2, threshold_amount: 0, valid_days: 7, status: 'active', notes: '', created_at: '', updated_at: '' }
  prize = { id: 9, activity_id: 11, name: 'Historical prize', prize_type: 'thanks', stock: 8, remaining_stock: 6, display_order: 0, status: 'active', created_at: '', updated_at: '' }
  activity = { id: 11, name: 'Historical activity', description: '', status: 'active', default_draw_times: 3, consume_threshold_amount: 0, wallet_cost_per_draw: 5, sort_order: 0, created_at: '', updated_at: '', prizes: [prize] }
  vi.mocked(adminMarketingAPI.getCouponTemplates).mockResolvedValue({ data: { items: [coupon] } } as never)
  vi.mocked(adminMarketingAPI.getLotteryActivities).mockResolvedValue({ data: { items: [activity] } } as never)
  vi.mocked(adminMarketingAPI.getLotteryDrawRecords).mockResolvedValue({ data: { items: [{ id: 21, user_email: 'history@example.test', prize_name: 'Retained reward', prize_type: 'thanks', wallet_paid_amount: 0, created_at: '2026-09-25' }] } } as never)
  vi.mocked(adminMarketingAPI.createCouponTemplate).mockResolvedValue({ data: coupon } as never)
  vi.mocked(adminMarketingAPI.updateCouponTemplate).mockResolvedValue({ data: coupon } as never)
  vi.mocked(adminMarketingAPI.createLotteryActivity).mockResolvedValue({ data: activity } as never)
  vi.mocked(adminMarketingAPI.updateLotteryActivity).mockResolvedValue({ data: activity } as never)
  vi.mocked(adminMarketingAPI.createLotteryPrize).mockResolvedValue({ data: prize } as never)
  vi.mocked(adminMarketingAPI.updateLotteryPrize).mockResolvedValue({ data: prize } as never)
  vi.mocked(adminMarketingAPI.deleteLotteryActivity).mockResolvedValue({} as never)
})
afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.unstubAllGlobals() })

describe('marketing admin read-only admission', () => {
  it('keeps coupon rows and refresh readable, disables controls and rejects direct stale handlers', async () => {
    allowed.value = false
    const view = await render('coupon'), vm = setup<CouponState>(view)
    expect(view.text()).toContain('Historical coupon')
    expect(view.get('[data-testid="marketing-readonly"]').text()).toContain('仅供查询')
    expect(button(view, 'adminCoupons.create').attributes('disabled')).toBeDefined()
    expect(button(view, 'common.edit').attributes('disabled')).toBeDefined()
    expect(view.get('[aria-label="adminCoupons.toggleStatus"]').attributes('disabled')).toBeDefined()
    vm.openCreate(); vm.openEdit(coupon); await vm.submit(); await vm.toggleStatus(coupon)
    await nextTick()
    expect(view.find('#coupon-template-form').exists()).toBe(false)
    noWrites()
    await button(view, 'common.refresh').trigger('click'); await flushPromises()
    expect(adminMarketingAPI.getCouponTemplates).toHaveBeenCalledTimes(2)
    expect(coupon.status).toBe('active')
  })
  it.each(['create', 'edit'] as const)('closes a coupon %s form on disable and does not automatically resubmit on re-enable', async mode => {
    const view = await render('coupon'), vm = setup<CouponState>(view)
    if (mode === 'create') vm.openCreate(); else vm.openEdit(coupon)
    await nextTick(); expect(view.find('#coupon-template-form').exists()).toBe(true)
    allowed.value = false
    await vm.submit(); await nextTick()
    expect(view.find('#coupon-template-form').exists()).toBe(false)
    allowed.value = true; await nextTick(); noWrites()
    expect(view.find('#coupon-template-form').exists()).toBe(false)
    if (mode === 'create') vm.openCreate(); else vm.openEdit(coupon)
    await vm.submit()
    expect(adminMarketingAPI[mode === 'create' ? 'createCouponTemplate' : 'updateCouponTemplate']).toHaveBeenCalledTimes(1)
  })
  it('keeps activity, prize and participant history readable while rejecting all nine write handlers', async () => {
    allowed.value = false
    const view = await render('lottery'), vm = setup<LotteryState>(view)
    expect(view.text()).toContain('Historical activity')
    expect(view.text()).toContain('Historical prize')
    for (const label of ['adminLottery.createActivity', 'adminLottery.createPrize', 'common.delete']) expect(button(view, label).attributes('disabled')).toBeDefined()
    for (const el of view.findAll('button').filter(b => b.text() === 'common.edit')) expect(el.attributes('disabled')).toBeDefined()
    for (const label of ['adminLottery.toggleActivityStatus', 'adminLottery.togglePrizeStatus']) expect(view.get(`[aria-label="${label}"]`).attributes('disabled')).toBeDefined()
    vm.openActivityCreate(); vm.openActivityEdit(activity); await vm.submitActivity()
    vm.openPrizeCreate(); vm.openPrizeEdit(prize); await vm.submitPrize()
    await vm.deleteActivity(activity); await vm.toggleActivityStatus(activity); await vm.togglePrizeStatus(prize)
    noWrites(); expect(confirm).not.toHaveBeenCalled()
    await button(view, 'adminLottery.viewParticipants').trigger('click'); await flushPromises()
    expect(adminMarketingAPI.getLotteryDrawRecords).toHaveBeenCalledWith(11, { page_size: 200 })
    expect(view.text()).toContain('history@example.test')
    expect(view.text()).toContain('Retained reward')
    expect(activity.status).toBe('active'); expect(prize.remaining_stock).toBe(6)
  })
  it.each(['activity-create', 'activity-edit', 'prize-create', 'prize-edit'] as const)('closes %s before stale submit, preserves participant drawer and supports a fresh enabled write', async mode => {
    const view = await render('lottery'), vm = setup<LotteryState>(view)
    const isActivity = mode.startsWith('activity'), edit = mode.endsWith('edit')
    const open = () => { if (isActivity) { if (edit) vm.openActivityEdit(activity); else vm.openActivityCreate() } else { if (edit) vm.openPrizeEdit(prize); else vm.openPrizeCreate() } }
    const submit = () => isActivity ? vm.submitActivity() : vm.submitPrize()
    const selector = isActivity ? '#lottery-activity-form' : '#lottery-prize-form'
    open(); await nextTick(); expect(view.find(selector).exists()).toBe(true)
    await vm.openParticipants(activity)
    allowed.value = false; await submit(); await nextTick()
    expect(view.find(selector).exists()).toBe(false)
    expect(view.text()).toContain('history@example.test'); noWrites()
    allowed.value = true; await nextTick()
    expect(view.find(selector).exists()).toBe(false); noWrites()
    open(); await submit()
    const api = isActivity ? (edit ? 'updateLotteryActivity' : 'createLotteryActivity') : (edit ? 'updateLotteryPrize' : 'createLotteryPrize')
    expect(adminMarketingAPI[api]).toHaveBeenCalledTimes(1)
  })
  it('rechecks admission after the delete confirmation returns', async () => {
    const view = await render('lottery'), vm = setup<LotteryState>(view)
    vi.mocked(confirm).mockImplementation(() => { allowed.value = false; return true })
    await vm.deleteActivity(activity); noWrites()
    expect(view.text()).toContain('Historical activity')
  })
  it('retains enabled status changes and confirmed deletion', async () => {
    const couponView = await render('coupon')
    await setup<CouponState>(couponView).toggleStatus(coupon)
    expect(adminMarketingAPI.updateCouponTemplate).toHaveBeenCalledWith(7, { status: 'disabled' })
    const view = await render('lottery'), vm = setup<LotteryState>(view)
    await vm.toggleActivityStatus(activity); await vm.togglePrizeStatus(prize); await vm.deleteActivity(activity)
    expect(adminMarketingAPI.updateLotteryActivity).toHaveBeenCalledWith(11, { status: 'inactive' })
    expect(adminMarketingAPI.updateLotteryPrize).toHaveBeenCalledWith(9, { status: 'inactive' })
    expect(adminMarketingAPI.deleteLotteryActivity).toHaveBeenCalledWith(11)
  })
})
