<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { iqMonitorAdminAPI, type EffortOption, type Options, type Target, type TargetInput } from './api'
import { errorMessage, formatMultiplier } from './format'

const props = defineProps<{ show: boolean; target: Target | null; options: Options | null }>()
const emit = defineEmits<{ close: []; saved: [] }>()

const form = reactive<TargetInput>({ group_id: 0, model: '', display_name: '', reasoning_effort: '', api_key_id: 0, enabled: true })
const models = ref<string[]>([])
const modelsError = ref('')
const efforts = ref<EffortOption[]>([])
const saving = ref(false)
const error = ref('')

const groups = computed(() => props.options?.groups ?? [])
const keys = computed(() => (props.options?.keys ?? []).filter(k => !form.group_id || k.group_ids.includes(form.group_id)))
const selectedEffort = computed(() => efforts.value.find(e => e.value === form.reasoning_effort))

watch(() => props.show, (open) => {
  if (!open) return
  error.value = ''
  const t = props.target
  Object.assign(form, t
    ? { group_id: t.group_id, model: t.model, display_name: t.display_name, reasoning_effort: t.reasoning_effort, api_key_id: t.api_key_id, enabled: t.enabled }
    : { group_id: 0, model: '', display_name: '', reasoning_effort: '', api_key_id: 0, enabled: true })
}, { immediate: true })

// A key bound to another group cannot reach this group's accounts.
watch(() => form.group_id, () => {
  if (form.api_key_id && !keys.value.some(k => k.id === form.api_key_id)) form.api_key_id = 0
  if (!form.api_key_id && keys.value.length === 1) form.api_key_id = keys.value[0].id
})

watch(() => form.api_key_id, async (id) => {
  models.value = []
  modelsError.value = ''
  if (!id) return
  try {
    const res = await iqMonitorAdminAPI.models(id)
    models.value = res.models
    modelsError.value = res.error ?? ''
  } catch (e) {
    modelsError.value = errorMessage(e, '读取模型列表失败')
  }
}, { immediate: true })

let effortSeq = 0
watch(() => [form.group_id, form.model.trim()] as const, async ([groupId, model]) => {
  const seq = ++effortSeq
  if (!groupId || !model) {
    efforts.value = []
    return
  }
  try {
    const res = await iqMonitorAdminAPI.efforts(groupId, model)
    if (seq !== effortSeq) return
    efforts.value = res.items
    if (form.reasoning_effort && !res.items.some(i => i.value === form.reasoning_effort)) form.reasoning_effort = ''
  } catch {
    if (seq === effortSeq) efforts.value = []
  }
}, { immediate: true })

function effortLabel(opt: EffortOption) {
  if (opt.preview.denied) return `${opt.value}（分组策略不允许）`
  if (opt.preview.sent && opt.preview.sent !== opt.value) return `${opt.value}（实际发送 ${opt.preview.sent}）`
  return opt.value
}

async function save() {
  error.value = ''
  if (!form.group_id || !form.api_key_id || !form.model.trim()) {
    error.value = '请选择分组、令牌并填写模型'
    return
  }
  saving.value = true
  try {
    await iqMonitorAdminAPI.saveTarget({ ...form, model: form.model.trim(), display_name: form.display_name.trim() }, props.target?.id)
    emit('saved')
  } catch (e) {
    error.value = errorMessage(e, '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog :show="show" :title="target ? '编辑监测目标' : '添加监测目标'" width="normal" @close="emit('close')">
    <form id="iq-target-form" class="space-y-4" @submit.prevent="save">
      <label class="block">
        <span class="input-label">分组</span>
        <select v-model.number="form.group_id" class="input">
          <option :value="0">请选择</option>
          <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}（{{ g.platform }} · {{ formatMultiplier(g.rate_multiplier) }}）{{ g.status !== 'active' ? ' · 已停用' : '' }}</option>
        </select>
      </label>
      <label class="block">
        <span class="input-label">探测令牌</span>
        <select v-model.number="form.api_key_id" class="input" :disabled="!options?.probe_user">
          <option :value="0">请选择</option>
          <option v-for="k in keys" :key="k.id" :value="k.id">{{ k.name || `#${k.id}` }}{{ k.status !== 'active' ? '（未启用）' : '' }}</option>
        </select>
        <span class="input-hint">
          <template v-if="!options?.probe_user">请先在「通用设置」中选择探测账号。</template>
          <template v-else-if="form.group_id && !keys.length">探测账号 {{ options.probe_user.email }} 没有绑定该分组的令牌，请先为它创建一个。</template>
          <template v-else>请求以该令牌发出，费用和使用记录计入探测账号。</template>
        </span>
      </label>
      <label class="block">
        <span class="input-label">模型</span>
        <input v-model="form.model" class="input font-mono" list="iq-target-models" maxlength="200" placeholder="例如 claude-sonnet-4-5" />
        <datalist id="iq-target-models">
          <option v-for="m in models" :key="m" :value="m" />
        </datalist>
        <span v-if="modelsError" class="input-hint text-amber-600">读取模型列表失败：{{ modelsError }}，可直接填写。</span>
      </label>
      <label class="block">
        <span class="input-label">显示名称</span>
        <input v-model="form.display_name" class="input" maxlength="100" placeholder="留空则显示模型名" />
      </label>
      <label class="block">
        <span class="input-label">思考深度</span>
        <select v-model="form.reasoning_effort" class="input" :disabled="!efforts.length">
          <option value="">不发送（使用模型默认）</option>
          <option v-for="opt in efforts" :key="opt.value" :value="opt.value" :disabled="opt.preview.denied">{{ effortLabel(opt) }}</option>
        </select>
        <span class="input-hint">
          <template v-if="selectedEffort?.preview.note">{{ selectedEffort.preview.note }}</template>
          <template v-else-if="!efforts.length">选择分组并填写模型后可选。</template>
          <template v-else>以 reasoning_effort 字段发送；网关按分组策略和上游协议转换后的值显示为「实际发送」。</template>
        </span>
      </label>
      <div class="flex items-center justify-between rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
        <span class="text-sm font-medium text-gray-900 dark:text-white">启用</span>
        <Toggle v-model="form.enabled" aria-label="启用该监测目标" />
      </div>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
    </form>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">取消</button>
        <button type="submit" form="iq-target-form" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存' }}</button>
      </div>
    </template>
  </BaseDialog>
</template>
