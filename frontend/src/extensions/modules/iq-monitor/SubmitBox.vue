<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { iqMonitorAPI, type MySubmissions } from './api'
import { errorMessage, topicClasses, topicLabels } from './format'

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
  <section class="card p-4">
    <div class="flex items-center justify-between">
      <h2 class="text-sm font-semibold text-gray-900 dark:text-white">投稿主题</h2>
      <span v-if="data?.enabled" class="font-mono text-xs tabular-nums text-gray-400">今日 {{ data.used }}/{{ data.quota }}</span>
    </div>
    <template v-if="data">
      <p v-if="!data.enabled" class="mt-2 text-xs text-gray-500 dark:text-dark-400">投稿暂未开放。</p>
      <form v-else class="mt-3" @submit.prevent="submit">
        <label for="iq-topic" class="sr-only">画图主题</label>
        <div class="relative">
          <input
            id="iq-topic"
            v-model="topic"
            class="input pr-14 text-sm"
            :maxlength="data.max_length * 2"
            placeholder="例如：一只橘猫在雨夜屋顶追萤火虫"
            :disabled="remaining === 0"
          />
          <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 font-mono text-[11px] tabular-nums" :class="tooLong ? 'text-rose-600' : 'text-gray-400'">{{ [...topic].length }}/{{ data.max_length }}</span>
        </div>
        <p v-if="remaining === 0" class="mt-2 text-xs leading-5 text-amber-600 dark:text-amber-400">今日 {{ data.quota }} 次投稿已用完，明天再来吧。</p>
        <p v-else class="mt-2 text-xs leading-5 text-gray-500 dark:text-dark-400">建议「主体 + 动作 + 场景」。经 AI 审核后排队，最早通过的主题优先使用。</p>
        <div class="mt-3 flex items-center justify-end gap-3">
          <span v-if="notice" class="min-w-0 flex-1 text-xs text-emerald-600 dark:text-emerald-400">{{ notice }}</span>
          <span v-else-if="error" role="alert" class="min-w-0 flex-1 text-xs text-rose-600">{{ error }}</span>
          <button type="submit" class="btn btn-primary btn-sm shrink-0" :disabled="!canSubmit">{{ busy ? '提交中…' : '提交' }}</button>
        </div>
      </form>

      <ul v-if="data.items.length" class="mt-3 space-y-2 border-t border-gray-100 pt-3 dark:border-dark-700">
        <li v-for="item in data.items" :key="item.id" class="text-xs">
          <div class="flex items-start gap-2">
            <p class="min-w-0 flex-1 break-words text-gray-700 dark:text-dark-200">{{ item.topic }}</p>
            <span class="shrink-0 rounded-full px-1.5 py-0.5 text-[11px]" :class="topicClasses[item.status]">{{ topicLabels[item.status] }}</span>
          </div>
          <p v-if="item.status === 'rejected' && item.reject_reason" class="mt-0.5 text-rose-600 dark:text-rose-400">原因：{{ item.reject_reason }}</p>
          <p v-else-if="item.status === 'failed'" class="mt-0.5 text-amber-600">审核多次失败，本条不占用次数</p>
        </li>
      </ul>
    </template>
    <p v-else-if="error" role="alert" class="mt-2 text-xs text-rose-600">{{ error }}</p>
  </section>
</template>
