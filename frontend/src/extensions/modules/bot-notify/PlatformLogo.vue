<script setup lang="ts">
defineProps<{ platform: string }>()
</script>

<template>
  <svg
    v-if="platform === 'telegram'"
    viewBox="0 0 24 24"
    role="img"
    aria-label="Telegram"
    class="platform-logo"
  >
    <circle cx="12" cy="12" r="12" fill="#229ED9" />
    <path
      fill="#fff"
      d="m5.1 11.7 12.1-4.8c.6-.2 1 .1.8.9l-2.1 10c-.2.7-.6.9-1.2.5l-3.2-2.4-1.6 1.5c-.2.2-.3.3-.6.3l.2-3.3 6-5.4c.3-.2-.1-.4-.4-.2l-7.4 4.7-3.2-1c-.7-.2-.7-.7.6-.8Z"
    />
  </svg>
  <svg v-else-if="platform === 'qq'" viewBox="0 0 32 32" role="img" aria-label="QQ" class="platform-logo">
    <path
      fill="#F9B32B"
      d="M12.9 25.9c-2.6-.7-5.9-.1-6.3 1.5-.3 1.4 3.2 2 6.4 1.2l3-1.2 3 1.2c3.2.8 6.7.2 6.4-1.2-.4-1.6-3.7-2.2-6.3-1.5Z"
    />
    <path
      fill="#222"
      d="M8.3 15c-1.8 1.8-3.3 5.1-2.6 6.8.3.9 1.4.6 2.8-.6.2 4.6 3.6 6.4 7.5 6.4s7.3-1.8 7.5-6.4c1.4 1.2 2.5 1.5 2.8.6.7-1.7-.8-5-2.6-6.8C23.5 7.9 21.3 3 16 3s-7.5 4.9-7.7 12Z"
    />
    <ellipse cx="16" cy="20" rx="6.2" ry="6.4" fill="#fff" />
    <path fill="#ED2939" d="M8.1 14.7c4.3 2.2 11.5 2.2 15.8 0l.4 3.1c-4.5 2.3-12.1 2.3-16.6 0Z" />
    <path fill="#ED2939" d="m10.3 17.4 3.2 1.1-1.8 4.5-3.1-.9Z" />
    <ellipse cx="13.2" cy="10.4" rx="1.8" ry="2.5" fill="#fff" />
    <ellipse cx="18.8" cy="10.4" rx="1.8" ry="2.5" fill="#fff" />
    <ellipse cx="13.9" cy="10.7" rx=".7" ry="1.2" fill="#222" />
    <ellipse cx="18.1" cy="10.7" rx=".7" ry="1.2" fill="#222" />
    <path fill="#F9B32B" d="M12.5 13.7c1-1.2 6-1.2 7 0-.7 1.7-6.3 1.7-7 0Z" />
  </svg>
  <svg
    v-else
    viewBox="0 0 24 24"
    role="img"
    aria-label="消息机器人"
    class="platform-logo text-gray-400"
    fill="none"
    stroke="currentColor"
    stroke-width="1.5"
  >
    <rect x="3" y="5" width="18" height="14" rx="4" />
    <path d="M8 10h.01M16 10h.01M8 14h8" />
  </svg>
</template>

<style scoped>
.platform-logo {
  display: inline-block;
  width: 1.75rem;
  height: 1.75rem;
  flex-shrink: 0;
}
</style>
