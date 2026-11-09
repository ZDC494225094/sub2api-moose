<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import ApplicationDetail from './ApplicationDetail.vue'
import { invoiceAdminAPI, type InvoiceApplication } from './api'
import { errorMessage } from './format'

const props = defineProps<{ app: InvoiceApplication | null }>()
const emit = defineEmits<{ close: []; changed: [app: InvoiceApplication] }>()

type Mode = 'view' | 'issue' | 'reject' | 'void'

const appStore = useAppStore()
const mode = ref<Mode>('view')
const invoiceNo = ref('')
const adminNote = ref('')
const reason = ref('')
const file = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const replaceInput = ref<HTMLInputElement | null>(null)
const busy = ref(false)
const error = ref('')
const actionForm = ref<HTMLElement | null>(null)

// The action form sits under the order list; bring it into view when it opens.
watch(mode, async (next) => {
  if (next === 'view') return
  await nextTick()
  actionForm.value?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' })
})

const MAX_BYTES = 10 * 1024 * 1024
const open = computed(() => props.app !== null)
const canProcess = computed(() => props.app?.status === 'pending')
const canIssue = computed(() => props.app?.status === 'pending' || props.app?.status === 'processing')
const canVoid = computed(() => props.app?.status === 'issued')

watch(() => props.app?.id, () => {
  mode.value = 'view'
  invoiceNo.value = props.app?.invoice_no ?? ''
  adminNote.value = props.app?.admin_note ?? ''
  reason.value = ''
  file.value = null
  error.value = ''
})

function pickFile(event: Event): File | null {
  const input = event.target as HTMLInputElement
  const picked = input.files?.[0] ?? null
  input.value = ''
  if (!picked) return null
  if (picked.size > MAX_BYTES) {
    error.value = '发票文件不能超过 10MB'
    return null
  }
  if (picked.type && picked.type !== 'application/pdf' && !picked.name.toLowerCase().endsWith('.pdf')) {
    error.value = '只支持上传 PDF 格式的电子发票'
    return null
  }
  error.value = ''
  return picked
}

async function run(action: () => Promise<InvoiceApplication>, success: string) {
  busy.value = true
  error.value = ''
  try {
    const updated = await action()
    appStore.showSuccess(success)
    mode.value = 'view'
    emit('changed', updated)
  } catch (e) {
    error.value = errorMessage(e, '操作失败，请稍后重试')
  } finally {
    busy.value = false
  }
}

function markProcessing() {
  if (props.app) void run(() => invoiceAdminAPI.process(props.app!.id), '已标记为开票中，用户将无法撤销')
}

function submitIssue() {
  if (!props.app) return
  if (!invoiceNo.value.trim()) {
    error.value = '请填写发票号码'
    return
  }
  void run(() => invoiceAdminAPI.issue(props.app!.id, invoiceNo.value.trim(), adminNote.value.trim(), file.value), '已登记开具，用户可在开票记录中查看')
}

function submitReason() {
  if (!props.app) return
  if (!reason.value.trim()) {
    error.value = '请填写原因'
    return
  }
  const id = props.app.id
  if (mode.value === 'reject') void run(() => invoiceAdminAPI.reject(id, reason.value.trim()), '已驳回，订单已恢复为可开票')
  else void run(() => invoiceAdminAPI.void(id, reason.value.trim()), '已作废，订单已恢复为可开票')
}

function onReplace(event: Event) {
  const picked = pickFile(event)
  if (picked && props.app) void run(() => invoiceAdminAPI.uploadFile(props.app!.id, picked), '发票文件已更新')
}

async function download() {
  if (!props.app) return
  try {
    await invoiceAdminAPI.download(props.app)
  } catch (e) {
    appStore.showError(errorMessage(e, '下载失败'))
  }
}
</script>

