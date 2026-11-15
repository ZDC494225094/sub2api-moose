import landing from './landing'
import common from './common'
import custom from './customPublic'
import modelPlaza from './modelPlaza'
import { mergeLocale } from '../merge'

// Public pages do not need the account, operations or admin settings dictionaries.
const base = { ...landing, ...common, modelPlaza }
const overrides = Object.fromEntries(Object.entries(custom).filter(([key]) => key in base))
export default mergeLocale(base, overrides)
