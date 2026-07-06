<template>
  <BaseDialog :show="show" :title="t('admin.groups.manageAccounts')" width="wide" @close="handleClose">
    <div v-if="group" class="space-y-4">
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="inline-flex items-center gap-1.5" :class="platformColorClass">
          <PlatformIcon :platform="group.platform" size="sm" />
          {{ t('admin.groups.platforms.' + group.platform) }}
        </span>
        <span class="text-gray-400">|</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.accountsCount', { count: localAccounts.length }) }}
        </span>
      </div>

      <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_380px]">
        <div class="rounded-lg border border-gray-200 dark:border-dark-600">
          <div class="flex items-center justify-between border-b border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-700">
            <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.groups.groupAccounts') }}
            </h4>
            <span class="text-xs text-gray-500 dark:text-gray-400">
              {{ localAccounts.length }}
            </span>
          </div>

          <div v-if="loading" class="flex justify-center py-10">
            <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
          </div>

          <div v-else-if="localAccounts.length === 0" class="py-10 text-center text-sm text-gray-400 dark:text-gray-500">
            {{ t('admin.groups.noGroupAccounts') }}
          </div>

          <VueDraggable
            v-else
            v-model="localAccounts"
            :animation="180"
            handle=".drag-handle"
            class="max-h-[520px] divide-y divide-gray-100 overflow-y-auto dark:divide-dark-600"
          >
            <div
              v-for="(account, index) in localAccounts"
              :key="account.id"
              class="bg-white px-3 py-2.5 transition-colors hover:bg-gray-50 dark:bg-dark-800 dark:hover:bg-dark-700/70"
            >
              <div class="flex items-start gap-3">
                <button
                  type="button"
                  class="drag-handle mt-1 cursor-grab rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 active:cursor-grabbing dark:hover:bg-dark-600 dark:hover:text-gray-300"
                  :title="t('admin.groups.dragSort')"
                >
                  <Icon name="menu" size="sm" />
                </button>
                <div class="mt-1 flex h-7 w-7 shrink-0 items-center justify-center rounded bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-300">
                  {{ index + 1 }}
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex min-w-0 items-center gap-2">
                    <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ account.name }}</span>
                    <span class="shrink-0 text-xs text-gray-400">#{{ account.id }}</span>
                  </div>
                  <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ account.type }}</span>
                    <span
                      :class="[
                        'rounded px-1.5 py-0.5 font-medium',
                        account.status === 'active'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                          : account.status === 'error'
                            ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                            : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-400'
                      ]"
                    >
                      {{ t('admin.accounts.status.' + account.status) }}
                    </span>
                    <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-600">
                      {{ t('admin.groups.accountRateMultiplier') }} {{ formatMultiplier(account) }}
                    </span>
                    <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-600">
                      {{ t('admin.groups.accountPriority') }} {{ account.priority ?? 0 }}
                    </span>
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-1">
                  <button
                    type="button"
                    class="rounded p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-600 dark:hover:text-gray-300"
                    :title="t('admin.groups.quickEditAccount')"
                    :disabled="isAccountUpdating(account.id)"
                    @click="startEditAccount(account)"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                  <button
                    type="button"
                    class="rounded p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                    :title="t('admin.groups.removeAccount')"
                    @click="removeLocal(account.id)"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </div>

              <div v-if="editingAccountId === account.id" class="mt-3 grid gap-2 rounded border border-primary-100 bg-primary-50/50 p-2 dark:border-primary-900/40 dark:bg-primary-900/10 sm:grid-cols-[1fr_1fr_auto]">
                <label class="text-xs font-medium text-gray-600 dark:text-gray-300">
                  {{ t('admin.groups.accountRateMultiplier') }}
                  <input v-model.number="editForm.rateMultiplier" type="number" min="0" step="0.001" class="input mt-1 h-8 text-sm" />
                </label>
                <label class="text-xs font-medium text-gray-600 dark:text-gray-300">
                  {{ t('admin.groups.accountPriority') }}
                  <input v-model.number="editForm.priority" type="number" min="0" step="1" class="input mt-1 h-8 text-sm" />
                </label>
                <div class="flex items-end gap-1">
                  <button
                    type="button"
                    class="btn btn-primary btn-sm h-8 px-2"
                    :title="t('admin.groups.saveAccountSettings')"
                    :disabled="isAccountUpdating(account.id)"
                    @click="saveAccountSettings(account)"
                  >
                    <Icon v-if="isAccountUpdating(account.id)" name="refresh" size="sm" class="animate-spin" />
                    <Icon v-else name="check" size="sm" />
                  </button>
                  <button
                    type="button"
                    class="btn btn-sm h-8 px-2"
                    :title="t('admin.groups.cancelAccountSettings')"
                    :disabled="isAccountUpdating(account.id)"
                    @click="cancelEditAccount"
                  >
                    <Icon name="x" size="sm" />
                  </button>
                </div>
              </div>
            </div>
          </VueDraggable>
        </div>

        <div class="rounded-lg border border-gray-200 dark:border-dark-600">
          <div class="border-b border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-700">
            <div class="flex items-center justify-between gap-3">
              <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.groups.availableAccounts') }}
              </h4>
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ candidateAccounts.length }}</span>
            </div>
            <div class="relative mt-2">
              <Icon
                name="search"
                size="sm"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                autocomplete="off"
                class="input h-9 w-full pl-9 text-sm"
                :placeholder="t('admin.groups.filterAccountPlaceholder')"
              />
            </div>
          </div>

          <div class="max-h-[520px] overflow-y-auto p-3">
            <div v-if="loading" class="flex justify-center py-8">
              <Icon name="refresh" size="md" class="animate-spin text-primary-500" />
            </div>
            <div v-else-if="candidateAccounts.length > 0" class="space-y-2">
              <div
                v-for="account in candidateAccounts"
                :key="account.id"
                class="rounded-lg border border-gray-200 bg-white p-2.5 transition-colors hover:border-primary-200 hover:bg-primary-50 dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-700 dark:hover:bg-primary-900/20"
              >
                <div class="flex items-start gap-3">
                  <button
                    type="button"
                    class="mt-0.5 rounded p-1.5 text-primary-500 transition-colors hover:bg-primary-100 dark:hover:bg-primary-900/30"
                    :title="t('admin.groups.addAccount')"
                    @click="addLocal(account)"
                  >
                    <Icon name="plus" size="sm" />
                  </button>
                  <div class="min-w-0 flex-1">
                    <div class="flex min-w-0 items-center gap-2">
                      <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ account.name }}</span>
                      <span class="shrink-0 text-xs text-gray-400">#{{ account.id }}</span>
                    </div>
                    <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                      <span>{{ account.type }}</span>
                      <span
                        :class="[
                          'rounded px-1.5 py-0.5 font-medium',
                          account.status === 'active'
                            ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                            : account.status === 'error'
                              ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                              : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-400'
                        ]"
                      >
                        {{ t('admin.accounts.status.' + account.status) }}
                      </span>
                      <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-600">
                        {{ t('admin.groups.accountRateMultiplier') }} {{ formatMultiplier(account) }}
                      </span>
                      <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-600">
                        {{ t('admin.groups.accountPriority') }} {{ account.priority ?? 0 }}
                      </span>
                    </div>
                  </div>
                  <button
                    type="button"
                    class="rounded p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-600 dark:hover:text-gray-300"
                    :title="t('admin.groups.quickEditAccount')"
                    :disabled="isAccountUpdating(account.id)"
                    @click="startEditAccount(account)"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                </div>

                <div v-if="editingAccountId === account.id" class="mt-3 grid gap-2 rounded border border-primary-100 bg-primary-50/50 p-2 dark:border-primary-900/40 dark:bg-primary-900/10">
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-300">
                    {{ t('admin.groups.accountRateMultiplier') }}
                    <input v-model.number="editForm.rateMultiplier" type="number" min="0" step="0.001" class="input mt-1 h-8 text-sm" />
                  </label>
                  <label class="text-xs font-medium text-gray-600 dark:text-gray-300">
                    {{ t('admin.groups.accountPriority') }}
                    <input v-model.number="editForm.priority" type="number" min="0" step="1" class="input mt-1 h-8 text-sm" />
                  </label>
                  <div class="flex justify-end gap-1">
                    <button
                      type="button"
                      class="btn btn-primary btn-sm h-8 px-2"
                      :title="t('admin.groups.saveAccountSettings')"
                      :disabled="isAccountUpdating(account.id)"
                      @click="saveAccountSettings(account)"
                    >
                      <Icon v-if="isAccountUpdating(account.id)" name="refresh" size="sm" class="animate-spin" />
                      <Icon v-else name="check" size="sm" />
                    </button>
                    <button
                      type="button"
                      class="btn btn-sm h-8 px-2"
                      :title="t('admin.groups.cancelAccountSettings')"
                      :disabled="isAccountUpdating(account.id)"
                      @click="cancelEditAccount"
                    >
                      <Icon name="x" size="sm" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="py-8 text-center text-sm text-gray-400 dark:text-gray-500">
              {{ t('admin.groups.noAccountCandidates') }}
            </div>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3 border-t border-gray-200 pt-4 dark:border-dark-600">
        <template v-if="isDirty">
          <span class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.groups.unsavedChanges') }}</span>
          <button
            type="button"
            class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
            @click="handleRevert"
          >
            {{ t('admin.groups.revertChanges') }}
          </button>
        </template>
        <div class="ml-auto flex items-center gap-3">
          <button type="button" class="btn btn-sm px-4 py-1.5" @click="handleClose">
            {{ t('common.close') }}
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm px-4 py-1.5"
            :disabled="saving || !isDirty"
            @click="handleSave"
          >
            <Icon v-if="saving" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account, AdminGroup } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const serverAccounts = ref<Account[]>([])