<template>
  <BaseDialog :show="open" :title="app ? `开票申请 #${app.id} · 用户 ${app.user_email || app.user_id}` : ''" width="wide" @close="emit('close')">
    <div v-if="app" class="space-y-5">
      <ApplicationDetail :app="app" admin />

      <form v-if="mode === 'issue'" ref="actionForm" class="space-y-4 rounded-2xl border border-gray-100 p-4 dark:border-dark-700" @submit.prevent="submitIssue">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">登记开具</h4>
        <label class="block">
          <span class="input-label">发票号码 <span class="text-red-500">*</span></span>
          <input v-model="invoiceNo" class="input font-mono" maxlength="64" placeholder="数电票 20 位号码" autocomplete="off" />
        </label>
        <div>
          <span class="input-label">电子发票 PDF</span>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="fileInput?.click()"><Icon name="upload" size="sm" />{{ file ? '重新选择' : '选择文件' }}</button>
            <span class="min-w-0 truncate text-sm text-gray-600 dark:text-dark-300">{{ file ? file.name : '选填，不上传时只登记发票号码' }}</span>
            <button v-if="file" type="button" class="btn btn-ghost btn-sm" @click="file = null">移除</button>
          </div>
          <input ref="fileInput" type="file" accept="application/pdf,.pdf" class="hidden" @change="file = pickFile($event)" />
        </div>
        <label class="block">
          <span class="input-label">内部备注</span>
          <input v-model="adminNote" class="input" maxlength="500" placeholder="仅管理员可见，例如：已发送至用户邮箱" />
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="mode = 'view'">取消</button>
          <button type="submit" class="btn btn-primary" :disabled="busy">{{ busy ? '提交中…' : '确认已开具' }}</button>
        </div>
      </form>

      <form v-else-if="mode === 'reject' || mode === 'void'" ref="actionForm" class="space-y-4 rounded-2xl border border-red-100 p-4 dark:border-red-900/40" @submit.prevent="submitReason">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ mode === 'reject' ? '驳回申请' : '作废发票' }}</h4>
        <p class="text-xs text-gray-500 dark:text-dark-400">
          {{ mode === 'reject' ? '原因会展示给用户。驳回后订单恢复为可开票，用户可以修改后重新申请。' : '请先在税务系统完成红冲或作废，再在此登记。作废后订单恢复为可开票。' }}
        </p>
        <textarea v-model="reason" class="input min-h-[80px]" maxlength="500" :placeholder="mode === 'reject' ? '例如：抬头名称与税号不一致' : '例如：订单已退款，发票已红冲'"></textarea>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="mode = 'view'">取消</button>
          <button type="submit" class="btn btn-danger" :disabled="busy">{{ busy ? '提交中…' : mode === 'reject' ? '确认驳回' : '确认作废' }}</button>
        </div>
      </form>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <input ref="replaceInput" type="file" accept="application/pdf,.pdf" class="hidden" @change="onReplace" />
    </div>
    <template #footer>
      <div v-if="app && mode === 'view'" class="flex flex-wrap justify-end gap-2">
        <button v-if="app.has_file" type="button" class="btn btn-secondary" @click="download"><Icon name="download" size="sm" />下载 PDF</button>
        <button v-if="canVoid" type="button" class="btn btn-secondary" :disabled="busy" @click="replaceInput?.click()"><Icon name="upload" size="sm" />{{ app.has_file ? '替换 PDF' : '上传 PDF' }}</button>
        <button v-if="canVoid" type="button" class="btn btn-secondary text-red-600 dark:text-red-400" @click="mode = 'void'">作废</button>
        <button v-if="canIssue" type="button" class="btn btn-secondary text-red-600 dark:text-red-400" @click="mode = 'reject'">驳回</button>
        <button v-if="canProcess" type="button" class="btn btn-secondary" :disabled="busy" @click="markProcessing">标记开票中</button>
        <button v-if="canIssue" type="button" class="btn btn-primary" @click="mode = 'issue'">登记开具</button>
        <button type="button" class="btn btn-secondary" @click="emit('close')">关闭</button>
      </div>
    </template>
  </BaseDialog>
</template>
