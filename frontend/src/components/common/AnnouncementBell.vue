<template>
  <div>
    <button
      type="button"
      class="relative flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition-all hover:scale-105 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-800"
      :class="{ 'text-blue-600 dark:text-blue-400': unreadCount > 0 }"
      :aria-label="t('announcements.title')"
      @click="openPanel"
    >
      <Icon name="bell" size="md" />
      <span v-if="unreadCount > 0" class="absolute right-1 top-1 flex h-2 w-2">
        <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-500 opacity-75"></span>
        <span class="relative inline-flex h-2 w-2 rounded-full bg-red-500"></span>
      </span>
    </button>

    <AnnouncementPanel
      :announcements="announcements"
      :open="isPanelOpen"
      @close="closePanel"
      @dismiss-today="handleDismissToday"
      @select="handleSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useI18n } from 'vue-i18n'
import AnnouncementPanel from '@/features/premium-home/runtime/AnnouncementPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAnnouncementStore, useAuthStore } from '@/stores'
import type { UserAnnouncement } from '@/types'

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const authStore = useAuthStore()
const { announcements } = storeToRefs(announcementStore)
const unreadCount = computed(() => announcementStore.unreadCount)
const isPanelOpen = ref(false)
const DISMISS_KEY_PREFIX = 'console-announcement-dismiss-date'

function todayDismissKey() {
  return new Date().toISOString().slice(0, 10)
}

function dismissStorageKey() {
  return `${DISMISS_KEY_PREFIX}:${authStore.user?.id ?? 'anonymous'}`
}

function openPanel() {
  isPanelOpen.value = true
}

function closePanel() {
  isPanelOpen.value = false
}

function handleDismissToday() {
  localStorage.setItem(dismissStorageKey(), todayDismissKey())
  closePanel()
}

function handleSelect(announcement: UserAnnouncement) {
  if (!announcement.read_at) {
    void announcementStore.markAsRead(announcement.id)
  }
}

watch(
  [isPanelOpen, () => announcementStore.currentPopup],
  ([panelOpen, popupOpen]) => {
    document.body.style.overflow = panelOpen || popupOpen ? 'hidden' : ''
  }
)

onBeforeUnmount(() => {
  document.body.style.overflow = ''
})
</script>
