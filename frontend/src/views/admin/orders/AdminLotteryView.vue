<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card overflow-hidden">
        <div class="border-b border-gray-100 bg-gradient-to-r from-amber-50 via-white to-sky-50 px-6 py-5 dark:border-dark-700 dark:from-amber-950/20 dark:via-dark-900 dark:to-sky-950/20">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.28em] text-amber-500">{{ t('nav.marketingLottery') }}</p>
              <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.marketingLottery') }}</h1>
              <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ t('adminLottery.description') }}</p>
            </div>
            <button class="btn btn-primary" @click="openActivityCreate">{{ t('adminLottery.createActivity') }}</button>
          </div>
        </div>

        <div class="p-6">
          <DataTable
            :columns="activityColumns"
            :data="activities"
            :loading="loadingActivities"
            row-clickable
            :row-class="activityRowClass"
            @row-click="setCurrentActivity"
          >
            <template #cell-name="{ row }">
              <div class="w-full px-1 py-1">
                <div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
                <div v-if="row.description" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ row.description }}</div>
              </div>
            </template>
            <template #cell-status="{ row }">
              <div class="flex items-center gap-2" @click.stop>
                <button
                  type="button"
                  class="relative inline-flex h-6 w-11 items-center rounded-full transition"
                  :class="row.status === 'active' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-600'"
                  :aria-label="t('adminLottery.toggleActivityStatus')"
                  @click="toggleActivityStatus(row)"
                >
                  <span
                    class="inline-block h-5 w-5 rounded-full bg-white shadow transition"
                    :class="row.status === 'active' ? 'translate-x-5' : 'translate-x-0.5'"
                  ></span>
                </button>
                <span class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium" :class="activityStatusClass(row.status)">
                  {{ activityStatusLabel(row.status) }}
                </span>
              </div>
            </template>
            <template #cell-default_draw_times="{ value }">
              <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ value }}</span>
            </template>
            <template #cell-wallet_cost_per_draw="{ value }">
              <span class="text-sm text-gray-800 dark:text-gray-200">¥{{ Number(value).toFixed(2) }}</span>
            </template>
            <template #cell-effective_period="{ row }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatPeriod(row.starts_at, row.ends_at) }}</span>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2" @click.stop>
                <button class="btn btn-secondary btn-sm" @click="openActivityEdit(row)">{{ t('common.edit') }}</button>
                <button class="btn btn-secondary btn-sm" @click="openParticipants(row)">{{ t('adminLottery.viewParticipants') }}</button>
                <button class="btn btn-secondary btn-sm text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20" @click="deleteActivity(row)">{{ t('common.delete') }}</button>
              </div>
            </template>
          </DataTable>
        </div>
      </section>

      <section v-if="currentActivity" class="card p-6">
        <div class="mb-5 flex flex-wrap items-center justify-between gap-4">
          <div>
            <p class="text-xs font-semibold uppercase tracking-[0.28em] text-violet-500">{{ t('adminLottery.prizesForCurrentActivity') }}</p>
            <h2 class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ currentActivity.name }}</h2>
          </div>
          <div class="flex items-center gap-2">
            <a href="/lottery" target="_blank" class="btn btn-secondary btn-sm">{{ t('adminLottery.startLottery') }}</a>
            <button class="btn btn-primary" @click="openPrizeCreate">{{ t('adminLottery.createPrize') }}</button>
          </div>
        </div>
        <DataTable :columns="prizeColumns" :data="prizes" :loading="false">
          <template #cell-name="{ row }">
            <div class="space-y-1">
              <div class="font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ prizeTypeLabel(row.prize_type) }}</div>
            </div>
          </template>
          <template #cell-prize_type="{ value }">
            <span class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium" :class="prizeTypeClass(value)">
              {{ prizeTypeLabel(value) }}
            </span>
          </template>
          <template #cell-stock="{ value }">
            <span class="text-sm text-gray-800 dark:text-gray-200">{{ value }}</span>
          </template>
          <template #cell-remaining_stock="{ value }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>
          <template #cell-status="{ row }">
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="relative inline-flex h-6 w-11 items-center rounded-full transition"
                :class="row.status === 'active' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-600'"
                :aria-label="t('adminLottery.togglePrizeStatus')"
                @click="togglePrizeStatus(row)"
              >
                <span
                  class="inline-block h-5 w-5 rounded-full bg-white shadow transition"
                  :class="row.status === 'active' ? 'translate-x-5' : 'translate-x-0.5'"
                ></span>
              </button>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ prizeStatusLabel(row.status) }}</span>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <button class="btn btn-secondary btn-sm" @click="openPrizeEdit(row)">{{ t('common.edit') }}</button>
          </template>
        </DataTable>
      </section>

      <BaseDialog :show="activityDialogOpen" :title="editingActivityId ? t('common.edit') : t('common.create')" @close="activityDialogOpen = false">
        <form id="lottery-activity-form" class="space-y-4" @submit.prevent="submitActivity">
          <div>
            <label class="input-label">{{ t('adminLottery.name') }}</label>
            <input v-model="activityForm.name" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('adminLottery.descriptionField') }}</label>
            <textarea v-model="activityForm.description" class="input" rows="3"></textarea>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('adminLottery.defaultDrawTimes') }}</label>
              <input v-model.number="activityForm.default_draw_times" class="input" type="number" min="0" />
            </div>
            <div>
              <label class="input-label">{{ t('adminLottery.consumeThresholdAmount') }}</label>
              <input v-model.number="activityForm.consume_threshold_amount" class="input" type="number" min="0" step="0.01" />
            </div>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('adminLottery.walletCostPerDraw') }}</label>
              <input v-model.number="activityForm.wallet_cost_per_draw" class="input" type="number" min="0" step="0.01" />
            </div>
            <div>
              <label class="input-label">{{ t('adminLottery.status') }}</label>
              <Select v-model="activityForm.status" :options="activityStatusOptions" />
            </div>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('adminLottery.startsAt') }}</label>
              <input v-model="activityForm.starts_at" class="input" type="datetime-local" />
            </div>
            <div>
              <label class="input-label">{{ t('adminLottery.endsAt') }}</label>
              <input v-model="activityForm.ends_at" class="input" type="datetime-local" />
            </div>
          </div>
        </form>
        <template #footer>
          <button class="btn btn-secondary" @click="activityDialogOpen = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" form="lottery-activity-form" type="submit">{{ t('common.save') }}</button>
        </template>
      </BaseDialog>

      <BaseDialog :show="prizeDialogOpen" :title="editingPrizeId ? t('common.edit') : t('common.create')" @close="prizeDialogOpen = false">
        <form id="lottery-prize-form" class="space-y-4" @submit.prevent="submitPrize">
          <div>
            <label class="input-label">{{ t('adminLottery.prizeName') }}</label>
            <input v-model="prizeForm.name" class="input" />
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('adminLottery.prizeType') }}</label>
              <Select v-model="prizeForm.prize_type" :options="prizeTypeOptions" />
            </div>
            <div>
              <label class="input-label">{{ t('adminLottery.stock') }}</label>
              <input v-model.number="prizeForm.stock" class="input" type="number" min="0" />
            </div>
            <div v-if="editingPrizeId">
              <label class="input-label">{{ t('adminLottery.remainingStock') }}</label>
              <input v-model.number="prizeForm.remaining_stock" class="input" type="number" min="0" />
            </div>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('adminLottery.balanceAmount') }}</label>
              <input v-model.number="prizeForm.balance_amount" class="input" type="number" min="0" step="0.01" :disabled="prizeForm.prize_type !== 'balance_redeem'" />
            </div>
            <div>
              <label class="input-label">{{ t('adminLottery.couponTemplateId') }}</label>
              <Select v-model="prizeForm.coupon_template_id" :options="couponTemplateOptions" />
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('adminLottery.status') }}</label>
            <Select v-model="prizeForm.status" :options="prizeStatusOptions" />
          </div>
        </form>
        <template #footer>
          <button class="btn btn-secondary" @click="prizeDialogOpen = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" form="lottery-prize-form" type="submit">{{ t('common.save') }}</button>
        </template>
      </BaseDialog>

      <!-- Participants drawer -->
      <Transition enter-active-class="transition-opacity duration-200" enter-from-class="opacity-0" enter-to-class="opacity-100" leave-active-class="transition-opacity duration-150" leave-from-class="opacity-100" leave-to-class="opacity-0">
        <div v-if="participantsOpen" class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 backdrop-blur-sm sm:items-center" @click.self="participantsOpen = false">
          <div class="mx-0 h-[80vh] w-full max-w-4xl overflow-hidden rounded-t-3xl bg-white shadow-2xl dark:bg-dark-900 sm:mx-4 sm:h-auto sm:max-h-[80vh] sm:rounded-2xl">
            <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <p class="text-base font-bold text-gray-900 dark:text-white">{{ t('adminLottery.viewParticipants') }} — {{ participantsActivity?.name }}</p>
              <button type="button" class="rounded-lg p-1 hover:bg-gray-100 dark:hover:bg-dark-700" @click="participantsOpen = false">
                <svg class="h-5 w-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/></svg>
              </button>
            </div>
            <div class="overflow-y-auto" style="max-height: calc(80vh - 65px);">
              <div v-if="participantsLoading" class="py-10 text-center text-sm text-gray-400">...</div>
              <div v-else-if="participantRecords.length === 0" class="py-10 text-center text-sm text-gray-400 dark:text-gray-500">{{ t('adminLottery.noParticipants') }}</div>
              <table v-else class="w-full text-sm">
                <thead class="border-b border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
                  <tr class="text-left text-xs font-medium text-gray-500 dark:text-gray-400">
                    <th class="whitespace-nowrap px-5 py-2">用户邮箱</th>
                    <th class="whitespace-nowrap px-5 py-2">奖品名称</th>
                    <th class="whitespace-nowrap px-5 py-2">奖品类型</th>
                    <th class="whitespace-nowrap px-5 py-2">奖励凭证</th>
                    <th class="whitespace-nowrap px-5 py-2">抽奖来源</th>
                    <th class="whitespace-nowrap px-5 py-2">钱包扣费</th>
                    <th class="whitespace-nowrap px-5 py-2">抽奖时间</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="rec in participantRecords" :key="rec.id" class="border-b border-gray-50 hover:bg-gray-50/60 dark:border-dark-800 dark:hover:bg-dark-800/50">
                    <td class="px-5 py-2.5 text-gray-700 dark:text-gray-300">{{ rec.user_email || rec.user_id }}</td>
                    <td class="whitespace-nowrap px-5 py-2.5 font-medium text-gray-900 dark:text-white">{{ rec.prize_name }}</td>
                    <td class="whitespace-nowrap px-5 py-2.5">
                      <span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="prizeTypeClass(rec.prize_type)">{{ prizeTypeLabel(rec.prize_type) }}</span>
                    </td>
                    <td class="px-5 py-2.5 font-mono text-xs text-gray-500 dark:text-gray-400">{{ rec.reward_reference || '—' }}</td>
                    <td class="whitespace-nowrap px-5 py-2.5 text-gray-600 dark:text-gray-300">{{ rec.chance_source === 'wallet' ? '自费抽奖' : '免费次数' }}</td>
                    <td class="whitespace-nowrap px-5 py-2.5 text-gray-500 dark:text-gray-400">{{ rec.chance_source === 'wallet' ? `¥${rec.wallet_amount.toFixed(2)}` : '—' }}</td>
                    <td class="whitespace-nowrap px-5 py-2.5 text-gray-400">{{ rec.created_at.replace('T', ' ').slice(0, 16) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </Transition>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { LotteryActivity, LotteryPrize, CouponTemplate, LotteryDrawRecord } from '@/types/payment'

const { t } = useI18n()
const appStore = useAppStore()

const activities = ref<LotteryActivity[]>([])
const prizes = ref<LotteryPrize[]>([])
const couponTemplates = ref<CouponTemplate[]>([])
const currentActivity = ref<LotteryActivity | null>(null)
const loadingActivities = ref(false)

const activityDialogOpen = ref(false)
const prizeDialogOpen = ref(false)
const editingActivityId = ref<number | null>(null)
const editingPrizeId = ref<number | null>(null)

const activityForm = reactive({
  name: '',
  description: '',
  default_draw_times: 0,
  consume_threshold_amount: 0,
  wallet_cost_per_draw: 0,
  status: 'draft' as LotteryActivity['status'],
  starts_at: '',
  ends_at: '',
})

const prizeForm = reactive({
  name: '',
  prize_type: 'thanks' as LotteryPrize['prize_type'],
  stock: 0,
  remaining_stock: 0,
  balance_amount: 0,
  coupon_template_id: 0,
  status: 'active' as LotteryPrize['status'],
})

const activityColumns = computed(() => [
  { key: 'name', label: t('adminLottery.name') },
  { key: 'status', label: t('adminLottery.status') },
  { key: 'default_draw_times', label: t('adminLottery.defaultDrawTimes') },
  { key: 'wallet_cost_per_draw', label: t('adminLottery.walletCostPerDraw') },
  { key: 'effective_period', label: t('adminLottery.effectivePeriod') },
  { key: 'actions', label: t('common.actions') },
])

const prizeColumns = computed(() => [
  { key: 'name', label: t('adminLottery.prizeName') },
  { key: 'prize_type', label: t('adminLottery.prizeType') },
  { key: 'stock', label: t('adminLottery.stock') },
  { key: 'remaining_stock', label: t('adminLottery.remainingStock') },
  { key: 'status', label: t('adminLottery.status') },
  { key: 'actions', label: t('common.actions') },
])

const activityStatusOptions = computed(() => [
  { value: 'draft', label: t('adminLottery.statusDraft') },
  { value: 'active', label: t('adminLottery.statusActive') },
  { value: 'inactive', label: t('adminLottery.statusInactive') },
  { value: 'ended', label: t('adminLottery.statusEnded') },
])

const prizeStatusOptions = computed(() => [
  { value: 'active', label: t('common.enabled') },
  { value: 'inactive', label: t('common.disabled') },
])

const prizeTypeOptions = computed(() => [
  { value: 'thanks', label: t('adminLottery.prizeThanks') },
  { value: 'balance_redeem', label: t('adminLottery.prizeBalance') },
  { value: 'coupon', label: t('adminLottery.prizeCoupon') },
])

const couponTemplateOptions = computed(() => [
  { value: 0, label: t('adminLottery.selectCouponTemplate') },
  ...couponTemplates.value.map((item) => ({
    value: item.id,
    label: `${item.name} / -¥${item.discount_amount.toFixed(2)} / ${scopeLabel(item.scope)}`,
  })),
])

async function loadActivities() {
  loadingActivities.value = true
  try {
    const selectedID = currentActivity.value?.id
    const response = await adminAPI.payment.getLotteryActivities({ page: 1, page_size: 100 })
    activities.value = response.data.items || []
    const selected = selectedID ? activities.value.find((item) => item.id === selectedID) : null
    if (selected) {
      currentActivity.value = selected
    } else if (!currentActivity.value && activities.value.length > 0) {
      currentActivity.value = activities.value[0]
    }
  } catch (error) {
    console.error(error)
    appStore.showError(t('adminLottery.loadFailed'))
  } finally {
    loadingActivities.value = false
  }
}

async function loadCouponTemplates() {
  try {
    const response = await adminAPI.payment.getCouponTemplates({ page: 1, page_size: 200, status: 'active' })
    couponTemplates.value = response.data.items || []
  } catch (error) {
    console.error(error)
  }
}

function setCurrentActivity(activity: LotteryActivity) {
  currentActivity.value = activity
}

function activityRowClass(activity: LotteryActivity) {
  if (currentActivity.value?.id !== activity.id) return ''
  return 'bg-amber-50/80 ring-1 ring-amber-200 dark:bg-amber-950/20 dark:ring-amber-800/40'
}

watch(currentActivity, (activity) => {
  prizes.value = activity ? (((activity as LotteryActivity & { prizes?: LotteryPrize[] }).prizes) || []).slice() : []
}, { immediate: true })

function openActivityCreate() {
  editingActivityId.value = null
  activityForm.name = ''
  activityForm.description = ''
  activityForm.default_draw_times = 0
  activityForm.consume_threshold_amount = 0
  activityForm.wallet_cost_per_draw = 0
  activityForm.status = 'draft'
  activityForm.starts_at = ''
  activityForm.ends_at = ''
  activityDialogOpen.value = true
}

function openActivityEdit(activity: LotteryActivity) {
  editingActivityId.value = activity.id
  activityForm.name = activity.name
  activityForm.description = activity.description
  activityForm.default_draw_times = activity.default_draw_times
  activityForm.consume_threshold_amount = activity.consume_threshold_amount
  activityForm.wallet_cost_per_draw = activity.wallet_cost_per_draw
  activityForm.status = activity.status
  activityForm.starts_at = activity.starts_at ? toLocalDatetime(activity.starts_at) : ''
  activityForm.ends_at = activity.ends_at ? toLocalDatetime(activity.ends_at) : ''
  activityDialogOpen.value = true
}

async function submitActivity() {
  try {
    const wasEditingID = editingActivityId.value
    const payload = {
      ...activityForm,
      starts_at: activityForm.starts_at ? new Date(activityForm.starts_at).toISOString() : undefined,
      ends_at: activityForm.ends_at ? new Date(activityForm.ends_at).toISOString() : undefined,
    }
    let saved: LotteryActivity
    if (editingActivityId.value) {
      const response = await adminAPI.payment.updateLotteryActivity(editingActivityId.value, payload)
      saved = response.data
    } else {
      const response = await adminAPI.payment.createLotteryActivity(payload)
      saved = response.data
    }
    activityDialogOpen.value = false
    currentActivity.value = wasEditingID ? { ...currentActivity.value, ...saved } as LotteryActivity : saved
    await loadActivities()
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error))
  }
}

