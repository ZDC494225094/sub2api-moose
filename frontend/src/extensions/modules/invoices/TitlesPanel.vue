<script setup lang="ts">
import { onMounted, ref } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import TitleDialog from './TitleDialog.vue'
import { invoiceAPI, type InvoiceTitle } from './api'
import { errorMessage, maskTail, titleTypeLabels } from './format'

defineProps<{ allowSpecial: boolean }>()
const emit = defineEmits<{ changed: [] }>()

const appStore = useAppStore()
const titles = ref<InvoiceTitle[]>([])
const loading = ref(true)
const error = ref('')
const editing = ref<InvoiceTitle | null>(null)
const dialogOpen = ref(false)
const deleting = ref<InvoiceTitle | null>(null)

async function load() {
  loading.value = true
  error.value = ''
  try {
    titles.value = await invoiceAPI.titles()
  } catch (e) {
    error.value = errorMessage(e, '加载发票抬头失败')
  } finally {
    loading.value = false
  }
}

function open(title: InvoiceTitle | null) {
  editing.value = title
  dialogOpen.value = true
}

async function onSaved() {
  dialogOpen.value = false
  appStore.showSuccess('发票抬头已保存')
  await load()
  emit('changed')
}

async function makeDefault(title: InvoiceTitle) {
  try {
    await invoiceAPI.saveTitle({ ...title, is_default: true }, title.id)
    await load()
    emit('changed')
  } catch (e) {
    appStore.showError(errorMessage(e, '设置默认抬头失败'))
  }
}

async function confirmDelete() {
  const target = deleting.value
  deleting.value = null
  if (!target) return
  try {
    await invoiceAPI.deleteTitle(target.id)
    appStore.showSuccess('发票抬头已删除')
    await load()
    emit('changed')
  } catch (e) {
    appStore.showError(errorMessage(e, '删除失败'))
  }
}

function specialReady(t: InvoiceTitle) {
  return t.title_type === 'company' && !!(t.address && t.phone && t.bank_name && t.bank_account)
}

onMounted(load)
</script>

<template>
  <section class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-gray-500 dark:text-dark-400">最多保存 10 个抬头。修改或删除抬头不会影响已经提交的申请。</p>
      <button type="button" class="btn btn-primary" :disabled="titles.length >= 10" @click="open(null)">
        <Icon name="plus" size="sm" />新增抬头
      </button>
    </div>

    <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <div v-if="loading" class="card p-6 text-sm text-gray-500 dark:text-dark-400">正在加载…</div>
    <div v-else-if="titles.length === 0" class="card empty-state">
      <Icon name="document" size="xl" class="empty-state-icon" />
      <p class="empty-state-title">还没有发票抬头</p>
      <p class="empty-state-description">添加个人或企业抬头后即可申请开票。</p>
    </div>
    <div v-else class="grid gap-4 md:grid-cols-2">
      <article v-for="t in titles" :key="t.id" class="card flex flex-col gap-3 p-5">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="badge badge-gray">{{ titleTypeLabels[t.title_type] }}</span>
              <span v-if="t.is_default" class="badge badge-primary">默认</span>
              <span v-if="allowSpecial && specialReady(t)" class="badge badge-success">可开专票</span>
            </div>
            <h3 class="mt-2 break-words text-base font-semibold text-gray-900 dark:text-white">{{ t.name }}</h3>
          </div>
        </div>
        <dl class="grid gap-1.5 text-sm">
          <div v-if="t.tax_no" class="flex gap-2"><dt class="w-20 shrink-0 text-gray-500 dark:text-dark-400">税号</dt><dd class="font-mono text-gray-800 dark:text-gray-200">{{ t.tax_no }}</dd></div>
          <div v-if="t.email" class="flex gap-2"><dt class="w-20 shrink-0 text-gray-500 dark:text-dark-400">接收邮箱</dt><dd class="break-all text-gray-800 dark:text-gray-200">{{ t.email }}</dd></div>
          <div v-if="t.bank_name" class="flex gap-2"><dt class="w-20 shrink-0 text-gray-500 dark:text-dark-400">开户行</dt><dd class="text-gray-800 dark:text-gray-200">{{ t.bank_name }} {{ maskTail(t.bank_account) }}</dd></div>
        </dl>
        <div class="mt-auto flex flex-wrap gap-2 pt-1">
          <button type="button" class="btn btn-secondary btn-sm" @click="open(t)"><Icon name="edit" size="sm" />编辑</button>
          <button v-if="!t.is_default" type="button" class="btn btn-ghost btn-sm" @click="makeDefault(t)">设为默认</button>
          <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="deleting = t"><Icon name="trash" size="sm" />删除</button>
        </div>
      </article>
    </div>

    <TitleDialog :show="dialogOpen" :title="editing" :allow-special="allowSpecial" @close="dialogOpen = false" @saved="onSaved" />
    <ConfirmDialog
      :show="!!deleting"
      :title="deleting ? `删除抬头「${deleting.name}」？` : ''"
      message="删除后不影响已提交的开票申请。"
      confirm-text="删除"
      danger
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </section>
</template>
