import type Icon from './Icon.vue'

export type IconName = InstanceType<typeof Icon>['$props']['name']

export const customMenuIconNames = [
  'link',
  'globe',
  'document',
  'book',
  'chat',
  'grid',
  'chart',
  'chartBar',
  'database',
  'server',
  'cloud',
  'terminal',
  'cog',
  'key',
  'shield',
  'users',
  'user',
  'creditCard',
  'gift',
  'bell',
  'calendar',
  'calculator',
  'sparkles',
  'bolt',
] as const satisfies readonly IconName[]

export type CustomMenuIconName = (typeof customMenuIconNames)[number]

const customMenuIconNameSet = new Set<string>(customMenuIconNames)

export function isCustomMenuIconName(value: unknown): value is CustomMenuIconName {
  return typeof value === 'string' && customMenuIconNameSet.has(value)
}

export function normalizeCustomMenuIconName(value: unknown): CustomMenuIconName | '' {
  return isCustomMenuIconName(value) ? value : ''
}