function openPrizeCreate() {
  if (!currentActivity.value) return
  editingPrizeId.value = null
  prizeForm.name = ''
  prizeForm.prize_type = 'thanks'
  prizeForm.stock = 0
  prizeForm.balance_amount = 0
  prizeForm.coupon_template_id = 0
  prizeForm.status = 'active'
  prizeDialogOpen.value = true
}

function openPrizeEdit(prize: LotteryPrize) {
  editingPrizeId.value = prize.id
  prizeForm.name = prize.name
  prizeForm.prize_type = prize.prize_type
  prizeForm.stock = prize.stock
  prizeForm.remaining_stock = prize.remaining_stock ?? prize.stock
  prizeForm.balance_amount = prize.balance_amount || 0
  prizeForm.coupon_template_id = prize.coupon_template_id || 0
  prizeForm.status = prize.status
  prizeDialogOpen.value = true
}

async function submitPrize() {
  if (!currentActivity.value) return
  try {
    const payload = {
      activity_id: currentActivity.value.id,
      name: prizeForm.name,
      prize_type: prizeForm.prize_type,
      stock: prizeForm.stock,
      remaining_stock: editingPrizeId.value ? prizeForm.remaining_stock : undefined,
      balance_amount: prizeForm.prize_type === 'balance_redeem' ? prizeForm.balance_amount : undefined,
      coupon_template_id: prizeForm.prize_type === 'coupon' && prizeForm.coupon_template_id > 0 ? prizeForm.coupon_template_id : undefined,
      status: prizeForm.status,
    }
    let saved: LotteryPrize
    if (editingPrizeId.value) {
      const response = await adminAPI.payment.updateLotteryPrize(editingPrizeId.value, payload)
      saved = response.data
    } else {
      const response = await adminAPI.payment.createLotteryPrize(payload)
      saved = response.data
    }
    prizeDialogOpen.value = false
    syncPrize(saved)
    await loadActivities()
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error))
  }
}