const localAccounts = ref<Account[]>([])
const allAccounts = ref<Account[]>([])
const searchQuery = ref('')
const editingAccountId = ref<number | null>(null)
const updatingAccountIds = ref<Set<number>>(new Set())
const editForm = reactive({
  priority: 0,
  rateMultiplier: 1
})

const platformColorClass = computed(() => {
  switch (props.group?.platform) {
    case 'anthropic': return 'text-orange-700 dark:text-orange-400'
    case 'openai': return 'text-emerald-700 dark:text-emerald-400'
    case 'antigravity': return 'text-purple-700 dark:text-purple-400'
    case 'grok': return 'text-zinc-700 dark:text-zinc-300'
    default: return 'text-blue-700 dark:text-blue-400'
  }
})

const selectedIDs = computed(() => new Set(localAccounts.value.map(account => account.id)))

const candidateAccounts = computed(() => {
  if (!props.group) return []
  const query = searchQuery.value.trim().toLowerCase()
  return allAccounts.value
    .filter(account => account.platform === props.group?.platform)
    .filter(account => !selectedIDs.value.has(account.id))
    .filter(account => !(props.group?.require_oauth_only && account.type === 'apikey'))
    .filter(account => {
      if (!query) return true
      return account.name.toLowerCase().includes(query) || String(account.id).includes(query)
    })
})

