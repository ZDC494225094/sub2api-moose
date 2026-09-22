<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-200 pb-5 dark:border-dark-600">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">充值活动</h1>
        <button class="btn btn-primary" @click="open()"><Icon name="plus" size="sm" />创建活动</button>
      </div>
      <p class="text-sm text-gray-500">启用后，有效期内所有用户自动享受符合充值门槛的活动，无需手选。重叠活动按每元实付可获得的余额择优，同等优惠取最新活动，不与优惠券叠加。活动订单仅按活动邀请规则返利（比例为 0 时无返利），替代该订单的常规邀请返利，到账后可在「邀请好友」转入余额，退款按比例追回。</p>
      <div class="flex items-center gap-3"><input v-model="search" class="input max-w-72" placeholder="搜索活动名称" aria-label="搜索活动" /><button class="btn btn-secondary" :disabled="loading" @click="load">刷新</button></div>
      <p v-if="error" class="text-red-500" role="alert">{{ error }}</p>
      <p v-if="loading" class="py-10 text-center text-gray-500">正在加载活动…</p>
      <div v-else-if="filtered.length" class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="a in filtered" :key="a.id" class="card flex flex-col p-6">
          <div class="flex items-start justify-between gap-3"><h2 class="font-semibold text-gray-900 dark:text-white">{{ a.name }}</h2><span class="rounded-full bg-teal-50 px-3 py-1 text-xs text-teal-700 dark:bg-teal-900/30 dark:text-teal-300">{{ campaignStatus(a) }}</span></div>
          <p class="mt-5 text-3xl font-bold text-primary-600">{{ campaignHeadline(a) }}</p>
          <p class="mt-3 line-clamp-2 text-sm text-gray-500">{{ a.description || '为用户提供更多充值价值' }}</p>
          <dl class="mt-5 space-y-2 text-sm text-gray-500"><div>充值门槛：{{ a.min_amount || '不限' }}</div><div>邀请奖励：{{ a.reward_percent }}% · 每单最高 ${{ a.reward_cap }}</div><div>{{ date(a.starts_at) }} 至 {{ date(a.ends_at) }}</div></dl>
          <div class="mt-auto flex flex-wrap gap-2 pt-6"><button class="btn btn-secondary btn-sm" @click="open(a)">编辑</button><button class="btn btn-secondary btn-sm" :disabled="saving" @click="toggle(a)">{{ a.enabled ? '停用' : '启用' }}</button><button class="btn btn-primary btn-sm" @click="sharing = a">宣传卡片</button></div>
        </article>
      </div>
      <div v-else class="card py-20 text-center text-gray-500">{{ search ? '没有匹配的活动' : '尚未创建充值活动，创建第一份充值福利吧。' }}</div>
    </div>
    <BaseDialog :show="editing" :title="form.id ? '编辑充值活动' : '创建充值活动'" @close="editing = false">
      <form id="campaign-form" class="space-y-4" @submit.prevent="save">
        <label class="block"><span class="input-label">活动名称</span><input v-model="form.name" class="input" required maxlength="60" /></label>
        <label class="block"><span class="input-label">宣传文案</span><textarea v-model="form.description" class="input" maxlength="500" rows="2" /></label>
        <div class="grid grid-cols-2 gap-4"><label><span class="input-label">开始时间（本地时区）</span><input v-model="start" type="datetime-local" class="input" required /></label><label><span class="input-label">结束时间</span><input v-model="end" type="datetime-local" class="input" required /></label></div>
        <p class="text-xs text-gray-500">定点活动可设置精确开始时间及短时结束时间，例如 20:00–20:10。按下单时间参与，以支付成功发放。</p>
        <div class="grid grid-cols-2 gap-4"><label><span class="input-label">优惠类型</span><Select v-model="form.kind" :options="[{value:'bonus',label:'充值赠送'},{value:'discount',label:'充值折扣'}]" /></label><label><span class="input-label">{{ form.kind === 'bonus' ? '赠送百分比（10 = 送10%）' : '实付百分比（90 = 9折）' }}</span><input v-model.number="form.percent" type="number" min="0.01" :max="form.kind === 'bonus' ? 100 : 99.99" step="0.01" class="input" required /></label></div>
        <div class="rounded-xl bg-primary-50 p-3 text-sm text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ form.kind === 'bonus' ? `充 100，赠 ${form.percent}，到账 ${Number((100 + form.percent).toFixed(2))}` : `充 100，实付 ${form.percent}，到账 100` }}（示例按余额换算倍率 1，未含支付手续费）</div>
        <label class="block"><span class="input-label">最低充值金额（0 = 不限）</span><input v-model.number="form.min_amount" type="number" min="0" max="1000000" step="0.01" class="input" required /></label>
        <div class="border-t border-gray-200 pt-4 dark:border-dark-600"><h3 class="font-semibold">邀请好友，一起获益</h3><p class="mt-1 text-xs text-gray-500">单层奖励，按折扣后的充值本金换算余额计算，赠送额和手续费不参与返利。建议 3%–5%，冻结 72 小时。</p></div>
        <div class="grid grid-cols-2 gap-4"><label><span class="input-label">邀请奖励比例（0 = 关闭）</span><input v-model.number="form.reward_percent" type="number" min="0" max="100" step="0.01" class="input" required /></label><label><span class="input-label">每单奖励上限（余额 $）</span><input v-model.number="form.reward_cap" type="number" :min="form.reward_percent ? 0.01 : 0" max="1000000" step="0.01" class="input" required /></label></div>
        <label class="block"><span class="input-label">奖励冻结小时数</span><input v-model.number="form.freeze_hours" type="number" min="0" max="8760" class="input" required /></label>
        <label class="flex items-center gap-2 text-sm"><input v-model="form.new_invitees_only" type="checkbox" />仅奖励活动期间新绑定的受邀用户</label>
        <label class="flex items-center gap-2 text-sm"><input v-model="form.enabled" type="checkbox" />启用活动（开始时间到达后生效）</label>
        <p v-if="formError" class="text-sm text-red-500" role="alert">{{ formError }}</p>
      </form>
      <template #footer><button class="btn btn-secondary" @click="editing = false">取消</button><button form="campaign-form" type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存活动' }}</button></template>
    </BaseDialog>
    <CampaignShareDialog :campaign="sharing" @close="sharing = null" />
  </AppLayout>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import CampaignShareDialog from '@/components/payment/CampaignShareDialog.vue'
