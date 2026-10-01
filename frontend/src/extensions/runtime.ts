import { onBeforeUnmount, onMounted, watch } from 'vue'
import type { Router } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { extensionFallback, extensionForPath } from './catalog'
import { extensionHistoricalPath } from './route-access'
import { EXTENSION_REFRESH_MS, EXTENSION_STORAGE_EVENT, useExtensionStore } from './store'

export function installCustomExtensionGuard(router: Router): void {
  router.beforeEach(async to => {
    if (to.path === '/setup') return
    const store = useExtensionStore()
    const id = extensionForPath(to.path)
    if (!id || extensionHistoricalPath(to.path)) {
      // An extension-state outage must not delay core recovery or historical reads.
      // Page and backend admission still reject new business when state is unknown.
      void store.refresh()
      return
    }
    await store.refresh()
    if (!store.enabled(id)) return { path: extensionFallback(id, useAuthStore().isAdmin), query: to.query }
  })
}

// Called once by App.vue. No timers are created by individual menus or pages.
export function useCustomExtensionRuntime(router: Router): void {
  const store = useExtensionStore()
  const auth = useAuthStore()
  let timer: ReturnType<typeof setInterval> | undefined
  const refresh = () => { if (document.visibilityState !== 'hidden' && router.currentRoute.value.path !== '/setup') void store.refresh(true) }
  const onStorage = (event: StorageEvent) => { if (event.key === EXTENSION_STORAGE_EVENT) refresh() }
  watch(() => [store.flags, router.currentRoute.value.path], () => {
    const path = router.currentRoute.value.path
    const id = extensionForPath(path)
    if (store.loaded && id && !extensionHistoricalPath(path) && !store.enabled(id)) {
      void router.replace({ path: extensionFallback(id, auth.isAdmin), query: router.currentRoute.value.query }).catch(() => {})
    }
  }, { deep: true })
  onMounted(() => {
    refresh()
    timer = setInterval(refresh, EXTENSION_REFRESH_MS)
    document.addEventListener('visibilitychange', refresh)
    window.addEventListener('storage', onStorage)
  })
  onBeforeUnmount(() => {
    if (timer) clearInterval(timer)
    document.removeEventListener('visibilitychange', refresh)
    window.removeEventListener('storage', onStorage)
  })
}

