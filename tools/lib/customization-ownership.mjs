// Ownership is maintenance responsibility, not proof of runtime isolation.
// Exact source-reviewed entries take precedence, but never erase rule candidates.
const kinds = new Set(['build', 'runtime-artifact', 'shared-host', 'compatibility', 'asset', 'docs', 'business'])
const nonempty = value => typeof value === 'string' && value.trim().length > 0
const safePath = value => nonempty(value) && value === value.trim() &&
  !/[\\:*?"<>|\u0000-\u001f\u007f\[\]{}]/.test(value) &&
  value.split('/').every(part => part && part !== '.' && part !== '..' && !/[. ]$/.test(part))

function compileRules(rules) {
  if (!Array.isArray(rules)) throw new Error('Ownership rules must be an array')
  const ids = new Set()
  return rules.map(group => {
    if (!group || typeof group.id !== 'string' || group.id === 'unclassified' || !/^[a-z][a-z0-9-]*$/.test(group.id) || ids.has(group.id) || !Array.isArray(group.patterns)) {
      throw new Error('Invalid or duplicate ownership rule')
    }
    ids.add(group.id)
    return { id: group.id, patterns: group.patterns.map(pattern => {
      if (!nonempty(pattern)) throw new Error('Ownership rule patterns must be nonempty strings')
      return new RegExp(pattern)
    }) }
  })
}

function reviewIndex(config, { ownerIds, knownPaths, baseline }) {
  if (config?.version !== 1) throw new Error('Unsupported ownership review version')
  if (!/^[a-f0-9]{40}$/.test(config.baseline ?? '') || config.baseline !== baseline) {
    throw new Error('Ownership review baseline must match the fixed merged upstream SHA')
  }
  if (!/^\d{4}-\d{2}-\d{2}$/.test(config.reviewed_at ?? '') ||
      !Number.isFinite(Date.parse(config.reviewed_at)) ||
      new Date(config.reviewed_at).toISOString().slice(0, 10) !== config.reviewed_at) {
    throw new Error('Ownership review requires a valid reviewed_at date')
  }
  if (!nonempty(config.note) || !Array.isArray(config.reviews)) throw new Error('Invalid ownership review document')
  const index = new Map()
  for (const entry of config.reviews) {
    if (!entry || !Array.isArray(entry.paths) || !entry.paths.length ||
        !Array.isArray(entry.owners) || !entry.owners.length ||
        new Set(entry.owners).size !== entry.owners.length ||
        entry.owners.some(owner => !ownerIds.has(owner))) {
      throw new Error('Review entries require exact paths and unique known owners')
    }
    if (!kinds.has(entry.kind) || !nonempty(entry.evidence) || !nonempty(entry.next_step)) {
      throw new Error('Review entries require a valid kind, evidence and next_step')
    }
    for (const file of entry.paths) {
      if (!safePath(file)) throw new Error('Unsafe or non-exact ownership review path: ' + file)
      if (index.has(file)) throw new Error('Duplicate ownership review path: ' + file)
      if (!knownPaths.has(file)) throw new Error('Unknown ownership review path: ' + file)
      index.set(file, entry)
    }
  }
  return index
}

export function evaluateOwnership(rules, config, { paths, knownPaths, baseline }) {
  const compiled = compileRules(rules)
  const violations = []
  let reviewed = new Map()
  try {
    reviewed = reviewIndex(config, { ownerIds: new Set(compiled.map(group => group.id)), knownPaths: new Set(knownPaths), baseline })
  } catch (error) {
    // Never partially accept a malformed review document as trusted ownership.
    violations.push({ file: 'customizations/ownership-reviewed.json', detail: error.message })
  }
  const modules = Object.create(null)
  const files = paths.map(file => {
    const candidate_owners = compiled.filter(group => group.patterns.some(pattern => pattern.test(file))).map(group => group.id)
    const review = reviewed.get(file)
    const owners = review ? [...review.owners] : candidate_owners
    const owner = owners[0] ?? 'unclassified'
    modules[owner] = (modules[owner] ?? 0) + 1
    return {
      path: file, owner, owners, candidate_owners,
      ownership_source: review ? 'reviewed' : owners.length ? 'rule' : 'unclassified',
      ...(review ? { review: { kind: review.kind, evidence: review.evidence, next_step: review.next_step } } : {}),
    }
  })
  const totals = {
    reviewed_paths: files.filter(file => file.ownership_source === 'reviewed').length,
    rule_classified_paths: files.filter(file => file.ownership_source === 'rule').length,
    unclassified: files.filter(file => file.ownership_source === 'unclassified').length,
    multi_owner_paths: files.filter(file => file.owners.length > 1).length,
    rule_ambiguous_paths: files.filter(file => file.ownership_source === 'rule' && file.owners.length > 1).length,
    runtime_artifacts: files.filter(file => file.review?.kind === 'runtime-artifact').length,
    ownership_violations: violations.length,
  }
  return { files, modules, totals, violations }
}

export function ownershipCheckFailed(result) {
  return result.violations.length > 0 || result.totals.unclassified > 0
}
