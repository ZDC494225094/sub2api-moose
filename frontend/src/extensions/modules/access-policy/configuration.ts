import { computed } from 'vue'
import { useExtensionStore } from '@/extensions/store'

export const accessPolicyConfigurationKeys = [
  'registration_email_domain_quota_enabled',
  'registration_proof_enabled',
  'registration_proof_difficulty',
  'mainland_china_access_restriction_enabled',
] as const

export function useAccessPolicyConfigurationAdmission() {
  const extensions = useExtensionStore()
  return computed(() => extensions.enabled('access-policy'))
}

// Apply immediately before submission, including a dialog opened before disable.
// Omission preserves persisted rules; sending false would silently clear security.
export function admitAccessPolicySettings<T extends object>(payload: T, enabled: boolean): T {
  const admitted = { ...payload }
  if (!enabled) {
    for (const key of accessPolicyConfigurationKeys) delete (admitted as Record<string, unknown>)[key]
  }
  return admitted
}
