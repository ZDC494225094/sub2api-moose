<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { iqMonitorAPI, type MySubmissions } from './api'
import { errorMessage, formatTime, topicClasses, topicLabels } from './format'

const data = ref<MySubmissions | null>(null)
const topic = ref('')
const busy = ref(false)
const error = ref('')
const notice = ref('')

const remaining = computed(() => Math.max(0, (data.value?.quota ?? 0) - (data.value?.used ?? 0)))
const tooLong = computed(() => !!data.value && [...topic.value].length > data.value.max_length)
const canSubmit = computed(() => !!data.value?.enabled && remaining.value > 0 && topic.value.trim() !== '' && !tooLong.value && !busy.value)

async function load() {
  try {
    data.value = await iqMonitorAPI.submissions()
  } catch (e) {
    error.value = errorMessage(e, '读取投稿失败')
  }
}

async function submit() {
  if (!canSubmit.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await iqMonitorAPI.submit(topic.value.trim())
    topic.value = ''
    notice.value = '已提交，审核通过后会按顺序用于后续轮次。'
    await load()
  } catch (e) {
    error.value = errorMessage(e, '提交失败')
  } finally {
    busy.value = false
  }
}

defineExpose({ reload: load })
onMounted(load)
</script>

<template>
  <section class="card p-5">
    <h2 class="text-base font-semibold text-gray-900 dark:text-white">投稿画图主题</h2>
    <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
      格式建议「主体 + 动作 + 场景」，例如「一只橘猫在雨夜的屋顶上追萤火虫」。投稿经 AI 审核后排队，最早通过的主题优先使用。
    </p>
    <template v-if="data">
      <p v-if="!data.enabled" class="mt-4 text-sm text-gray-500">投稿暂未开放。</p>
      <form v-else class="mt-4 space-y-2" @submit.prevent="submit">
        <label for="iq-topic" class="sr-only">画图主题</label>
        <div class="flex flex-col gap-2 sm:flex-row">
          <input id="iq-topic" v-model="topic" class="input flex-1" :maxlength="data.max_length * 2" placeholder="主体 + 动作 + 场景" :disabled="remaining === 0" />
          <button type="submit" class="btn btn-primary shrink-0" :disabled="!canSubmit">{{ busy ? '提交中…' : '提交' }}</button>
        </div>
        <p class="text-xs" :class="tooLong ? 'text-rose-600' : 'text-gray-400'">
          今日剩余 {{ remaining }}/{{ data.quota }} 次 · {{ [...topic].length }}/{{ data.max_length }} 字<template v-if="remaining === 0">，明天再来吧</template>
        </p>
      </form>
      <p v-if="notice" class="mt-2 text-sm text-emerald-600 dark:text-emerald-400">{{ notice }}</p>
      <p v-if="error" role="alert" class="mt-2 text-sm text-rose-600">{{ error }}</p>

      <ul v-if="data.items.length" class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="item in data.items" :key="item.id" class="flex flex-wrap items-start justify-between gap-2 py-2.5 text-sm">
          <div class="min-w-0 flex-1">
            <p class="break-words text-gray-900 dark:text-white">{{ item.topic }}</p>
            <p v-if="item.status === 'rejected' && item.reject_reason" class="mt-0.5 text-xs text-rose-600 dark:text-rose-400">原因：{{ item.reject_reason }}</p>
            <p v-else-if="item.status === 'failed'" class="mt-0.5 text-xs text-amber-600">审核多次失败，本条不占用次数</p>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <span class="text-xs text-gray-400">{{ formatTime(item.created_at) }}</span>
            <span class="rounded-full px-2 py-0.5 text-xs" :class="topicClasses[item.status]">{{ topicLabels[item.status] }}</span>
          </div>
        </li>
      </ul>
    </template>
    <p v-else-if="error" role="alert" class="mt-4 text-sm text-rose-600">{{ error }}</p>
  </section>
</template>
