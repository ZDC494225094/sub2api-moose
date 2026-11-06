<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { archiveAPI, type ArchiveBucket, type ArchiveBucketInput, type TestResult } from './api'
import { errorMessage } from './format'

const props = defineProps<{
  show: boolean
  /** null creates a new bucket. */
  bucket: ArchiveBucket | null
  today: string
}>()

const emit = defineEmits<{
  close: []
  saved: [bucket: ArchiveBucket]
}>()

const blank = (): ArchiveBucketInput => ({
  name: '',
  provider: 'aws',
  bucket: '',
  region: 'us-east-1',
  endpoint: '',
  prefix: '',
  force_path_style: false,
  enabled: true,
  priority: 100,
})

const form = reactive<ArchiveBucketInput>(blank())
const accessKey = ref('')
const secretKey = ref('')
const sessionToken = ref('')
const clearCredentials = ref(false)
const clearToken = ref(false)
const saving = ref(false)
const testing = ref(false)
const error = ref('')
const test = ref<TestResult | null>(null)

const editing = computed(() => !!props.bucket)
const title = computed(() => (editing.value ? `编辑存储桶 · ${props.bucket?.name || props.bucket?.bucket}` : '新增存储桶'))
const samplePath = computed(() => {
  const prefix = (form.prefix || '').replace(/^s3:\/\/[^/]*\/?/, '').replace(/^\/+|\/+$/g, '')
  const bucket = form.bucket || (form.prefix.startsWith('s3://') ? form.prefix.slice(5).split('/')[0] : '') || 'bucket'
  return `s3://${bucket}/${prefix ? prefix + '/' : ''}${props.today}/Opus5-${props.today}.zip`
})

watch(
  () => [props.show, props.bucket] as const,
  ([show, bucket]) => {
    if (!show) return
    Object.assign(form, blank(), bucket
      ? {
          id: bucket.id, name: bucket.name, provider: bucket.provider, bucket: bucket.bucket, region: bucket.region,
          endpoint: bucket.endpoint, prefix: bucket.prefix, force_path_style: bucket.force_path_style,
          enabled: bucket.enabled, priority: bucket.priority,
        }
      : { id: undefined })
    accessKey.value = ''
    secretKey.value = ''
    sessionToken.value = ''
    clearCredentials.value = false
    clearToken.value = false
    error.value = ''
    test.value = null
  },
  { immediate: true }
)

function payload(): ArchiveBucketInput {
  return {
    ...form,
    priority: Number(form.priority) || 0,
    access_key_id: accessKey.value.trim(),
    secret_access_key: secretKey.value.trim(),
    session_token: sessionToken.value.trim(),
    clear_credentials: clearCredentials.value,
    clear_session_token: clearToken.value,
  }
}

async function runTest() {
  testing.value = true
  error.value = ''
  test.value = null
  try {
    test.value = await archiveAPI.testBucket(payload())
  } catch (e) {
    error.value = errorMessage(e, '连接测试失败')
  } finally {
    testing.value = false
  }
}

