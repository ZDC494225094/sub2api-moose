<template>
  <AnnouncementPanel
    :announcements="announcementStore.announcements"
    :open="!!announcementStore.currentPopup"
    @close="handleDismiss"
    @dismiss-today="handleDismissToday"
    @select="handleSelect"
  />
</template>

<script setup lang="ts">
import { watch } from 'vue'
import AnnouncementPanel from '@/features/premium-home/runtime/AnnouncementPanel.vue'
import { useAnnouncementStore, useAuthStore } from '@/stores'
import type { UserAnnouncement } from '@/types'

const announcementStore = useAnnouncementStore()
const authStore = useAuthStore()
const DISMISS_KEY_PREFIX = 'console-announcement-dismiss-date'

function todayDismissKey() {
  return new Date().toISOString().slice(0, 10)
}

function dismissStorageKey() {
  return `${DISMISS_KEY_PREFIX}:${authStore.user?.id ?? 'anonymous'}`
}

function isDismissedToday() {
  return localStorage.getItem(dismissStorageKey()) === todayDismissKey()
}

function handleDismiss() {
  announcementStore.dismissPopup()
}

function handleDismissToday() {
  localStorage.setItem(dismissStorageKey(), todayDismissKey())
  handleDismiss()
}

function handleSelect(announcement: UserAnnouncement) {
  if (!announcement.read_at) {
    void announcementStore.markAsRead(announcement.id)
  }
}

// Keep the console's existing modal scroll-lock behavior. The header bell
// restores body scrolling when every announcement surface is closed.
watch(
  () => announcementStore.currentPopup,
  (popup) => {
    if (!popup) return
    if (isDismissedToday()) {
      handleDismiss()
      return
    }
    document.body.style.overflow = 'hidden'
  }
)
</script>
