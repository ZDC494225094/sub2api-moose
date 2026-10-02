import { computed } from 'vue'
import { useExtensionStore } from './store'

export function useAdminEfficiency() {
  const extensions = useExtensionStore()
  return computed(() => extensions.enabled('admin-efficiency'))
}
