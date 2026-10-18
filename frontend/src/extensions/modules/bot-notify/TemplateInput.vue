<script setup lang="ts">
import { nextTick, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
const model = defineModel<string>({ required: true })
const props = defineProps<{ label: string }>()
const input = ref<HTMLTextAreaElement>()
const placeholders = [
  ['{channel_status}', '全部渠道状态：从 sub 内部渠道监控服务读取'],
  ['{channel_status_abnormal}', '异常渠道：从 sub 内部渠道监控服务读取'],
  ['{announcements}', '最新五条有效公告：从 sub 内部公告服务读取'],
  ['{site_name}', '站点名称：从 sub 内部设置读取'],
  ['{now}', '当前服务器时间'],
] as const
async function insert(value: string) {
  const start = input.value?.selectionStart ?? model.value.length
  const end = input.value?.selectionEnd ?? start
  model.value = model.value.slice(0, start) + value + model.value.slice(end)
  await nextTick()
  input.value?.focus()
  input.value?.setSelectionRange(start + value.length, start + value.length)
}
</script>
<template>
  <div class="template-input min-w-0 space-y-3">
    <label class="block text-sm font-medium text-gray-700 dark:text-gray-200">
      {{ props.label }}
      <textarea
        ref="input"
        v-model="model"
        class="input mt-2 min-h-36 w-full resize-y font-mono !text-[13px] !leading-7"
        rows="5"
        maxlength="32000"
      />
    </label>
    <details
      class="overflow-hidden rounded-xl border border-gray-100 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-800/40"
    >
      <summary
        class="flex cursor-pointer list-none items-center gap-2 px-3.5 py-3 text-xs font-medium text-gray-600 dark:text-gray-300"
      >
        <Icon name="lightbulb" size="sm" class="shrink-0 text-primary-600 dark:text-primary-400" />
        可用占位符（5）<span class="ml-auto text-[11px] font-normal text-gray-400">展开插入</span>
        <Icon name="chevronDown" size="xs" class="placeholder-chevron" />
      </summary>
      <div class="border-t border-gray-100 p-3.5 dark:border-dark-700">
        <p class="mb-3 text-[11px] leading-5 text-gray-400">点击插入到光标处，其他文字按原样发送。</p>
        <div class="grid gap-x-5 gap-y-3 sm:grid-cols-2">
          <div v-for="[value, description] in placeholders" :key="value" class="min-w-0 space-y-1.5">
            <button type="button" class="placeholder-button" @click="insert(value)">{{ value }}</button>
            <p class="text-[11px] leading-5 text-gray-500 dark:text-gray-400">{{ description }}</p>
          </div>
        </div>
      </div>
    </details>
  </div>
</template>

<style scoped>
summary::-webkit-details-marker {
  display: none;
}
details[open] .placeholder-chevron {
  transform: rotate(180deg);
}
.placeholder-button {
  @apply max-w-full rounded-md border border-primary-100 bg-white px-2 py-1.5 text-left font-mono text-[11px] leading-5 text-primary-700 transition-colors hover:border-primary-300 hover:bg-primary-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-primary-800/60 dark:bg-primary-950/30 dark:text-primary-400 dark:hover:bg-primary-950/60;
  overflow-wrap: anywhere;
}
@media (max-width: 639px) {
  .placeholder-button {
    min-height: 40px;
  }
  .template-input textarea.input {
    font-size: 16px !important;
  }
}
</style>
