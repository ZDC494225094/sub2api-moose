import { describe, expect, it } from 'vitest'
import {
  MAINLAND_CHINA_ACCESS_RESTRICTED_PATH,
  resolveMainlandChinaAccessRestrictionRedirect,
} from '../mainlandChinaAccessRestriction'

describe('resolveMainlandChinaAccessRestrictionRedirect', () => {
  it('routes a mainland-China decision to the access-restricted page', () => {
    expect(resolveMainlandChinaAccessRestrictionRedirect('/dashboard', {
      mainland_china_access_restriction_enabled: true,
      mainland_china_access_restricted: true,
    }, true)).toBe(MAINLAND_CHINA_ACCESS_RESTRICTED_PATH)
  })

  it('returns to the home page when a refreshed decision is no longer restricted', () => {
    expect(resolveMainlandChinaAccessRestrictionRedirect(MAINLAND_CHINA_ACCESS_RESTRICTED_PATH, {
      mainland_china_access_restriction_enabled: true,
      mainland_china_access_restricted: false,
    }, true)).toBe('/')
  })

  it('waits for the forced IP decision before leaving the access-restricted page', () => {
    expect(resolveMainlandChinaAccessRestrictionRedirect(MAINLAND_CHINA_ACCESS_RESTRICTED_PATH, {
      mainland_china_access_restriction_enabled: true,
      mainland_china_access_restricted: false,
    }, false)).toBeNull()
  })
})
