export const MAINLAND_CHINA_ACCESS_RESTRICTED_PATH = '/access-restricted'

interface MainlandChinaAccessRestrictionSettings {
  mainland_china_access_restriction_enabled?: boolean
  mainland_china_access_restricted?: boolean
}

export function resolveMainlandChinaAccessRestrictionRedirect(
  currentPath: string,
  settings: MainlandChinaAccessRestrictionSettings | null | undefined,
  decisionResolved: boolean,
): string | null {
  const restricted =
    settings?.mainland_china_access_restriction_enabled === true &&
    settings.mainland_china_access_restricted === true

  if (restricted && currentPath !== MAINLAND_CHINA_ACCESS_RESTRICTED_PATH) {
    return MAINLAND_CHINA_ACCESS_RESTRICTED_PATH
  }

  if (decisionResolved && !restricted && currentPath === MAINLAND_CHINA_ACCESS_RESTRICTED_PATH) {
    return '/'
  }

  return null
}
