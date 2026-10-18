<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformLogo from './PlatformLogo.vue'
import TemplateInput from './TemplateInput.vue'
import {
  botAPI,
  type AiConfig,
  type AiKeyOption,
  type AiRecord,
  type BotEntry,
  type Rule,
  type Status,
  type BotMessage,
} from './api'

const bots = ref<BotEntry[]>([])
const statuses = ref<Record<number, Status>>({})
const selected = ref<BotEntry | null>(null)
const tab = ref('机器人配置')
const token = ref('')
const chats = ref('')
const testChat = ref('')
const rules = ref<Rule[]>([])
const messages = ref<BotMessage[]>([])
const total = ref(0)
const page = ref(1)
const chatFilter = ref('')
const keywordFilter = ref('')
const error = ref('')
const notice = ref('')
const busy = ref(false)
const loaded = ref(false)
const draft = ref<Rule | null>(null)
const creating = ref(false)
const newName = ref('')
const newPlatform = ref('telegram')
const newToken = ref('')
const newAppID = ref('')
const newSecret = ref('')
const aiConfig = ref<AiConfig>({
  enabled: false,
  api_key_id: null,
  model: '',
  max_input_chars: 4000,
  max_output_tokens: 800,
  cooldown_seconds: 10,
  daily_limit: 100,
  allow_recharge: true,
  allow_technical: true,
})
const aiKeys = ref<AiKeyOption[]>([])
const aiKeysError = ref('')
const aiModelSuggestions = [
  'gpt-6.1-sol',
  'gpt-6-astra',
  'gpt-6-luna',
  'gpt-5.6-luna',
  'claude-sonnet-5-5',
  'claude-opus-5-5',
  'claude-fable-5',
]
const aiRecords = ref<AiRecord[]>([])
const aiTotal = ref(0)
const aiPage = ref(1)
const aiKeyword = ref('')
const aiBlocked = ref('')
let timer: ReturnType<typeof setInterval> | undefined
let revision = 0

const tabs = ['机器人配置', '收到的消息', '关键词回复', 'AI 客服'] as const
const matchLabels: Record<Rule['match_type'], string> = {
  contains: '包含',
  exact: '完全匹配',
  command: '命令',
}

const runningBots = computed(
  () =>
    bots.value.filter(
      (bot) =>
        statuses.value[bot.id]?.polling ||
        statuses.value[bot.id]?.pushing ||
        statuses.value[bot.id]?.announcing,
    ).length,
)
const targetCount = computed(() => chats.value.split('\n').filter((value) => value.trim()).length)
const tabIcons = { 机器人配置: 'cog', 收到的消息: 'inbox', 关键词回复: 'chat', 'AI 客服': 'lightbulb' } as const
function platformLabel(platform: string) {
  return platform === 'telegram' ? 'Telegram' : platform === 'qq' ? 'QQ' : platform
}
function ruleLabel(id: number | null) {
  return id ? rules.value.find((rule) => rule.id === id)?.keyword || '#' + id : '未命中规则'
}
function showCreateBot() {
  error.value = ''
  creating.value = true
}

async function action(fn: () => Promise<void>, success = '') {
  if (busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await fn()
    notice.value = success
  } catch (e: unknown) {
    // The shared apiClient rejects with a flat { status, code, message }
    // object (no .response), so the server's reason lives on e.message.
    const detail = e as { response?: { data?: { message?: string } }; message?: string }
    error.value = detail.response?.data?.message || detail.message || '操作失败，请重试。'
  } finally {
    busy.value = false
  }
}

async function refreshStatuses() {
  const entries = await Promise.all(
    bots.value.map(async (bot) => {
      try {
        return [bot.id, await botAPI.status(bot.id)] as const
      } catch {
        return null
      }
    }),
  )
  const next: Record<number, Status> = { ...statuses.value }
  for (const entry of entries) if (entry) next[entry[0]] = entry[1]
  statuses.value = next
}

async function loadBots() {
  bots.value = await botAPI.listBots()
  await refreshStatuses()
}

async function createBot() {
  const credential =
    newPlatform.value === 'qq'
      ? JSON.stringify({ app_id: newAppID.value.trim(), client_secret: newSecret.value.trim() })
      : newToken.value.trim()
  const created = await botAPI.createBot({
    name: newName.value.trim(),
    platform: newPlatform.value,
    bot_token: credential,
    receive_enabled: true,
    push_enabled: false,
    chat_ids: [],
    interval_minutes: 60,
    push_template: '{channel_status}',
    announce_sync_enabled: false,
  })
  creating.value = false
  newName.value = ''
  newToken.value = ''
  newAppID.value = ''
  newSecret.value = ''
  await loadBots()
  await openBot(created)
}

async function removeBot(bot: BotEntry) {
  if (!window.confirm(`删除机器人「${bot.name}」？它的关键词规则和已收消息会一并删除。`)) return
  await action(async () => {
    await botAPI.deleteBot(bot.id)
    if (selected.value?.id === bot.id) closeBot()
    await loadBots()
  }, '机器人已删除')
}

async function openBot(bot: BotEntry) {
  const fresh = await botAPI.getBot(bot.id)
  selected.value = fresh
  token.value = ''
  chats.value = fresh.chat_ids.join('\n')
  testChat.value = ''
  tab.value = '机器人配置'
  draft.value = null
  page.value = 1
  chatFilter.value = ''
  keywordFilter.value = ''
  rules.value = await botAPI.rules(fresh.id)
  await loadAi()
  await loadMessages()
}

function closeBot() {
  selected.value = null
  rules.value = []
  messages.value = []
  total.value = 0
  aiRecords.value = []
  aiTotal.value = 0
}

async function loadAi() {
  if (!selected.value) return
  // Load each resource independently: a failing key list must not take the
  // saved configuration down with it, and its error is shown inline.
  try {
    aiConfig.value = { ...aiConfig.value, ...(await botAPI.aiConfig(selected.value.id)) }
  } catch {
    // Keep the defaults; saving still works once the endpoint responds.
  }
  try {
    aiKeys.value = await botAPI.aiKeys(selected.value.id)
    aiKeysError.value = ''
  } catch (e) {
    aiKeys.value = []
    aiKeysError.value = (e as { message?: string })?.message || '加载失败'
  }
  await loadAiRecords()
}
async function loadAiRecords(next = 1) {
  if (!selected.value) return
  try {
    const result = await botAPI.aiRecords(selected.value.id, next, aiKeyword.value, aiBlocked.value)
    aiRecords.value = result.items
    aiTotal.value = result.total
    aiPage.value = next
  } catch {
    aiRecords.value = []
    aiTotal.value = 0
  }
}
async function saveAi() {
  if (!selected.value) return
  aiConfig.value = await botAPI.saveAiConfig(selected.value.id, aiConfig.value)
  await loadAiRecords()
}