async function save() {
  saving.value = true
  error.value = ''
  try {
    const input = payload()
    const saved = props.bucket ? await archiveAPI.updateBucket(props.bucket.id, input) : await archiveAPI.createBucket(input)
    emit('saved', saved)
  } catch (e) {
    error.value = errorMessage(e, '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog :show="show" :title="title" width="wide" @close="emit('close')">
    <form class="space-y-5" @submit.prevent="save">
      <div class="grid gap-4 sm:grid-cols-2">
        <label>
          <span class="input-label">名称</span>
          <input v-model="form.name" class="input" maxlength="64" placeholder="如 主存储 / R2 备份" autocomplete="off" />
        </label>
        <label>
          <span class="input-label">服务商</span>
          <select v-model="form.provider" class="input">
            <option value="aws">Amazon Web Services / Amazon S3</option>
            <option value="s3-compatible">S3 兼容服务（R2 / OSS / MinIO 等）</option>
          </select>
        </label>
        <label>
          <span class="input-label">Bucket / 存储桶 <span class="text-red-500">*</span></span>
          <input v-model="form.bucket" class="input font-mono" placeholder="incoming-04-20260624" autocomplete="off" />
        </label>
        <label>
          <span class="input-label">Region / 地区代码</span>
          <input v-model="form.region" class="input font-mono" placeholder="us-east-1" autocomplete="off" />
        </label>
        <label class="sm:col-span-2">
          <span class="input-label">Endpoint / 服务地址<span v-if="form.provider === 's3-compatible'" class="text-red-500"> *</span></span>
          <input
            v-model="form.endpoint"
            class="input font-mono"
            :placeholder="form.provider === 'aws' ? '可留空，使用 AWS 默认地址' : 'https://<account>.r2.cloudflarestorage.com'"
            autocomplete="off"
          />
        </label>
        <label class="sm:col-span-2">
          <span class="input-label">S3 路径前缀</span>
          <input v-model="form.prefix" class="input font-mono" placeholder="可留空；也可直接粘贴 s3://bucket/前缀" autocomplete="off" />
          <span class="input-hint break-all">上传到「前缀/日期/模型-日期.zip」，例如 {{ samplePath }}</span>
        </label>
      </div>

      <fieldset class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <legend class="px-1 text-sm font-semibold text-gray-700 dark:text-gray-200">访问凭证</legend>
        <p class="mb-3 text-xs text-gray-500 dark:text-dark-400">
          加密保存，读取时只显示是否已设置。全部留空则使用服务器环境凭证（如 IAM 角色）。
        </p>
        <div class="grid gap-4 sm:grid-cols-2">
          <label>
            <span class="input-label">Access Key ID / AK</span>
            <input v-model="accessKey" class="input font-mono" autocomplete="off" :placeholder="bucket?.access_key_id || '未设置'" />
          </label>
          <label>
            <span class="input-label">Secret Access Key / SK</span>
            <input
              v-model="secretKey"
              type="password"
              class="input font-mono"
              autocomplete="new-password"
              :placeholder="bucket?.secret_access_key_set ? '已设置，留空保持不变' : '未设置'"
            />
          </label>
          <label class="sm:col-span-2">
            <span class="input-label">Session Token</span>
            <input
              v-model="sessionToken"
              type="password"
              class="input font-mono"
              autocomplete="new-password"
              :placeholder="bucket?.session_token_set ? '已设置，留空保持不变' : '可选，临时凭证使用'"
            />
          </label>
        </div>
        <div v-if="bucket?.secret_access_key_set || bucket?.session_token_set" class="mt-3 flex flex-wrap gap-x-6 gap-y-2 text-sm text-gray-700 dark:text-gray-300">
          <label v-if="bucket?.session_token_set" class="flex items-center gap-2">
            <input v-model="clearToken" type="checkbox" />清除已保存的 Session Token
          </label>
          <label v-if="bucket?.secret_access_key_set" class="flex items-center gap-2">
            <input v-model="clearCredentials" type="checkbox" />清除已保存的全部凭证
          </label>
        </div>
      </fieldset>

      <div class="grid gap-4 sm:grid-cols-3">
        <label>
          <span class="input-label">优先级</span>
          <input v-model.number="form.priority" type="number" min="0" max="10000" class="input" />
          <span class="input-hint">数值越小越先使用</span>
        </label>
        <div class="flex items-center justify-between gap-3 rounded-xl border border-gray-200 px-4 py-3 dark:border-dark-700 sm:col-span-2">
          <div class="min-w-0">
            <p class="text-sm font-medium text-gray-900 dark:text-white">启用</p>
            <p class="text-xs text-gray-500 dark:text-dark-400">关闭后不再上传新压缩包，已上传的数据仍可读取</p>
          </div>
          <Toggle v-model="form.enabled" />
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300 sm:col-span-3">
          <input v-model="form.force_path_style" type="checkbox" />使用 Path-Style 访问（MinIO 等需要）
        </label>
      </div>

      <div v-if="test" class="rounded-xl border p-3 text-sm" :class="test.ok ? 'border-emerald-200 bg-emerald-50/60 dark:border-emerald-800/60 dark:bg-emerald-950/20' : 'border-red-200 bg-red-50/60 dark:border-red-900/60 dark:bg-red-950/20'">
        <p class="mb-1 font-medium" :class="test.ok ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'">
          {{ test.ok ? '连接测试通过，可以写入存储桶' : '连接测试未通过' }}
        </p>
        <p v-for="s in test.steps" :key="s.name" class="break-all" :class="s.ok ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-600 dark:text-red-300'">
          {{ s.ok ? '✓' : '✗' }} {{ s.name }}<span v-if="s.error">：{{ s.error }}</span>
        </p>
      </div>
      <p v-if="error" role="alert" class="flex items-start gap-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
        <Icon name="exclamationCircle" size="sm" class="mt-0.5 shrink-0" /><span class="break-words">{{ error }}</span>
      </p>
    </form>

    <template #footer>
      <button type="button" class="btn btn-secondary mr-auto" :disabled="testing || saving" @click="runTest">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': testing }" />测试连接
      </button>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="emit('close')">取消</button>
      <button type="button" class="btn btn-primary" :disabled="saving || testing" @click="save">
        {{ saving ? '保存中…' : editing ? '保存修改' : '添加存储桶' }}
      </button>
    </template>
  </BaseDialog>
</template>
