import { computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useExtensionStore } from '@/extensions/store'
import { iqMonitorAPI } from './api'

// Per-user visibility of the 降智监测 entry: the page may be open to every
// signed-in user or to an allowlist, so the sidebar asks once per user.
const allowed = ref(false)
const loadedFor = ref<number | null>(null)
let pending: Promise<boolean> | null = null

export const IQ_MONITOR_PATH = '/iq-monitor'

async function load(force = false): Promise<boolean> {
  const auth = useAuthStore()
  const userId = auth.user?.id ?? null
  if (!auth.isAuthenticated || userId === null || !useExtensionStore().enabled('iq-monitor')) {
    allowed.value = false
    loadedFor.value = null
    return false
  }
  if (!force && loadedFor.value === userId) return allowed.value
  if (pending && !force) return pending
  pending = iqMonitorAPI.access()
    .then((ok) => {
      allowed.value = ok
      loadedFor.value = userId
      return ok
    })
    .catch(() => {
      allowed.value = false
      loadedFor.value = null
      return false
    })
    .finally(() => { pending = null })
  return pending
}

export function useIQMonitorAccess() {
  return {
    canViewIQMonitor: computed(() => allowed.value),
    refreshIQMonitorAccess: load,
  }
}
