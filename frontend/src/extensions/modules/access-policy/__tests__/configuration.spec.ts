import { describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useExtensionStore } from '@/extensions/store'
import { accessPolicyConfigurationKeys, admitAccessPolicySettings, useAccessPolicyConfigurationAdmission } from '../configuration'

describe('security configuration admission', () => {
  it('keeps native settings and omits custom fields rather than clearing enforced rules', () => {
    const payload = { site_name: 'native', registration_email_suffix_whitelist: ['@example.com'], registration_proof_enabled: true, registration_proof_difficulty: 20, registration_email_domain_quota_enabled: true, mainland_china_access_restriction_enabled: true }
    expect(admitAccessPolicySettings(payload, true)).toEqual(payload)
    const admitted = admitAccessPolicySettings(payload, false)
    for (const key of accessPolicyConfigurationKeys) expect(admitted).not.toHaveProperty(key)
    expect(admitted.site_name).toBe('native')
    expect(admitted.registration_email_suffix_whitelist).toEqual(['@example.com'])
    expect(payload.registration_proof_enabled).toBe(true)
  })
  it('reacts to unknown/off/on and revokes an already-open form at submission', () => {
    setActivePinia(createPinia())
    const store = useExtensionStore()
    const enabled = useAccessPolicyConfigurationAdmission()
    expect(enabled.value).toBe(false)
    store.flags = { 'access-policy': true }
    expect(enabled.value).toBe(true)
    const stalePayload = { registration_proof_enabled: false }
    store.flags = { 'access-policy': false }
    expect(admitAccessPolicySettings(stalePayload, enabled.value)).toEqual({})
    store.flags = {}
    expect(enabled.value).toBe(false)
  })
})
