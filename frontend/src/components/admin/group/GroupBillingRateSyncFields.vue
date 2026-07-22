<template>
  <div class="border-t border-gray-200 pt-4 dark:border-dark-600" data-testid="group-billing-rate-sync-fields">
    <div class="mb-3">
      <p class="text-sm font-medium text-gray-800 dark:text-gray-200">
        {{ t('admin.groups.billingRateSync.title') }}
      </p>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.groups.billingRateSync.hint') }}
      </p>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.groups.billingRateSync.referenceAccount') }}</label>
        <Select
          v-model="accountModel"
          :options="accountOptions"
          :placeholder="loading ? t('common.loading') : t('admin.groups.billingRateSync.referencePlaceholder')"
          :empty-text="loadFailed ? t('admin.groups.billingRateSync.loadFailed') : t('admin.groups.billingRateSync.noAccounts')"
          :disabled="disabled || loading"
          :searchable="true"
          :clearable="true"
          data-testid="group-billing-rate-sync-account"
        >
          <template #option="{ option }">
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{{ option.label }}</p>
              <p class="truncate text-xs text-gray-400">#{{ option.value }} · {{ option.description }}</p>
            </div>
          </template>
        </Select>
      </div>

      <label class="block">
        <span class="input-label">{{ t('admin.groups.billingRateSync.markup') }}</span>
        <input
          v-model.number="markupModel"
          type="number"
          min="0"
          step="0.001"
          class="input"
          :disabled="disabled"
          data-testid="group-billing-rate-markup"
        />
      </label>
    </div>

    <p
      v-if="selectedDetectedRate != null"
      class="mt-3 text-xs font-medium text-emerald-700 dark:text-emerald-400"
      data-testid="group-billing-rate-preview"
    >
      {{
        t('admin.groups.billingRateSync.preview', {
          detected: formatRate(selectedDetectedRate),
          markup: formatRate(normalizedMarkup),
          final: formatRate(selectedDetectedRate + normalizedMarkup),
        })
      }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import type { Account } from '@/types'

const props = withDefaults(defineProps<{
  accountId: number | null
  markup: number
  disabled?: boolean
}>(), {
  disabled: false
})

const emit = defineEmits<{
  'update:accountId': [value: number | null]
  'update:markup': [value: number]
}>()

const { t } = useI18n()
const accounts = ref<Account[]>([])
const loading = ref(false)
const loadFailed = ref(false)
let loadSequence = 0

const accountModel = computed<number | null>({
  get: () => props.accountId,
  set: (value) => emit('update:accountId', typeof value === 'number' ? value : null)
})

const markupModel = computed<number>({
  get: () => props.markup,
  set: (value) => emit('update:markup', typeof value === 'number' ? value : Number(value))
})

const normalizedMarkup = computed(() =>
  Number.isFinite(props.markup) && props.markup >= 0 ? props.markup : 0
)

const accountOptions = computed<SelectOption[]>(() => accounts.value.map((account) => ({
  value: account.id,
  label: account.name,
  description: t(`admin.accounts.status.${account.status}`)
})))

const selectedAccount = computed(() =>
  accounts.value.find((account) => account.id === props.accountId) ?? null
)

const selectedDetectedRate = computed(() => {
  const value = selectedAccount.value?.extra?.upstream_billing_probe?.data?.effective_rate_multiplier
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null
})

const formatRate = (value: number) => Number(value.toPrecision(12))

const loadAccounts = async () => {
  const sequence = ++loadSequence
  loading.value = true
  loadFailed.value = false
  try {
    const loaded: Account[] = []
    const pageSize = 100
    for (let page = 1; ; page += 1) {
      const result = await adminAPI.accounts.list(page, pageSize, {
        platform: 'openai',
        type: 'apikey',
        sort_by: 'name',
        sort_order: 'asc'
      })
      loaded.push(...result.items)
      if (loaded.length >= result.total || result.items.length === 0) break
    }
    if (sequence !== loadSequence) return
    accounts.value = loaded
  } catch {
    if (sequence !== loadSequence) return
    accounts.value = []
    loadFailed.value = true
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

watch(() => props.accountId, async (accountId) => {
  if (accountId == null || accounts.value.some((account) => account.id === accountId)) return
  try {
    const account = await adminAPI.accounts.getById(accountId)
    if (account.platform === 'openai' && account.type === 'apikey') {
      accounts.value = [...accounts.value, account]
    }
  } catch {
    // The backend will reject a missing or ineligible reference on save.
  }
}, { immediate: true })

onMounted(loadAccounts)
</script>
