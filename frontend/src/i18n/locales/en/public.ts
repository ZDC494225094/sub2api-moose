import landing from './landing'
import common from './common'
import custom from './custom'
import modelPlaza from './modelPlaza'
import { mergeLocale } from '../merge'

const base = { ...landing, ...common, modelPlaza }
const overrides = Object.fromEntries(Object.entries(custom).filter(([key]) => key in base))
export default mergeLocale(base, overrides)
