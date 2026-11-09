import { computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useExtensionStore } from '@/extensions/store'
import { invoiceAPI } from './api'

// Per-user visibility of the invoice entry. The module switch alone is not
// enough: administrators may open the feature to selected users only, so the
// sidebar asks the overview endpoint once per signed-in user.
const allowed = ref(false)
const loadedFor = ref<number | null>(null)
let pending: Promise<boolean> | null = null

export const INVOICE_PATH = '/invoices'

async function load(force = false): Promise<boolean> {
  const auth = useAuthStore()
  const userId = auth.user?.id ?? null
  if (!auth.isAuthenticated || userId === null || !useExtensionStore().enabled('invoices')) {
    allowed.value = false
    loadedFor.value = null
    return false
  }
  if (!force && loadedFor.value === userId) return allowed.value
  if (pending && !force) return pending
  pending = invoiceAPI.overview()
    .then((overview) => {
      allowed.value = overview.allowed === true
      loadedFor.value = userId
      return allowed.value
    })
    .catch(() => {
      allowed.value = false
      loadedFor.value = null
      return false
    })
    .finally(() => { pending = null })
  return pending
}

export function useInvoiceAccess() {
  return {
    canUseInvoices: computed(() => allowed.value),
    refreshInvoiceAccess: load,
  }
}
