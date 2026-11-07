import { defineStore } from 'pinia'
import { ref } from 'vue'
import { extensionAPI } from './api'
import { extensionHistoricalPath } from './route-access'
import { extensionForPath, extensionIds, publicExtensionIds, type ExtensionId } from './catalog'

export const EXTENSION_REFRESH_MS = 15_000
export const EXTENSION_STORAGE_EVENT = 'custom-extensions-updated-at'

export const useExtensionStore = defineStore('customExtensions', () => {
  const flags = ref<Partial<Record<ExtensionId, boolean>>>({})
  const loaded = ref(false)
  const error = ref('')
  let lastFetch = 0
  // Which audience the current flags were loaded for; logging in or out
  // invalidates the cache so admin switches appear or disappear at once.
  let loadedAsAdmin = false
  // Installed by the runtime (router guard / App) so this store does not
  // depend on the auth store; without it every session is treated as public.
  let adminSession: () => boolean = () => false
  function setAdminSession(resolver: () => boolean): void { adminSession = resolver }
  let revision = 0
  let inFlight: Promise<void> | null = null

  function enabled(id: ExtensionId): boolean { return flags.value[id] === true }
  function pathEnabled(path: string): boolean {
    const id = extensionForPath(path)
    return !id || extensionHistoricalPath(path) || enabled(id)
  }

  function refresh(force = false): Promise<void> {
    if (inFlight) return inFlight
    const asAdmin = adminSession() === true
    if (!force && loaded.value && asAdmin === loadedAsAdmin && Date.now() - lastFetch < EXTENSION_REFRESH_MS) return Promise.resolve()
    const requestRevision = revision
    inFlight = (async () => {
      try {
        const { data } = await extensionAPI.publicState()
        if (!data?.enabled || publicExtensionIds.some(id => typeof data.enabled[id] !== 'boolean')) {
          throw new Error('Invalid custom extension state')
        }
        const next: Partial<Record<ExtensionId, boolean>> = {}
        for (const id of publicExtensionIds) next[id] = data.enabled[id]
        let adminError = ''
        if (asAdmin) {
          try {
            const { data: admin } = await extensionAPI.list()
            const items = new Map((admin?.items ?? []).map(item => [item.id, item]))
            if (extensionIds.some(id => typeof items.get(id)?.enabled !== 'boolean')) throw new Error('Invalid admin extension state')
            for (const id of extensionIds) next[id] = items.get(id)!.enabled === true
          } catch {
            // Public switches stay usable; admin-only ones fail closed.
            adminError = '无法读取管理端插件状态，请稍后重试。'
          }
        }
        if (requestRevision !== revision) return
        flags.value = next
        loadedAsAdmin = asAdmin
        error.value = adminError
      } catch {
        if (requestRevision !== revision) return
        flags.value = {} // Unknown state never admits new business; explicit historical pages remain readable.
        loadedAsAdmin = asAdmin
        error.value = '无法读取二开插件状态，请稍后重试。'
      } finally {
        if (requestRevision === revision) {
          loaded.value = true
          lastFetch = Date.now()
        }
        inFlight = null
      }
    })()
    return inFlight
  }

  async function setEnabled(id: ExtensionId, value: boolean): Promise<void> {
    const { data } = await extensionAPI.update(id, value)
    if (data.id !== id || data.enabled !== value) throw new Error('Invalid custom extension update')
    // Ignore public requests that started before this successful save.
    revision++
    flags.value = { ...flags.value, [id]: value }
    lastFetch = 0
    try { localStorage.setItem(EXTENSION_STORAGE_EVENT, `${Date.now()}:${id}:${value}`) } catch { /* storage unavailable */ }
  }

  return { flags, loaded, error, enabled, pathEnabled, refresh, setEnabled, setAdminSession }
})

