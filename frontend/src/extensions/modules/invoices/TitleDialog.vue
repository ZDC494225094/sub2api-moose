<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { invoiceAPI, type InvoiceTitle, type InvoiceTitleInput } from './api'
import { errorMessage } from './format'

const props = defineProps<{
  show: boolean
  /** null creates a new title. */
  title: InvoiceTitle | null
  allowSpecial: boolean
}>()

const emit = defineEmits<{
  close: []
  saved: [title: InvoiceTitle]
}>()

const blank = (): InvoiceTitleInput => ({
  title_type: 'personal', name: '', tax_no: '', address: '', phone: '', bank_name: '', bank_account: '', email: '', is_default: false,
})

const form = reactive<InvoiceTitleInput>(blank())
const saving = ref(false)
const error = ref('')
const isCompany = computed(() => form.title_type === 'company')
const dialogTitle = computed(() => (props.title ? '编辑发票抬头' : '新增发票抬头'))

watch(() => [props.show, props.title] as const, ([show, title]) => {
  if (!show) return
  error.value = ''
  Object.assign(form, blank(), title ? { ...title } : {})
}, { immediate: true })

async function save() {
  error.value = ''
  if (!form.name.trim()) {
    error.value = '请填写抬头名称'
    return
  }
  if (isCompany.value && !form.tax_no.trim()) {
    error.value = '企业抬头必须填写纳税人识别号'
    return
  }
  saving.value = true
  try {
    const saved = await invoiceAPI.saveTitle({ ...form }, props.title?.id)
    emit('saved', saved)
  } catch (e) {
    error.value = errorMessage(e, '保存失败，请稍后重试')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog :show="show" :title="dialogTitle" width="wide" @close="emit('close')">
    <form id="invoice-title-form" class="space-y-5" @submit.prevent="save">
      <div class="tabs inline-flex" role="radiogroup" aria-label="抬头类型">
        <button
          v-for="option in [{ value: 'personal', label: '个人' }, { value: 'company', label: '企业' }] as const"
          :key="option.value"
          type="button"
          role="radio"
          class="tab"
          :class="{ 'tab-active': form.title_type === option.value }"
          :aria-checked="form.title_type === option.value"
          @click="form.title_type = option.value"
        >
          {{ option.label }}
        </button>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <label class="sm:col-span-2">
          <span class="input-label">抬头名称 <span class="text-red-500">*</span></span>
          <input v-model="form.name" class="input" maxlength="100" :placeholder="isCompany ? '与营业执照一致的公司全称' : '个人姓名'" autocomplete="off" />
        </label>
        <label :class="isCompany ? 'sm:col-span-2' : ''">
          <span class="input-label">纳税人识别号 <span v-if="isCompany" class="text-red-500">*</span></span>
          <input v-model="form.tax_no" class="input font-mono uppercase" maxlength="20" :placeholder="isCompany ? '统一社会信用代码（18 位）' : '选填'" autocomplete="off" />
        </label>
        <label :class="isCompany ? 'sm:col-span-2' : ''">
          <span class="input-label">接收邮箱</span>
          <input v-model="form.email" type="email" class="input" maxlength="255" placeholder="用于接收电子发票（选填）" autocomplete="email" />
        </label>
      </div>

      <fieldset v-if="isCompany" class="space-y-3 rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
        <legend class="px-1 text-sm font-medium text-gray-700 dark:text-gray-200">
          企业开票信息<span class="ml-2 text-xs font-normal text-gray-500 dark:text-dark-400">{{ allowSpecial ? '申请增值税专用发票时必填' : '选填' }}</span>
        </legend>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="sm:col-span-2">
            <span class="input-label">注册地址</span>
            <input v-model="form.address" class="input" maxlength="200" autocomplete="off" />
          </label>
          <label>
            <span class="input-label">注册电话</span>
            <input v-model="form.phone" class="input" maxlength="32" autocomplete="off" />
          </label>
          <label>
            <span class="input-label">开户银行</span>
            <input v-model="form.bank_name" class="input" maxlength="100" autocomplete="off" />
          </label>
          <label class="sm:col-span-2">
            <span class="input-label">银行账号</span>
            <input v-model="form.bank_account" class="input font-mono" maxlength="64" autocomplete="off" />
          </label>
        </div>
      </fieldset>

      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
        <input v-model="form.is_default" type="checkbox" class="h-4 w-4 rounded" />
        设为默认抬头
      </label>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">取消</button>
        <button type="submit" form="invoice-title-form" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
      </div>
    </template>
  </BaseDialog>
</template>
