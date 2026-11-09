<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AccessPanel from './AccessPanel.vue'
import AdminApplicationsPanel from './AdminApplicationsPanel.vue'
import SettingsPanel from './SettingsPanel.vue'
import { invoiceAdminAPI, type InvoiceAdminSettings } from './api'
import { errorMessage, formatMoney } from './format'

type Tab = 'review' | 'access' | 'settings'

const tab = ref<Tab>('review')
const settings = ref<InvoiceAdminSettings | null>(null)
const error = ref('')

const tabs: Array<{ value: Tab; label: string }> = [
  { value: 'review', label: '开票审核' },
  { value: 'access', label: '开放名单' },
  { value: 'settings', label: '开票设置' },
]

async function loadSettings() {
  try {
    settings.value = await invoiceAdminAPI.settings()
  } catch (e) {
    error.value = errorMessage(e, '加载开票设置失败')
  }
}

onMounted(loadSettings)
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6">
      <header class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div class="flex min-w-0 items-start gap-4">
          <div class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-800/40">
            <Icon name="document" size="lg" />
          </div>
          <div class="min-w-0">
            <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">发票管理</h1>
            <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-500 dark:text-dark-400">
              审核用户的开票申请，登记发票号码并上传电子发票。
              <template v-if="settings">
                当前{{ settings.access_mode === 'all' ? '对全部用户开放' : '仅对开放名单中的用户开放' }}，开票门槛
                {{ Number(settings.min_amount) > 0 ? formatMoney(settings.min_amount) : '不限' }}。
              </template>
            </p>
          </div>
        </div>
      </header>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>

      <div class="tabs inline-flex flex-wrap" role="tablist" aria-label="发票管理">
        <button
          v-for="item in tabs"
          :key="item.value"
          type="button"
          role="tab"
          class="tab"
          :class="{ 'tab-active': tab === item.value }"
          :aria-selected="tab === item.value"
          @click="tab = item.value"
        >
          {{ item.label }}
        </button>
      </div>

      <AdminApplicationsPanel v-if="tab === 'review'" />
      <AccessPanel v-else-if="tab === 'access'" :mode="settings?.access_mode ?? null" @switch-to-settings="tab = 'settings'" />
      <SettingsPanel v-else :settings="settings" @saved="settings = $event" />
    </div>
  </AppLayout>
</template>
