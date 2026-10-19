> **三分钟速览**：注册账号 → 在线充值（支付宝）或卡密兑换 → 创建 API 密钥 → 选分组 → 一键导入 CC Switch → 装 Claude Code / Codex → 开始用。下面每一步都有图和要点，概念部分（分组、倍率、订阅）看第五节。

## 一、注册账号

1. 打开 https://moosecloud.cc ，点右上角「注册」
2. 填邮箱、密码（至少 6 位）、确认密码；邀请码选填，有就填
3. 勾选同意服务条款，点「继续」，按提示完成邮箱验证

![注册页面](/guide-images/register.jpg)

也可以直接用 GitHub、Google 或 Linux.do 账号一键登录，跳过注册。注册成功后进入控制台（总览页），以后从首页「登录」进入。

## 二、让账户有额度：充值或卡密兑换

调用模型按量扣费，先给账户备额度，两条路任选：

**路线 A：在线充值（支付宝）**

1. 左侧菜单进「充值/订阅」，停在「充值」页签，选金额——人民币 1:1 到账美元额度（充 ¥100 = $100 额度），大额还有赠送（如充 500 到账 $550）
2. 支付方式只有**支付宝**，扫码支付，成功后余额即时到账

![充值页（在线充值，支付宝）](/guide-images/pay.jpg)

**路线 B：卡密充值**

1. 左侧菜单点「卡密充值」，会打开外部发卡商城（麋鹿云发卡铺），在商城里购买卡密
2. 回到站内，左侧菜单进「兑换」，输入卡密——按卡密类型兑换成**余额**或**订阅**

![卡密充值页（购买卡密）](/guide-images/card.jpg)

![兑换页（输入卡密兑换余额或订阅）](/guide-images/redeem.jpg)

> 订阅（包月套餐）在「充值/订阅」页的「订阅」页签里用支付宝买。余额和订阅的区别看第五节。

## 三、创建 API 密钥

1. 左侧菜单进「API 密钥」，点右上角「创建密钥」
2. 起个名字（如「我的 Codex」），先选厂商页签：Claude 选 Anthropic，GPT 选 OpenAI
3. 再选一个分组（怎么选看下一节），点创建

![创建密钥弹窗（选厂商与分组）](/guide-images/keyscreate.jpg)

4. 生成的 `sk-` 密钥立即复制保存。密钥行的「导入到 CCS」和「使用密钥」后面第六节会用到

![API 密钥页（导入到 CCS 与使用密钥）](/guide-images/keys.jpg)

> **密钥安全**：密钥 = 你账户的钥匙，拿到就能花你的钱。不要发群、不要截图外传、不要传公开仓库。泄露了立刻回本页删除旧的、新建一把。

## 四、选择分组

**一句话**：分组就是供货线路套餐——每个密钥绑一个分组，走哪条线、按什么价，都由分组定。GPT / Codex 方向这样选：

| 分组 | 倍率 | 定位 | 适合谁 |
|---|---|---|---|
| Codex-【Pro号池】 | 0.35 | 日常主力 | 稳定、不降智，大多数人日常用这个 |
| Codex-【plus特价渠道】 | 0.15 | 特价 | 价格敏感可选；稳定性比较差，智商不保证（没有定时测智商） |
| Codex-【企业商用】 | 0.5 | 企业级 | 高并发，追求极致稳定与智商 |

![模型广场页（分组与倍率）](/guide-images/plaza.jpg)

**新手选法**：默认选 Pro 分组（0.35）先跑通；想再省钱换特价分组试试；量大、并发高、稳定性优先就上企业分组。用着不合适，回「API 密钥」页**点分组列直接换**，不用重建密钥。Claude 系列以及生图、视频等分组在「模型广场」都标了定位和倍率，按需选即可。

## 五、关键概念

### 倍率 = 折扣系数

**实际扣费 = 模型官方价 × 倍率**，页面标价和扣费记录都已乘过，不用自己算。

| 官方价（输入 / 百万 token） | 分组倍率 | 你实付 |
|---|---|---|
| $3 | 0.15 | $0.45 |
| $3 | 0.35 | $1.05 |
| $3 | 1 | $3（原价） |

个别用户有平台给的「专属倍率」，有则优先按专属倍率算。

### 余额 vs 订阅

| | 余额 | 订阅（包月套餐） |
|---|---|---|
| 怎么来 | 支付宝充值 / 卡密兑换 | 「充值/订阅」页订阅页签支付宝买 / 卡密兑换 |
| 怎么扣 | 用多少扣多少 | 套餐期内扣套餐额度（按周期重置） |
| 过期 | 不过期 | 到期作废、没用完不退 |
| 适合 | 用量不固定 | 每天都用、用量大 |

现有套餐举例（以套餐页为准）：包月 100 ¥31.5 / 30 天、包月 200 ¥63 / 30 天、包月 500 ¥157.5 / 30 天（Pro 号池、倍率 0.35）。

> **扣费从哪出，看密钥绑的分组**：普通分组扣余额，订阅分组扣套餐额度。可以建两把密钥分开用。

## 六、导入 CC Switch（推荐）

CC Switch 是免费的供应商管理工具：把麋鹿云加进去一次，Claude Code、Claude Desktop、Codex 都能一键切换，不用手改配置文件。

1. 到官网下载安装 CC Switch：https://ccswitch.io/zh/ （Windows / macOS / Linux 都有）
2. 回网站「API 密钥」页，点密钥行的「**导入到 CCS**」
3. 浏览器询问是否打开 CC Switch → 允许 → 在 CC Switch 里确认导入
4. 到对应工具的页面启用：Claude Code 页里麋鹿云卡片显示「路由中」、Codex 页里显示「使用中」，就是已生效

