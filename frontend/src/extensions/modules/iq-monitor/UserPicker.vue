<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { iqMonitorAdminAPI, type UserOption } from './api'

defineProps<{ placeholder?: string }>()
const emit = defineEmits<{ pick: [user: UserOption] }>()

const q = ref('')
const items = ref<UserOption[]>([])
const open = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

watch(q, (value) => {
  clearTimeout(timer)
  const text = value.trim()
  if (!text) {
    items.value = []
    return
  }
  timer = setTimeout(async () => {
    const mine = ++seq
    try {
      const list = await iqMonitorAdminAPI.users(text)
      if (mine === seq) {
        items.value = list
        open.value = true
      }
    } catch {
      if (mine === seq) items.value = []
    }
  }, 250)
})

function pick(u: UserOption) {
  emit('pick', u)
  q.value = ''
  items.value = []
  open.value = false
}

let blurTimer: ReturnType<typeof setTimeout> | undefined
function close() {
  blurTimer = setTimeout(() => { open.value = false }, 150)
}

onBeforeUnmount(() => {
  clearTimeout(timer)
  clearTimeout(blurTimer)
})
</script>

<template>
  <div class="relative">
    <input
      v-model="q"
      class="input"
      :placeholder="placeholder ?? '输入邮箱、用户名或 ID 搜索'"
      role="combobox"
      :aria-expanded="open && items.length > 0"
      @focus="open = true"
      @blur="close"
    />
    <ul v-if="open && items.length" class="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-xl bg-white py-1 text-sm shadow-lg ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700" role="listbox">
      <li v-for="u in items" :key="u.id">
        <button type="button" class="block w-full px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-dark-700" @mousedown.prevent="pick(u)">
          <span class="text-gray-900 dark:text-white">{{ u.email }}</span>
          <span class="ml-2 text-xs text-gray-400">#{{ u.id }}<template v-if="u.username"> · {{ u.username }}</template></span>
        </button>
      </li>
    </ul>
  </div>
</template>