const participantsOpen = ref(false)
const participantsLoading = ref(false)
const participantRecords = ref<LotteryDrawRecord & { user_email?: string }[]>([])
const participantsActivity = ref<LotteryActivity | null>(null)

async function openParticipants(activity: LotteryActivity) {
  participantsActivity.value = activity
  participantsOpen.value = true
  participantsLoading.value = true
  try {
    const r = await adminAPI.payment.getLotteryDrawRecords(activity.id, { page_size: 200 })
    participantRecords.value = (r.data.items || []) as (LotteryDrawRecord & { user_email?: string })[]
  } catch (e) {
    console.error(e)
  } finally {
    participantsLoading.value = false
  }
}

async function deleteActivity(activity: LotteryActivity) {
  if (!confirm(t('adminLottery.deleteActivityConfirm'))) return
  try {
    await adminAPI.payment.deleteLotteryActivity(activity.id)
    appStore.showSuccess(t('adminLottery.deleteActivitySuccess'))
    if (currentActivity.value?.id === activity.id) currentActivity.value = null
    await loadActivities()
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error) || t('adminLottery.deleteActivityFailed'))
  }
}

async function toggleActivityStatus(activity: LotteryActivity) {
  const nextStatus: LotteryActivity['status'] = activity.status === 'active' ? 'inactive' : 'active'
  try {
    const response = await adminAPI.payment.updateLotteryActivity(activity.id, { status: nextStatus })
    const updated = response.data
    Object.assign(activity, updated)
    if (currentActivity.value?.id === activity.id) {
      currentActivity.value = { ...currentActivity.value, ...updated }
    }
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error))
  }
}

