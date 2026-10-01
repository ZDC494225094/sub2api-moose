import { computed } from 'vue'
import { useExtensionStore } from '../../store'

// New business requires an explicit shared enablement. Historical reads and
// settlement do not use this gate; missing/failed state never permits new work.
export function useMarketingAdmission() {
  const extensions = useExtensionStore()
  return computed(() => extensions.enabled('marketing-tools'))
}
