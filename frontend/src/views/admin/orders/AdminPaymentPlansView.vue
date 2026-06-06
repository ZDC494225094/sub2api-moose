<template>
  <AppLayout>
    <div class="space-y-4">
      <!-- Actions -->
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="flex flex-wrap items-end gap-2">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('payment.admin.bulkIncreasePurchaseCount') }}</label>
            <input
              v-model.number="bulkIncreaseAmount"
              type="number"
              min="1"
              step="1"
              class="input w-36"
              :placeholder="t('payment.admin.bulkIncreaseAmountPlaceholder')"
            />
          </div>
          <button
            @click="openBulkIncreaseDialog"
            :disabled="plansLoading || bulkIncreasing || plans.length === 0"
            class="btn btn-secondary"
          >
            {{ t('payment.admin.bulkIncreaseApply') }}
          </button>
        </div>
        <div class="flex items-center justify-end gap-2">
          <button @click="loadPlans" :disabled="plansLoading" class="btn btn-secondary" :title="t('common.refresh')">
            <Icon name="refresh" size="md" :class="plansLoading ? 'animate-spin' : ''" />
          </button>
          <button @click="openPlanEdit(null)" class="btn btn-primary">{{ t('payment.admin.createPlan') }}</button>
        </div>
      </div>

      <!-- Plans Table -->
      <DataTable :columns="planColumns" :data="plans" :loading="plansLoading">
        <template #cell-name="{ value, row }">
          <span class="text-sm font-medium" :class="getPlanNameClass(row.group_id)">{{ value }}</span>
        </template>
        <template #cell-group_id="{ value }">
          <span v-if="isGroupMissing(value)" class="text-sm">
            <span class="text-gray-400">#{{ value }}</span>
            <span class="ml-1 badge badge-danger">{{ t('payment.admin.groupMissing') }}</span>
          </span>
          <GroupBadge
            v-else-if="getGroup(value)"
            :name="getGroup(value)!.name"
            :platform="getGroup(value)!.platform"
            :rate-multiplier="getGroup(value)!.rate_multiplier"
          />
          <span v-else class="text-sm text-gray-400">-</span>
        </template>
        <template #cell-price="{ value, row }">
          <div class="text-sm">
            <span class="font-medium text-gray-900 dark:text-white">${{ (value ?? 0).toFixed(2) }}</span>
            <span v-if="row.original_price" class="ml-1 text-xs text-gray-400 line-through">${{ row.original_price.toFixed(2) }}</span>
          </div>
        </template>
        <template #cell-validity_days="{ value, row }">
          <span class="text-sm">{{ value }} {{ t('payment.admin.' + (row.validity_unit || 'days')) }}</span>
        </template>
        <template #cell-display_purchase_count="{ value }">
          <span class="text-sm text-gray-700 dark:text-gray-300">{{ value || 0 }}</span>
        </template>
        <template #cell-for_sale="{ value, row }">
          <button
            type="button"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              value ? 'bg-primary-500' : 'bg-gray-300 dark:bg-dark-600'
            ]"
            @click="toggleForSale(row)"
          >
            <span :class="[
              'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
              value ? 'translate-x-4' : 'translate-x-0'
            ]" />
          </button>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <button @click="openPlanEdit(row)" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400">
              <Icon name="edit" size="sm" />
              <span class="text-xs">{{ t('common.edit') }}</span>
            </button>
            <button @click="confirmDeletePlan(row)" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400">
              <Icon name="trash" size="sm" />
              <span class="text-xs">{{ t('common.delete') }}</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Plan Edit Dialog -->
    <PlanEditDialog :show="showPlanDialog" :plan="editingPlan" :groups="groups" @close="showPlanDialog = false" @saved="loadPlans" />

    <ConfirmDialog
      :show="showBulkIncreaseDialog"
      :title="t('payment.admin.bulkIncreaseConfirm')"
      :message="t('payment.admin.bulkIncreaseConfirmMessage', { count: plans.length, amount: bulkIncreaseAmount || 0 })"
      :confirm-text="t('payment.admin.bulkIncreaseApply')"
      @confirm="handleBulkIncrease"
      @cancel="showBulkIncreaseDialog = false"
    />

    <ConfirmDialog :show="showDeletePlanDialog" :title="t('payment.admin.deletePlan')" :message="t('payment.admin.deletePlanConfirm')" :confirm-text="t('common.delete')" danger @confirm="handleDeletePlan" @cancel="showDeletePlanDialog = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import adminAPI from '@/api/admin'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlanEditDialog from './PlanEditDialog.vue'
