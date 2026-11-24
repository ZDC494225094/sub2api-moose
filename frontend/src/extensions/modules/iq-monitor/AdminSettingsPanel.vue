<script setup lang="ts">
import { computed, onMounted, ref, toRef, watch } from 'vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import UserPicker from './UserPicker.vue'
import { iqMonitorAdminAPI, type ConfigView, type Options, type UserOption, type Viewer } from './api'
import { errorMessage } from './format'
import { useConfigForm } from './useConfigForm'

const props = defineProps<{ config: ConfigView; options: Options | null }>()
const emit = defineEmits<{ saved: [cfg: ConfigView] }>()

const appStore = useAppStore()
const { form, saving, error, save } = useConfigForm(toRef(props, 'config'), cfg => emit('saved', cfg))

const probe = ref<UserOption | null>(props.options?.probe_user ?? null)
watch(() => props.options?.probe_user, (u) => { probe.value = u ?? null })

const roundsPerDay = computed(() => form.value.interval_minutes > 0 ? Math.floor(1440 / form.value.interval_minutes) : 0)
const bots = computed(() => props.options?.bots ?? [])

function pickProbe(u: UserOption) {
  probe.value = u
  form.value.probe_user_id = u.id
}

function toggleBot(id: number, on: boolean) {
  const ids = form.value.notify.bot_ids.filter(b => b !== id)
  if (on) ids.push(id)
  form.value.notify.bot_ids = ids
}

function submit() {
  void save((cfg) => {
    if (cfg.interval_minutes < 10 || cfg.interval_minutes > 720) return '轮次间隔需在 10 到 720 分钟之间'
    if (cfg.thresholds.unstable >= cfg.thresholds.normal) return '「偶有失败」阈值必须小于「正常」阈值'
    return ''
  })
}

// Viewer allowlist, used when visibility is "allowlist".
const viewers = ref<Viewer[]>([])
const viewerNote = ref('')
const viewerError = ref('')

async function loadViewers() {
  try {
    viewers.value = await iqMonitorAdminAPI.viewers()
    viewerError.value = ''
  } catch (e) {
    viewerError.value = errorMessage(e, '读取名单失败')
  }
}

async function addViewer(u: UserOption) {
  try {
    await iqMonitorAdminAPI.addViewer(u.id, viewerNote.value.trim())
    viewerNote.value = ''
    await loadViewers()
  } catch (e) {
    appStore.showError(errorMessage(e, '添加失败'))
  }
}

async function removeViewer(id: number) {
  try {
    await iqMonitorAdminAPI.removeViewer(id)
    await loadViewers()
  } catch (e) {
    appStore.showError(errorMessage(e, '移除失败'))
  }
}

onMounted(loadViewers)
</script>

