<script setup lang="ts">
import { computed, ref } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { archiveAPI, type DetectorConfig, type DetectorKind, type DetectorStatus, type DetectorTestResult } from './api'
import { errorMessage, formatBytes, formatTime } from './format'

const detector = defineModel<DetectorConfig>({ required: true })
const props = defineProps<{ keySet: boolean; status?: DetectorStatus | null; disabled?: boolean }>()

const kindOptions: { value: DetectorKind; label: string; hint?: string }[] = [
  { value: 'private_person', label: '姓名' },
  { value: 'phone', label: '电话' },
  { value: 'email', label: '邮箱' },
  { value: 'private_address', label: '地址' },
  { value: 'account', label: '证件号 / 账号' },
  { value: 'payment_card', label: '银行卡' },
  { value: 'common_secret', label: '密钥 / 口令' },
  { value: 'ip_address', label: 'IP 地址' },
  { value: 'private_date', label: '日期', hint: '会命中大量普通日期' },
  { value: 'url', label: '网址', hint: '会命中文档和代码中的链接' },
]
const kindLabel = (kind: string) => kindOptions.find(k => k.value === kind)?.label || kind

const apiKey = ref('')
const testing = ref(false)
const testError = ref('')
const result = ref<DetectorTestResult | null>(null)

// The saved key travels only when the admin types a new one.
function setKey(value: string) {
  apiKey.value = value
  detector.value.api_key = value.trim() || undefined
  if (value.trim()) detector.value.clear_api_key = false
}

function clearKey() {
  apiKey.value = ''
  detector.value.api_key = undefined
  detector.value.clear_api_key = true
}

const keyState = computed(() => {
  if (detector.value.api_key) return '保存后替换为新密钥'
  if (detector.value.clear_api_key) return '保存后清除已保存的密钥'
  return props.keySet ? '已保存密钥，留空保持不变' : '尚未保存密钥'
})

function toggleKind(kind: DetectorKind, on: boolean) {
  const kinds = detector.value.kinds.filter(k => k !== kind)
  if (on) kinds.push(kind)
  detector.value.kinds = kinds
}

const confidencePercent = computed({
  get: () => Math.round(detector.value.min_confidence * 100),
  set: (v: number) => {
    detector.value.min_confidence = Math.min(1, Math.max(0.05, (Number(v) || 80) / 100))
  },
})

async function test() {
  testing.value = true
  testError.value = ''
  result.value = null
  try {
    result.value = await archiveAPI.testDetector({
      url: detector.value.url,
      api_key: detector.value.api_key,
      timeout_seconds: detector.value.timeout_seconds,
      min_confidence: detector.value.min_confidence,
      kinds: detector.value.kinds,
    })
  } catch (e) {
    testError.value = errorMessage(e, '测试失败')
  } finally {
    testing.value = false
  }
}
</script>