async function togglePrizeStatus(prize: LotteryPrize) {
  const nextStatus: LotteryPrize['status'] = prize.status === 'active' ? 'inactive' : 'active'
  try {
    const response = await adminAPI.payment.updateLotteryPrize(prize.id, { status: nextStatus })
    syncPrize(response.data)
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error))
  }
}

function syncPrize(saved: LotteryPrize) {
  const index = prizes.value.findIndex((item) => item.id === saved.id)
  if (index >= 0) {
    prizes.value.splice(index, 1, saved)
  } else {
    prizes.value.unshift(saved)
  }
  if (currentActivity.value) {
    currentActivity.value = {
      ...currentActivity.value,
      prizes: prizes.value.slice(),
    } as LotteryActivity
  }
}

function activityStatusLabel(value: LotteryActivity['status']) {
  switch (value) {
    case 'active':
      return t('adminLottery.statusActive')
    case 'inactive':
      return t('adminLottery.statusInactive')
    case 'ended':
      return t('adminLottery.statusEnded')
    default:
      return t('adminLottery.statusDraft')
  }
}

function activityStatusClass(value: LotteryActivity['status']) {
  switch (value) {
    case 'active':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
    case 'inactive':
      return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
    case 'ended':
      return 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
    default:
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
  }
}

function prizeTypeLabel(value: LotteryPrize['prize_type']) {
  switch (value) {
    case 'balance_redeem':
      return t('adminLottery.prizeBalance')
    case 'coupon':
      return t('adminLottery.prizeCoupon')
    default:
      return t('adminLottery.prizeThanks')
  }
}

