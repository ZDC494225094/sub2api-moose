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
  timer = setInterval(() => {
    void refreshStatuses().catch(() => undefined)
    if (selected.value) {
      const id = selected.value.id
      void botAPI.status(id).then(s => { statuses.value = { ...statuses.value, [id]: s } }).catch(() => undefined)
    }
  }, 5000)
})
onUnmounted(() => { if (timer) clearInterval(timer); revision++ })
</script>

<template>
  <AppLayout>
    <div class="space-y-6">
      <header>
        <h1 class="text-2xl font-semibold">消息机器人</h1>
        <p class="mt-2 text-sm text-gray-500">Telegram · 定时推送、渠道状态与公告查询、关键词回复</p>
      </header>
      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-red-700">{{ error }}</p>
      <p v-if="notice" role="status" class="rounded-lg bg-green-50 p-3 text-green-700">{{ notice }}</p>

      <!-- 机器人列表 -->
      <fieldset v-if="!selected" :disabled="busy || !loaded" class="min-w-0 space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-sm text-gray-500">共 {{ bots.length }} 个机器人，每个机器人独立配置、独立接收消息与关键词规则。</p>
          <button class="btn btn-primary" @click="creating = !creating">新增机器人</button>
        </div>
        <form v-if="creating" class="max-w-2xl space-y-4 rounded-xl border p-5" @submit.prevent="action(createBot, '机器人已创建，已生成默认关键词规则')">
          <h2 class="font-semibold">新增机器人</h2>
          <label class="block text-sm">名称（备注名）<input v-model="newName" class="input mt-2 w-full" placeholder="例如：渠道监控机器人" /></label>
          <label class="block text-sm">平台
            <select v-model="newPlatform" class="input mt-2 w-full">
              <option value="telegram">Telegram</option>
              <option value="qq" disabled>QQ（即将支持）</option>
            </select>
          </label>
          <label class="block text-sm">Bot Token<input v-model="newToken" type="password" autocomplete="new-password" required class="input mt-2 w-full" placeholder="从 BotFather 获取" /></label>
          <p class="text-xs text-gray-500">创建时自动验证 token 并读取机器人用户名，同时生成默认规则：发送「渠道状态」或 /status 查询渠道状态。</p>
          <div class="flex gap-3"><button type="submit" class="btn btn-primary">创建并配置</button><button type="button" class="btn btn-secondary" @click="creating = false">取消</button></div>
        </form>
        <div class="overflow-x-auto rounded-xl border">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 dark:bg-dark-800"><tr><th>名称</th><th>平台</th><th>机器人</th><th>收消息</th><th>定时推送</th><th>运行状态</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="bot in bots" :key="bot.id" class="border-t">
                <td class="font-medium">{{ bot.name }}</td>
                <td>{{ bot.platform === 'telegram' ? 'Telegram' : bot.platform }}</td>
                <td>{{ bot.bot_username ? '@' + bot.bot_username : '未配置 token' }}</td>
                <td>{{ bot.receive_enabled ? '开' : '关' }}</td>
                <td>{{ bot.push_enabled ? '开' : '关' }}</td>
                <td>
                  <span v-if="!bot.bot_token">未配置</span>
                  <span v-else-if="statuses[bot.id]?.polling || statuses[bot.id]?.pushing">运行中</span>
                  <span v-else>已停止</span>
                  <span v-if="statuses[bot.id]?.last_error" class="block max-w-xs truncate text-xs text-red-600" :title="statuses[bot.id]?.last_error || ''">{{ statuses[bot.id]?.last_error }}</span>
                </td>
                <td class="space-x-3 whitespace-nowrap">
                  <button class="btn btn-secondary" @click="action(() => openBot(bot))">配置</button>
                  <button class="text-red-600" @click="removeBot(bot)">删除</button>
                </td>
              </tr>
              <tr v-if="!bots.length"><td colspan="7" class="p-8 text-center text-gray-500">暂无机器人，点击「新增机器人」添加第一个。</td></tr>
            </tbody>
          </table>
        </div>
      </fieldset>

      <!-- 单个机器人管理 -->
      <fieldset v-else :disabled="busy" class="min-w-0 space-y-6">
        <div>
          <button class="text-sm text-primary-600" @click="closeBot">← 返回机器人列表</button>
          <h2 class="mt-3 text-xl font-semibold">{{ selected.name }}<span v-if="selected.bot_username" class="ml-2 text-base font-normal text-gray-500">@{{ selected.bot_username }}</span></h2>
        </div>
        <nav class="flex gap-2 overflow-x-auto border-b pb-3" aria-label="机器人管理页签"><button v-for="name in ['机器人配置', '收到的消息', '关键词回复']" :key="name" class="btn whitespace-nowrap" :class="tab === name ? 'btn-primary' : 'btn-secondary'" @click="tab = name">{{ name }}</button></nav>
        <form v-if="tab === '机器人配置'" class="grid gap-6 lg:grid-cols-2" @submit.prevent="action(save, '机器人配置已保存，任务正在更新')">
          <section class="space-y-5 rounded-xl border p-5">
            <h3 class="font-semibold">Telegram 连接</h3>
            <label class="block text-sm">名称（备注名）<input v-model="selected.name" required class="input mt-2 w-full" /></label>
            <label class="block text-sm">Bot Token <span class="text-gray-500">{{ selected.bot_token ? `已保存 ${selected.bot_token}` : '尚未配置' }}</span><input v-model="token" type="password" autocomplete="new-password" class="input mt-2 w-full" placeholder="留空保留原 token" /></label>
            <p class="text-xs text-gray-500">保存时自动验证 getMe 并读取机器人用户名。群聊需 @机器人或回复机器人消息才触发规则。</p>
            <label class="flex gap-2 text-sm"><input v-model="selected.receive_enabled" type="checkbox" />启用收消息</label>
            <label class="flex gap-2 text-sm"><input v-model="selected.push_enabled" type="checkbox" />启用定时推送</label>
            <label class="block text-sm">推送目标 chat_id（每行一个）<textarea v-model="chats" class="input mt-2 w-full font-mono" rows="4" placeholder="-1001234567890" /></label>
            <label class="block text-sm">推送间隔（分钟）<input v-model.number="selected.interval_minutes" type="number" min="1" max="43200" required class="input mt-2 w-full" /></label>
            <button type="submit" class="btn btn-primary">{{ busy ? '保存中…' : '保存配置' }}</button>
          </section>
          <section class="space-y-5 rounded-xl border p-5">
            <TemplateInput v-model="selected.push_template" label="推送模板" />
            <div class="rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800"><p>机器人：{{ statuses[selected.id]?.bot.username ? '@' + statuses[selected.id]?.bot.username : (selected.bot_username ? '@' + selected.bot_username : '未配置') }}</p><p class="mt-2">轮询：{{ statuses[selected.id]?.polling ? '运行中' : '已停止' }} · 定时推送：{{ statuses[selected.id]?.pushing ? '运行中' : '已停止' }}</p><p v-if="statuses[selected.id]?.last_error" class="mt-2 break-words text-red-600">最近错误：{{ statuses[selected.id]?.last_error }} {{ statuses[selected.id]?.last_error_at ? new Date(statuses[selected.id]?.last_error_at || '').toLocaleString() : '' }}</p></div>
            <label class="block text-sm">测试 chat_id<input v-model="testChat" class="input mt-2 w-full" placeholder="指定测试接收人或群聊 ID" /></label>
            <button type="button" class="btn btn-secondary" :disabled="!testChat" @click="action(() => botAPI.test(selected!.id, testChat), '测试消息已发送')">发送测试消息</button>
          </section>
        </form>
        <section v-else-if="tab === '收到的消息'" class="space-y-4">
          <form class="flex flex-wrap gap-3" @submit.prevent="action(() => loadMessages())"><input v-model="chatFilter" class="input" placeholder="chat_id" aria-label="聊天过滤" /><input v-model="keywordFilter" class="input" placeholder="消息关键词" aria-label="关键词过滤" /><button class="btn btn-secondary">查询</button></form>
          <div class="overflow-x-auto rounded-xl border"><table class="w-full text-left text-sm"><thead class="bg-gray-50 dark:bg-dark-800"><tr><th>时间</th><th>平台</th><th>聊天</th><th>发送人</th><th>内容</th><th>命中规则</th><th>已回复</th></tr></thead><tbody><tr v-for="m in messages" :key="m.id" class="border-t"><td class="whitespace-nowrap">{{ new Date(m.created_at).toLocaleString() }}</td><td>{{ m.platform }}</td><td>{{ m.chat_id }}<small class="block">{{ m.chat_type }}</small></td><td>{{ m.sender_name || m.sender_id }}</td><td class="min-w-48 max-w-lg whitespace-pre-wrap break-words">{{ m.text }}</td><td>{{ m.rule_id ? rules.find(r => r.id === m.rule_id)?.keyword || '#' + m.rule_id : '—' }}</td><td>{{ m.replied ? '是' : '否' }}</td></tr><tr v-if="!messages.length"><td colspan="7" class="p-8 text-center text-gray-500">暂无消息</td></tr></tbody></table></div>
          <div class="flex items-center gap-3"><button class="btn btn-secondary" :disabled="page <= 1" @click="action(() => loadMessages(page - 1))">上一页</button><span class="text-sm">第 {{ page }} 页 · 共 {{ total }} 条</span><button class="btn btn-secondary" :disabled="page * 20 >= total" @click="action(() => loadMessages(page + 1))">下一页</button></div>
          <p class="text-xs text-gray-500">保留最近 30 天且最多 2000 条消息；清理任务每小时执行。</p>
        </section>
        <section v-else class="space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3"><p class="text-sm text-gray-500">按排序值、ID 升序匹配，只回复第一条命中规则。规则只对当前机器人 {{ selected.name }} 生效。</p><button class="btn btn-primary" @click="edit()">新增规则</button></div>
          <div class="overflow-x-auto rounded-xl border"><table class="w-full text-left text-sm"><thead><tr><th>关键词</th><th>匹配方式</th><th>启用</th><th>排序</th><th>操作</th></tr></thead><tbody><tr v-for="rule in rules" :key="rule.id" class="border-t"><td>{{ rule.keyword }}</td><td>{{ { contains: '包含', exact: '完全匹配', command: '命令' }[rule.match_type] }}</td><td>{{ rule.enabled ? '是' : '否' }}</td><td>{{ rule.sort }}</td><td class="space-x-3"><button class="text-primary-600" @click="edit(rule)">编辑</button><button class="text-red-600" @click="removeRule(rule)">删除</button></td></tr></tbody></table></div>
          <form v-if="draft" class="max-w-3xl space-y-4 rounded-xl border p-5" @submit.prevent="action(saveRule, '规则已保存')"><h3 class="font-semibold">{{ draft.id ? '编辑规则' : '新增规则' }}</h3><label class="block text-sm">关键词<input v-model="draft.keyword" required maxlength="500" class="input mt-2 w-full" /></label><label class="block text-sm">匹配方式<select v-model="draft.match_type" class="input ml-3"><option value="contains">包含</option><option value="exact">完全匹配</option><option value="command">命令</option></select></label><label class="flex gap-2 text-sm"><input v-model="draft.enabled" type="checkbox" />启用</label><label class="block text-sm">排序<input v-model.number="draft.sort" type="number" required class="input ml-3" /></label><TemplateInput v-model="draft.reply_template" label="回复模板" /><div class="flex gap-3"><button class="btn btn-primary">保存规则</button><button type="button" class="btn btn-secondary" @click="draft = null">取消</button></div></form>
        </section>
      </fieldset>
    </div>
  </AppLayout>
</template>

<style scoped>
th, td { padding: 0.85rem 1rem; }
</style>