const isDirty = computed(() => {
  if (serverAccounts.value.length !== localAccounts.value.length) return true
  return localAccounts.value.some((account, index) => account.id !== serverAccounts.value[index]?.id)
})

const cloneAccounts = (accounts: Account[]) => accounts.map(account => ({ ...account }))

const mergeWithGlobalAccounts = (accounts: Account[], globalAccounts = allAccounts.value) => {
  const globalByID = new Map(globalAccounts.map(account => [account.id, account]))
  return accounts.map(account => {
    const globalAccount = globalByID.get(account.id)
    return globalAccount ? { ...account, ...globalAccount } : { ...account }
  })
}

const fetchAllPlatformAccounts = async (): Promise<Account[]> => {
  if (!props.group) return []
  const pageSize = 200
  const result: Account[] = []
  for (let page = 1; page <= 50; page += 1) {
    const res = await adminAPI.accounts.list(page, pageSize, {
      platform: props.group.platform,
      sort_by: 'priority',
      sort_order: 'asc'
    })
    result.push(...res.items)
    if (result.length >= res.total || res.items.length < pageSize) break
  }
  return result
}

const loadAccounts = async () => {
  if (!props.group) return
  loading.value = true
  editingAccountId.value = null
  try {
    const [groupAccounts, platformAccounts] = await Promise.all([
      adminAPI.groups.getGroupAccounts(props.group.id),
      fetchAllPlatformAccounts()
    ])
    allAccounts.value = cloneAccounts(platformAccounts)
    serverAccounts.value = mergeWithGlobalAccounts(groupAccounts, platformAccounts)
    localAccounts.value = cloneAccounts(serverAccounts.value)
    searchQuery.value = ''
  } catch (error: any) {
    appStore.showError(error?.message || error?.response?.data?.message || t('admin.groups.failedToLoadAccounts'))
    console.error('Error loading group accounts:', error)
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (value) => {
  if (value && props.group) {
    loadAccounts()
  }
})

const formatMultiplier = (account: Account) => `${(account.rate_multiplier ?? 1).toFixed(3).replace(/\.?0+$/, '')}x`

const addLocal = (account: Account) => {
  if (localAccounts.value.some(item => item.id === account.id)) return
  localAccounts.value.push({ ...account })
}

const removeLocal = (accountID: number) => {
  localAccounts.value = localAccounts.value.filter(account => account.id !== accountID)
}

const startEditAccount = (account: Account) => {
  editingAccountId.value = account.id
  editForm.priority = account.priority ?? 0
  editForm.rateMultiplier = account.rate_multiplier ?? 1
}

const cancelEditAccount = () => {
  editingAccountId.value = null
}

const isAccountUpdating = (accountID: number) => updatingAccountIds.value.has(accountID)

const setAccountUpdating = (accountID: number, value: boolean) => {
  const next = new Set(updatingAccountIds.value)
  if (value) {
    next.add(accountID)
  } else {
    next.delete(accountID)
  }
  updatingAccountIds.value = next
}

const replaceAccountEverywhere = (updated: Account) => {
  const replace = (account: Account) => account.id === updated.id ? { ...account, ...updated } : account
  allAccounts.value = allAccounts.value.map(replace)
  serverAccounts.value = serverAccounts.value.map(replace)
  localAccounts.value = localAccounts.value.map(replace)
}

const saveAccountSettings = async (account: Account) => {
  const priority = Number(editForm.priority)
  const rateMultiplier = Number(editForm.rateMultiplier)
  if (!Number.isFinite(priority) || priority < 0 || !Number.isInteger(priority)) {
    appStore.showError(t('admin.groups.invalidAccountPriority'))
    return
  }
  if (!Number.isFinite(rateMultiplier) || rateMultiplier < 0) {
    appStore.showError(t('admin.groups.invalidAccountRateMultiplier'))
    return
  }

  setAccountUpdating(account.id, true)
  try {
    const updated = await adminAPI.accounts.update(account.id, {
      priority,
      rate_multiplier: rateMultiplier
    })
    replaceAccountEverywhere(updated)
    editingAccountId.value = null
    appStore.showSuccess(t('admin.groups.accountSettingsUpdated'))
  } catch (error: any) {
    appStore.showError(error?.message || error?.response?.data?.message || t('admin.groups.failedToUpdateAccountSettings'))
    console.error('Error updating account settings:', error)
  } finally {
    setAccountUpdating(account.id, false)
  }
}

const handleRevert = () => {
  localAccounts.value = cloneAccounts(serverAccounts.value)
  editingAccountId.value = null
}

const handleSave = async () => {
  if (!props.group) return
  saving.value = true
  try {
    const accountIDs = localAccounts.value.map(account => account.id)
    const accounts = await adminAPI.groups.updateGroupAccounts(props.group.id, accountIDs)
    serverAccounts.value = mergeWithGlobalAccounts(accounts)
    localAccounts.value = cloneAccounts(serverAccounts.value)
    appStore.showSuccess(t('admin.groups.accountsUpdated'))
    emit('success')
    emit('close')
  } catch (error: any) {
    appStore.showError(error?.message || error?.response?.data?.message || t('admin.groups.failedToUpdateAccounts'))
    console.error('Error saving group accounts:', error)
  } finally {
    saving.value = false
  }
}

const handleClose = () => {
  handleRevert()
  emit('close')
}
</script>
