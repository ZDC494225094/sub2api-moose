<template>
  <AnnouncementPanel
    :announcements="panelAnnouncements"
    :open="!!displayedAnnouncement"
    :show-dismiss-today="!preview"
    :initial-selected-announcement="displayedAnnouncement"
    close-on-reader-close
    @close="handleDismiss"
    @dismiss-today="handleDismissToday"
    @select="handleSelect"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import AnnouncementPanel from '@/features/premium-home/runtime/AnnouncementPanel.vue'
import { useAnnouncementStore, useAuthStore } from '@/stores'
import type { Announcement, UserAnnouncement } from '@/types'

type PreviewAnnouncement = Pick<
  Announcement | UserAnnouncement,
  'id' | 'title' | 'content' | 'notify_mode' | 'created_at' | 'updated_at' | 'starts_at' | 'ends_at'
>

const props = withDefaults(defineProps<{
  announcement?: PreviewAnnouncement | null
  preview?: boolean
}>(), {
  announcement: null,
  preview: false,
})

const emit = defineEmits<{
  close: []
}>()

const announcementStore = useAnnouncementStore()
const authStore = useAuthStore()
const DISMISS_KEY_PREFIX = 'console-announcement-dismiss-date'

const displayedAnnouncement = computed<UserAnnouncement | null>(() => {
  const announcement = props.preview ? props.announcement : announcementStore.currentPopup
  if (!announcement) return null
  return { ...announcement } as UserAnnouncement
})

const panelAnnouncements = computed(() => {
  if (props.preview) return displayedAnnouncement.value ? [displayedAnnouncement.value] : []
  return announcementStore.announcements
})

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
  if (props.preview) {
    emit('close')
    return
  }
  announcementStore.dismissPopup()
}

function handleDismissToday() {
  localStorage.setItem(dismissStorageKey(), todayDismissKey())
  handleDismiss()
}

function handleSelect(announcement: UserAnnouncement) {
  if (!props.preview && !announcement.read_at) {
    void announcementStore.markAsRead(announcement.id)
  }
}

watch(
  displayedAnnouncement,
  (popup) => {
    if (popup && !props.preview && isDismissedToday()) {
      handleDismiss()
      return
    }
    document.body.style.overflow = popup ? 'hidden' : ''
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  if (displayedAnnouncement.value) document.body.style.overflow = ''
})
</script>
