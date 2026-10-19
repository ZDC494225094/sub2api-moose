<template>
  <AppLayout>
    <div class="guide-admin">
      <div class="guide-admin-head">
        <div>
          <h1 class="guide-admin-title">用户手册</h1>
          <p class="guide-admin-sub">
            文档中心（/docs）显示的手册内容。保存后立即生效；图片放在站内 /guide-images/ 目录下，正文里用 Markdown 图片语法引用。
          </p>
        </div>
        <div class="guide-admin-actions">
          <button class="btn-secondary" :disabled="busy" @click="confirmReset = true">恢复默认手册</button>
          <button class="btn-primary" :disabled="busy || !dirty" @click="save">保存</button>
        </div>
      </div>

      <p v-if="error" class="guide-admin-error">{{ error }}</p>
      <p v-if="notice" class="guide-admin-notice">{{ notice }}</p>

      <div class="guide-admin-meta">
        <span v-if="isDefault">当前使用的是内置默认手册（尚未保存过自定义内容）。</span>
        <span v-else-if="updatedAt">最后保存：{{ formatTime(updatedAt) }}</span>
        <span v-if="dirty" class="guide-admin-dirty">有未保存的修改</span>
      </div>

      <label class="guide-admin-label" for="guide-title">手册标题</label>
      <input id="guide-title" v-model="title" class="guide-admin-input" type="text" maxlength="200" />

      <div class="guide-admin-editor-head">
        <label class="guide-admin-label" for="guide-content">正文（Markdown）</label>
        <div class="guide-admin-toggle">
          <button :class="{ active: mode === 'edit' }" @click="mode = 'edit'">编辑</button>
          <button :class="{ active: mode === 'split' }" @click="mode = 'split'">对照预览</button>
          <button :class="{ active: mode === 'preview' }" @click="mode = 'preview'">预览</button>
        </div>
      </div>
      <div class="guide-admin-editor" :class="`mode-${mode}`">
        <textarea
          v-show="mode !== 'preview'"
          id="guide-content"
          v-model="content"
          class="guide-admin-textarea"
          spellcheck="false"
          placeholder="在这里编写手册正文（Markdown）"
        ></textarea>
        <div v-show="mode !== 'edit'" class="guide-admin-preview">
          <h2 class="guide-admin-preview-title">{{ title }}</h2>
          <!-- Admin-authored markdown preview, rendered through marked + DOMPurify. -->
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div class="markdown-body guide-preview-body" v-html="previewHtml"></div>
        </div>
      </div>
      <p class="guide-admin-hint">
        支持 Markdown：## 二级标题会进入左侧目录；截图用 ![说明](/guide-images/文件名.jpg) 引用站内图片，也可以填外部图片地址；代码块用三个反引号包裹。
      </p>
    </div>

    <BaseDialog
      :show="confirmReset"
      title="恢复默认手册"
      width="normal"
      :close-on-escape="!busy"
      :show-close-button="!busy"
      @close="confirmReset = false"
    >
      <p>将清空当前保存的标题与正文，文档中心恢复为内置默认手册。此操作不可撤销，确定恢复吗？</p>
      <template #footer>
        <button class="btn-secondary" :disabled="busy" @click="confirmReset = false">取消</button>
        <button class="btn-primary" :disabled="busy" @click="reset">确认恢复</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { guideAPI } from './api'
import { renderMarkdown } from './render'
import './guide.css'

const title = ref('')
const content = ref('')
const savedTitle = ref('')
const savedContent = ref('')
const updatedAt = ref<string | null>(null)
const isDefault = ref(true)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const mode = ref<'edit' | 'split' | 'preview'>('split')
const confirmReset = ref(false)

const dirty = computed(() => title.value !== savedTitle.value || content.value !== savedContent.value)
const previewHtml = computed(() => renderMarkdown(content.value).html)

function formatTime(v: string) {
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? v : d.toLocaleString('zh-CN', { hour12: false })
}

function apply(cfg: { title: string; content_md: string; updated_at: string | null; is_default: boolean }) {
  title.value = cfg.title
  content.value = cfg.content_md
  savedTitle.value = cfg.title
  savedContent.value = cfg.content_md
  updatedAt.value = cfg.updated_at
  isDefault.value = cfg.is_default
}

async function load() {
  busy.value = true
  error.value = ''
  try {
    apply(await guideAPI.getConfig())
  } catch {
    error.value = '加载手册配置失败，请刷新重试。'
  } finally {
    busy.value = false
  }
}