async function loadMessages(next = 1) {
  if (!selected.value) return
  const current = ++revision
  const result = await botAPI.messages(selected.value.id, next, chatFilter.value, keywordFilter.value)
  if (current !== revision) return
  messages.value = result.items
  total.value = result.total
  page.value = next
}

async function save() {
  if (!selected.value) return
  const credential =
    selected.value.platform === 'qq'
      ? token.value.trim()
        ? JSON.stringify({ app_id: selected.value.app_id || '', client_secret: token.value.trim() })
        : ''
      : token.value.trim()
  selected.value = await botAPI.saveBot(selected.value.id, {
    ...selected.value,
    bot_token: credential,
    chat_ids: chats.value
      .split('\n')
      .map((value) => value.trim())
      .filter(Boolean),
  })
  token.value = ''
  await refreshStatuses()
}

function edit(rule?: Rule) {
  if (!selected.value) return
  error.value = ''
  draft.value = rule
    ? { ...rule }
    : {
        id: 0,
        bot_id: selected.value.id,
        keyword: '',
        match_type: 'contains',
        reply_template: '{channel_status}',
        enabled: true,
        sort: 0,
      }
}
async function saveRule() {
  if (!draft.value || !selected.value) return
  await botAPI.saveRule(selected.value.id, draft.value)
  rules.value = await botAPI.rules(selected.value.id)
  draft.value = null
}
async function removeRule(rule: Rule) {
  if (!selected.value) return
  if (!window.confirm(`删除规则「${rule.keyword}」？`)) return
  await action(async () => {
    await botAPI.deleteRule(selected.value!.id, rule.id)
    rules.value = await botAPI.rules(selected.value!.id)
  }, '规则已删除')
}

