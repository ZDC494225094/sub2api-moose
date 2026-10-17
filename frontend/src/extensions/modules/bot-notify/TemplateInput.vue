<script setup lang="ts">
import { nextTick, ref } from 'vue'
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
  <div class="space-y-2">
    <label class="block text-sm">{{ props.label }}<textarea ref="input" v-model="model" class="input mt-2 w-full font-mono" rows="5" maxlength="32000" /></label>
    <p class="text-xs text-gray-500">可用占位符（点击插入；其他文字按原样发送）</p>
    <div v-for="[value, description] in placeholders" :key="value" class="flex flex-wrap items-center gap-2 text-xs">
      <button type="button" class="rounded bg-primary-50 px-2 py-1 font-mono text-primary-700" @click="insert(value)">{{ value }}</button><span class="text-gray-500">{{ description }}</span>
    </div>
  </div>
</template>
