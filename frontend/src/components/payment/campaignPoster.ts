import type { RechargeCampaign } from '@/api/rechargeCampaigns'
import { campaignHeadline } from '@/api/rechargeCampaigns'

export async function loadPosterImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    const timeout = window.setTimeout(() => { img.src = ''; reject(new Error('Image loading timed out')) }, 8000)
    img.crossOrigin = 'anonymous'
    img.onload = () => { clearTimeout(timeout); resolve(img) }
    img.onerror = () => { clearTimeout(timeout); reject(new Error('Image unavailable')) }
    img.src = url
  })
}

export function drawCampaignPoster(canvas: HTMLCanvasElement, options: {
  campaign: RechargeCampaign
  siteName: string
  logo: HTMLImageElement
  globe: HTMLCanvasElement
  providers: { name: string; image: HTMLImageElement }[]
  qr: HTMLCanvasElement
  host: string
  sharer?: { name: string; avatar?: HTMLImageElement }
}) {
  canvas.width = 900
  canvas.height = options.sharer ? 1720 : 1600
  const ctx = canvas.getContext('2d')!
  const { campaign: a } = options
  const ink = '#172625', muted = '#586b69', teal = '#087f74'
  ctx.fillStyle = '#f5faf8'; ctx.fillRect(0, 0, 900, canvas.height)
  ctx.fillStyle = '#087f74'; ctx.fillRect(0, 0, 900, 10)
  const font = (size: number, weight = 400) => { ctx.font = `${weight} ${size}px "Arial", "PingFang SC", "Microsoft YaHei", sans-serif` }
  const text = (value: string, x: number, y: number, size: number, color = ink, width = 772, weight = 400) => {
    font(size, weight)
    while (ctx.measureText(value).width > width && size > 14) font(--size, weight)
    ctx.fillStyle = color; ctx.fillText(value, x, y)
  }
  const lines = (value: string, x: number, y: number, width: number, size: number, max: number, color = muted) => {
    font(size, 400); ctx.fillStyle = color
    const chars = Array.from(value.replace(/\s+/g, ' '))
    let line = '', row = 0
    for (let i = 0; i < chars.length; i++) {
      if (ctx.measureText(line + chars[i]).width > width) {
        if (row === max - 1) { ctx.fillText(line.slice(0, -1) + '…', x, y + row * (size + 12)); return }
        ctx.fillText(line, x, y + row++ * (size + 12)); line = ''
      }
      line += chars[i]
    }
    ctx.fillText(line, x, y + row * (size + 12))
  }
  const logoScale = Math.min(80 / options.logo.naturalWidth, 80 / options.logo.naturalHeight)
  ctx.drawImage(options.logo, 64 + (80 - options.logo.naturalWidth * logoScale) / 2, 52 + (80 - options.logo.naturalHeight * logoScale) / 2, options.logo.naturalWidth * logoScale, options.logo.naturalHeight * logoScale)
  text(options.siteName, 164, 89, 36, ink, 670, 600)
  text('一站式 AI API 聚合平台', 164, 126, 22, muted)
  text('让顶尖 AI，成为你的创造力。', 64, 214, 44, ink, 772, 600)
  text('ONE API. MORE POSSIBILITIES.', 64, 261, 20, teal)
  ctx.drawImage(options.globe, 195, 274, 510, 510)
  // Provider marks use the same local SVG source as the homepage.
  options.providers.forEach((provider, i) => {
    const x = 64 + i * 130
    ctx.drawImage(provider.image, x + 34, 745, 44, 44)
    ctx.textAlign = 'center'; text(provider.name, x + 56, 819, 19, muted, 120); ctx.textAlign = 'left'
  })
  ctx.fillStyle = '#ffffff'; ctx.fillRect(0, 856, 900, 744)
  text('限时充值礼遇', 64, 906, 23, teal)
  lines(a.name, 64, 958, 772, 34, 2, ink)
  text(campaignHeadline(a), 64, 1084, 66, a.kind === 'discount' ? '#c44058' : teal, 772, 600)
  lines(a.description || '为灵感补充能量，让每一次创造更从容。', 64, 1133, 772, 24, 2)
  ctx.fillStyle = '#dce9e5'; ctx.fillRect(64, 1200, 772, 1)
  text(a.reward_percent > 0 ? `邀请好友充值，享 ${a.reward_percent}% 奖励` : '活动期间充值，自动享受优惠', 64, 1247, 29, ink)
  text(a.reward_percent > 0 ? `每单最高 $${a.reward_cap} · 冻结 ${a.freeze_hours} 小时${a.new_invitees_only ? ' · 仅活动期新邀请' : ''}` : '无需领券 · 满足门槛即可参与', 64, 1284, 20, muted)
  ctx.drawImage(options.qr, 58, 1320, 216, 216)
  text('扫码充值，开启下一次创造', 310, 1375, 31, ink, 530)
  text(`充值门槛：${a.min_amount > 0 ? a.min_amount : '不限'}`, 310, 1422, 23, teal, 530)
  text(options.host, 310, 1462, 21, muted, 530)
  const date = (v: string) => new Date(v).toLocaleString('zh-CN', { hour12: false, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  text(`${date(a.starts_at)} 至 ${date(a.ends_at)}`, 310, 1504, 18, muted, 530)
  text('以充值页面结算为准 · 不与优惠券叠加', 64, 1560, 19, muted)
  if (options.sharer) {
    ctx.fillStyle = '#eef6f2'; ctx.fillRect(0, 1600, 900, 120)
    ctx.save(); ctx.beginPath(); ctx.arc(98, 1660, 32, 0, Math.PI * 2); ctx.clip()
    ctx.fillStyle = '#cce8dd'; ctx.fillRect(66, 1628, 64, 64)
    if (options.sharer.avatar) {
      const avatar = options.sharer.avatar
      const side = Math.min(avatar.naturalWidth, avatar.naturalHeight)
      const scale = 64 / side
      ctx.drawImage(avatar, 66 - (avatar.naturalWidth * scale - 64) / 2, 1628 - (avatar.naturalHeight * scale - 64) / 2, avatar.naturalWidth * scale, avatar.naturalHeight * scale)
    } else { text(Array.from(options.sharer.name)[0] || '友', 82, 1670, 28, teal) }
    ctx.restore()
    text(options.sharer.name, 152, 1656, 25, ink, 660, 600)
    text('把这份创作福利，分享给你', 152, 1690, 20, muted)
  }
}
