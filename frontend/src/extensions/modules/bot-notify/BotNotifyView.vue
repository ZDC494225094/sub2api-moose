<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TemplateInput from './TemplateInput.vue'
import { botAPI, type BotEntry, type Rule, type Status, type BotMessage } from './api'

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
let timer: ReturnType<typeof setInterval> | undefined
let revision = 0

const tabs = ['机器人配置', '收到的消息', '关键词回复']
const matchLabels: Record<Rule['match_type'], string> = { contains: '包含', exact: '完全匹配', command: '命令' }

async function action(fn: () => Promise<void>, success = '') {
  if (busy.value) return
  busy.value = true; error.value = ''; notice.value = ''
  try { await fn(); notice.value = success } catch (e: unknown) {
    const detail = e as { response?: { data?: { message?: string } } }
    error.value = detail.response?.data?.message || '操作失败，请重试。'
  } finally { busy.value = false }
}

async function refreshStatuses() {
  const entries = await Promise.all(bots.value.map(async bot => {
    try { return [bot.id, await botAPI.status(bot.id)] as const } catch { return null }
  }))
  const next: Record<number, Status> = { ...statuses.value }
  for (const entry of entries) if (entry) next[entry[0]] = entry[1]
  statuses.value = next
}

async function loadBots() {
  bots.value = await botAPI.listBots()
  await refreshStatuses()
}

async function createBot() {
  const created = await botAPI.createBot({
    name: newName.value.trim(),
    platform: newPlatform.value,
    bot_token: newToken.value.trim(),
    receive_enabled: true,
    push_enabled: false,
    chat_ids: [],
    interval_minutes: 60,
    push_template: '{channel_status}',
  })
  creating.value = false; newName.value = ''; newToken.value = ''
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
  token.value = ''; chats.value = fresh.chat_ids.join('\n'); testChat.value = ''
  tab.value = '机器人配置'; draft.value = null
  page.value = 1; chatFilter.value = ''; keywordFilter.value = ''
  rules.value = await botAPI.rules(fresh.id)
  await loadMessages()
}

function closeBot() {
  selected.value = null
  rules.value = []; messages.value = []; total.value = 0
}

async function loadMessages(next = 1) {
  if (!selected.value) return
  const current = ++revision
  const result = await botAPI.messages(selected.value.id, next, chatFilter.value, keywordFilter.value)
  if (current !== revision) return
  messages.value = result.items; total.value = result.total; page.value = next
}

async function save() {
  if (!selected.value) return
  selected.value = await botAPI.saveBot(selected.value.id, {
    ...selected.value,
    bot_token: token.value.trim(),
    chat_ids: chats.value.split('\n').map(value => value.trim()).filter(Boolean),
  })
  token.value = ''
  await refreshStatuses()
}

function edit(rule?: Rule) {
  if (!selected.value) return
  draft.value = rule ? { ...rule } : { id: 0, bot_id: selected.value.id, keyword: '', match_type: 'contains', reply_template: '{channel_status}', enabled: true, sort: 0 }
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
  await action(async () => { await botAPI.deleteRule(selected.value!.id, rule.id); rules.value = await botAPI.rules(selected.value!.id) }, '规则已删除')
}

onMounted(async () => {
  await action(async () => { await loadBots(); loaded.value = true })
  timer = setInterval(() => { void refreshStatuses().catch(() => undefined) }, 5000)
})
onUnmounted(() => { if (timer) clearInterval(timer); revision++ })
</script>