onMounted(async () => {
  await action(async () => {
    await loadBots()
    loaded.value = true
  })
  timer = setInterval(() => {
    void refreshStatuses().catch(() => undefined)
  }, 5000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
  revision++
})
</script>

<template>
  <AppLayout>
    <div class="bot-page mx-auto w-full max-w-7xl space-y-6">
      <header class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex min-w-0 items-start gap-3 sm:gap-4">
          <div class="page-symbol"><Icon name="chat" size="lg" /></div>
          <div class="min-w-0">
            <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">消息机器人</h1>
            <p class="mt-1.5 text-sm leading-6 text-gray-500 dark:text-gray-400">
              连接 Telegram / QQ，让渠道状态与公告及时送达。
            </p>
          </div>
        </div>
        <button
          v-if="!selected"
          type="button"
          class="btn btn-primary w-full shrink-0 sm:w-auto"
          :disabled="busy"
          @click="showCreateBot"
        >
          <Icon name="plus" size="sm" />新增机器人
        </button>
        <button v-else type="button" class="btn btn-secondary w-full shrink-0 sm:w-auto" @click="closeBot">
          <Icon name="arrowLeft" size="sm" />返回机器人列表
        </button>
      </header>

      <p v-if="error && !creating && !draft" role="alert" class="feedback feedback-error">
        <Icon name="exclamationCircle" size="sm" /><span class="min-w-0 break-words">{{ error }}</span>
      </p>
      <p v-if="notice" role="status" class="feedback feedback-success">
        <Icon name="checkCircle" size="sm" /><span class="min-w-0 break-words">{{ notice }}</span>
      </p>

      <div v-if="!selected" class="space-y-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">
            我的机器人 <span class="ml-1 font-normal text-gray-400">{{ bots.length }}</span>
          </h2>
          <span v-if="loaded" class="inline-flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400"
            ><span class="status-dot" :class="runningBots ? 'bg-emerald-500' : 'bg-gray-400'" />{{
              runningBots
            }}
            个运行中<span class="hidden sm:inline"> · 配置与规则相互独立</span></span
          >
        </div>
        <div v-if="!loaded && !error" aria-label="正在加载机器人" class="grid gap-5 lg:grid-cols-2">
          <div v-for="n in 2" :key="n" class="bot-panel space-y-5 p-6 motion-safe:animate-pulse">
            <div class="h-5 w-1/3 rounded bg-gray-100 dark:bg-dark-700" />
            <div class="h-4 w-2/3 rounded bg-gray-100 dark:bg-dark-700" />
            <div class="h-16 rounded-xl bg-gray-50 dark:bg-dark-800" />
          </div>
        </div>
        <div v-if="!bots.length && loaded" class="bot-panel empty-state">
          <div class="empty-symbol"><Icon name="chat" size="xl" /></div>
          <h3 class="mt-4 font-semibold text-gray-900 dark:text-white">还没有连接机器人</h3>
          <p class="mt-2 max-w-sm text-sm leading-6 text-gray-500 dark:text-gray-400">
            点击「新增机器人」，连接 Telegram 或 QQ，开始设置你的消息通知。
          </p>
        </div>
        <div class="grid gap-5 lg:grid-cols-2">
          <article v-for="bot in bots" :key="bot.id" class="bot-panel flex min-w-0 flex-col p-5 sm:p-6">
            <div class="flex min-w-0 items-start gap-3">
              <div
                class="platform-symbol"
                :class="bot.platform === 'qq' ? 'platform-qq' : 'platform-telegram'"
              >
                <PlatformLogo :platform="bot.platform" />
              </div>
              <div class="min-w-0 flex-1">
                <h3 class="break-words text-base font-semibold text-gray-900 dark:text-gray-100">
                  {{ bot.name }}
                </h3>
                <p class="mt-1 break-all text-xs leading-5 text-gray-500 dark:text-gray-400">
                  {{
                    bot.platform === 'qq'
                      ? (bot.bot_username || 'QQ 机器人') + (bot.app_id ? ' · AppID ' + bot.app_id : '')
                      : bot.bot_username
                        ? '@' + bot.bot_username
                        : '未配置凭证'
                  }}
                </p>
              </div>
              <span class="badge badge-neutral shrink-0">{{ platformLabel(bot.platform) }}</span>
            </div>
            <div
              class="mt-5 grid grid-cols-3 divide-x divide-gray-200 rounded-xl bg-gray-50 py-3 dark:divide-dark-600 dark:bg-dark-800/60"
            >
              <div
                v-for="[label, enabled] in [
                  ['收消息', bot.receive_enabled],
                  ['定时推送', bot.push_enabled],
                  ['公告同步', bot.announce_sync_enabled],
                ]"
                :key="String(label)"
                class="px-2 text-center"
              >
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ label }}</p>
                <p
                  class="mt-1.5 text-sm font-medium"
                  :class="
                    enabled ? 'text-primary-700 dark:text-primary-400' : 'text-gray-400 dark:text-gray-500'
                  "
                >
                  {{ enabled ? '已开启' : '未开启' }}
                </p>
              </div>
            </div>
            <p
              v-if="statuses[bot.id]?.last_error"
              class="mt-4 rounded-lg bg-red-50 p-3 text-xs leading-6 text-red-700 [overflow-wrap:anywhere] dark:bg-red-950/30 dark:text-red-300"
            >
              {{ statuses[bot.id]?.last_error }}
            </p>
            <div
              class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4 dark:border-dark-700"
              :class="statuses[bot.id]?.last_error ? 'mt-4' : 'mt-5'"
            >
              <span class="inline-flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400"
                ><span
                  class="status-dot"
                  :class="
                    !bot.bot_token
                      ? 'bg-amber-500'
                      : statuses[bot.id]?.polling || statuses[bot.id]?.pushing || statuses[bot.id]?.announcing
                        ? 'bg-emerald-500'
                        : 'bg-gray-400'
                  "
                />{{
                  !bot.bot_token
                    ? '未配置'
                    : statuses[bot.id]?.polling || statuses[bot.id]?.pushing || statuses[bot.id]?.announcing
                      ? '运行中'
                      : '已停止'
                }}</span
              >
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="btn btn-ghost !px-3 text-red-600 dark:text-red-400"
                  :disabled="busy"
                  :aria-label="`删除机器人 ${bot.name}`"
                  @click="removeBot(bot)"
                >
                  <Icon name="trash" size="sm" /><span class="hidden sm:inline">删除</span></button
                ><button
                  type="button"
                  class="btn btn-secondary"
                  :disabled="busy"
                  @click="action(() => openBot(bot))"
                >
                  管理配置<Icon name="arrowRight" size="sm" />
                </button>
              </div>
            </div>
          </article>
        </div>
      </div>

      <div v-else class="space-y-5">
        <div class="bot-panel overflow-hidden">
          <div class="flex min-w-0 flex-wrap items-center gap-3 p-5 sm:gap-4 sm:px-6">
            <div
              class="platform-symbol"
              :class="selected.platform === 'qq' ? 'platform-qq' : 'platform-telegram'"
            >
              <PlatformLogo :platform="selected.platform" />
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="break-words text-lg font-semibold text-gray-900 dark:text-white">
                {{ selected.name }}
              </h2>
              <p class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400">
                {{ platformLabel(selected.platform)
                }}<span v-if="selected.bot_username">
                  ·
                  {{ selected.platform === 'qq' ? selected.bot_username : '@' + selected.bot_username }}</span
                >
              </p>
            </div>
            <div class="hidden shrink-0 sm:block">
              <span class="badge badge-neutral"><Icon name="clock" size="xs" />状态每 5 秒更新</span>
            </div>
          </div>
          <nav
            class="grid grid-cols-2 border-t border-gray-100 px-2 dark:border-dark-700 sm:grid-cols-4 sm:px-5"
            aria-label="机器人管理页签"
          >
            <button
              v-for="(name, index) in tabs"
              :key="name"
              type="button"
              class="tab-button"
              :class="tab === name ? 'tab-active' : ''"
              :aria-current="tab === name ? 'page' : undefined"
              @click="tab = name"
            >
              <Icon :name="tabIcons[name]" size="sm" class="hidden shrink-0 sm:block" /><span>{{ name }}</span
              ><span
                v-if="index === 1 || index === 2 || index === 3"
                class="hidden rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] leading-none text-gray-500 dark:bg-dark-700 dark:text-gray-400 sm:inline"
                >{{ index === 1 ? total : index === 2 ? rules.length : aiTotal }}</span
              >
            </button>
          </nav>
        </div>

        <fieldset :disabled="busy" class="min-w-0">
          <form
            v-if="tab === '机器人配置'"
            class="bot-panel overflow-hidden"
            @submit.prevent="action(save, '机器人配置已保存，任务正在更新')"
          >
            <div class="config-status">
              <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
                <span class="text-xs font-medium text-gray-500 dark:text-gray-400">运行状态</span>
                <span
                  v-for="[label, active] in [
                    ['收消息', statuses[selected.id]?.polling],
                    ['定时推送', statuses[selected.id]?.pushing],
                    ['公告同步', statuses[selected.id]?.announcing],
                  ]"
                  :key="String(label)"
                  class="inline-flex items-center gap-2 text-xs"
                  :class="active ? 'text-emerald-700 dark:text-emerald-400' : 'text-gray-400'"
                >
                  <span
                    class="status-dot"
                    :class="active ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-500'"
                  />{{ label }} · {{ active ? '运行中' : '已停止' }}
                </span>
              </div>
              <p
                v-if="statuses[selected.id]?.last_error"
                class="mt-3 text-xs leading-6 text-red-600 [overflow-wrap:anywhere] dark:text-red-400"
              >
                最近错误：{{ statuses[selected.id]?.last_error
                }}<span v-if="statuses[selected.id]?.last_error_at" class="ml-2 opacity-70">{{
                  new Date(statuses[selected.id]?.last_error_at || '').toLocaleString()
                }}</span>
              </p>
            </div>
            <div class="config-section">
              <div class="config-section-title">
                <h3>基础连接</h3>
                <p>设置备注名称和平台凭证。</p>
              </div>
              <div class="min-w-0 space-y-4">
                <label class="field-label"
                  >名称（备注名）<input v-model="selected.name" required class="input mt-2"
                /></label>
                <label v-if="selected.platform === 'qq'" class="field-label"
                  >AppID<input v-model="selected.app_id" class="input mt-2"
                /></label>
                <label class="field-label"
                  ><span class="flex flex-wrap justify-between gap-2"
                    >{{ selected.platform === 'qq' ? 'AppSecret' : 'Bot Token'
                    }}<span class="text-xs font-normal text-gray-400">{{
                      selected.bot_token ? '已保存 ' + selected.bot_token : '尚未配置'
                    }}</span></span
                  ><input
                    v-model="token"
                    type="password"
                    autocomplete="new-password"
                    class="input mt-2"
                    :placeholder="
                      selected.platform === 'qq'
                        ? '留空保留原密钥；更换 AppID 时必须重填'
                        : '留空保留原 token'
                    "
                /></label>
                <p class="helper">保存时自动验证凭证并读取机器人名称。</p>
              </div>
            </div>
            <div class="config-section">
              <div class="config-section-title">
                <h3>接收与推送</h3>
                <p>选择需要运行的任务。</p>
              </div>
              <div class="min-w-0 divide-y divide-gray-100 dark:divide-dark-700">
                <label class="setting-toggle"
                  ><span
                    ><span class="field-label">接收消息与关键词回复</span
                    ><span class="helper mt-1 block">群聊需 @机器人或回复机器人消息才触发规则。</span></span
                  ><input v-model="selected.receive_enabled" type="checkbox" class="bot-check"
                /></label>
                <label class="setting-toggle"
                  ><span
                    ><span class="field-label">定时推送</span
                    ><span class="helper mt-1 block">按设定间隔发送推送模板。</span></span
                  ><input v-model="selected.push_enabled" type="checkbox" class="bot-check"
                /></label>
                <label class="setting-toggle"
                  ><span
                    ><span class="field-label">公告自动同步</span
                    ><span class="helper mt-1 block"
                      >发布或修改公告后同步发送，Markdown 转为纯文本；首次开启不补发历史。</span
                    ></span
                  ><input v-model="selected.announce_sync_enabled" type="checkbox" class="bot-check"
                /></label>
              </div>
            </div>
            <div class="config-section">
              <div class="config-section-title">
                <h3>推送内容</h3>
                <p>定时推送与公告同步共用目标。</p>
              </div>
              <div class="min-w-0 space-y-5">
                <label class="field-label"
                  ><span class="flex flex-wrap justify-between gap-2"
                    >推送目标<span class="text-xs font-normal text-gray-400"
                      >{{ targetCount }} 个目标 · 每行一个</span
                    ></span
                  ><textarea
                    v-model="chats"
                    rows="3"
                    class="input mt-2 font-mono"
                    :placeholder="
                      selected.platform === 'qq' ? 'group:群OpenID\nuser:用户OpenID' : '-1001234567890'
                    "
                  /><span class="helper mt-2 block">{{
                    selected.platform === 'qq'
                      ? '群聊填写 group:群OpenID，私聊填写 user:用户OpenID。'
                      : '填写完整 chat_id，保留群聊 ID 的负号；可从「收到的消息」查看。'
                  }}</span></label
                >
                <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:gap-5">
                  <label class="field-label sm:w-40 sm:shrink-0"
                    >推送间隔（分钟）<input
                      v-model.number="selected.interval_minutes"
                      type="number"
                      min="1"
                      max="43200"
                      required
                      class="input mt-2"
                  /></label>
                  <p class="helper">对齐整点：60 分钟为每小时 00 分；30 分钟为每小时 00 分、30 分。</p>
                </div>
                <TemplateInput v-model="selected.push_template" label="推送模板" />
                <div class="flex flex-col items-start gap-3 sm:flex-row sm:items-center">
                  <button
                    type="button"
                    class="btn btn-secondary w-full shrink-0 sm:w-auto"
                    :disabled="!selected.bot_token"
                    @click="action(() => botAPI.pushNow(selected!.id), '已按推送模板发送到全部推送目标')"
                  >
                    <Icon name="mail" size="sm" />立即推送一次
                  </button>
                  <p class="helper">使用已保存配置。修改目标或模板后请先保存。</p>
                </div>
              </div>
            </div>
            <div class="config-section">
              <div class="config-section-title">
                <h3>发送测试</h3>
                <p>验证指定目标是否能收到消息。</p>
              </div>
              <div class="min-w-0">
                <div class="flex flex-col items-stretch gap-3 sm:flex-row sm:items-end">
                  <label class="field-label min-w-0 flex-1"
                    >测试目标<input
                      v-model="testChat"
                      class="input mt-2 font-mono"
                      :placeholder="
                        selected.platform === 'qq'
                          ? 'group:群OpenID 或 user:用户OpenID'
                          : '接收人或群聊 chat_id'
                      " /></label
                  ><button
                    type="button"
                    class="btn btn-secondary shrink-0"
                    :disabled="!testChat"
                    @click="action(() => botAPI.test(selected!.id, testChat), '测试消息已发送')"
                  >
                    发送测试消息
                  </button>
                </div>
              </div>
            </div>
            <div class="config-footer">
              <p class="helper">保存后自动更新机器人任务。</p>
              <button type="submit" class="btn btn-primary w-full sm:w-auto">
                <Icon name="check" size="sm" />{{ busy ? '保存中…' : '保存配置' }}
              </button>
            </div>
          </form>

          <section v-else-if="tab === '收到的消息'" class="space-y-5">
            <form class="bot-panel p-4 sm:p-5" @submit.prevent="action(() => loadMessages())">
              <div
                class="grid items-end gap-4 sm:grid-cols-2 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]"
              >
                <label class="field-label"
                  >聊天 ID<input
                    v-model="chatFilter"
                    class="input mt-2"
                    placeholder="按 chat_id 过滤"
                    aria-label="聊天过滤" /></label
                ><label class="field-label"
                  >消息关键词<input
                    v-model="keywordFilter"
                    class="input mt-2"
                    placeholder="输入消息内容关键词"
                    aria-label="关键词过滤" /></label
                ><button class="btn btn-primary sm:col-span-2 lg:col-span-1">
                  <Icon name="search" size="sm" />查询消息
                </button>
              </div>
            </form>
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h3 class="text-sm font-semibold">
                消息记录 <span class="ml-1 font-normal text-gray-400">{{ total }} 条</span>
              </h3>
              <p class="text-xs text-gray-500 dark:text-gray-400">最近 30 天 · 最多 2000 条</p>
            </div>
            <div v-if="!messages.length" class="bot-panel empty-state">
              <div class="empty-symbol"><Icon name="inbox" size="xl" /></div>
              <h3 class="mt-4 font-semibold">暂无消息记录</h3>
              <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                机器人收到消息后会显示在这里，也可以调整筛选条件再查询。
              </p>
            </div>
            <template v-else>
              <div class="bot-panel hidden overflow-hidden md:block">
                <table class="message-table">
                  <thead>
                    <tr>
                      <th>时间 / 平台</th>
                      <th>聊天 / 发送人</th>
                      <th class="w-[36%]">消息内容</th>
                      <th>命中规则</th>
                      <th>回复状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="m in messages" :key="m.id">
                      <td>
                        <time class="block text-xs leading-5 [overflow-wrap:anywhere]">{{
                          new Date(m.created_at).toLocaleString()
                        }}</time
                        ><span class="mt-2 inline-block text-xs text-gray-400">{{
                          platformLabel(m.platform)
                        }}</span>
                      </td>
                      <td>
                        <p class="font-medium [overflow-wrap:anywhere]">{{ m.sender_name || m.sender_id }}</p>
                        <p class="mt-1 font-mono text-xs text-gray-500 [overflow-wrap:anywhere]">
                          {{ m.chat_id }}
                        </p>
                        <p class="mt-1 text-xs text-gray-400">{{ m.chat_type }}</p>
                      </td>
                      <td>
                        <p class="whitespace-pre-wrap leading-6 [overflow-wrap:anywhere]">{{ m.text }}</p>
                      </td>
                      <td>
                        <span class="text-xs leading-6 text-gray-500 [overflow-wrap:anywhere]">{{
                          ruleLabel(m.rule_id)
                        }}</span>
                      </td>
                      <td>
                        <span
                          class="badge whitespace-nowrap"
                          :class="m.replied ? 'badge-success' : 'badge-neutral'"
                          >{{ m.replied ? '已回复' : '未回复' }}</span
                        >
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="space-y-3 md:hidden">
                <article v-for="m in messages" :key="m.id" class="bot-panel min-w-0 p-4 sm:p-5">
                  <div class="flex flex-wrap items-start justify-between gap-x-3 gap-y-2">
                    <h3 class="min-w-0 break-all text-sm font-semibold">
                      {{ m.sender_name || m.sender_id }}
                    </h3>
                    <time class="text-xs text-gray-400">{{ new Date(m.created_at).toLocaleString() }}</time>
                  </div>
                  <p class="mt-2 text-xs leading-5 text-gray-500 [overflow-wrap:anywhere]">
                    {{ platformLabel(m.platform) }} · {{ m.chat_type }} ·
                    <span class="font-mono">{{ m.chat_id }}</span>
                  </p>
                  <p
                    class="my-4 whitespace-pre-wrap text-sm leading-7 text-gray-700 [overflow-wrap:anywhere] dark:text-gray-200"
                  >
                    {{ m.text }}
                  </p>
                  <div
                    class="flex flex-wrap items-center gap-2 border-t border-gray-100 pt-3 dark:border-dark-700"
                  >
                    <span class="badge badge-neutral min-w-0 [overflow-wrap:anywhere]">{{
                      m.rule_id ? '命中：' + ruleLabel(m.rule_id) : '未命中规则'
                    }}</span
                    ><span class="badge" :class="m.replied ? 'badge-success' : 'badge-neutral'">{{
                      m.replied ? '已回复' : '未回复'
                    }}</span>
                  </div>
                </article>
              </div>
            </template>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <span
                class="order-first w-full text-center text-xs text-gray-500 dark:text-gray-400 sm:order-none sm:w-auto sm:text-left"
                >第 {{ page }} 页 · 共 {{ total }} 条</span
              >
              <div class="flex w-full justify-between gap-3 sm:w-auto">
                <button
                  type="button"
                  class="btn btn-secondary"
                  :disabled="page <= 1"
                  @click="action(() => loadMessages(page - 1))"
                >
                  <Icon name="chevronLeft" size="sm" />上一页</button
                ><button
                  type="button"
                  class="btn btn-secondary"
                  :disabled="page * 20 >= total"
                  @click="action(() => loadMessages(page + 1))"
                >
                  下一页<Icon name="chevronRight" size="sm" />
                </button>
              </div>
            </div>
            <p class="helper">消息清理任务每小时执行。</p>
          </section>

          <section v-else-if="tab === '关键词回复'" class="space-y-5">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <h3 class="font-semibold">
                  关键词回复 <span class="ml-1 font-normal text-gray-400">{{ rules.length }}</span>
                </h3>
                <p class="helper mt-1">按排序值从小到大匹配，只回复第一条命中的规则。</p>
              </div>
              <button type="button" class="btn btn-primary shrink-0" @click="edit()">
                <Icon name="plus" size="sm" />新增规则
              </button>
            </div>

            <div v-if="!rules.length" class="bot-panel empty-state">
              <div class="empty-symbol"><Icon name="chat" size="xl" /></div>
              <h3 class="mt-4 font-semibold">暂无回复规则</h3>
              <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                添加关键词与回复模板，为当前机器人设置自动回复。
              </p>
            </div>
            <div v-if="rules.length" class="bot-panel hidden overflow-hidden md:block">
              <table class="rules-table">
                <thead>
                  <tr>
                    <th class="w-16">排序</th>
                    <th class="w-[18%]">关键词</th>
                    <th class="w-24">匹配方式</th>
                    <th>回复模板</th>
                    <th class="w-20">状态</th>
                    <th class="w-32 text-right">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="rule in rules" :key="rule.id">
                    <td class="text-gray-400">{{ rule.sort }}</td>
                    <td class="font-medium [overflow-wrap:anywhere]">{{ rule.keyword }}</td>
                    <td>
                      <span class="badge badge-neutral">{{ matchLabels[rule.match_type] }}</span>
                    </td>
                    <td>
                      <p
                        class="line-clamp-3 whitespace-pre-wrap font-mono text-xs leading-6 text-gray-500 [overflow-wrap:anywhere] dark:text-gray-400"
                        :title="rule.reply_template"
                      >
                        {{ rule.reply_template }}
                      </p>
                    </td>
                    <td>
                      <span class="badge" :class="rule.enabled ? 'badge-success' : 'badge-neutral'">{{
                        rule.enabled ? '启用' : '停用'
                      }}</span>
                    </td>
                    <td>
                      <div class="flex justify-end gap-3">
                        <button type="button" class="table-action" @click="edit(rule)">编辑</button
                        ><button type="button" class="table-action !text-red-500" @click="removeRule(rule)">
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="space-y-3 md:hidden">
              <article v-for="rule in rules" :key="rule.id" class="bot-panel p-4">
                <div class="flex flex-wrap items-start justify-between gap-2">
                  <h3 class="min-w-0 flex-1 font-medium [overflow-wrap:anywhere]">{{ rule.keyword }}</h3>
                  <span class="badge" :class="rule.enabled ? 'badge-success' : 'badge-neutral'">{{
                    rule.enabled ? '启用' : '停用'
                  }}</span>
                </div>
                <p class="mt-2 text-xs text-gray-400">
                  排序 {{ rule.sort }} · {{ matchLabels[rule.match_type] }}
                </p>
                <p
                  class="mt-3 line-clamp-3 whitespace-pre-wrap rounded-lg bg-gray-50 p-3 font-mono text-xs leading-6 text-gray-500 [overflow-wrap:anywhere] dark:bg-dark-800 dark:text-gray-400"
                >
                  {{ rule.reply_template }}
                </p>
                <div class="mt-3 flex justify-end gap-3 border-t border-gray-100 pt-2 dark:border-dark-700">
                  <button type="button" class="btn btn-ghost text-red-500" @click="removeRule(rule)">
                    删除</button
                  ><button type="button" class="btn btn-secondary" @click="edit(rule)">编辑规则</button>
                </div>
              </article>
            </div>
          </section>
          <section v-else class="space-y-5">
            <form class="bot-panel overflow-hidden" @submit.prevent="action(saveAi, 'AI 客服配置已保存')">
              <div class="config-status flex items-start gap-3">
                <Icon name="shield" size="sm" class="mt-0.5 shrink-0 text-emerald-600" />
                <p class="helper">
                  AI 仅使用经过服务端脱敏的公开资料回答，不会读取管理员数据、密钥或任意数据库。敏感请求会在调用模型前拦截。
                </p>
              </div>
              <div class="config-section">
                <div class="config-section-title">
                  <h3>启用与模型</h3>
                  <p>关键词未命中的消息由 AI 依据公开资料回答。</p>
                </div>
                <div class="min-w-0 space-y-4">
                  <label class="setting-toggle">
                    <span class="field-label">启用 AI 客服</span>
                    <input v-model="aiConfig.enabled" type="checkbox" class="bot-check" />
                  </label>
                  <label class="field-label">
                    API Key
                    <select v-model="aiConfig.api_key_id" class="input mt-2">
                      <option :value="null">请选择已创建的 API Key</option>
                      <option
                        v-if="
                          aiConfig.api_key_id != null &&
                          !aiKeys.some((key) => String(key.id) === String(aiConfig.api_key_id))
                        "
                        :value="aiConfig.api_key_id"
                      >
                        已保存的 Key（#{{ aiConfig.api_key_id }}）
                      </option>
                      <option v-for="key in aiKeys" :key="key.id" :value="key.id">
                        {{ key.name }}{{ key.platform ? ` · ${key.platform}` : ''
                        }}{{ key.status && key.status !== 'active' ? ` · ${key.status}` : '' }}
                      </option>
                    </select>
                    <span v-if="aiKeysError" class="mt-1 block text-xs font-normal text-red-500">
                      Key 列表加载失败：{{ aiKeysError }}
                    </span>
                    <span v-else class="helper mt-1 block">
                      只列出你名下的 Key，与「API 密钥」页一致；仅保存 Key 引用，前端不会接触密钥明文。
                    </span>
                  </label>
                  <label class="field-label">
                    模型
                    <input
                      v-model="aiConfig.model"
                      class="input mt-2"
                      list="ai-model-suggestions"
                      placeholder="如 gpt-6.1-sol"
                    />
                    <datalist id="ai-model-suggestions">
                      <option v-for="model in aiModelSuggestions" :key="model" :value="model" />
                    </datalist>
                    <span class="helper mt-1 block">
                      填所选 Key 分组支持的模型名，可从候选中选或直接输入。
                    </span>
                  </label>
                </div>
              </div>
              <div class="config-section">
                <div class="config-section-title">
                  <h3>限额与频率</h3>
                  <p>控制单次长度、同一会话的冷却和每日总量。</p>
                </div>
                <div class="min-w-0 space-y-4">
                  <div class="grid gap-4 sm:grid-cols-3">
                    <label class="field-label">
                      最大输入字符
                      <input
                        v-model.number="aiConfig.max_input_chars"
                        type="number"
                        min="500"
                        max="20000"
                        class="input mt-2"
                      />
                    </label>
                    <label class="field-label">
                      最大输出 Token
                      <input
                        v-model.number="aiConfig.max_output_tokens"
                        type="number"
                        min="100"
                        max="4000"
                        class="input mt-2"
                      />
                    </label>
                    <label class="field-label">
                      单聊冷却（秒）
                      <input
                        v-model.number="aiConfig.cooldown_seconds"
                        type="number"
                        min="0"
                        max="3600"
                        class="input mt-2"
                      />
                    </label>
                  </div>
                  <label class="field-label">
                    每日上限
                    <input
                      v-model.number="aiConfig.daily_limit"
                      type="number"
                      min="0"
                      max="100000"
                      class="input mt-2"
                    />
                    <span class="helper mt-1 block">当前机器人每天最多回答的条数，0 表示不限制。</span>
                  </label>
                </div>
              </div>
              <div class="config-section">
                <div class="config-section-title">
                  <h3>咨询范围</h3>
                  <p>关闭的类型会引导用户联系人工客服。</p>
                </div>
                <div class="min-w-0 space-y-4">
                  <label class="setting-toggle">
                    <span class="field-label">允许充值咨询</span>
                    <input v-model="aiConfig.allow_recharge" type="checkbox" class="bot-check" />
                  </label>
                  <label class="setting-toggle">
                    <span class="field-label">允许技术咨询</span>
                    <input v-model="aiConfig.allow_technical" type="checkbox" class="bot-check" />
                  </label>
                </div>
              </div>
              <div class="config-footer">
                <p class="helper">建议先用测试群验证敏感问题拦截和回答范围。</p>
                <button type="submit" class="btn btn-primary">保存 AI 配置</button>
              </div>
            </form>

            <div class="bot-panel overflow-hidden">
              <div
                class="config-status flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
              >
                <div>
                  <h3 class="font-semibold">
                    AI 回答记录
                    <span class="ml-1 font-normal text-gray-400">{{ aiTotal }} 条</span>
                  </h3>
                  <p class="helper mt-1">仅展示脱敏后的问题、回答和拦截原因。</p>
                </div>
                <form class="flex flex-wrap gap-2" @submit.prevent="action(() => loadAiRecords())">
                  <input v-model="aiKeyword" class="input" placeholder="搜索问题或聊天 ID" />
                  <select v-model="aiBlocked" class="input">
                    <option value="">全部状态</option>
                    <option value="false">已回答</option>
                    <option value="true">已拦截</option>
                  </select>
                  <button class="btn btn-secondary">筛选</button>
                </form>
              </div>
              <div v-if="!aiRecords.length" class="empty-state">
                <div class="empty-symbol"><Icon name="lightbulb" size="xl" /></div>
                <h3 class="mt-4 font-semibold">暂无 AI 回答记录</h3>
                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  启用 AI 客服并收到消息后，记录会显示在这里。
                </p>
              </div>
              <div v-else class="overflow-x-auto">
                <table class="message-table min-w-[760px]">
                  <thead>
                    <tr>
                      <th>时间 / 聊天</th>
                      <th>问题</th>
                      <th>回答</th>
                      <th>状态</th>
                      <th>模型 / 耗时</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="record in aiRecords" :key="record.id">
                      <td>
                        <time class="block text-xs">{{ new Date(record.created_at).toLocaleString() }}</time>
                        <span class="mt-1 block font-mono text-xs text-gray-500">{{ record.chat_id }}</span>
                        <span class="mt-1 block text-xs text-gray-400">{{ record.sender_name || '—' }}</span>
                      </td>
                      <td class="max-w-xs whitespace-pre-wrap break-words">{{ record.question }}</td>
                      <td class="max-w-md whitespace-pre-wrap break-words">{{ record.answer || '—' }}</td>
                      <td>
                        <span class="badge" :class="record.blocked || record.error_code ? 'badge-neutral' : 'badge-success'">
                          {{ record.blocked ? '已拦截' : record.error_code ? '失败' : '已回答' }}
                        </span>
                        <span v-if="record.blocked_reason" class="mt-1 block max-w-32 text-xs text-gray-500">
                          {{ record.blocked_reason }}
                        </span>
                      </td>
                      <td>
                        <span class="block text-xs">{{ record.model || '—' }}</span>
                        <span v-if="record.latency_ms" class="mt-1 block text-xs text-gray-400">
                          {{ record.latency_ms }} ms
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="config-footer">
                <span class="text-xs text-gray-500">第 {{ aiPage }} 页 · 共 {{ aiTotal }} 条</span>
                <div class="flex gap-2">
                  <button
                    class="btn btn-secondary"
                    :disabled="aiPage <= 1"
                    @click="action(() => loadAiRecords(aiPage - 1))"
                  >
                    上一页
                  </button>
                  <button
                    class="btn btn-secondary"
                    :disabled="aiPage * 20 >= aiTotal"
                    @click="action(() => loadAiRecords(aiPage + 1))"
                  >
                    下一页
                  </button>
                </div>
              </div>
            </div>
          </section>
        </fieldset>
      </div>
    </div>

    <BaseDialog
      :show="creating"
      title="新增机器人"
      width="normal"
      :close-on-escape="!busy"
      :show-close-button="!busy"
      @close="creating = false"
    >
      <form
        id="bot-create-form"
        class="bot-form space-y-5"
        @submit.prevent="action(createBot, '机器人已创建，已生成默认关键词规则')"
      >
        <p v-if="error" role="alert" class="dialog-error">{{ error }}</p>
        <fieldset :disabled="busy" class="min-w-0 space-y-5">
          <div>
            <p class="field-label mb-2">选择平台</p>
            <div class="grid grid-cols-2 gap-3">
              <button
                v-for="platform in ['telegram', 'qq']"
                :key="platform"
                type="button"
                class="platform-option"
                :class="newPlatform === platform ? 'platform-option-active' : ''"
                :aria-label="platformLabel(platform)"
                :aria-pressed="newPlatform === platform"
                @click="newPlatform = platform"
              >
                <PlatformLogo :platform="platform" /><span>{{ platformLabel(platform) }}</span
                ><Icon
                  v-if="newPlatform === platform"
                  name="checkCircle"
                  size="sm"
                  class="ml-auto text-primary-600"
                />
              </button>
            </div>
          </div>
          <label class="field-label"
            >名称（备注名）<input v-model="newName" class="input mt-2" placeholder="例如：渠道监控机器人"
          /></label>
          <template v-if="newPlatform === 'qq'"
            ><label class="field-label"
              >AppID<input
                v-model="newAppID"
                required
                class="input mt-2"
                placeholder="QQ 开放平台的机器人 AppID" /></label
            ><label class="field-label"
              >AppSecret<input
                v-model="newSecret"
                type="password"
                autocomplete="new-password"
                required
                class="input mt-2"
                placeholder="QQ 开放平台的机器人密钥" /></label
          ></template>
          <label v-else class="field-label"
            >Bot Token<input
              v-model="newToken"
              type="password"
              autocomplete="new-password"
              required
              class="input mt-2"
              placeholder="从 BotFather 获取"
          /></label>
          <p class="helper">创建时自动验证凭证，并生成「渠道状态」和 /status 默认回复规则。</p>
        </fieldset>
      </form>
      <template #footer
        ><div class="bot-form flex w-full flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="creating = false">
            取消</button
          ><button type="submit" form="bot-create-form" class="btn btn-primary" :disabled="busy">
            {{ busy ? '创建中…' : '创建并配置' }}
          </button>
        </div></template
      >
    </BaseDialog>
    <BaseDialog
      :show="!!draft"
      :title="draft?.id ? '编辑规则' : '新增规则'"
      width="wide"
      :close-on-escape="!busy"
      :show-close-button="!busy"
      @close="draft = null"
    >
      <form
        v-if="draft"
        id="bot-rule-form"
        class="bot-form space-y-5"
        @submit.prevent="action(saveRule, '规则已保存')"
      >
        <p v-if="error" role="alert" class="dialog-error">{{ error }}</p>
        <fieldset :disabled="busy" class="min-w-0 space-y-5">
          <div class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_160px]">
            <label class="field-label"
              >关键词<input
                v-model="draft.keyword"
                required
                maxlength="500"
                class="input mt-2"
                placeholder="例如：渠道状态" /></label
            ><label class="field-label"
              >匹配方式<select v-model="draft.match_type" class="input mt-2">
                <option value="contains">包含</option>
                <option value="exact">完全匹配</option>
                <option value="command">命令</option>
              </select></label
            >
          </div>
          <div class="flex flex-col gap-4 sm:flex-row sm:items-end sm:gap-8">
            <label class="field-label sm:w-40"
              >排序（小的先匹配）<input
                v-model.number="draft.sort"
                type="number"
                required
                class="input mt-2" /></label
            ><label class="flex min-h-11 items-center gap-3 text-sm"
              ><input v-model="draft.enabled" type="checkbox" class="bot-check" />启用此规则</label
            >
          </div>
          <TemplateInput v-model="draft.reply_template" label="回复模板" />
        </fieldset>
      </form>
      <template #footer
        ><div class="bot-form flex w-full flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="draft = null">取消</button
          ><button type="submit" form="bot-rule-form" class="btn btn-primary" :disabled="busy">
            {{ busy ? '保存中…' : '保存规则' }}
          </button>
        </div></template
      >
    </BaseDialog>
  </AppLayout>
