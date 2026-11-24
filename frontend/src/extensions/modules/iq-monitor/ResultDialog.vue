<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DrawingFrame from './DrawingFrame.vue'
import { iqMonitorAPI, iqMonitorAdminAPI, type ResultDetail } from './api'
import { errorMessage, formatMs, formatTime, formatTokens, statusLabels } from './format'

const props = defineProps<{ resultId: number | null; admin?: boolean }>()
const emit = defineEmits<{ close: [] }>()

const detail = ref<ResultDetail | null>(null)
const loading = ref(false)
const error = ref('')
const fullscreen = ref<HTMLElement | null>(null)

watch(() => props.resultId, async (id) => {
  detail.value = null
  error.value = ''
  if (!id) return
  loading.value = true
  try {
    detail.value = props.admin ? await iqMonitorAdminAPI.result(id) : await iqMonitorAPI.result(id)
  } catch (e) {
    error.value = errorMessage(e, '读取结果失败')
  } finally {
    loading.value = false
  }
}, { immediate: true })

const r = computed(() => detail.value?.result)
const title = computed(() => {
  if (!r.value) return '测试结果'
  const kind = r.value.kind === 'logic' ? '逻辑测试' : '画图测试'
  return `${r.value.display_name || r.value.model} · ${kind}`
})
const verdictClass = computed(() => {
  switch (r.value?.status) {
    case 'pass': return 'text-emerald-600 dark:text-emerald-400'
    case 'retry_pass': return 'text-lime-600 dark:text-lime-400'
    case 'fail': case 'error': return 'text-rose-600 dark:text-rose-400'
    default: return 'text-gray-500'
  }
})

function openFullscreen() {
  void fullscreen.value?.requestFullscreen?.()
}
</script>

<template>
  <BaseDialog :show="resultId !== null" :title="title" width="extra-wide" @close="emit('close')">
    <div v-if="loading" class="py-12 text-center text-sm text-gray-500">加载中…</div>
    <p v-else-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    <div v-else-if="detail && r" class="space-y-5">
      <div class="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm">
        <span class="font-semibold" :class="verdictClass">{{ statusLabels[r.status] }}</span>
        <span v-if="r.reason" class="text-gray-600 dark:text-dark-300">{{ r.reason }}</span>
        <span class="text-gray-500 dark:text-dark-400">分组 {{ r.group_name || '—' }}</span>
        <span v-if="r.reasoning_effort" class="text-gray-500 dark:text-dark-400">
          思考深度 {{ r.reasoning_effort }}<template v-if="r.sent_effort && r.sent_effort !== r.reasoning_effort">（实际发送 {{ r.sent_effort }}）</template>
        </span>
        <span class="text-gray-500 dark:text-dark-400">首字 {{ formatMs(r.first_token_ms) }} · 总耗时 {{ formatMs(r.total_ms) }}</span>
        <span class="text-gray-500 dark:text-dark-400">输入 {{ formatTokens(r.input_tokens) }} / 输出 {{ formatTokens(r.output_tokens) }} tokens · {{ r.request_count }} 次请求</span>
        <span v-if="r.truncated" class="rounded-full bg-amber-50 px-2 py-0.5 text-xs text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">达到输出上限被截断</span>
        <span class="text-gray-400">{{ formatTime(r.created_at) }}</span>
      </div>

      <template v-if="r.kind === 'logic'">
        <section class="rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
          <h4 class="text-xs font-medium uppercase tracking-wide text-gray-400">题目</h4>
          <p class="mt-2 whitespace-pre-wrap text-sm text-gray-900 dark:text-white">{{ detail.question }}</p>
          <h4 class="mt-4 text-xs font-medium uppercase tracking-wide text-gray-400">可接受答案</h4>
          <div class="mt-2 flex flex-wrap gap-2">
            <span v-for="a in detail.answers" :key="a" class="rounded-lg bg-gray-100 px-2 py-1 font-mono text-xs text-gray-700 dark:bg-dark-700 dark:text-dark-200">{{ a }}</span>
          </div>
        </section>
        <section v-for="resp in detail.responses" :key="resp.id" class="rounded-2xl border p-4"
          :class="resp.correct ? 'border-emerald-200 dark:border-emerald-800/50' : 'border-gray-100 dark:border-dark-700'">
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
            <span class="font-medium text-gray-700 dark:text-dark-200">
              第 {{ resp.attempt }} 次回答
              <span :class="resp.correct ? 'text-emerald-600' : 'text-rose-600'">· {{ resp.error ? '请求出错' : resp.correct ? '正确' : '错误' }}</span>
            </span>
            <span class="text-gray-400">提取答案：{{ resp.extracted || '（未找到）' }} · {{ formatMs(resp.total_ms) }} · {{ resp.output_tokens }} tokens<template v-if="resp.truncated"> · 被截断</template></span>
          </div>
          <p v-if="resp.error" class="mt-2 text-sm text-rose-600">{{ resp.error }}</p>
          <pre class="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-3 text-xs leading-5 text-gray-800 dark:bg-dark-900 dark:text-dark-200">{{ resp.content || '（无内容）' }}</pre>
        </section>
      </template>

      <template v-else>
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="text-sm text-gray-600 dark:text-dark-300">主题：{{ detail.topic }}</p>
          <div v-if="detail.preview_url" class="flex gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="openFullscreen">全屏查看</button>
            <a :href="detail.preview_url" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm">新窗口打开</a>
          </div>
        </div>
        <div v-if="detail.preview_url" ref="fullscreen" class="mx-auto max-w-4xl overflow-hidden rounded-2xl ring-1 ring-gray-200 dark:ring-dark-700">
          <DrawingFrame :src="detail.preview_url" :title="title" interactive />
        </div>
        <details v-for="resp in detail.responses" :key="resp.id" class="rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
          <summary class="cursor-pointer text-xs text-gray-500">模型原始回复（{{ resp.output_tokens }} tokens<template v-if="resp.finish_reason"> · {{ resp.finish_reason }}</template>）</summary>
          <p v-if="resp.error" class="mt-2 text-sm text-rose-600">{{ resp.error }}</p>
          <pre class="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-3 text-xs leading-5 text-gray-800 dark:bg-dark-900 dark:text-dark-200">{{ resp.content || '（无内容）' }}</pre>
        </details>
      </template>
    </div>
  </BaseDialog>
</template>
