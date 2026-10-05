// Generates documentation only; no Git/index/database operations.
import { readFileSync, writeFileSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { renderRuntimeEvidence } from './lib/customization-contracts.mjs'
const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const config = JSON.parse(readFileSync(resolve(root, 'customizations/runtime-evidence.json'), 'utf8'))
writeFileSync(resolve(root, 'customizations/RUNTIME_EVIDENCE.md'), renderRuntimeEvidence(config, file => readFileSync(resolve(root, file), 'utf8')))
