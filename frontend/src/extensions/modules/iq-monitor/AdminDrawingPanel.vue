<script setup lang="ts">
import { computed, onMounted, ref, toRef, watch } from 'vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { iqMonitorAdminAPI, type ConfigView, type Options, type Topic, type TopicStatus } from './api'
import { errorMessage, formatTime, topicClasses, topicLabels } from './format'
import { useConfigForm } from './useConfigForm'

const props = defineProps<{ config: ConfigView; options: Options | null }>()
const emit = defineEmits<{ saved: [cfg: ConfigView] }>()

const appStore = useAppStore()
const { form, saving, error, save } = useConfigForm(toRef(props, 'config'), cfg => emit('saved', cfg))

const topicCount = computed(() => form.value.drawing.topics.split('\n').filter(l => l.trim()).length)
const reviewModels = ref<string[]>([])

watch(() => form.value.submissions.review_api_key_id, async (id) => {
  reviewModels.value = []
  if (!id) return
  try {
    reviewModels.value = (await iqMonitorAdminAPI.models(id)).models
  } catch { /* free text still works */ }
}, { immediate: true })

function submit() {
  void save((cfg) => {
    if (!cfg.drawing.prompt_template.includes('{topic}')) return '画图提示词模板必须包含 {topic}'
    if (!cfg.submissions.review_prompt.includes('{topic}')) return '审核提示词必须包含 {topic}'
    if (cfg.submissions.enabled && (!cfg.submissions.review_api_key_id || !cfg.submissions.review_model.trim())) return '开放投稿前请选择审核令牌并填写审核模型'
    return ''
  })
}

// Submission moderation list.
const status = ref<'' | TopicStatus>('')
const page = ref(1)
const pageSize = 20
const topics = ref<Topic[]>([])
const total = ref(0)
const listError = ref('')
const deleting = ref<Topic | null>(null)
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

async function loadTopics() {
  try {
    const res = await iqMonitorAdminAPI.topics({ status: status.value || undefined, page: page.value, page_size: pageSize })
    topics.value = res.items
    total.value = res.total
    listError.value = ''
  } catch (e) {
    listError.value = errorMessage(e, '读取投稿失败')
  }
}

watch(status, () => {
  page.value = 1
  void loadTopics()
})

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await iqMonitorAdminAPI.deleteTopic(deleting.value.id)
    deleting.value = null
    await loadTopics()
  } catch (e) {
    appStore.showError(errorMessage(e, '删除失败'))
  }
}

function go(p: number) {
  page.value = p
  void loadTopics()
}

onMounted(loadTopics)
</script>