<template>
  <div class="space-y-6">
    <form class="space-y-6" @submit.prevent="submit">
      <section class="card space-y-5 p-6">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">探测账号与请求</h2>
        <div>
          <span class="input-label">探测账号</span>
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <div class="min-w-0 flex-1 rounded-xl border border-gray-100 px-3 py-2 text-sm dark:border-dark-700">
              <template v-if="probe">{{ probe.email }} <span class="text-xs text-gray-400">#{{ probe.id }}</span></template>
              <template v-else-if="form.probe_user_id">用户 #{{ form.probe_user_id }}</template>
              <span v-else class="text-gray-400">未选择</span>
            </div>
            <div class="sm:w-72"><UserPicker placeholder="搜索并更换探测账号" @pick="pickProbe" /></div>
          </div>
          <p class="input-hint">建议使用一个专用账号。所有测试请求用它的令牌经本站网关发出，费用和使用记录都计在它名下，请保证余额充足。更换账号后需要重新选择各目标的令牌。</p>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <label>
            <span class="input-label">轮次间隔（分钟）</span>
            <input v-model.number="form.interval_minutes" type="number" min="10" max="720" class="input" />
            <span class="input-hint">10–720 分钟，每天约 {{ roundsPerDay }} 轮。</span>
          </label>
          <label>
            <span class="input-label">单次请求超时（秒）</span>
            <input v-model.number="form.timeout_seconds" type="number" min="30" max="900" class="input" />
          </label>
          <label>
            <span class="input-label">请求出错重试次数</span>
            <input v-model.number="form.request_retries" type="number" min="0" max="5" class="input" />
            <span class="input-hint">仅对超时、限流、5xx 等请求错误重试，答错不算。</span>
          </label>
          <label>
            <span class="input-label">输出 token 上限</span>
            <input v-model.number="form.max_output_tokens" type="number" min="0" max="200000" step="1000" class="input" />
            <span class="input-hint">0 表示不发送 max_tokens。达到上限的回复标记为「被截断」。</span>
          </label>
          <label>
            <span class="input-label">记录保留天数</span>
            <input v-model.number="form.retention_days" type="number" min="1" max="365" class="input" />
            <span class="input-hint">过期的轮次、结果和回复会被删除，排队中的投稿保留。</span>
          </label>
        </div>
      </section>

      <section class="card space-y-5 p-6">
        <div>
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">状态判定</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">按最近 24 小时通过率判定。开启逻辑测试时以逻辑测试为准，只开画图测试时以画图测试为准。</p>
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <label>
            <span class="input-label">「正常」阈值（%）</span>
            <input v-model.number="form.thresholds.normal" type="number" min="1" max="100" step="0.1" class="input" />
            <span class="input-hint">通过率不低于此值为「正常」。</span>
          </label>
          <label>
            <span class="input-label">「偶有失败」阈值（%）</span>
            <input v-model.number="form.thresholds.unstable" type="number" min="0" max="100" step="0.1" class="input" />
            <span class="input-hint">介于两值之间为「偶有失败」，低于此值为「疑似降智」。</span>
          </label>
        </div>
        <div class="flex items-start justify-between gap-4 rounded-2xl border border-gray-100 p-4 dark:border-dark-700">
          <span>
            <span class="block text-sm font-medium text-gray-900 dark:text-white">进入「疑似降智」时推送机器人通知</span>
            <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">每个模型进入该状态时推送一次，恢复后再次进入才会重新推送。需要开启「消息机器人」插件。</span>
          </span>
          <Toggle v-model="form.notify.enabled" aria-label="推送机器人通知" />
        </div>
        <div v-if="form.notify.enabled" class="space-y-2">
          <p v-if="!bots.length" class="text-sm text-gray-500">没有可用的机器人，请先在「消息机器人」中添加。</p>
          <label v-for="b in bots" :key="b.id" class="flex items-center gap-3 text-sm">
            <input type="checkbox" class="h-4 w-4" :checked="form.notify.bot_ids.includes(b.id)" @change="toggleBot(b.id, ($event.target as HTMLInputElement).checked)" />
            <span class="text-gray-900 dark:text-white">{{ b.name }}</span>
            <span class="text-xs text-gray-400">{{ b.platform }} · {{ b.targets }} 个推送目标</span>
          </label>
        </div>
      </section>

      <section class="card space-y-4 p-6">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">用户页面可见范围</h2>
        <div class="grid gap-3 sm:grid-cols-2">
          <label
            v-for="option in [
              { value: 'all', title: '全部登录用户', desc: '所有登录用户都能看到降智监测页面' },
              { value: 'allowlist', title: '指定用户', desc: '只有下方名单中的用户能看到（管理员始终可见）' },
            ] as const"
            :key="option.value"
            class="flex cursor-pointer gap-3 rounded-2xl border p-4 transition-colors"
            :class="form.visibility === option.value ? 'border-primary-400 bg-primary-50/50 dark:border-primary-600 dark:bg-primary-900/10' : 'border-gray-100 hover:border-gray-200 dark:border-dark-700'"
          >
            <input v-model="form.visibility" type="radio" class="mt-1 h-4 w-4" :value="option.value" name="iq-visibility" />
            <span>
              <span class="block text-sm font-medium text-gray-900 dark:text-white">{{ option.title }}</span>
              <span class="mt-0.5 block text-xs text-gray-500 dark:text-dark-400">{{ option.desc }}</span>
            </span>
          </label>
        </div>
      </section>

      <p v-if="error" role="alert" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">{{ error }}</p>
      <div class="flex justify-end">
        <button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存设置' }}</button>
      </div>
    </form>

    <section class="card p-6">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">可见名单（{{ viewers.length }}）</h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
        可见范围为「指定用户」时生效<template v-if="config.visibility === 'all'">，当前对全部登录用户开放</template>。
      </p>
      <div class="mt-4 flex flex-col gap-2 sm:flex-row">
        <div class="flex-1"><UserPicker placeholder="搜索用户并添加" @pick="addViewer" /></div>
        <input v-model="viewerNote" class="input sm:w-56" maxlength="200" placeholder="备注（可选，先填再选用户）" />
      </div>
      <p v-if="viewerError" role="alert" class="mt-3 text-sm text-rose-600">{{ viewerError }}</p>
      <ul v-if="viewers.length" class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="v in viewers" :key="v.user_id" class="flex items-center gap-3 py-2 text-sm">
          <span class="min-w-0 flex-1 truncate text-gray-900 dark:text-white">
            {{ v.email }} <span class="text-xs text-gray-400">#{{ v.user_id }}<template v-if="v.username"> · {{ v.username }}</template></span>
            <span v-if="v.note" class="ml-2 text-xs text-gray-500">{{ v.note }}</span>
          </span>
          <button type="button" class="btn btn-secondary btn-sm text-rose-600" @click="removeViewer(v.user_id)">移除</button>
        </li>
      </ul>
    </section>
  </div>
</template>
