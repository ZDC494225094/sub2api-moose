import { useExtensionStore } from '@/extensions/store'
import { computed } from 'vue'

/**
 * Site-customization extension admission hook.
 * Controls visibility of custom UI elements (menus, footer links, announcements).
 *
 * @returns true if custom UI elements should be shown
 */
export function useSiteCustomizationAdmission() {
  const extensionStore = useExtensionStore()
  return computed(() => extensionStore.enabled('site-customization'))
}
