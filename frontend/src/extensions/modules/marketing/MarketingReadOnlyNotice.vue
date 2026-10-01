<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
const props = withDefaults(defineProps<{ admin?: boolean }>(), { admin: false })
const { locale } = useI18n()
const message = computed(() => locale.value.startsWith('zh')
  ? (props.admin ? '营销功能已暂停：管理页面仅供查询，暂不可创建、修改或删除；历史数据仍保留。' : '营销功能已暂停：暂不可发起新抽奖，历史次数和抽奖记录仍保留。')
  : (props.admin ? 'Marketing is paused: administration is read-only. Creating, editing and deleting are unavailable; existing data is preserved.' : 'Marketing is paused: new draws are unavailable. Existing chances and draw history are preserved.'))
</script>

<template>
  <p role="status" data-testid="marketing-readonly" class="mb-4 rounded-xl bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-950 dark:text-amber-200">{{ message }}</p>
</template>
