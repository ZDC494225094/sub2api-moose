<script setup lang="ts">
defineProps<{ options: { newUsersOnly: boolean } }>()
const emit = defineEmits<{ 'update:newUsersOnly': [value: boolean] }>()
</script>

<template>
  <div class="rounded-lg border border-primary-200 bg-primary-50 p-3 text-sm dark:border-primary-800 dark:bg-primary-900/20">
    <label class="flex items-center gap-2 font-medium">
      <input :checked="options.newUsersOnly" type="checkbox" class="rounded" @change="emit('update:newUsersOnly', ($event.target as HTMLInputElement).checked)" />
      仅新注册用户可用
    </label>
    <p class="mt-2 text-gray-600 dark:text-gray-400">仅允许注册时间不早于本批次生成时间的用户兑换。未勾选则不限制注册时间；同批次仍每人限兑一次。</p>
  </div>
</template>
