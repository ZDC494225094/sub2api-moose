import { reactive } from 'vue'
import { useExtensionStore } from '../../store'
import { generate as nativeGenerate } from '@/api/admin/redeem'
import { redeemMarketingAPI } from './api'

// Form state belongs to this page instance, never a process-wide singleton.
export function useMarketingGeneration() {
  const extensions = useExtensionStore()
  const options = reactive({ newUsersOnly: false })
  const generate: typeof nativeGenerate = async (count, type, value, group, days, expiry) => {
    // Do not silently downgrade a selected restriction when a switch changes.
    if (type === 'marketing' && options.newUsersOnly) {
      if (!extensions.enabled('redeem-marketing')) throw new Error('营销兑换码转化插件已关闭，请刷新页面后重新确认生成选项。')
      const result = await redeemMarketingAPI.generate(count, value, expiry, true)
      options.newUsersOnly = false
      return result
    }
    return nativeGenerate(count, type, value, group, days, expiry)
  }
  return { options, generate }
}