<template>
  <div class="space-y-6">
    <form class="space-y-6" @submit.prevent="submit">
      <section class="card space-y-5 p-6">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">画图测试</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
              让模型用内联 SVG 画同一个主题的动画。只判断有没有画出东西：空回复、被截断、没有 SVG 或拒绝作答记为失败，不做评分。
            </p>
          </div>
          <Toggle v-model="form.drawing.enabled" aria-label="启用画图测试" />
        </div>
        <label class="block">
          <span class="input-label">提示词模板</span>
          <textarea v-model="form.drawing.prompt_template" class="input min-h-[120px] font-mono text-xs" maxlength="8000"></textarea>
          <span class="input-hint">
            {topic} 会被替换为本轮主题。
            <button type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="form.drawing.prompt_template = config.default_draw_prompt">恢复默认</button>
          </span>
        </label>
        <label class="block">
          <span class="input-label">主题列表（每行一个，共 {{ topicCount }} 个）</span>
          <textarea v-model="form.drawing.topics" class="input min-h-[200px] text-sm" placeholder="主体 + 动作 + 场景，例如：一只橘猫在雨夜的屋顶上追萤火虫"></textarea>
          <span class="input-hint">
            有审核通过的用户投稿时优先使用最早通过的一条，否则从这里随机选一行。
            <button type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="form.drawing.topics = config.default_topics">填入默认主题</button>
          </span>
        </label>
      </section>

      <section class="card space-y-5 p-6">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">用户投稿</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">登录用户可以投稿画图主题，后台用 AI 审核，通过后排队使用。审核失败 5 次的投稿标记为「审核失败」，不占用次数。</p>
          </div>
          <Toggle v-model="form.submissions.enabled" aria-label="开放用户投稿" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <label>
            <span class="input-label">每人每日投稿次数</span>
            <input v-model.number="form.submissions.daily_quota" type="number" min="0" max="50" class="input" />
            <span class="input-hint">按站点时区计算自然日。</span>
          </label>
          <label>
            <span class="input-label">主题长度上限（字）</span>
            <input v-model.number="form.submissions.max_length" type="number" min="10" max="200" class="input" />
          </label>
          <label>
            <span class="input-label">审核令牌</span>
            <select v-model.number="form.submissions.review_api_key_id" class="input">
              <option :value="0">未选择</option>
              <option v-for="k in options?.keys ?? []" :key="k.id" :value="k.id">{{ k.name || `#${k.id}` }}</option>
            </select>
            <span class="input-hint">探测账号的令牌，审核费用计入探测账号。</span>
          </label>
          <label>
            <span class="input-label">审核模型</span>
            <input v-model="form.submissions.review_model" class="input font-mono" list="iq-review-models" maxlength="200" />
            <datalist id="iq-review-models">
              <option v-for="m in reviewModels" :key="m" :value="m" />
            </datalist>
          </label>
        </div>
        <label class="block">
          <span class="input-label">审核提示词</span>
          <textarea v-model="form.submissions.review_prompt" class="input min-h-[200px] font-mono text-xs" maxlength="8000"></textarea>
          <span class="input-hint">
            {topic} 会被替换为投稿内容。模型需返回 JSON：{"approved": true/false, "reason": "…"}，拒绝原因会展示给投稿人。
            <button type="button" class="text-primary-600 hover:underline dark:text-primary-400" @click="form.submissions.review_prompt = config.default_review_prompt">恢复默认</button>
          </span>
        </label>
      </section>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div class="flex justify-end">
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
      </div>
    </form>

    <section class="card">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 p-5 dark:border-dark-700">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">投稿记录（{{ total }}）</h2>
        <select v-model="status" class="input w-auto" aria-label="按状态筛选">
          <option value="">全部状态</option>
          <option v-for="(label, key) in topicLabels" :key="key" :value="key">{{ label }}</option>
        </select>
      </div>
      <p v-if="listError" role="alert" class="m-5 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ listError }}</p>
      <p v-else-if="!topics.length" class="p-5 text-sm text-gray-500">暂无投稿。</p>
      <ul class="divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="t in topics" :key="t.id" class="flex flex-wrap items-start gap-3 px-5 py-3 text-sm">
          <div class="min-w-0 flex-1">
            <p class="break-words text-gray-900 dark:text-white">{{ t.topic }}</p>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
              {{ t.username || t.email || `用户 #${t.user_id}` }}<template v-if="t.username && t.email">（{{ t.email }}）</template>
              · {{ formatTime(t.created_at) }}
              <template v-if="t.round_id"> · 用于第 {{ t.round_id }} 轮</template>
            </p>
            <p v-if="t.reject_reason" class="mt-0.5 text-xs text-rose-600 dark:text-rose-400">拒绝原因：{{ t.reject_reason }}</p>
            <p v-if="t.last_error && t.status !== 'approved' && t.status !== 'used'" class="mt-0.5 text-xs text-amber-600">审核出错（第 {{ t.attempts }} 次）：{{ t.last_error }}</p>
          </div>
          <span class="shrink-0 rounded-full px-2 py-0.5 text-xs" :class="topicClasses[t.status]">{{ topicLabels[t.status] }}</span>
          <button type="button" class="btn btn-secondary btn-sm text-rose-600" @click="deleting = t">删除</button>
        </li>
      </ul>
      <div v-if="pages > 1" class="flex items-center justify-end gap-2 border-t border-gray-100 px-5 py-3 text-sm dark:border-dark-700">
        <button type="button" class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="go(page - 1)">上一页</button>
        <span class="tabular-nums text-gray-500">{{ page }} / {{ pages }}</span>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="page >= pages" @click="go(page + 1)">下一页</button>
      </div>
    </section>

    <ConfirmDialog
      :show="!!deleting"
      title="删除这条投稿？"
      message="删除后不再排队使用，已用过的轮次仍保留主题文字。删除当天的投稿会让投稿人当日的次数恢复一次。"
      confirm-text="删除"
      danger
      @confirm="confirmDelete"
      @cancel="deleting = null"
    />
  </div>
</template>
