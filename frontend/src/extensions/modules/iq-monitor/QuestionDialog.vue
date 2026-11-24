<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { iqMonitorAdminAPI, type Question } from './api'
import { errorMessage } from './format'

const props = defineProps<{ show: boolean; question: Question | null }>()
const emit = defineEmits<{ close: []; saved: [] }>()

const form = reactive({ question: '', answers: '', enabled: true })
const saving = ref(false)
const error = ref('')

watch(() => props.show, (open) => {
  if (!open) return
  error.value = ''
  const q = props.question
  Object.assign(form, q ? { question: q.question, answers: q.answers.join('\n'), enabled: q.enabled } : { question: '', answers: '', enabled: true })
}, { immediate: true })

async function save() {
  const answers = form.answers.split('\n').map(a => a.trim()).filter(Boolean)
  if (!form.question.trim() || !answers.length) {
    error.value = '请填写题目和至少一个答案'
    return
  }
  saving.value = true
  error.value = ''
  try {
    await iqMonitorAdminAPI.saveQuestion({ question: form.question.trim(), answers, enabled: form.enabled }, props.question?.id)
    emit('saved')
  } catch (e) {
    error.value = errorMessage(e, '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog :show="show" :title="question ? '编辑题目' : '添加题目'" width="wide" @close="emit('close')">
    <form id="iq-question-form" class="space-y-4" @submit.prevent="save">
      <label class="block">
        <span class="input-label">题目</span>
        <textarea v-model="form.question" class="input min-h-[160px]" maxlength="4000" placeholder="题目内容，会替换提示词模板中的 {question}"></textarea>
      </label>
      <label class="block">
        <span class="input-label">可接受答案（每行一个）</span>
        <textarea v-model="form.answers" class="input min-h-[96px] font-mono" placeholder="例如：&#10;42&#10;四十二"></textarea>
        <span class="input-hint">
          判定时取回复最后一行「答案：」或 \boxed{} 中的内容，忽略全半角、大小写和标点，数字按数值比较（0.5 与 1/2、3.0 与 3 视为相同），带单位的回答（如「42个」）可匹配纯数字答案。
          答案只能完整匹配，不会匹配到更长数字的一部分。
        </span>
      </label>
      <div class="flex items-center justify-between rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
        <span class="text-sm font-medium text-gray-900 dark:text-white">参与出题</span>
        <Toggle v-model="form.enabled" aria-label="参与出题" />
      </div>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">取消</button>
        <button type="submit" form="iq-question-form" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
      </div>
    </template>
  </BaseDialog>
</template>