<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-3xl space-y-5">
      <header>
        <h1 class="text-2xl font-semibold">消息机器人</h1>
        <p class="mt-1 text-sm text-gray-500">Telegram · 定时推送、渠道状态与公告查询、关键词回复</p>
      </header>
      <p v-if="error" role="alert" class="rounded-xl bg-red-50 p-3 text-sm text-red-700">{{ error }}</p>
      <p v-if="notice" role="status" class="rounded-xl bg-green-50 p-3 text-sm text-green-700">{{ notice }}</p>

      <!-- 机器人列表 -->
      <div v-if="!selected" class="space-y-4">
        <div class="flex items-center justify-between gap-3">
          <p class="text-sm text-gray-500">共 {{ bots.length }} 个机器人，各自独立配置与规则。</p>
          <button class="btn btn-primary shrink-0" @click="creating = !creating">新增机器人</button>
        </div>

        <form v-if="creating" class="space-y-4 rounded-2xl border p-5" @submit.prevent="action(createBot, '机器人已创建，已生成默认关键词规则')">
          <h2 class="font-semibold">新增机器人</h2>
          <label class="block text-sm">名称（备注名）<input v-model="newName" class="input mt-2 w-full" placeholder="例如：渠道监控机器人" /></label>
          <label class="block text-sm">平台
            <select v-model="newPlatform" class="input mt-2 w-full">
              <option value="telegram">Telegram</option>
              <option value="qq" disabled>QQ（即将支持）</option>
            </select>
          </label>
          <label class="block text-sm">Bot Token<input v-model="newToken" type="password" autocomplete="new-password" required class="input mt-2 w-full" placeholder="从 BotFather 获取" /></label>
          <p class="text-xs leading-relaxed text-gray-500">创建时自动验证 token 并读取机器人用户名，同时生成默认规则：发送「渠道状态」或 /status 查询渠道状态。</p>
          <div class="flex gap-3"><button type="submit" class="btn btn-primary flex-1">创建并配置</button><button type="button" class="btn btn-secondary" @click="creating = false">取消</button></div>
        </form>

        <div v-if="!bots.length && loaded" class="rounded-2xl border p-8 text-center text-sm text-gray-500">暂无机器人，点击「新增机器人」添加第一个。</div>

        <article v-for="bot in bots" :key="bot.id" class="space-y-3 rounded-2xl border p-5">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="truncate text-base font-semibold">{{ bot.name }}</h2>
              <p class="mt-0.5 truncate text-sm text-gray-500">{{ bot.bot_username ? '@' + bot.bot_username : '未配置 token' }}</p>
            </div>
            <span class="shrink-0 rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-800">{{ bot.platform === 'telegram' ? 'Telegram' : bot.platform }}</span>
          </div>
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <span class="rounded-lg px-2 py-1" :class="statuses[bot.id]?.polling || statuses[bot.id]?.pushing ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500 dark:bg-dark-800'">
              {{ !bot.bot_token ? '未配置' : (statuses[bot.id]?.polling || statuses[bot.id]?.pushing ? '运行中' : '已停止') }}
            </span>
            <span class="rounded-lg bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-800">收消息 {{ bot.receive_enabled ? '开' : '关' }}</span>
            <span class="rounded-lg bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-800">定时推送 {{ bot.push_enabled ? '开' : '关' }}</span>
          </div>
          <p v-if="statuses[bot.id]?.last_error" class="break-words text-xs text-red-600">{{ statuses[bot.id]?.last_error }}</p>
          <div class="flex gap-3 pt-1">
            <button class="btn btn-primary flex-1" :disabled="busy" @click="action(() => openBot(bot))">配置</button>
            <button class="btn btn-secondary text-red-600" :disabled="busy" @click="removeBot(bot)">删除</button>
          </div>
        </article>
      </div>

      <!-- 单个机器人管理 -->
      <div v-else class="space-y-5">
        <div>
          <button class="text-sm text-primary-600" @click="closeBot">← 返回机器人列表</button>
          <h2 class="mt-2 text-xl font-semibold">{{ selected.name }}</h2>
          <p v-if="selected.bot_username" class="mt-0.5 text-sm text-gray-500">@{{ selected.bot_username }}</p>
        </div>

        <nav class="grid grid-cols-3 gap-1 rounded-2xl bg-gray-100 p-1 dark:bg-dark-800" aria-label="机器人管理页签">
          <button v-for="name in tabs" :key="name" class="rounded-xl py-2 text-sm" :class="tab === name ? 'bg-white font-medium shadow-sm dark:bg-dark-900' : 'text-gray-500'" @click="tab = name">{{ name }}</button>
        </nav>

        <fieldset :disabled="busy" class="min-w-0 space-y-5">
          <!-- 配置 -->
          <form v-if="tab === '机器人配置'" class="space-y-5" @submit.prevent="action(save, '机器人配置已保存，任务正在更新')">
            <section class="space-y-4 rounded-2xl border p-5">
              <h3 class="font-semibold">连接设置</h3>
              <label class="block text-sm">名称（备注名）<input v-model="selected.name" required class="input mt-2 w-full" /></label>
              <label class="block text-sm">Bot Token <span class="text-gray-500">{{ selected.bot_token ? `已保存 ${selected.bot_token}` : '尚未配置' }}</span><input v-model="token" type="password" autocomplete="new-password" class="input mt-2 w-full" placeholder="留空保留原 token" /></label>
              <p class="text-xs leading-relaxed text-gray-500">保存时自动验证并读取机器人用户名。群聊里需 @机器人或回复机器人消息才触发关键词规则。</p>
              <label class="flex items-center gap-3 py-1 text-sm"><input v-model="selected.receive_enabled" type="checkbox" class="h-4 w-4" />启用收消息（关键词回复）</label>
            </section>

            <section class="space-y-4 rounded-2xl border p-5">
              <h3 class="font-semibold">定时推送</h3>
              <label class="flex items-center gap-3 py-1 text-sm"><input v-model="selected.push_enabled" type="checkbox" class="h-4 w-4" />启用定时推送</label>
              <label class="block text-sm">推送目标 chat_id（每行一个）<textarea v-model="chats" class="input mt-2 w-full font-mono" rows="4" placeholder="-1001234567890" /></label>
              <label class="block text-sm">推送间隔（分钟）<input v-model.number="selected.interval_minutes" type="number" min="1" max="43200" required class="input mt-2 w-full" /></label>
              <p class="text-xs leading-relaxed text-gray-500">推送时刻对齐整点：间隔 60 分钟即每小时 00 分推送，30 分钟即每小时 00 分、30 分推送。</p>
              <TemplateInput v-model="selected.push_template" label="推送模板" />
              <button type="button" class="btn btn-secondary w-full" :disabled="!selected.bot_token" @click="action(() => botAPI.pushNow(selected!.id), '已按推送模板发送到全部推送目标')">立即推送一次</button>
              <p class="text-xs text-gray-500">按已保存的配置推送，刚改过目标或模板请先保存。</p>
            </section>

            <section class="space-y-4 rounded-2xl border p-5">
              <h3 class="font-semibold">状态与测试</h3>
              <div class="space-y-1.5 text-sm">
                <p>轮询：{{ statuses[selected.id]?.polling ? '运行中' : '已停止' }} · 定时推送：{{ statuses[selected.id]?.pushing ? '运行中' : '已停止' }}</p>
                <p v-if="statuses[selected.id]?.last_error" class="break-words text-red-600">最近错误：{{ statuses[selected.id]?.last_error }}<span v-if="statuses[selected.id]?.last_error_at">（{{ new Date(statuses[selected.id]?.last_error_at || '').toLocaleString() }}）</span></p>
              </div>
              <label class="block text-sm">测试 chat_id<input v-model="testChat" class="input mt-2 w-full" placeholder="指定测试接收人或群聊 ID" /></label>
              <button type="button" class="btn btn-secondary w-full" :disabled="!testChat" @click="action(() => botAPI.test(selected!.id, testChat), '测试消息已发送')">发送测试消息</button>
            </section>

            <div class="sticky bottom-4">
              <button type="submit" class="btn btn-primary w-full py-3 shadow-lg">{{ busy ? '保存中…' : '保存配置' }}</button>
            </div>
          </form>

          <!-- 收到的消息 -->
          <section v-else-if="tab === '收到的消息'" class="space-y-4">
            <form class="space-y-3 rounded-2xl border p-4" @submit.prevent="action(() => loadMessages())">
              <input v-model="chatFilter" class="input w-full" placeholder="按 chat_id 过滤" aria-label="聊天过滤" />
              <input v-model="keywordFilter" class="input w-full" placeholder="按消息关键词过滤" aria-label="关键词过滤" />
              <button class="btn btn-secondary w-full">查询</button>
            </form>
            <div v-if="!messages.length" class="rounded-2xl border p-8 text-center text-sm text-gray-500">暂无消息</div>
            <article v-for="m in messages" :key="m.id" class="space-y-2 rounded-2xl border p-4">
              <div class="flex items-baseline justify-between gap-3">
                <span class="min-w-0 truncate font-medium">{{ m.sender_name || m.sender_id }}</span>
                <time class="shrink-0 text-xs text-gray-500">{{ new Date(m.created_at).toLocaleString() }}</time>
              </div>
              <p class="text-xs text-gray-500">{{ m.chat_id }} · {{ m.chat_type }} · {{ m.platform }}</p>
              <p class="whitespace-pre-wrap break-words text-sm leading-relaxed">{{ m.text }}</p>
              <div class="flex flex-wrap gap-2 text-xs">
                <span class="rounded-lg bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-800">{{ m.rule_id ? '命中规则：' + (rules.find(r => r.id === m.rule_id)?.keyword || '#' + m.rule_id) : '未命中规则' }}</span>
                <span class="rounded-lg px-2 py-1" :class="m.replied ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500 dark:bg-dark-800'">{{ m.replied ? '已回复' : '未回复' }}</span>
              </div>
            </article>
            <div class="flex items-center justify-between gap-3">
              <button class="btn btn-secondary" :disabled="page <= 1" @click="action(() => loadMessages(page - 1))">上一页</button>
              <span class="text-sm text-gray-500">第 {{ page }} 页 · 共 {{ total }} 条</span>
              <button class="btn btn-secondary" :disabled="page * 20 >= total" @click="action(() => loadMessages(page + 1))">下一页</button>
            </div>
            <p class="text-xs text-gray-500">保留最近 30 天且最多 2000 条消息；清理任务每小时执行。</p>
          </section>

          <!-- 关键词回复 -->
          <section v-else class="space-y-4">
            <div class="flex items-center justify-between gap-3">
              <p class="text-sm text-gray-500">只回复第一条命中的规则，仅对 {{ selected.name }} 生效。</p>
              <button class="btn btn-primary shrink-0" @click="edit()">新增规则</button>
            </div>
            <form v-if="draft" class="space-y-4 rounded-2xl border p-5" @submit.prevent="action(saveRule, '规则已保存')">
              <h3 class="font-semibold">{{ draft.id ? '编辑规则' : '新增规则' }}</h3>
              <label class="block text-sm">关键词<input v-model="draft.keyword" required maxlength="500" class="input mt-2 w-full" /></label>
              <label class="block text-sm">匹配方式
                <select v-model="draft.match_type" class="input mt-2 w-full"><option value="contains">包含</option><option value="exact">完全匹配</option><option value="command">命令</option></select>
              </label>
              <label class="flex items-center gap-3 py-1 text-sm"><input v-model="draft.enabled" type="checkbox" class="h-4 w-4" />启用</label>
              <label class="block text-sm">排序（小的先匹配）<input v-model.number="draft.sort" type="number" required class="input mt-2 w-full" /></label>
              <TemplateInput v-model="draft.reply_template" label="回复模板" />
              <div class="flex gap-3"><button class="btn btn-primary flex-1">保存规则</button><button type="button" class="btn btn-secondary" @click="draft = null">取消</button></div>
            </form>
            <div v-if="!rules.length" class="rounded-2xl border p-8 text-center text-sm text-gray-500">暂无规则</div>
            <article v-for="rule in rules" :key="rule.id" class="space-y-3 rounded-2xl border p-4">
              <div class="flex items-start justify-between gap-3">
                <h3 class="min-w-0 break-words font-semibold">{{ rule.keyword }}</h3>
                <div class="flex shrink-0 gap-2 text-xs">
                  <span class="rounded-lg bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-800">{{ matchLabels[rule.match_type] }}</span>
                  <span class="rounded-lg px-2 py-1" :class="rule.enabled ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500 dark:bg-dark-800'">{{ rule.enabled ? '启用' : '停用' }}</span>
                </div>
              </div>
              <p class="whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-3 text-xs leading-relaxed text-gray-600 dark:bg-dark-800">{{ rule.reply_template }}</p>
              <div class="flex items-center gap-3">
                <span class="text-xs text-gray-400">排序 {{ rule.sort }}</span>
                <span class="flex-1"></span>
                <button class="btn btn-secondary" @click="edit(rule)">编辑</button>
                <button class="btn btn-secondary text-red-600" @click="removeRule(rule)">删除</button>
              </div>
            </article>
          </section>
        </fieldset>
      </div>
    </div>
  </AppLayout>
</template>
