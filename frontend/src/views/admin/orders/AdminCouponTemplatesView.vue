<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="search" class="input max-w-72" :placeholder="t('adminCoupons.search')" @input="loadData" />
          <Select v-model="scope" :options="scopeOptions" class="w-44" @change="loadData" />
          <Select v-model="status" :options="statusOptions" class="w-36" @change="loadData" />
          <div class="ml-auto flex gap-2">
            <button class="btn btn-secondary" @click="loadData">{{ t('common.refresh') }}</button>
            <button class="btn btn-primary" @click="openCreate">{{ t('adminCoupons.create') }}</button>
          </div>
        </div>
      </template>
      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-name="{ row }">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ row.name }}</p>
              <p v-if="row.description" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ row.description }}</p>
            </div>
          </template>
          <template #cell-scope="{ value }">
            <span class="inline-flex rounded-full bg-sky-50 px-2.5 py-1 text-xs font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-200">
              {{ scopeLabel(value) }}
            </span>
          </template>
          <template #cell-discount_amount="{ value }">¥{{ Number(value).toFixed(2) }}</template>
          <template #cell-threshold_amount="{ value }">¥{{ Number(value).toFixed(2) }}</template>
          <template #cell-status="{ row }">
            <div class="flex items-center gap-3">
              <button
                type="button"
                :class="[
                  'relative inline-flex h-6 w-11 flex-shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                  row.status === 'active' ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-600'
                ]"
                :aria-label="t('adminCoupons.toggleStatus')"
                @click="toggleStatus(row)"
              >
                <span
                  :class="[
                    'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow transition duration-200 ease-in-out',
                    row.status === 'active' ? 'translate-x-5' : 'translate-x-0'
                  ]"
                />
              </button>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ statusLabel(row.status) }}</span>
            </div>
          </template>
          <template #cell-actions="{ row }">
            <button class="btn btn-secondary btn-sm" @click="openEdit(row)">{{ t('common.edit') }}</button>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <BaseDialog :show="dialogOpen" :title="editingId ? t('common.edit') : t('common.create')" @close="dialogOpen = false">
      <form id="coupon-template-form" class="space-y-4" @submit.prevent="submit">
        <div>
          <label class="input-label">{{ t('adminCoupons.name') }}</label>
          <input v-model="form.name" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('adminCoupons.scope') }}</label>
          <Select v-model="form.scope" :options="scopeOptions.filter(option => option.value)" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('adminCoupons.discountAmount') }}</label>
            <input v-model.number="form.discount_amount" class="input" type="number" min="0.01" step="0.01" />
          </div>
          <div>
            <label class="input-label">{{ t('adminCoupons.thresholdAmount') }}</label>
            <input v-model.number="form.threshold_amount" class="input" type="number" min="0" step="0.01" />
          </div>
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('adminCoupons.validDays') }}</label>
            <input v-model.number="form.valid_days" class="input" type="number" min="1" />
          </div>
          <div>
            <label class="input-label">{{ t('adminCoupons.status') }}</label>
            <Select v-model="form.status" :options="statusOptions.filter(option => option.value)" />
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('adminCoupons.description') }}</label>
          <textarea v-model="form.description" class="input" rows="3"></textarea>
        </div>
        <div>
          <label class="input-label">{{ t('adminCoupons.notes') }}</label>
          <textarea v-model="form.notes" class="input" rows="2"></textarea>
        </div>
      </form>
      <template #footer>
        <button class="btn btn-secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" form="coupon-template-form" type="submit">{{ t('common.save') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { CouponTemplate, CouponScope } from '@/types/payment'

const { t } = useI18n()
const appStore = useAppStore()
const items = ref<CouponTemplate[]>([])
const loading = ref(false)
const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const search = ref('')
const scope = ref('')
const status = ref('')

const form = reactive({
  name: '',
  scope: 'universal' as CouponScope,
  discount_amount: 1,
  threshold_amount: 0,
  valid_days: 7,
  status: 'active' as 'active' | 'disabled',
  description: '',
  notes: '',
})

const columns = computed(() => [
  { key: 'name', label: t('adminCoupons.name') },
  { key: 'scope', label: t('adminCoupons.scope') },
  { key: 'discount_amount', label: t('adminCoupons.discountAmount') },
  { key: 'threshold_amount', label: t('adminCoupons.thresholdAmount') },
  { key: 'status', label: t('adminCoupons.status') },
  { key: 'actions', label: t('common.actions') },
])

const scopeOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'balance', label: t('adminCoupons.scopeBalance') },
  { value: 'subscription', label: t('adminCoupons.scopeSubscription') },
  { value: 'universal', label: t('adminCoupons.scopeUniversal') },
])

const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'active', label: t('common.active') },
  { value: 'disabled', label: t('common.disabled') },
])

function resetForm() {
  form.name = ''
  form.scope = 'universal'
  form.discount_amount = 1
  form.threshold_amount = 0
  form.valid_days = 7
  form.status = 'active'
  form.description = ''
  form.notes = ''
}

function scopeLabel(scopeValue: CouponTemplate['scope']) {
  switch (scopeValue) {
    case 'balance':
      return t('adminCoupons.scopeBalance')
    case 'subscription':
      return t('adminCoupons.scopeSubscription')
    default:
      return t('adminCoupons.scopeUniversal')
  }
}

function statusLabel(statusValue: CouponTemplate['status']) {
  return statusValue === 'active' ? t('common.enabled') : t('common.disabled')
}

async function loadData() {
  loading.value = true
  try {
    const response = await adminAPI.payment.getCouponTemplates({
      page: 1,
      page_size: 100,
      scope: scope.value || undefined,
      status: status.value || undefined,
      search: search.value || undefined,
    })
    items.value = response.data.items
  } catch (error) {
    console.error(error)
    appStore.showError(t('adminCoupons.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  resetForm()
  dialogOpen.value = true
}

function openEdit(item: CouponTemplate) {
  editingId.value = item.id
  form.name = item.name
  form.scope = item.scope
  form.discount_amount = item.discount_amount
  form.threshold_amount = item.threshold_amount
  form.valid_days = item.valid_days || 7
  form.status = item.status
  form.description = item.description
  form.notes = item.notes
  dialogOpen.value = true
}

async function submit() {
  try {
    const payload = { ...form }
    if (editingId.value) {
      await adminAPI.payment.updateCouponTemplate(editingId.value, payload)
    } else {
      await adminAPI.payment.createCouponTemplate(payload)
    }
    dialogOpen.value = false
    loadData()
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error))
  }
}

async function toggleStatus(item: CouponTemplate) {
  const nextStatus: CouponTemplate['status'] = item.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.payment.updateCouponTemplate(item.id, { status: nextStatus })
    item.status = nextStatus
  } catch (error: any) {
    console.error(error)
    appStore.showError(errorMessage(error))
  }
}

function errorMessage(error: any) {
  return error?.message || error?.response?.data?.detail || error?.response?.data?.message || t('common.error')
}

onMounted(loadData)
</script>
