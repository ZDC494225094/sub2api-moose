<script setup lang="ts">
import { onMounted, ref, toRef } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import QuestionDialog from './QuestionDialog.vue'
import { iqMonitorAdminAPI, type ConfigView, type Question } from './api'
import { errorMessage, formatTime } from './format'
import { useConfigForm } from './useConfigForm'

const props = defineProps<{ config: ConfigView }>()
const emit = defineEmits<{ saved: [cfg: ConfigView] }>()

const appStore = useAppStore()
const { form, saving, error, save } = useConfigForm(toRef(props, 'config'), cfg => emit('saved', cfg))

const questions = ref<Question[]>([])
const loadError = ref('')
const dialogOpen = ref(false)
const editing = ref<Question | null>(null)
const deleting = ref<Question | null>(null)

async function loadQuestions() {
  try {
    questions.value = await iqMonitorAdminAPI.questions()
    loadError.value = ''
  } catch (e) {
    loadError.value = errorMessage(e, '读取题库失败')
  }
}

function submit() {
  void save((cfg) => {
    if (!cfg.logic.prompt_template.includes('{question}')) return '提示词模板必须包含 {question}'
    if (cfg.logic.enabled && !questions.value.some(q => q.enabled)) return '题库中没有启用的题目，无法开启逻辑测试'
    return ''
  })
}

function openEdit(q: Question | null) {
  editing.value = q
  dialogOpen.value = true
}

async function onSaved() {
  dialogOpen.value = false
  await loadQuestions()
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await iqMonitorAdminAPI.deleteQuestion(deleting.value.id)
    deleting.value = null
    await loadQuestions()
  } catch (e) {
    appStore.showError(errorMessage(e, '删除失败'))
  }
}

onMounted(loadQuestions)
</script>

<template>
  <div class="space-y-6">
    <form class="card space-y-5 p-6" @submit.prevent="submit">
      <div class="flex items-start justify-between gap-4">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">逻辑测试</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">每轮从题库中选一道题（优先选最久没出过的），所有目标回答同一道题。</p>
        </div>
        <Toggle v-model="form.logic.enabled" aria-label="启用逻辑测试" />
      </div>
      <label class="block">
        <span class="input-label">提示词模板</span>
        <textarea v-model="form.logic.prompt_template" class="input min-h-[120px] font-mono text-xs" maxlength="8000"></textarea>
        <span class="input-hint">
          作为唯一的用户消息发送，不附加系统提示词。{question} 会被替换为题目。
          <button type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="form.logic.prompt_template = config.default_logic_prompt">恢复默认</button>
        </span>
      </label>
      <label class="block max-w-xs">
        <span class="input-label">答错后重问次数</span>
        <input v-model.number="form.logic.retry_wrong" type="number" min="0" max="3" step="1" class="input" />
        <span class="input-hint">0–3 次，用同一道题重新提问；之后答对记为「重试后正确」。每次回答都会保存并计入用量。</span>
      </label>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div class="flex justify-end">
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
      </div>
    </form>

    <section class="card">
      <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">题库</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">共 {{ questions.length }} 道，启用 {{ questions.filter(q => q.enabled).length }} 道。每道题可以有多个可接受答案。</p>
        </div>
        <button type="button" class="btn btn-primary" @click="openEdit(null)">添加题目</button>
      </div>
      <p v-if="loadError" role="alert" class="m-5 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ loadError }}</p>
      <p v-if="!questions.length && !loadError" class="p-5 text-sm text-gray-500">题库为空，添加题目后才能开启逻辑测试。</p>
      <ul class="divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="q in questions" :key="q.id" class="flex flex-wrap items-start gap-3 px-5 py-3" :class="q.enabled ? '' : 'opacity-60'">
          <div class="min-w-0 flex-1">
            <p class="line-clamp-3 whitespace-pre-wrap text-sm text-gray-900 dark:text-white">{{ q.question }}</p>
            <p class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
              答案
              <span v-for="a in q.answers" :key="a" class="rounded bg-gray-100 px-1.5 py-0.5 font-mono dark:bg-dark-700">{{ a }}</span>
              <span class="ml-2">上次出题 {{ formatTime(q.last_used_at) }}</span>
              <span v-if="!q.enabled" class="ml-2">· 已停用</span>
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="openEdit(q)">编辑</button>
          <button type="button" class="btn btn-secondary btn-sm text-rose-600" @click="deleting = q">删除</button>
        </li>
      </ul>
    </section>

    <QuestionDialog :show="dialogOpen" :question="editing" @close="dialogOpen = false" @saved="onSaved" />
    <ConfirmDialog
      :show="!!deleting"
      title="删除这道题？"
      message="已有轮次保存了题目快照，历史结果不受影响。"
      confirm-text="删除"
      danger
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </div>
</template>