![CC Switch 的 Claude Code 页（麋鹿云路由中）](/guide-images/ccsclaude.jpg)

![CC Switch 的 Codex 页（直连、使用中）](/guide-images/ccscodex.jpg)

> 导入失败或想手动加：在工具页点右上角「添加供应商」→ 自定义，把 API Key 和 API 请求地址（https://moosecloud.cc）按红框填好，保存后启用。

![CC Switch 手动添加供应商（Codex）](/guide-images/ccscodexedit.jpg)

## 七、安装工具：桌面端与 CLI

Claude 和 Codex 都有两种形态：桌面端（图形界面）和 CLI（终端命令行）。不管装哪种，连接配置都建议走第六节的 CC Switch——导入一次，各端共用。

### Claude 桌面端（Claude Desktop）

1. 在 Claude 官网下载安装 Claude Desktop
2. 打开 CC Switch，左侧选「Claude Desktop」，顶部切到「**路由**」，选中麋鹿云（显示「路由中」）
3. 打开 Claude Desktop，即可使用站内的 Claude 模型

![CC Switch 的 Claude Desktop 页（切到路由）](/guide-images/ccsdesktop.jpg)

模型对应关系在供应商编辑页维护：请求地址填 https://moosecloud.cc，上游格式选 Anthropic Messages（原生），模型映射把 Sonnet / Opus / Fable / Haiku 分别填站内实际模型名（下图红框）。

![Claude Desktop 供应商编辑页（上游格式与模型映射）](/guide-images/ccsdesktopedit.jpg)

### Claude Code（CLI）

```bash
# macOS / Linux
curl -fsSL https://claude.ai/install.sh | bash
# Windows PowerShell
irm https://claude.ai/install.ps1 | iex
# 或 npm（需要 Node 22+）：npm install -g @anthropic-ai/claude-code
```

装完在项目目录运行 `claude`。走 CC Switch 导入启用的直接就能用；手动配置则用网站「使用密钥」里 Claude Code 页签的内容。

> **易错点**：Claude Code 的 ANTHROPIC_BASE_URL **不要带 /v1**（它自己会拼，写重了报 404）。

### Codex 桌面端

1. 从 Codex 官网（chatgpt.com/codex）下载安装桌面版
2. 打开 CC Switch，左侧选「Codex」，选中麋鹿云（直连，显示「使用中」）
3. **完全退出 Codex 桌面版再重新打开**才生效——只关窗口不行（macOS 按 CMD+Q 彻底退出）；编辑器插件同理，要重开编辑器窗口

### Codex（CLI）

```bash
# macOS / Linux
curl -fsSL https://chatgpt.com/codex/install.sh | sh
# Windows PowerShell
powershell -ExecutionPolicy ByPass -c "irm https://chatgpt.com/codex/install.ps1 | iex"
# 或 npm：npm install -g @openai/codex ；macOS 也可 brew install --cask codex
```

装完运行 `codex`。走 CC Switch 的直接能用；手动配置把「使用密钥」里 Codex 页签的内容存到 `~/.codex/config.toml`（Windows：`%userprofile%\.codex\config.toml`）。

> **易错点**：Codex 手动配置的 base_url **要带 /v1**，和 Claude Code 正好相反。之前用 ChatGPT 账号登录过 Codex 的，先 `codex logout` 清掉官方登录。

## 八、开始使用

1. 终端进入项目文件夹：`cd 你的项目路径`
2. 运行 `claude` 或 `codex`，直接中文提需求
3. 花了多少：控制台「总览」看余额，「使用记录」看每次调用的模型、token 和扣费明细

## 常见问题

**Q：报 401 / 认证失败？**
查四点：密钥复制完整（sk- 开头、无空格）→ 密钥已分配分组 → CC Switch 里选中的是这个供应商且已启用 → Codex 手动改配置后要完全退出重开。

**Q：报余额不足 / 402？**
普通分组：去「充值/订阅」充值或用卡密兑换。订阅分组：看套餐是否过期或本周期额度用完。

**Q：报模型不存在 / 404？**
当前分组没有这个模型。去「模型广场」确认哪个分组有，给密钥换分组。另注意 Claude Code 的 BASE_URL 别误带 /v1。

**Q：偶尔报 400 Failed to read request body？**
请求上传途中被截断，多是本地网络或代理软件问题。重试、关掉系统代理 / VPN、别在弱网发超长上下文。

**Q：回答到一半断了 / 特别慢？**
多半是当前分组线路拥挤。看「渠道状态」页，再给密钥换个分组。

**Q：充值付了没到账？**
去「订单」看状态，渠道偶有延迟，等一两分钟刷新。订单成功但余额没变，截图订单号点网站悬浮「联系客服」或加 QQ 1076623975。

**Q：点「导入到 CCS」没反应？**
确认 CC Switch 已装且版本较新、浏览器没拦打开外部应用的弹窗；不行就用「使用密钥」手动配置，效果一样。

**Q：密钥泄露了？**
立刻回「API 密钥」页禁用或删除那把，再新建——旧的删掉即失效。

**Q：订阅和余额都有，先扣哪个？**
看分组：订阅分组扣套餐额度，普通分组扣余额，互不影响。

**Q：每个模型多少钱在哪看？**
「模型广场」页按分组、倍率列了每个模型的输入 / 输出实付价。

> 网站自带「文档中心」有偏命令行的接入速查，可和本手册对照看。还有问题点悬浮「联系客服」或加 QQ 1076623975。