<template>
  <div class="space-y-4 rounded-xl border border-gray-100 p-4 dark:border-dark-700">
    <div class="flex items-center justify-between gap-4">
      <div class="min-w-0">
        <p class="text-sm font-medium text-gray-900 dark:text-white">模型脱敏服务（AstrLink）</p>
        <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
          在内置规则之外，调用独立部署的 AstrLink 隐私模型识别自然语言中的姓名、地址等信息。仅对上传时选择脱敏的存储桶生效。
          部署见 <code class="font-mono">deploy/docker-compose.privacy-detector.yml</code>。
        </p>
      </div>
      <Toggle v-model="detector.enabled" aria-label="启用模型脱敏服务" />
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <label class="sm:col-span-2">
        <span class="input-label">服务地址</span>
        <input v-model.trim="detector.url" class="input font-mono" placeholder="http://privacy-detector:8090" aria-label="脱敏服务地址" />
      </label>
      <label class="sm:col-span-2">
        <span class="input-label">服务密钥</span>
        <div class="flex gap-2">
          <input
            :value="apiKey"
            type="password"
            autocomplete="new-password"
            class="input flex-1 font-mono"
            :placeholder="keySet ? '••••••••（留空保持不变）' : 'PRIVACY_DETECTOR_API_KEY 的值'"
            aria-label="脱敏服务密钥"
            @input="setKey(($event.target as HTMLInputElement).value)"
          />
          <button v-if="keySet && !detector.clear_api_key" type="button" class="btn btn-secondary" @click="clearKey">清除</button>
        </div>
        <span class="input-hint">{{ keyState }}。密钥加密保存，不会返回到页面。</span>
      </label>
      <label>
        <span class="input-label">置信度阈值（%）</span>
        <input v-model.number="confidencePercent" type="number" min="5" max="100" class="input" aria-label="置信度阈值" />
        <span class="input-hint">低于阈值的模型结果不脱敏，默认 80</span>
      </label>
      <label>
        <span class="input-label">单次请求超时（秒）</span>
        <input v-model.number="detector.timeout_seconds" type="number" min="5" max="600" class="input" aria-label="单次请求超时" />
      </label>
    </div>

    <div>
      <p class="input-label">识别并脱敏的类型</p>
      <div class="mt-1 flex flex-wrap gap-x-4 gap-y-2">
        <label v-for="kind in kindOptions" :key="kind.value" class="inline-flex items-center gap-1.5 text-sm text-gray-700 dark:text-dark-200" :title="kind.hint">
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300"
            :checked="detector.kinds.includes(kind.value)"
            :aria-label="`识别${kind.label}`"
            @change="toggleKind(kind.value, ($event.target as HTMLInputElement).checked)"
          />
          {{ kind.label }}<span v-if="kind.hint" class="text-xs text-gray-400">（{{ kind.hint }}）</span>
        </label>
      </div>
    </div>

    <div class="flex items-center justify-between gap-4 rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
      <div class="min-w-0">
        <p class="text-sm font-medium text-gray-900 dark:text-white">服务不可用时仍上传</p>
        <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">
          默认关闭：服务不可用时暂停上传并自动重试，保证上传内容经过模型识别。开启后改为只用内置规则脱敏并继续上传。
        </p>
      </div>
      <Toggle v-model="detector.fail_open" aria-label="服务不可用时仍上传" />
    </div>

    <div class="flex flex-wrap items-center gap-3">
      <button type="button" class="btn btn-secondary" :disabled="disabled || testing || !detector.url" @click="test">
        <Icon name="play" size="sm" />{{ testing ? '测试中…' : '测试脱敏服务' }}
      </button>
      <span class="text-xs text-gray-500 dark:text-dark-400">用示例文本和当前填写的设置（未保存也可）调用服务</span>
    </div>
    <p v-if="testError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ testError }}</p>
    <div v-if="result" class="space-y-2 rounded-lg border border-gray-100 p-3 text-sm dark:border-dark-700" data-testid="detector-test-result">
      <p v-if="!result.ok" class="text-red-600 dark:text-red-400">连接失败（{{ result.elapsed_ms }} ms）：{{ result.error }}</p>
      <template v-else>
        <p class="text-emerald-700 dark:text-emerald-300">连接正常，耗时 {{ result.elapsed_ms }} ms<span v-if="result.model">，模型 {{ result.model }}</span></p>
        <p class="text-xs text-gray-500 dark:text-dark-400">原文</p>
        <p class="break-words font-mono text-xs">{{ result.sample }}</p>
        <p class="text-xs text-gray-500 dark:text-dark-400">上传内容（内置规则 + 模型）</p>
        <p class="break-words font-mono text-xs">{{ result.redacted }}</p>
        <ul v-if="result.findings?.length" class="flex flex-wrap gap-1.5">
          <li
            v-for="(f, i) in result.findings"
            :key="i"
            class="rounded-full px-2 py-0.5 text-xs"
            :class="f.accepted ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400'"
          >
            {{ kindLabel(f.label) }} · {{ f.text }} · {{ Math.round(f.score * 100) }}%<span v-if="!f.accepted">（未采用）</span>
          </li>
        </ul>
        <p v-else class="text-xs text-gray-500 dark:text-dark-400">模型没有额外发现</p>
      </template>
    </div>

    <div v-if="detector.enabled && status?.enabled" class="grid gap-x-4 gap-y-1 text-xs text-gray-500 sm:grid-cols-3 dark:text-dark-400" data-testid="detector-status">
      <span>本节点请求 {{ status.requests }} 次</span>
      <span>送检 {{ status.texts }} 段 / {{ formatBytes(status.bytes) }}</span>
      <span>缓存命中 {{ status.cache_hits }} 段</span>
      <span :class="{ 'text-red-600 dark:text-red-400': status.errors }">失败 {{ status.errors }} 次</span>
      <span v-if="status.fail_open_skips" class="text-amber-600">未经模型上传 {{ status.fail_open_skips }} 段</span>
      <span v-if="status.last_error" class="break-words text-red-600 sm:col-span-3 dark:text-red-400">
        最近错误（{{ formatTime(status.last_error_at || null) }}）：{{ status.last_error }}
      </span>
    </div>
  </div>
</template>
