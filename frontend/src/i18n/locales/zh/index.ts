import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import user from './user'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import custom from './custom'
import { mergeLocale } from '../merge'

const base = {
  ...landing,
  ...common,
  ...dashboard,
  ...user,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
}

export default mergeLocale(base, custom)