async function save() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    apply(await guideAPI.saveConfig(title.value.trim(), content.value))
    notice.value = '已保存，文档中心立即生效。'
  } catch {
    error.value = '保存失败，请检查标题与正文后重试。'
  } finally {
    busy.value = false
  }
}

async function reset() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    apply(await guideAPI.resetConfig())
    confirmReset.value = false
    notice.value = '已恢复为内置默认手册。'
  } catch {
    error.value = '恢复失败，请重试。'
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.guide-admin {
  max-width: 1200px;
  margin: 0 auto;
  padding: 8px 4px 32px;
}

.guide-admin-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  flex-wrap: wrap;
}

.guide-admin-title {
  font-size: 22px;
  font-weight: 700;
  margin: 0;
}

.guide-admin-sub {
  color: #64748b;
  font-size: 13.5px;
  margin: 6px 0 0;
  max-width: 720px;
}

.guide-admin-actions {
  display: flex;
  gap: 10px;
}

.btn-primary,
.btn-secondary {
  border-radius: 8px;
  padding: 8px 16px;
  font-size: 14px;
  cursor: pointer;
  border: 1px solid transparent;
}

.btn-primary {
  background: #2563eb;
  color: #fff;
}

.btn-primary:disabled {
  background: #94a3b8;
  cursor: not-allowed;
}

.btn-secondary {
  background: #fff;
  border-color: #cbd5e1;
  color: #334155;
}

.btn-secondary:disabled {
  color: #94a3b8;
  cursor: not-allowed;
}

.guide-admin-error {
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #b91c1c;
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
}

.guide-admin-notice {
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  color: #15803d;
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
}

.guide-admin-meta {
  display: flex;
  gap: 14px;
  color: #64748b;
  font-size: 13px;
  margin: 14px 0 4px;
}

.guide-admin-dirty {
  color: #b45309;
}

.guide-admin-label {
  display: block;
  font-weight: 600;
  font-size: 14px;
  margin: 14px 0 6px;
}

.guide-admin-input {
  width: 100%;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  padding: 9px 12px;
  font-size: 15px;
  background: #fff;
}

.guide-admin-editor-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}

.guide-admin-toggle {
  display: flex;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  overflow: hidden;
}

.guide-admin-toggle button {
  background: #fff;
  border: none;
  padding: 6px 12px;
  font-size: 13px;
  cursor: pointer;
  color: #475569;
}

.guide-admin-toggle button.active {
  background: #2563eb;
  color: #fff;
}

.guide-admin-editor {
  display: grid;
  grid-template-columns: 1fr;
  gap: 14px;
}

.guide-admin-editor.mode-split {
  grid-template-columns: 1fr 1fr;
}

.guide-admin-textarea {
  width: 100%;
  min-height: 560px;
  border: 1px solid #cbd5e1;
  border-radius: 10px;
  padding: 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13.5px;
  line-height: 1.7;
  background: #fff;
  resize: vertical;
}

.guide-admin-preview {
  border: 1px solid #e7dfce;
  border-radius: 10px;
  background: #fffdf7;
  padding: 18px 20px;
  min-height: 560px;
  overflow-y: auto;
  max-height: 78vh;
}

.guide-admin-preview-title {
  font-size: 19px;
  margin: 0 0 10px;
}

.guide-preview-body {
  line-height: 1.8;
  font-size: 14.5px;
  color: #2c3e55;
}

.guide-preview-body img {
  max-width: 100%;
  border-radius: 8px;
  border: 1px solid #e7dfce;
}

.guide-preview-body pre {
  background: #1f2937;
  color: #e5e7eb;
  border-radius: 8px;
  padding: 10px 12px;
  overflow-x: auto;
}

.guide-preview-body code {
  background: #f1ead8;
  border-radius: 4px;
  padding: 0 4px;
}

.guide-preview-body pre code {
  background: none;
  padding: 0;
}

.guide-preview-body table {
  border-collapse: collapse;
  width: 100%;
}

.guide-preview-body th,
.guide-preview-body td {
  border: 1px solid #e7dfce;
  padding: 6px 8px;
  text-align: left;
}

.guide-preview-body blockquote {
  background: #fbf3dc;
  border-left: 4px solid #d97706;
  margin: 10px 0;
  padding: 8px 12px;
  border-radius: 6px;
}

.guide-admin-hint {
  color: #94a3b8;
  font-size: 12.5px;
  margin-top: 10px;
}

@media (max-width: 900px) {
  .guide-admin-editor.mode-split {
    grid-template-columns: 1fr;
  }
}
</style>
