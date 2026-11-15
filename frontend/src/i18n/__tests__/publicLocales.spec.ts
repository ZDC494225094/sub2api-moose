import { describe, expect, it } from 'vitest'
import enPublic from '../locales/en/public'
import en from '../locales/en'
import zhPublic from '../locales/zh/public'
import zh from '../locales/zh'
import { i18n, loadLocaleMessages } from '../index'
import enCustom from '../locales/en/custom'
import zhCustom from '../locales/zh/custom'
import enCustomPublic from '../locales/en/customPublic'
import zhCustomPublic from '../locales/zh/customPublic'

describe('public page translations', () => {
  it.each([[enPublic, enCustom, enCustomPublic], [zhPublic, zhCustom, zhCustomPublic]])(
    'keeps every public override in the small shared dictionary', (publicMessages, custom, customPublic) => {
      expect(Object.fromEntries(Object.entries(custom).filter(([key]) => key in publicMessages))).toEqual(customPublic)
    },
  )
  it.each([['en', enPublic, en], ['zh', zhPublic, zh]] as const)(
    '%s preserves the complete dictionary values for every public namespace',
    (_locale, publicMessages, fullMessages) => {
      for (const [key, messages] of Object.entries(publicMessages)) {
        expect(messages).toEqual(fullMessages[key as keyof typeof fullMessages])
      }
      expect(publicMessages).toHaveProperty('modelPlaza.filters.searchPlaceholder')
      expect(publicMessages).not.toHaveProperty('admin')
      expect(publicMessages).not.toHaveProperty('dashboard')
    }
  )

  it('upgrades public translations for console navigation without a later public load losing keys', async () => {
    await Promise.all([loadLocaleMessages('zh', 'public'), loadLocaleMessages('zh', 'public')])
    expect(i18n.global.getLocaleMessage('zh')).not.toHaveProperty('admin')
    await loadLocaleMessages('zh')
    expect(i18n.global.getLocaleMessage('zh')).toEqual(zh)
    await loadLocaleMessages('zh', 'public')
    expect(i18n.global.getLocaleMessage('zh')).toEqual(zh)
  })

  it('keeps the complete dictionary when full and public loads overlap', async () => {
    await Promise.all([loadLocaleMessages('en'), loadLocaleMessages('en', 'public')])
    expect(i18n.global.getLocaleMessage('en')).toEqual(en)
  })
})