function prizeTypeClass(value: LotteryPrize['prize_type']) {
  switch (value) {
    case 'balance_redeem':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200'
    case 'coupon':
      return 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200'
    default:
      return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
  }
}

function prizeStatusLabel(value: LotteryPrize['status']) {
  return value === 'active' ? t('common.enabled') : t('common.disabled')
}

function scopeLabel(scope: CouponTemplate['scope']) {
  switch (scope) {
    case 'balance':
      return t('adminCoupons.scopeBalance')
    case 'subscription':
      return t('adminCoupons.scopeSubscription')
    default:
      return t('adminCoupons.scopeUniversal')
  }
}

function toLocalDatetime(value: string) {
  const date = new Date(value)
  const pad = (num: number) => String(num).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function formatPeriod(startsAt?: string | null, endsAt?: string | null) {
  const fmt = (s: string) => new Date(s).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).replace(/\//g, '-')
  if (!startsAt && !endsAt) return t('adminLottery.noLimit')
  if (startsAt && endsAt) return `${fmt(startsAt)} ~ ${fmt(endsAt)}`
  if (startsAt) return `${t('adminLottery.startsAt')}: ${fmt(startsAt)}`
  return `${t('adminLottery.endsAt')}: ${fmt(String(endsAt))}`
}

function errorMessage(error: any) {
  return error?.message || error?.response?.data?.detail || error?.response?.data?.message || t('common.error')
}

onMounted(loadActivities)
onMounted(loadCouponTemplates)
</script>
