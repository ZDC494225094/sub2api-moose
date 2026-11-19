<script setup lang="ts">
import { computed, ref } from 'vue'
import { searchUsers, type SimpleUser } from '@/api/admin/usage'
import { getById } from '@/api/admin/users'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const ids = defineModel<number[]>({ required: true })
const results = ref<SimpleUser[]>([])
const labels = ref<Record<number, string>>({})
const loading = ref(false)
const emptyText = ref('请输入用户 ID、邮箱或用户名')
let sequence = 0

const options = computed(() => results.value.filter(u => !ids.value.includes(u.id)).map(u => ({
  value: u.id, label: `${u.email} · #${u.id}${u.deleted ? '（已删除）' : ''}`,
})))

async function search(query: string) {
  const current = ++sequence
  results.value = []
  loading.value = Boolean(query)
  emptyText.value = query ? '没有可添加的匹配用户' : '请输入用户 ID、邮箱或用户名'
  if (!query) return
  try {
    let users: SimpleUser[]
    if (/^\d+$/.test(query)) {
      const id = Number(query)
      if (!Number.isSafeInteger(id) || id <= 0) return
      const user = await getById(id, true)
      users = [{ id: user.id, email: user.email, deleted: Boolean(user.deleted_at) }]
    } else {
      users = await searchUsers(query)
    }
    if (current === sequence) results.value = users
  } catch {
    if (current === sequence) emptyText.value = '未能查到用户，请核对输入后重试'
  } finally {
    if (current === sequence) loading.value = false
  }
}

function add(value: unknown) {
  const user = results.value.find(u => u.id === value)
  if (!user || ids.value.includes(user.id) || ids.value.length >= 1000) return
  labels.value[user.id] = user.email
  ids.value = [...ids.value, user.id]
  results.value = []
}
</script>

<template>
  <div class="space-y-2">
    <Select
      :model-value="null" :options="options" remote :loading="loading" :empty-text="emptyText" :disabled="ids.length >= 1000"
      placeholder="添加排除用户" aria-label="添加排除用户" search-placeholder="搜索用户 ID、邮箱或用户名"
      @search="search" @update:model-value="add"
    />
    <p class="text-xs text-gray-500 dark:text-dark-400">已排除 {{ ids.length }} / 1000 人</p>
    <ul v-if="ids.length" class="flex max-h-40 flex-wrap gap-2 overflow-y-auto" aria-label="排除用户列表">
      <li v-for="id in ids" :key="id" class="badge badge-gray gap-1.5">
        <span class="max-w-52 truncate">{{ labels[id] }} #{{ id }}</span>
        <button type="button" :aria-label="`移除排除用户 ${id}`" class="rounded hover:text-red-600" @click="ids = ids.filter(v => v !== id)">
          <Icon name="x" size="xs" />
        </button>
      </li>
    </ul>
  </div>
</template>
