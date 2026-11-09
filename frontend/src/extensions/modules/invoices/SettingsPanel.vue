<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { invoiceAdminAPI, type InvoiceAdminSettings } from './api'
import { errorMessage, formatTime } from './format'

const props = defineProps<{ settings: InvoiceAdminSettings | null }>()
const emit = defineEmits<{ saved: [settings: InvoiceAdminSettings] }>()

const appStore = useAppStore()
const form = reactive({
  min_amount: '0', allow_special: true, order_window_days: 365, access_mode: 'allowlist' as InvoiceAdminSettings['access_mode'],
  item_name: '', notice: '',
})
const saving = ref(false)
const error = ref('')

watch(() => props.settings, (s) => {
  if (!s) return
  Object.assign(form, {
    min_amount: s.min_amount, allow_special: s.allow_special, order_window_days: s.order_window_days,
    access_mode: s.access_mode, item_name: s.item_name, notice: s.notice,
  })
}, { immediate: true })

async function save() {
  error.value = ''
  const amount = Number(form.min_amount)
  if (!Number.isFinite(amount) || amount < 0) {
    error.value = '开票门槛必须是不小于 0 的金额'
    return
  }
  const days = Number(form.order_window_days)
  if (!Number.isInteger(days) || days < 0 || days > 3650) {
    error.value = '可开票订单时限需为 0 到 3650 之间的整数'
    return
  }
  saving.value = true
  try {
    const saved = await invoiceAdminAPI.saveSettings({ ...form, min_amount: amount.toFixed(2), order_window_days: days })
    appStore.showSuccess('开票设置已保存')
    emit('saved', saved)
  } catch (e) {
    error.value = errorMessage(e, '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <form class="card max-w-3xl space-y-6 p-6" @submit.prevent="save">
    <fieldset class="space-y-3">
      <legend class="text-base font-semibold text-gray-900 dark:text-white">开放范围</legend>
      <div class="grid gap-3 sm:grid-cols-2">
        <label
          v-for="option in [
            { value: 'allowlist', title: '指定用户', desc: '只有开放名单中的用户能看到开票入口（推荐）' },
            { value: 'all', title: '全部用户', desc: '所有登录用户都能申请开票' },
          ] as const"
          :key="option.value"
          class="flex cursor-pointer gap-3 rounded-2xl border p-4 transition-colors"
          :class="form.access_mode === option.value ? 'border-primary-400 bg-primary-50/50 dark:border-primary-600 dark:bg-primary-900/10' : 'border-gray-100 hover:border-gray-200 dark:border-dark-700'"
        >
          <input v-model="form.access_mode" type="radio" class="mt-1 h-4 w-4" :value="option.value" name="access-mode" />
          <span>
            <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ option.title }}</span>
            <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">{{ option.desc }}</span>
          </span>
        </label>
      </div>
    </fieldset>

    <fieldset class="grid gap-4 sm:grid-cols-2">
      <legend class="mb-3 text-base font-semibold text-gray-900 dark:text-white">开票规则</legend>
      <label>
        <span class="input-label">开票门槛（元）</span>
        <input v-model="form.min_amount" type="number" min="0" step="0.01" class="input" />
        <span class="input-hint">单次申请的订单合计金额需不少于此值，0 表示不限。用户可合并多笔订单达到门槛。</span>
      </label>
      <label>
        <span class="input-label">可开票订单时限（天）</span>
        <input v-model.number="form.order_window_days" type="number" min="0" max="3650" step="1" class="input" />
        <span class="input-hint">只能为该天数内支付的订单申请开票，0 表示不限。</span>
      </label>
      <label class="sm:col-span-2">
        <span class="input-label">开票内容</span>
        <input v-model="form.item_name" class="input" maxlength="100" placeholder="例如：*信息技术服务*技术服务费" />
        <span class="input-hint">展示给用户，并记录在每条申请上，便于开票时填写商品名称。</span>
      </label>
      <div class="flex items-center justify-between gap-4 rounded-2xl border border-gray-100 p-4 sm:col-span-2 dark:border-dark-700">
        <span>
          <span class="block text-sm font-medium text-gray-900 dark:text-white">允许申请增值税专用发票</span>
          <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">关闭后用户只能申请电子普通发票；专票只能开给填写了完整开票信息的企业抬头。</span>
        </span>
        <Toggle v-model="form.allow_special" aria-label="允许申请增值税专用发票" />
      </div>
      <label class="sm:col-span-2">
        <span class="input-label">开票说明</span>
        <textarea v-model="form.notice" class="input min-h-[110px]" maxlength="2000" placeholder="展示在用户的发票管理页顶部，例如：电子发票一般在 3 个工作日内开具，开具后可在开票记录中下载。"></textarea>
      </label>
    </fieldset>

    <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <div class="flex items-center justify-between gap-3">
      <span class="text-xs text-gray-500 dark:text-dark-400">{{ settings?.updated_at ? `上次保存：${formatTime(settings.updated_at)}` : '' }}</span>
      <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存设置' }}</button>
    </div>
  </form>
</template>