import { campaignAPI, campaignHeadline, campaignStatus, type RechargeCampaign } from '@/api/rechargeCampaigns'
import { useAppStore } from '@/stores/app'
const app = useAppStore()
const items = ref<RechargeCampaign[]>([]), sharing = ref<RechargeCampaign | null>(null)
const loading = ref(false), saving = ref(false), editing = ref(false), search = ref(''), error = ref(''), formError = ref('')
const defaults = (): RechargeCampaign => ({ id:0,name:'',description:'',enabled:false,starts_at:'',ends_at:'',kind:'bonus',percent:10,min_amount:0,reward_percent:0,reward_cap:10,freeze_hours:72,new_invitees_only:true })
const form = ref(defaults()), start = ref(''), end = ref('')
const filtered = computed(() => items.value.filter(a => a.name.includes(search.value)))
const date = (value:string) => new Date(value).toLocaleString('zh-CN',{hour12:false})
const local = (value:Date) => new Date(value.getTime()-value.getTimezoneOffset()*60000).toISOString().slice(0,16)
const message = (e:unknown) => (e as {response?:{data?:{message?:string}}})?.response?.data?.message || (e as {message?: string})?.message || '操作失败，请稍后重试'
async function load() { loading.value=true;error.value='';try{items.value=(await campaignAPI.list()).data}catch(e){error.value=message(e)}finally{loading.value=false} }
function open(a?:RechargeCampaign){form.value=a?{...a}:defaults();start.value=local(a?new Date(a.starts_at):new Date());end.value=local(a?new Date(a.ends_at):new Date(Date.now()+7*86400000));formError.value='';editing.value=true}
async function save(){
  if(new Date(end.value)<=new Date(start.value)){formError.value='结束时间必须晚于开始时间';return}
  saving.value=true;formError.value=''
  try{await campaignAPI.save({...form.value,starts_at:new Date(start.value).toISOString(),ends_at:new Date(end.value).toISOString()});editing.value=false;app.showSuccess('活动已保存');await load()}catch(e){formError.value=message(e)}finally{saving.value=false}
}
async function toggle(a:RechargeCampaign){saving.value=true;try{await campaignAPI.save({...a,enabled:!a.enabled});await load()}catch(e){app.showError(message(e))}finally{saving.value=false}}
onMounted(load)
</script>
