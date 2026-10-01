import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { evaluateOwnership, ownershipCheckFailed } from '../lib/customization-ownership.mjs'

const baseline = 'a'.repeat(40)
const rules = [
  { id: 'shared-infrastructure', patterns: ['^frontend/'] },
  { id: 'plugin-a', patterns: ['widget'] },
  { id: 'plugin-b', patterns: [] },
  { id: 'local-artifacts', patterns: [] },
]
const config = () => ({
  version: 1, baseline, reviewed_at: '2026-09-26', note: 'Source review; not runtime isolation.',
  reviews: [{ paths: ['frontend/widget.vue'], owners: ['plugin-b', 'plugin-a'], kind: 'shared-host', evidence: 'UI uses both modules.', next_step: 'Extract the shared port.' }],
})
const check = (value = config(), paths = ['frontend/widget.vue'], knownPaths = ['frontend/widget.vue']) =>
  evaluateOwnership(rules, value, { paths, knownPaths, baseline })

test('exact review overrides primary ownership without erasing rule candidates or multiple owners', () => {
  const value = config(), before = structuredClone(value), result = check(value)
  assert.equal(result.files[0].owner, 'plugin-b')
  assert.deepEqual(result.files[0].owners, ['plugin-b', 'plugin-a'])
  assert.deepEqual(result.files[0].candidate_owners, ['shared-infrastructure', 'plugin-a'])
  assert.equal(result.files[0].ownership_source, 'reviewed')
  assert.deepEqual(result.files[0].review, { kind: 'shared-host', evidence: value.reviews[0].evidence, next_step: value.reviews[0].next_step })
  assert.equal(result.totals.reviewed_paths, 1)
  assert.equal(result.totals.rule_classified_paths, 0)
  assert.equal(result.totals.multi_owner_paths, 1)
  assert.equal(result.totals.rule_ambiguous_paths, 0)
  assert.equal(ownershipCheckFailed(result), false)
  assert.deepEqual(value, before)
})
test('rule matches are explicitly provisional and retain overlapping candidates', () => {
  const result = check(config(), ['frontend/other-widget.vue'])
  assert.equal(result.files[0].ownership_source, 'rule')
  assert.deepEqual(result.files[0].owners, ['shared-infrastructure', 'plugin-a'])
  assert.equal(result.files[0].review, undefined)
  assert.equal(result.totals.rule_classified_paths, 1)
  assert.equal(result.totals.rule_ambiguous_paths, 1)
})
test('unknown new paths are not swallowed by empty patterns or historical reviews', () => {
  const result = check(config(), ['new-business.go'])
  assert.equal(result.files[0].owner, 'unclassified')
  assert.equal(result.files[0].ownership_source, 'unclassified')
  assert.deepEqual(result.files[0].candidate_owners, [])
  assert.equal(result.totals.unclassified, 1)
  assert.equal(ownershipCheckFailed(result), true)
})
test('historical reviews remain valid for files restored to upstream equivalence', () => {
  const result = check(config(), [])
  assert.deepEqual(result.files, [])
  assert.deepEqual(result.violations, [])
  assert.equal(ownershipCheckFailed(result), false)
})
test('unknown owners and duplicate owner IDs fail the gate', () => {
  for (const owners of [[], ['missing'], ['plugin-a', 'plugin-a']]) {
    const value = config(); value.reviews[0].owners = owners
    assert.match(check(value).violations[0].detail, /known owners/)
    assert.equal(ownershipCheckFailed(check(value)), true)
  }
})
test('rejects absolute, traversal, ambiguous, glob and control-character review paths', () => {
  for (const file of ['/etc/passwd', '../secret', 'a/../b', './a', 'C:/secret', 'a\\b', 'a//b', 'a/', 'a./b', 'a /b', ' a', 'a*', 'a?', '[a]', '{a}', 'a\0b', 'a\nb', '']) {
    const value = config(); value.reviews[0].paths = [file]
    const result = check(value, [], [file])
    assert.match(result.violations[0]?.detail ?? '', /path/i, JSON.stringify(file))
    assert.equal(ownershipCheckFailed(result), true)
  }
})
test('rejects phantom paths and duplicate paths including duplicates inside one group', () => {
  const phantom = config(); phantom.reviews[0].paths = ['frontend/typo.vue']
  assert.match(check(phantom).violations[0].detail, /Unknown ownership/)
  const duplicate = config(); duplicate.reviews.push(structuredClone(duplicate.reviews[0]))
  assert.match(check(duplicate).violations[0].detail, /Duplicate ownership/)
  const sameGroup = config(); sameGroup.reviews[0].paths.push(sameGroup.reviews[0].paths[0])
  assert.match(check(sameGroup).violations[0].detail, /Duplicate ownership/)
})
test('rejects unknown versions, mismatched baselines and invalid dates', () => {
  for (const patch of [{ version: 2 }, { baseline: 'b'.repeat(40) }, { baseline: 'origin/main' }, { reviewed_at: '2026-02-30' }, { reviewed_at: 'today' }, { note: ' ' }, { reviews: {} }]) {
    const result = check({ ...config(), ...patch })
    assert.equal(result.violations.length, 1)
    assert.equal(ownershipCheckFailed(result), true)
  }
  assert.equal(ownershipCheckFailed(check(null)), true)
})
test('requires source evidence and next steps, never trusts part of an invalid review', () => {
  for (const patch of [{ evidence: '' }, { next_step: ' ' }, { kind: 'isolated' }, { paths: [] }]) {
    const value = config(); value.reviews.push({ ...value.reviews[0], ...patch })
    const result = check(value)
    assert.equal(result.violations.length, 1)
    assert.equal(result.totals.reviewed_paths, 0)
    assert.equal(result.files[0].ownership_source, 'rule')
  }
})
test('reports local runtime artifacts separately, without pretending they are a plugin', () => {
  const value = config(); value.reviews[0] = { ...value.reviews[0], paths: ['.runtime/port.txt', '启动命令.txt'], owners: ['local-artifacts'], kind: 'runtime-artifact' }
  const result = check(value, value.reviews[0].paths, value.reviews[0].paths)
  assert.equal(result.totals.runtime_artifacts, 2)
  assert.equal(result.totals.reviewed_paths, 2)
  assert.equal(result.modules['local-artifacts'], 2)
  assert.deepEqual(result.violations, [])
})
test('invalid rule schemas and regexes fail loudly rather than hiding unclassified files', () => {
  for (const invalid of [{}, [{ patterns: [] }], [...rules, rules[0]], [{ id: 'unclassified', patterns: [] }], [{ id: 'plugin', patterns: ['['] }], [{ id: 'plugin', patterns: [null] }]]) {
    assert.throws(() => evaluateOwnership(invalid, config(), { paths: [], knownPaths: [], baseline }))
  }
})
test('actual reviewed ledger contains only explicit entries and known owners', () => {
  const load = file => JSON.parse(readFileSync(new URL('../../customizations/' + file, import.meta.url), 'utf8'))
  const actualRules = load('ownership.json'), reviews = load('ownership-reviewed.json')
  const actualPaths = reviews.reviews.flatMap(entry => entry.paths)
  const result = evaluateOwnership(actualRules, reviews, { paths: actualPaths, knownPaths: actualPaths, baseline: load('upstream.json').merged_upstream })
  assert.deepEqual(result.violations, [])
  assert.equal(result.totals.unclassified, 0)
  assert.equal(result.totals.reviewed_paths, actualPaths.length)
  assert.deepEqual(actualRules.find(group => group.id === 'local-artifacts').patterns, [])
})
test('Docker context explicitly excludes known local artifacts but keeps release source assets', () => {
  // Structural contract only; this is not a Docker build or an ignore parser.
  const lines = readFileSync(new URL('../../.dockerignore', import.meta.url), 'utf8').split(/\r?\n/).map(line => line.trim())
  for (const required of ['/.runtime/', '/.tmp*/', '/dump.rdb', '/generated/', '/frontend/temp-dist/']) assert.ok(lines.includes(required), required)
  for (const requiredSource of ['/frontend/public/', '/frontend/public/logo.png', '/backend/internal/pkg/geo/']) assert.ok(!lines.includes(requiredSource), requiredSource)
})