import { platformTextClass } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()

// ==================== Groups ====================

const groups = ref<AdminGroup[]>([])

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch { /* ignore */ }
}

function getGroup(id: number): AdminGroup | undefined {
  return groups.value.find(g => g.id === id)
}

function isGroupMissing(id: number): boolean {
  return id > 0 && !groups.value.find(g => g.id === id)
}

function getPlanNameClass(groupId: number): string {
  const group = getGroup(groupId)
  return group ? platformTextClass(group.platform) : 'text-gray-900 dark:text-white'
}


// ==================== Plans ====================

const plansLoading = ref(false)
const plans = ref<SubscriptionPlan[]>([])
const showPlanDialog = ref(false)
const showDeletePlanDialog = ref(false)
const showBulkIncreaseDialog = ref(false)
const editingPlan = ref<SubscriptionPlan | null>(null)
const deletingPlanId = ref<number | null>(null)
const bulkIncreaseAmount = ref<number | null>(null)
const bulkIncreasing = ref(false)

const planColumns = computed((): Column[] => [
  { key: 'id', label: 'ID' },
  { key: 'name', label: t('payment.admin.planName') },
  { key: 'group_id', label: t('payment.admin.group') },
  { key: 'price', label: t('payment.admin.price') },
  { key: 'validity_days', label: t('payment.admin.validityDays') },
  { key: 'display_purchase_count', label: t('payment.admin.displayPurchaseCount') },
  { key: 'for_sale', label: t('payment.admin.forSale') },
  { key: 'sort_order', label: t('payment.admin.sortOrder') },
  { key: 'actions', label: t('common.actions') },
])

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    // Backend returns features as newline-separated string; parse to array
    plans.value = (res.data || []).map((p: Omit<SubscriptionPlan, 'features'> & { features: string | string[] }) => ({
      ...p,
      features: typeof p.features === 'string'
        ? p.features.split('\n').map((f: string) => f.trim()).filter(Boolean)
        : (p.features || []),
    }))
  }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { plansLoading.value = false }
}

function openPlanEdit(plan: SubscriptionPlan | null) {
  editingPlan.value = plan
  showPlanDialog.value = true
}

function openBulkIncreaseDialog() {
  if (!plans.value.length) {
    appStore.showInfo(t('payment.admin.bulkIncreaseNoPlans'))
    return
  }
  const amount = Number(bulkIncreaseAmount.value ?? 0)
  if (!Number.isInteger(amount) || amount <= 0) {
    appStore.showError(t('payment.admin.bulkIncreaseRequired'))
    return
  }
  showBulkIncreaseDialog.value = true
}

async function handleBulkIncrease() {
  if (bulkIncreasing.value) return
  const amount = Number(bulkIncreaseAmount.value ?? 0)
  if (!Number.isInteger(amount) || amount <= 0) {
    appStore.showError(t('payment.admin.bulkIncreaseRequired'))
    return
  }
  bulkIncreasing.value = true
  try {
    const res = await adminPaymentAPI.bulkIncreaseDisplayPurchaseCount(amount)
    appStore.showSuccess(t('payment.admin.bulkIncreaseSuccess', { count: res.data?.affected || 0 }))
    showBulkIncreaseDialog.value = false
    await loadPlans()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    bulkIncreasing.value = false
  }
}


/** Quick toggle for_sale from the list */
async function toggleForSale(plan: SubscriptionPlan) {
  try {
    await adminPaymentAPI.updatePlan(plan.id, { for_sale: !plan.for_sale })
    plan.for_sale = !plan.for_sale
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function confirmDeletePlan(plan: SubscriptionPlan) { deletingPlanId.value = plan.id; showDeletePlanDialog.value = true }
async function handleDeletePlan() {
  if (!deletingPlanId.value) return
  try { await adminPaymentAPI.deletePlan(deletingPlanId.value); appStore.showSuccess(t('common.deleted')); showDeletePlanDialog.value = false; loadPlans() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

// ==================== Lifecycle ====================

onMounted(() => {
  loadGroups()
  loadPlans()
})
</script>