</template>

<style scoped>
.config-status {
  @apply border-b border-gray-100 bg-gray-50/60 px-5 py-4 dark:border-dark-700 dark:bg-dark-800/30 sm:px-7;
}
.config-section {
  @apply grid min-w-0 gap-5 border-b border-gray-100 p-5 dark:border-dark-700 sm:p-7 lg:grid-cols-[180px_minmax(0,1fr)] lg:gap-10;
}
.config-section-title h3 {
  @apply text-sm font-semibold;
}
.config-section-title p {
  @apply mt-2 text-xs leading-6 text-gray-400;
}
.config-section > div:last-child {
  max-width: 760px;
}
.setting-toggle {
  @apply flex cursor-pointer items-center justify-between gap-4 py-4 first:pt-0 last:pb-0;
}
.config-footer {
  @apply flex flex-col items-center justify-between gap-3 bg-gray-50/60 px-5 py-4 dark:bg-dark-800/30 sm:flex-row sm:px-7;
}
.rules-table {
  @apply w-full table-fixed text-left text-sm;
}
.rules-table th {
  @apply bg-gray-50 px-4 py-3 text-xs font-medium text-gray-500 dark:bg-dark-800/50;
}
.rules-table td {
  @apply border-t border-gray-100 px-4 py-4 align-top dark:border-dark-700;
}
.table-action {
  @apply inline-flex min-h-8 items-center text-xs font-medium text-primary-600 hover:underline dark:text-primary-400;
}
.platform-option {
  @apply flex min-w-0 items-center gap-3 rounded-xl border border-gray-200 p-3 text-sm font-medium dark:border-dark-600;
}
.platform-option-active {
  @apply border-primary-500 bg-primary-50/60 ring-1 ring-primary-500 dark:bg-primary-950/40;
}
.dialog-error {
  @apply rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-950/40 dark:text-red-400;
}
.bot-form :deep(textarea) {
  resize: vertical;
}
@media (max-width: 639px) {
  .bot-form :deep(input:not([type='checkbox'])),
  .bot-form :deep(textarea),
  .bot-form :deep(select) {
    font-size: 16px;
  }
  .bot-form button {
    min-height: 44px;
  }
}

