<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AdminOverviewPanel from './AdminOverviewPanel.vue'
import AdminTargetsPanel from './AdminTargetsPanel.vue'
import AdminLogicPanel from './AdminLogicPanel.vue'
import AdminDrawingPanel from './AdminDrawingPanel.vue'
import AdminSettingsPanel from './AdminSettingsPanel.vue'
import { iqMonitorAdminAPI, type ConfigView, type Options } from './api'
import { errorMessage } from './format'

type Tab = 'overview' | 'targets' | 'logic' | 'drawing' | 'settings'

const tab = ref<Tab>('overview')
const config = ref<ConfigView | null>(null)
const options = ref<Options | null>(null)
const error = ref('')

const tabs: Array<{ value: Tab; label: string }> = [
  { value: 'overview', label: '运行概览' },
  { value: 'targets', label: '监测目标' },
  { value: 'logic', label: '逻辑测试' },
  { value: 'drawing', label: '画图与投稿' },
  { value: 'settings', label: '通用设置' },
]

async function loadConfig() {
  try {
    config.value = await iqMonitorAdminAPI.config()
  } catch (e) {
    error.value = errorMessage(e, '读取监测设置失败')
  }
}

async function loadOptions() {
  try {
    options.value = await iqMonitorAdminAPI.options()
  } catch (e) {
    error.value = errorMessage(e, '读取分组和令牌失败')
  }
}

function onSaved(cfg: ConfigView) {
  const probeChanged = config.value?.probe_user_id !== cfg.probe_user_id
  config.value = cfg
  if (probeChanged) void loadOptions()
}

onMounted(() => {
  void loadConfig()
  void loadOptions()
})
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6">
      <header class="flex min-w-0 items-start gap-4">
        <div class="grid h-12 w-12 shrink-0 place-items-center rounded-2xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-800/40">
          <Icon name="beaker" size="lg" />
        </div>
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">降智监测</h1>
          <p class="mt-1 max-w-3xl text-sm leading-6 text-gray-500 dark:text-dark-400">
            定时用同一道逻辑题和同一个画图主题测试各分组模型，经本站网关转发，费用记在探测账号上。
            用户页面地址：<RouterLink to="/iq-monitor" class="whitespace-nowrap text-primary-600 hover:underline dark:text-primary-400">/iq-monitor</RouterLink>
          </p>
        </div>
      </header>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>

      <div class="tabs inline-flex flex-wrap" role="tablist" aria-label="降智监测">
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

      <div v-if="!config" class="card p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
      <template v-else>
        <AdminOverviewPanel v-if="tab === 'overview'" :config="config" :options="options" @goto="tab = $event" />
        <AdminTargetsPanel v-else-if="tab === 'targets'" :config="config" :options="options" @goto="tab = $event" />
        <AdminLogicPanel v-else-if="tab === 'logic'" :config="config" @saved="onSaved" />
        <AdminDrawingPanel v-else-if="tab === 'drawing'" :config="config" :options="options" @saved="onSaved" />
        <AdminSettingsPanel v-else :config="config" :options="options" @saved="onSaved" />
      </template>
    </div>
  </AppLayout>
</template>
