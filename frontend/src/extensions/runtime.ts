import { onBeforeUnmount, onMounted, watch } from 'vue'
import type { Router } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { extensionFallback, extensionForPath } from './catalog'
import { extensionHistoricalPath } from './route-access'
import { EXTENSION_STORAGE_EVENT, useExtensionStore } from './store'
import { syncConsoleTheme } from './modules/console-theme'
import { useThemeModeRuntime } from './modules/console-theme/themeMode'

export function installCustomExtensionGuard(router: Router): void {
  router.beforeEach((to, from) => {
    // Hash/query changes inside one page are navigation, not permission checks.
    if (to.path === '/setup' || (from.matched.length > 0 && to.path === from.path)) return
    const store = useExtensionStore()
    store.setAdminSession(() => useAuthStore().isAdmin === true)
    const id = extensionForPath(to.path)
    if (!id || extensionHistoricalPath(to.path)) return
    // Render immediately. The backend gates every protected read/write; this
    // cache controls entry visibility only and is never an authorization grant.
    void store.refresh()
    if (store.flags[id] === false) {
      const publicPage = to.meta.requiresAuth === false
      return { path: publicPage ? '/home' : extensionFallback(id, useAuthStore().isAdmin), query: to.query, hash: to.hash }
    }
  })
}

/** Warm the presentation cache in parallel with translations. */
export function prefetchCustomExtensionState(): void {
  const store = useExtensionStore()
  store.setAdminSession(() => useAuthStore().isAdmin === true)
  void store.refresh()
}

// Called once by App.vue. No timers are created by individual menus or pages.
export function useCustomExtensionRuntime(router: Router): void {
  const store = useExtensionStore()
  const auth = useAuthStore()
  store.setAdminSession(() => auth.isAdmin === true)
  syncConsoleTheme(store)
  useThemeModeRuntime()
  // One background initialization is also needed for slots and menus on core
  // pages. It never delays routing; only auth changes or explicit saves refresh.
  const refreshFromExplicitChange = (event: StorageEvent) => {
    if (event.key !== EXTENSION_STORAGE_EVENT || router.currentRoute.value.path === '/setup') return
    void store.refresh(true)
  }
  watch(() => [auth.isAdmin, auth.isAuthenticated], () => {
    if (router.currentRoute.value.path !== '/setup') void store.refresh(true)
  })
  watch(() => router.currentRoute.value.path, (path, previous) => {
    if (previous === '/setup' && path !== '/setup') void store.refresh()
  })
  onMounted(() => {
    if (router.currentRoute.value.path !== '/setup') void store.refresh()
    window.addEventListener('storage', refreshFromExplicitChange)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('storage', refreshFromExplicitChange)
  })
}

