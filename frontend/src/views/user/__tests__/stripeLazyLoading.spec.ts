// @vitest-environment node
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { loadConfigFromFile } from 'vite'

const frontendRoot = resolve(__dirname, '../../../..')
const stripeConsumers = [
  'src/views/user/StripePaymentView.vue',
  'src/views/user/StripePopupView.vue',
  'src/components/payment/StripePaymentInline.vue',
]

function readFrontendFile(path: string): string {
  return readFileSync(resolve(frontendRoot, path), 'utf8')
}

describe('Stripe lazy-loading contract', () => {
  it.each(stripeConsumers)('%s uses the side-effect-free Stripe loader', (path) => {
    const source = readFrontendFile(path)

    expect(source).toContain("await import('@stripe/stripe-js/pure')")
    expect(source).not.toMatch(/await import\(['"]@stripe\/stripe-js['"]\)/)
  })

  it('keeps Stripe out of the shared vendor chunk', async () => {
    const loaded = await loadConfigFromFile(
      { command: 'build', mode: 'production' },
      resolve(frontendRoot, 'vite.config.ts'),
    )
    const output = loaded?.config.build?.rollupOptions?.output
    const manualChunks = (Array.isArray(output) ? output[0] : output)?.manualChunks
    if (typeof manualChunks !== 'function') throw new Error('Expected manual chunk configuration')
    const moduleInfo = { getModuleIds: () => [][Symbol.iterator](), getModuleInfo: () => null }

    expect(manualChunks('/node_modules/@stripe/stripe-js/dist/pure.mjs', moduleInfo)).toBe('vendor-stripe')
    expect(manualChunks('/node_modules/.pnpm/@stripe+stripe-js@9.0.1/node_modules/@stripe/stripe-js/dist/pure.mjs', moduleInfo)).toBe('vendor-stripe')
    expect(manualChunks('/node_modules/axios/index.js', moduleInfo)).not.toBe('vendor-stripe')
    expect(manualChunks('/node_modules/vue/dist/vue.runtime.esm-bundler.js', moduleInfo)).toBe('vendor-vue')
  })
})