.bot-page {
  @apply min-w-0 text-gray-900 dark:text-gray-100;
}
.bot-panel {
  @apply min-w-0 rounded-2xl border border-gray-200/80 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900;
}
.page-symbol {
  @apply flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-primary-100 bg-primary-50 text-primary-700 dark:border-primary-800/60 dark:bg-primary-950/40 dark:text-primary-400;
}
.platform-symbol {
  @apply flex h-11 w-11 shrink-0 items-center justify-center rounded-xl;
}
.platform-telegram {
  @apply bg-sky-50 text-sky-600 dark:bg-sky-950/50 dark:text-sky-400;
}
.platform-qq {
  @apply bg-amber-50 text-amber-600 dark:bg-amber-950/40 dark:text-amber-400;
}
.field-label {
  @apply block min-w-0 text-sm font-medium text-gray-700 dark:text-gray-200;
}
.helper {
  @apply text-xs font-normal leading-6 text-gray-500 dark:text-gray-400;
}
.bot-check {
  @apply h-5 w-5 shrink-0 cursor-pointer rounded border-gray-300 accent-primary-600;
}
.status-dot {
  @apply inline-block h-1.5 w-1.5 shrink-0 rounded-full;
}
.badge {
  @apply inline-flex max-w-full items-center gap-1.5 rounded-md px-2 py-1 text-[11px] font-medium leading-5;
}
.badge-neutral {
  @apply bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-gray-400;
}
.badge-success {
  @apply bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400;
}
.tab-button {
  @apply relative flex min-h-14 min-w-0 items-center justify-center gap-2 border-b-2 border-transparent px-1 py-3 text-xs font-medium text-gray-500 transition-colors hover:text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:text-gray-400 dark:hover:text-gray-100 sm:text-sm;
}
.tab-active {
  @apply border-primary-500 text-primary-700 dark:text-primary-400;
}
.feedback {
  @apply flex items-start gap-2.5 rounded-xl border p-4 text-sm leading-6;
}
.feedback > svg {
  @apply mt-1 shrink-0;
}
.feedback-error {
  @apply border-red-100 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300;
}
.feedback-success {
  @apply border-emerald-100 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-300;
}
.empty-state {
  @apply flex flex-col items-center px-5 py-14 text-center;
}
.empty-symbol {
  @apply flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-50 text-gray-400 dark:bg-dark-800 dark:text-gray-500;
}
.message-table {
  @apply w-full table-fixed text-left text-sm;
}
.message-table th {
  @apply bg-gray-50 px-5 py-3.5 text-xs font-medium text-gray-500 dark:bg-dark-800/50 dark:text-gray-400;
}
.message-table td {
  @apply border-t border-gray-100 px-5 py-4 align-top dark:border-dark-700;
}
.message-table th:first-child {
  width: 18%;
}
.message-table th:nth-child(2) {
  width: 22%;
}
.message-table th:nth-child(4) {
  width: 14%;
}
.message-table th:last-child {
  width: 10%;
}
.bot-page :deep(textarea) {
  @apply resize-y;
}
@media (max-width: 639px) {
  .bot-page :deep(input:not([type='checkbox'])),
  .bot-page :deep(textarea),
  .bot-page :deep(select) {
    font-size: 16px;
  }
  .bot-page :deep(button.btn) {
    min-height: 44px;
  }
}
</style>
