import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(currentDir, '../GroupsView.vue'), 'utf8')

describe('admin group billing rate sync controls', () => {
  it('wires reference account and additive markup into create and edit forms', () => {
    expect(source.match(/<GroupBillingRateSyncFields/g)).toHaveLength(2)
    expect(source).toContain('v-model:account-id="createForm.billing_rate_sync_account_id"')
    expect(source).toContain('v-model:account-id="editForm.billing_rate_sync_account_id"')
    expect(source).toContain('billing_rate_sync_account_id:')
    expect(source).toContain('billing_rate_markup: createForm.platform === "openai" ? billingRateMarkup : 0')
    expect(source).toContain('billing_rate_markup: editForm.platform === "openai" ? billingRateMarkup : 0')
  })
})
