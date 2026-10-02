# 二开扩展管理、隔离改造与上游升级

## 1. 结论与当前边界

目标架构是 **上游宿主 + 内置业务扩展 + 稳定接入端口 + 升级门禁**：未启用的模块使用上游行为，启用时注入独立二开；关闭不删除配置、业务记录、历史权益或在途订单。

当前清单的 **14 个模块均已接入真实运行时开关，待解耦条目为 0**。固定清单保留改造前 814 个差异路径，其中包含资源、测试、生成代码和构建配置，不等于 814 项功能。Stage31–33 补齐访问策略、媒体准入及订阅发放归属；宿主仍保留共享事务、协议传输、历史兼容和模型适配。这是清单模块的开关边界完成，不等于所有代码均已物理搬迁、生产验收完成，**不能承诺未来任意上游版本直接拉取永远零冲突**。

- 管理入口：管理员 → **二开插件管理**（`/admin/custom-extensions`）。
- 当前清单十四个已管理模块、零个待解耦条目；数量来自后端 catalog，不能仅改页面统计。
- 上游 OAuth 传输插件 `/admin/plugins` 保持原样，不与业务扩展混用。
- 本方案是编译期组合、运行时开关，不是上传 ZIP、任意热加载或卸载代码的机制。
- 已合并基线：上游 v0.2.7，`aea725f2ea644d5592d0bbb1d63b607efa7e200a`。
- 改造前本地 HEAD：`d754fe1dc`。没有代为 fetch、merge、commit、push 或部署；本地缓存 ref 不代表最新上游。

详细架构、模块矩阵、实施顺序和验收标准见 [实施方案](customizations/IMPLEMENTATION_PLAN.md)。

## 2. 当前清单已管理的十四个模块

| 模块 | 已落地的独立边界 | 关闭行为与保留项 |
| --- | --- | --- |
| 品牌首页/文档 `premium-home` | 独立路由与原有功能目录；`HomeView.vue` 已恢复上游基线 | `/`、`/docs` 回到原生 `/home`，保留查询参数；原站点配置不删除 |
| 体验中心 `playground` | 独立处理器通过扩展容器注册；前后端入口门禁 | 禁止新任务；旧任务查询、下载、取消继续；底层媒体服务仍共享 |
| 无限画布 `infinite-canvas` | 宿主页面、配置、新建接口门禁 | 停止宿主新任务；旧任务生命周期与浏览器本地画布数据保留；独立 CDN/Vite 壳不受 Go 静态页面门禁控制 |
| 运营分析 `operations-analytics` | 独立 Handler / Service / 只读 Repository；前端页面/API/组件/测试迁入模块目录 | 关闭运营分析及营销邮件入口；原生仪表盘不变，旧邮件记录不删除 |
| 充值活动 `recharge-campaigns` | 前端 checkout 扩展端口与 UI 插槽；后端目录、报价、快照、返佣/退款规则独立，宿主保留支付事务协调 | 关闭后新订单回到标准充值；携带旧活动参数拒绝并要求重新确认；旧订单按快照履约/返佣/退款 |
| 客服 `customer-support` | 独立组件与应用/布局插槽，品牌首页按钮也接入同一开关 | 移除客服浮窗和二开联系按钮；保留联系方式配置，不影响上游帮助入口 |

| 优惠券/抽奖 `marketing-tools` | 独立规则、HTTP/API、页面与 checkout 插槽；真实设置准入及独立接纳迁移 | 禁止新增发券、抽奖、券预留和管理写入；保留历史查询、券核销/释放、履约重试及宿主支持的退款 |
| 多分组密钥 `multi-group-billing` | 选组/绑定/分组生命周期规则独立；创建时持久化路由归属；KeysView 路由模式端口 | 新建密钥回到上游单分组、余额优先；既有多分组密钥按原归属继续路由计费，可保持或缩减，不能新增分组 |

| 计费与调度增强 `billing-scheduling` | 当前清单已接后端倍率、分时定价、自定义用量和组合调度门禁；本轮补齐前端 ID 登记 | 关闭行为见管理页；历史用量和账单应保留。本轮未重新验收全部计费关闭路径 |
| 站点定制 `site-customization` | 自定义 UI 配置过滤与新公告准入；本轮补齐文件所有权 | 隐藏自定义菜单/页脚等配置，停止创建公告，已发布公告和站点名称保留 |

| 管理效率 `admin-efficiency` | 独立账号排序、上游分组目录、分组成员替换及用户批量操作规则/API/UI；宿主注入设置与仓储端口 | 关闭时拒绝所有这些二开写入，保留历史标签、显示顺序和成员关系；原生账号/用户 CRUD、原生分组排序不受影响 |

| 访问与注册策略 `access-policy` | 独立规则/凭证协议、严格配置读取、配置写入准入和页面访问边界 | 禁止修改二开安全配置；已有安全规则和旧凭证继续执行，读取故障不放行；原生白名单仍可管理 |

| 视频与媒体协议 `media-gateway` | 独立端点注册与准入；Grok 图片/视频/语音、Seedance、Veo 及 Grok Responses HTTP/WS 服务端生图工具接入 | 拒绝新增任务，不再申请槽位/选账号；旧任务查询、下载、取消和结算保留，已准入任务继续；原生文本和 OpenAI 图片不受影响 |
| 订阅与兑换 `subscription-extensions` | 独立发放策略和 Ent schema 贡献；订单/兑换码冻结策略，原生续期按用户锁串行化 | 新发放采用原生归属的单实例分配/续期；旧独立订阅及已创建订单/兑换码按冻结策略履约；不重算历史用量，不恢复全局唯一性约束 |

“已管理”表示已有真实准入和关闭契约，**不表示该模块所有底层依赖都已从共享业务中迁出**。

### 充值与历史交易的关键规则

- `PaymentView.vue` 只接统一 `useCheckoutExtensions` 和 checkout 插槽，不再直接依赖活动 API、优惠算法、倒计时和分享逻辑。
- 没有活动贡献时保留原生报价。多个扩展同时替换价格时拒绝提交，不按注册先后暗中选一个。
- 活动首次加载未完成或报价加载失败时禁止不确定下单；关闭活动后恢复原生报价。
- off→on、卸载、分享请求等异步结果有失效保护，迟到响应不会恢复已关闭的组件或旧活动。
- 活动计算以提交时请求金额为准；后端仍是权威报价和准入检查。
- 订阅购买与签名的 `wechat_resume_token` 恢复请求不加入实时活动报价，避免重算已受理交易。
- 后端充值活动模块不导入共享 `PaymentService` 或生成的 Ent 类型。目录、活动择优/计价、订单快照、邀请资格查询及奖励冲回 SQL 已迁出；宿主只适配原生金额、准入、订单/退款 DTO 和事务端口。优惠券/抽奖仍未接入通用生命周期，不能据此宣布全部交易二开已隔离。
- 返点写入和退款冲回使用宿主已有事务/幂等声明，不另开事务；关闭活动不阻止历史快照继续处理。退款状态、余额/冻结额和流水任一步失败应整体回滚。
- 保留旧快照 JSON 的字段顺序、金额精度、返佣上限与冻结/解冻语义；折扣活动沿用宿主原生到账额，赠送活动仍在倍率和活动加成后只舍入一次。
- 拆分保持活动表名、配置 JSON、revision 算法、API 地址、响应封装不变。新增回归检查旧 revision 摘要、URL 主键权威性、公开活动过滤、SQL 错误传播。

## 3. 代码与配置入口

```text
backend/internal/customize/
  catalog.json                 # 模块清单、真实关闭语义、迁移状态
  registry.go / http.go         # 持久化状态、管理 API、路由门禁
  wiring/providers.go          # 扩展组合根
  modules/rechargecampaigns/   # 目录/报价/快照/奖励规则/HTTP/测试
backend/internal/handler/custom_extensions.go  # 单一 ExtensionHandlers 容器
backend/internal/server/routes/custom_extensions.go
frontend/src/extensions/
  routes.ts / runtime.ts / store.ts
  checkout.ts                 # 类型化报价/提交贡献
  slots.ts / components/      # 通用 UI 插槽、局部错误隔离
  modules/                    # 已迁移的前端模块
customizations/               # 实施方案、固定清单、归属、契约
```

原生路由在未构造扩展容器时仍可注册；核心页面导航不会被扩展状态请求阻塞。开关状态在可见页约每 15 秒刷新并做跨标签通知；服务端每次新业务准入读取持久化状态。

### API

| 方法 | 地址 | 权限 |
| --- | --- | --- |
| GET | `/api/v1/custom-extensions` | 公开、沿用公开 IP 限流，只返回可管理布尔状态 |
| GET | `/api/v1/admin/custom-extensions` | 原管理员鉴权、限流、审计及合规链 |
| PUT | `/api/v1/admin/custom-extensions/:id` | 同上，请求体 `{"enabled":false}` |

- 待隔离模块 `enabled:null`；未知 ID 404、待隔离 ID 409、非法请求体 400。
- 关闭的新业务入口 403，错误码 `CUSTOM_EXTENSION_DISABLED`；活动准入也在领域层检查，不能绕开页面直接创建优惠订单。
- 状态错误/存储不可用 fail closed，不静默当作开启；单模块读取不会被另一个坏键干扰。
- 使用 `settings` 的 `custom_extensions.<id>.enabled`，严格接受 `true` / `false`。每模块单独键，避免不同开关更新互相覆盖。

### 默认状态与老部署接纳

- 缺失开关默认关闭：新安装、从纯上游迁入，以及以后新增但尚未显式启用的插件，不会自动改变上游行为。
- 在执行本版本宿主 SQL **之前**，用固定的历史二开迁移文件名识别已有 fork；结果持久化到 `custom_extension_installation`。不会根据用户数、订单数或当前功能表是否存在来猜测。
- 已有 fork 对冻结 v1 的六个模块执行一次接纳，营销通过新增独立迁移接纳，补齐缺失开关为 true；已有 true/false（以及错误值）都不覆盖。错误值仍 fail closed，不能默默修成开启。
- 新安装或纯上游实例补齐六个 false。v1 接纳名单固定，不会随目录中模块增多而扩大。营销、多分组密钥及管理效率分别通过独立迁移接纳；待隔离模块不伪造开关。
- 接纳使用单事务并记录在独立 `custom_extension_schema_migrations` 账本；失败回滚且可重试。即使宿主迁移执行到一半失败，重试也不会把新安装重新识别为老站。
- 完成接纳后不再补写旧开关；配置键删除后按缺失规则关闭，不会在下次启动时重新开启。

新扩展迁移在 `backend/internal/customize/migrations/sql/<module-id>/<YYYYMMDDHHmm>_<name>.sql` 登记，与上游数字迁移编号隔离。校验和保持不可变、规范化 CRLF/LF；暂只支持事务内 SQL。安装向导和普通启动使用同一生命周期，在上游已有迁移锁的同一连接上执行，未另占连接。

**历史迁移文件没有移动、改名或改内容**，历史表和权益保持原状；关闭插件也不会停止其数据结构兼容升级。旧编号文件若与未来上游命名碰撞，仍须审查，不能直接覆盖。详见 [迁移与接纳契约](customizations/MIGRATIONS.md)。

本轮只运行隔离内存 SQL 和 SQL mock 回归，未连接或写入真实数据库。上线前仍须对备份恢复的新装/老部署 PostgreSQL 副本做演练。

## 4. 共享宿主边界与部署验收（不能隐藏）

1. 清单中待解耦条目已清零；所有权账本和契约仍保留，不以“零待解耦”宣称全部共享实现已迁走。历史章节中的阶段数量保留原时点含义。
2. 媒体模块控制新任务准入，宿主保留鉴权、协议传输、任务归属及计费结算；订阅模块控制新发放策略，宿主保留事务、历史多订阅选择、用量、过期及缓存兼容。
3. 订阅计划 `display_purchase_count` 是品牌首页的展示贡献，不是真实交易量；品牌首页关闭后不展示该页面，共享计划配置不删除，不重复挂在订阅发放开关下。
4. 访问安全规则不能由普通插件开关绕过；停用现有安全规则需先在安全设置中明确修改。独立 CDN/Vite 画布壳与宿主服务的状态边界仍须部署方协调。
5. 新装/老部署 PostgreSQL 副本迁移、并发首次发放、真实支付回调、媒体在途任务、浏览器操作和升级回退仍需部署验收；不能以 SQLite/sqlmock/模拟浏览器或编译结果替代。
6. 原先未匹配的 70 个路径和新增 Seedance 接入点已有精确归属证据；其余规则分类路径不等于全部业务语义已人工审查。新增不明归属继续阻止门禁。

历史 migration 按文件名和 checksum 入账，**不得改名、移走、重写已有迁移，不随插件关闭删表/列或清空记录**。

## 5. 统一校验与组合构建

在仓库根目录运行（使用已安装、锁定的依赖）：

```powershell
node tools/customization-verify.mjs --check
node tools/customization-verify.mjs --full
node tools/customization-verify.mjs --build
```

| 模式 | 内容 |
| --- | --- |
| `--check` | 29 项 Node 门禁自测 + 固定 814 路径账本 + 边界/接入点/上游等价检查 |
| `--full` | check + Go 全量、unit 标签全量、前端 ESLint、Vitest 全量、vue-tsc |
| `--build` | full + 前端生产构建 + `/canvas/` 基路径画布构建 + 资源合成 + 最后执行 Go embed 构建 |

- 默认核对 `customizations/upstream.json` 中已合并 SHA；可传 `--upstream <本地ref>`，不会自行更新远程。
- 脚本不 fetch/merge、不安装依赖、不重写生成源码、不操作生产数据库、不部署、不提交代码；测试可能启动测试内 HTTP/SQLite 等隔离资源。
- Go 使用 `-mod=readonly`，embed 使用 `CGO_ENABLED=0`；画布资源必须在前端清空 dist 之后合入、在 embed 之前完成。
- 产物：`.tmp/customization-release/sub2api.exe`（Windows）或 `sub2api`；记录：`.tmp/customization-verification/result.json`、`audit.json`。
- 每一步失败立即停止，不能绕过测试只输出一个“成功”二进制。
- Windows Go 备份测试需要现有 Git Bash 的 `sh`；本机沙箱内子进程查找失败，已通过经批准的沙箱外执行验证，不能改测试绕过。
- 独立 CI：`.github/workflows/custom-extensions.yml` 执行契约和全量回归，上传的仅是上述报告。**新增 workflow 不等于远程 CI 已执行通过。**

## 6. 上游升级与回退

1. 保存当前工作、记录部署版本、新建未使用的 `codex/backup-before-upstream-日期` 回退分支；涉及数据前先备份并验证恢复。
2. 核对 remote，再执行 `git fetch origin --tags`。严禁把旧缓存的 `origin/main` 当成最新。
3. 运行 `node tools/customization-audit.mjs --upstream origin/main --output .tmp/customization-audit.json --check`。退出码 2 是归属未明/人工归属无效、接入/清单/契约违规或同文件变更需审查，不等同已经确认 Git 冲突。
4. 在隔离检出/测试环境合并明确 SHA。上游重叠实现以上游为准，本地独有功能按固定账本重新接入；不用覆盖目录、`reset --hard`、强制 ours/theirs 代替。
5. 根据上游接口变化适配端口，运行 `--build` 及真实业务回归。不能为“无冲突”删除数据、丢弃二开或放松账本。
6. 验证完成后才更新已合并 SHA，发布自己的前后端一致组合产物；直接使用纯上游镜像不会包含这些扩展。
7. 行为回退可用已隔离模块开关；版本回退使用已验证旧组合产物。数据库走已验证的前向兼容或恢复流程，不手工删除扩展表。

长期目标是上游更新只影响少量宿主端口，而不是到处解决二开冲突；遇到上游契约或数据库变化仍需适配，不承诺未来“零维护”。

## 7. 本轮验证记录（2026-09-26）

记录区分阶段、代码变化和验证边界，不能用旧构建覆盖后续修改。

### 上一阶段最终产物与前端模拟浏览器

- 统一 `--build` 的迁移生命周期最终验证已于 2026-09-26 04:42:16 UTC 完成：11 步全部通过，前端 333 文件 / 2425 测试通过，Go 全量/unit、ESLint、vue-tsc、前端/画布构建、资源合成、Go embed 均通过。记录已保存在 `.tmp/custom-extensions-stage4-build-result.json`，日志为 `.tmp/custom-extensions-stage4-build.log`。
- 使用上述前端产物、独立 6287 端口 mock API 验证了活动开/关/重新开启、跨标签实时变化、客服面板卸载/恢复、状态接口 503、活动接口 503，以及品牌首页关闭后回到上游页面并保留 query。状态失败时显示“未知”并禁用开关；报价失败禁止提交，关闭活动后恢复原生报价。
- 充值金额 50 在开关往返中保留：开启显示活动到账 55，关闭回到原生报价。模拟下单固定返回 409，未触发真实支付。fixture 已停止，测试页已关闭，六个模拟开关恢复开启、故障注入恢复关闭。
- 页面证据：`.tmp/custom-extensions-qa-on.png`、`.tmp/custom-extensions-qa-off.png`；请求记录：`.tmp/custom-extensions-browser-requests.jsonl`。这里只证明前端模拟交互，不证明真实服务、数据库、计费或支付端到端。

### 本阶段新增：精确归属和活动交易规则提取

- Node 门禁现为 **29 项**，专项执行通过。固定账本仍为原始 814 路径 / 24 relocation，只追加提取目标，不重建基线。
- 对已合并上游基线的只读审计：891 个当前差异路径，其中人工记录 70、规则候选 821；未归类 0、归属格式错误 0、丢失路径 0、边界违规 0、丢失接入点 0。207 个多规则候选仍需后续人工核对。基线相同得到的 incoming overlap 0 **不是最新上游无冲突的证明**。
- 活动模块专项和宿主开关/报价/退款/返佣专项已通过；移走的旧活动算法回归随实现保留，新增单次舍入、旧快照精确 round-trip、历史奖励、冻结/已转出债务、部分退款精度、SQL 错误传播、宿主重复退款拦截及失败回滚测试。
- 本阶段统一 `--build` 已于 2026-09-26 05:46:17 UTC 完成：11 步全部通过。独立保存 `.tmp/custom-extensions-stage5-build-result.json`，日志为 `.tmp/custom-extensions-stage5-build.log`；本地产物未部署。画布仍有 Node 版本建议与大 chunk 警告，并非零警告。
- Docker 构建上下文新增精确排除本地运行配置、Redis 快照、临时缓存、生成预览和 temp-dist 副本；不删除已跟踪文件，不排除真实发布 logo 或 geo 源数据。只做结构契约检查，未构建 Docker 镜像。

### Stage6：营销核心与实际事务连接（本阶段本地构建通过，整体迁移未完成）

- 券/抽奖模型、接口、规则迁入独立 marketing 模块；旧构造器/DTO 保持兼容桥接，Wire 由真实命令生成，营销依赖集中注册。
- 修正旧仓储收到 Ent 事务 context 却使用根连接池的问题；营销 SQL、原生兑换码、钱包扣款现在共享抽奖事务，条件扣款失败不发奖。
- 订单和优惠券预留/折扣快照改为同事务提交，不再先提交订单后尝试预留、失败再删除订单；预留结果与报价不一致时回滚。无券订单保持原生路径。
- 新增真实仓储 + 单连接池 SQLmock 回归、宿主实际 SQLite 提交/回滚回归；原规则回归保留。覆盖三类奖品、失败回滚、预留竞争、余额条件扣减、报价变化和历史券状态更新。
- 本阶段统一 `--build` 于 2026-09-26 06:44:05 UTC（北京时间 14:44:05）完成，11 步全部通过：29 项 Node 门禁、固定基线审计、Go 全量/unit、前端 lint/333 文件 2425 测试/types/build、画布构建/合成、后端 embed。结果独立保存 `.tmp/custom-extensions-stage6-build-result.json`，日志 `.tmp/custom-extensions-stage6-build.log`；本地产物未部署。
- 基线审计为 906 个差异路径、70 个精确归属、836 个规则候选，未归类/边界违规/丢失清单均为 0；212 个多规则候选仍待人工审核。固定原始账本保持 814 路径 / 24 relocation。另对本地缓存 `origin/main` 的审计拦截一个 `backend/cmd/server/VERSION` 同文件改动，报告保存在 `.tmp/customization-audit-stage6.json`。未 fetch，未将缓存引用当成最新，未合并该引用。
- SQLmock/SQLite 不等于真实 PostgreSQL 并发或支付端到端；构建仍有 Node 版本、Browserslist 数据和大 chunk 警告，不能称为零警告。
- 营销仍为 pending：处理器/页面、只读 Overview 与发放分离、供应商失败/取消/过期/回调/退款重试及关闭后的旧权益策略尚未完成，不开放假开关。

仍未完成：真实 PostgreSQL 新装/接纳/并发/回退演练、真实支付/退款、生产部署、远程 CI、获取最新上游并实际合并演练。当前 Node 20.17.0 低于画布 Vite 7 推荐版本，构建还有大 chunk 警告，不能称为零警告。

即便本地校验全部成功，仍需完成第 4 节和实施方案各阶段，才能宣布本项目二开插件化目标完成。

### Stage7：营销 HTTP 隔离与历史核销事务（专项验证，尚非最终全量产物）

- 营销用户/管理处理器和全部 14 个路由迁入独立模块；共享 PaymentHandler 移除营销字段/方法，构造器恢复原生两个参数。扩展助手在原安全分组注册，保留鉴权、限流、管理权限和审计，不提供 public 营销入口。
- 身份解析通过窄接口注入，模块不导入宿主 middleware/service/Ent；新增身份防伪、分页/错误/泛型奖励 JSON、14 路由安全继承、营销独立注册和 typed-nil 防护测试。Wire 已实际重新生成。
- 历史核销接入移至 payment_marketing.go：两次券状态写入使用一个真实宿主事务；复用外层事务时不越权提交。SQLite 回归覆盖提交、第二次写入失败回滚、外层回滚、重复调用及失败后重试。
- 已通过 customize 全部受影响包、用户/管理 handler、路由 unit 测试及后端普通编译；核销修改后 TestMarketing 专项及 service 的 Payment|Marketing unit 回归通过。统一 --check 的29 项 Node 门禁、固定基线审计与 diff whitespace 检查通过。固定原始账本仍为 814 路径 /24 relocation，追加抽取记录后为16 extraction。
- 基线审计 `.tmp/customization-audit-stage7-base.json`：910 差异路径、384 上游修改路径、70 精确归属、840 规则候选、237 多归属路径、213 多规则候选；未归类/契约违规/缺失路径均为0。规则归类不等同业务语义逐项审核。
- Stage6 全量构建早于本阶段代码，不代表本阶段最终构建。当前未部署、未写真实数据库、未获取或合并新上游。
- 核销两次写入原子化不等同订单状态和权益全生命周期已原子化：Completed 状态仍在核销之前写入，依靠重试补偿；取消/过期释放错误、已支付履约失败释放后重试的权益风险、默认次数并发发放、前端迁移及关闭语义仍待完成。营销继续 managed=false。

### Stage8：支付状态与营销券权益的共同提交（进行中）

- 新增窄宿主事务适配器 withPaymentCouponTransition；未配置营销服务时使用原有根客户端路径，配置时回调明确使用事务客户端，避免根连接逃逸。
- markCompleted 的租约条件更新、折扣快照核销、用户券核销共同提交；核销失败回滚 Completed 状态，仍可原租约重试。审计和通知在事务提交后执行。
- cancelCore 在供应商检查之后才开启数据库事务；Pending 到 Cancelled/Expired 与两步券释放共同提交，不再忽略释放错误。失败保留 Pending，可重试；条件更新未命中不释放券。
- markFailed 不再释放券：履约失败不等同支付取消，已折扣支付订单仍可重试，不能让同一券被另一订单重复使用。
- 新增真实 SQLite 生命周期回归：完成/取消/过期的第二次扩展写入失败同时回滚状态和第一次写入，修复后重试成功；重复调用不追加权益；履约失败不得调用任何释放写入。Payment/Marketing/Fulfillment unit 专项已通过。
- 尚未解决：原生 toPaid 允许取消订单和宽限期内过期订单由迟到回调恢复支付，但释放后的券可能已被用于另一订单；需继续实现明确的权益冲突处理，不可仅把两步 UPDATE 的0行当核销成功。历史旧版本失败订单、供应商创建失败和退款策略仍待核对。营销继续 managed=false。

- Stage8 验证结果：service 全包 unit 通过（212.625s），customize 各包 unit 通过；仓储首次沙箱执行因无法找到已安装 Git Bash 的 sh 失败，经授权沙箱外重跑全部 repository unit 通过（4.100s）。后端普通编译通过，产物 `.tmp/custom-extensions-stage8-server.exe` 未部署。统一 --check 的29项门禁与基线审计通过，git diff --check 通过（保留既有换行提示）。本阶段未运行前端/画布/embed完整组合构建，不能替代最终发布验证。

### Stage9：迟到支付与旧失败订单的券归属恢复

- 模块新增 ReconcilePaidOrderCoupon：从原订单折扣快照核对券归属，不按当前模板/有效期重新报价。仍可用的券通过条件更新重新占用，released 快照只恢复状态和时间，不改原金额；两步在宿主真实事务内共同提交。
- 余额和订阅履约均在取得原生租约之后、任何充值/订阅写入之前执行恢复。恢复失败保留已支付事实，记录履约失败原因；券已被另一订单使用/占用时明确返回 PAYMENT_COUPON_OWNERSHIP_CONFLICT，不抢券、不重复优惠。需退款或解决归属冲突后再重试。
- 新增12种模块状态回归（无券、原订单预留/已用、释放恢复、旧失败记录、他单预留/已用、用户不符、并发抢占失败、快照恢复失败、缺失券/引用）。真实SQLite覆盖两类履约的前置阻断、PaidAt保留、恢复第二步失败整体回滚及重试；单连接SQLmock验证真实仓储条件更新和事务连接。
- 上述专项测试通过。原生迟到支付确认流程保留，营销仍 managed=false；当前仍待历史 Completed 异常核销修复、供应商失败/退款策略、只读抽奖Overview、前端迁移和关闭后历史权益处理。未操作真实数据库、未部署、未拉取或合并上游。

- Stage9 本地结果：Payment/Marketing/Fulfillment/Coupon unit 回归、恢复专项（service/module/repository）、后端普通编译、统一 --check 与 git diff --check 均通过。产物 `.tmp/custom-extensions-stage9-server.exe` 未部署；门禁日志 `.tmp/custom-extensions-stage9-check.log`。不是全量前端/画布/embed构建或真实支付验证，不能沿用Stage6完整产物代表当前源码。

### Stage10：历史已完成订单修复与只读抽奖概览

- Completed 的回调、余额/订阅重入、租约完成重试统一在真实事务内核对归属并修复核销；同订单半核销可修复，他单归属冲突不抢券、不改完成状态。只修券账，不重发余额或订阅；复用外层事务不提前提交。
- Overview 改用纯 SELECT，不创建状态、不发默认次数；未建状态返回零值，新增 pending_default_draw_times 独立预览可领取权益。真正默认次数发放留在 Draw 的活动/用户状态锁和统一事务中。
- 前端新增独立 marketing/lottery-state 助手，显示既有次数与可领取预览；抽奖返回已发放状态后忽略旧预览，避免重复计数。零次数且无钱包抽奖时禁用按钮。
- 专项验证通过：Payment/Marketing/Fulfillment/Coupon/Lottery unit（service/module/repository）；Completed 四入口各含归属正常/冲突和重复调用；Overview 新用户/已有状态/已发放/门槛未达只读测试；SQLmock 只允许 SELECT。前端3项次数预览回归、vue-tsc和改动文件ESLint通过。统一 --check（29门禁/固定账本审计）与 diff检查通过。
- 尚未真实 PostgreSQL 并发、真实支付/退款或浏览器交互验证；前端页面整体迁移、供应商失败/退款以及插件关闭后历史权益仍待完成，营销继续 managed=false。


#### Stage10 完整组合验证（2026-09-26）

- 首轮全量验证在 backend-unit-tests 停止：TestPinnedOpenAIModelsListMixedAccountsShareColdCacheAcrossGroups 检出共享冷缓存重复上游请求（期望1次，实际2次），失败证据保留在 `.tmp/custom-extensions-stage10-build-result.json`。
- 修复缓存查询与 singleflight 进入之间的竞态：在共享任务内部重新检查 fresh 状态，避免前一任务已完成时迟到调用再次请求上游。新增确定性 late-joiner 回归，与原失败测试各重复100次通过；不放宽原断言、不跳过测试。
- 修复后 `node tools/customization-verify.mjs --build` 的全部11步通过，包括29门禁、基线审计、Go普通及unit全量、前端lint/类型检查、334文件2428测试、前端/画布构建与嵌入式后端构建。证据：`.tmp/custom-extensions-stage10-retry-build-result.json` 与对应 `.log`；产物 `.tmp/customization-release/sub2api.exe` 未部署。
- 构建仍有警告：画布工具提示当前Node 20.17.0低于其要求的20.19+/22.12+；另有大包和动态/静态混合导入提示。本轮退出码为0，不等同于消除了工具链兼容风险。
- 本轮未迁移营销前端页面/API/路由，未改变 managed=false。完整本地构建通过不代表全部二开已插件化，也不代表真实PostgreSQL、支付/退款、浏览器交互或未来上游兼容性通过。未fetch/merge/提交/部署/修改真实数据库。


### Stage11：营销前端页面、API和路由隔离

- 将用户抽奖、管理优惠券模板、管理抽奖三个完整页面迁入 `frontend/src/extensions/modules/marketing/`。公共 user/admin payment API 移出14个营销方法，统一使用模块 `api.ts`，保留HTTP方法、URL、参数及Axios响应封装；原有全部调用者和支付页mock同步更新。
- 三条营销路由进入模块 `routes.ts`，通过现有 `extensions/routes.ts` 接入；原URL、名称、鉴权、管理员、支付开关和国际化元数据不变。共享router不再声明营销专属路径。
- 支付页优惠券加载通过既有checkout扩展端口获得数据，传输逻辑属于模块 `checkout.ts`；宿主不直接导入模块实现。该过渡不代表支付内联券选择、折扣计算和下单参数已完成隔离。
- 分开记录“模块路由归属”和“可开关状态”：pendingExtensionPaths/extensionOwnerForPath只用于归属；原extensionForPath和managed catalog仍只识别真正支持开关的六项。营销继续managed=false，不能因搬目录就自动变成可关闭模块。
- 原始814路径不变，页面搬迁另记3条（累计27 relocations）；API/路由提取及支付券加载记录在extractions。新增公共API/router防回流规则、required files，保留支付页既有“不导入modules实现”门禁。
- 首轮全量前端测试暴露旧契约把所有模块路由都等同于已支持开关；没有添加虚假营销开关，也没有跳过归属检查。改为验证每条路由都有归属，并严格验证待迁移路由存在、后端managed=false、未进入开关列表且不会被其他模块的关闭状态重定向。
- 专项66项测试、类型检查及改动文件ESLint已通过。最终全量前端复验及构建结果待追加。未修改后端业务源码、数据库或Git状态；未部署、未fetch/merge。真实支付/PostgreSQL/浏览器验证和整体插件化仍未完成。


- Stage11 最终前端复验：336个测试文件、2450项测试全部通过（`.tmp/custom-extensions-stage11-all-frontend-tests-retry.log`）；最终类型检查、改动文件ESLint、生产构建通过（`.tmp/custom-extensions-stage11-frontend-build.log`）。前端构建后重新接入未改动且此前已构建验证的画布dist，当前源码embed编译通过：`.tmp/custom-extensions-stage11-server.exe`，未部署。未重跑全部后端测试或重新构建画布，不能称本阶段重新执行了统一11步构建；Stage10完整结果保留，不冒充Stage11全量证据。
- 当前门禁29项及审计通过：原始814路径、27 relocations、19 extractions；未归属/契约违例/遗漏路径均为0。仍有构建大包警告，不代表浏览器、真实支付或未来上游兼容性已验证。

### Stage12：支付页优惠券结算隔离（2026-09-26，阶段验证）

- 将券选择、摘要、资格检查、折扣和下单字段交给营销模块；PaymentView仅调用通用checkout端口及两个UI插槽。pending插槽明确表示归属，不表示模块已经支持关闭。营销仍managed=false。
- 核对后端payment_order.go及payment/fee.go：门槛和扣券依据手续费/换汇前订单本金，然后交宿主进行支付币种换算与手续费投影。无券保持原生报价；互斥报价拒绝下单；微信恢复订单不附加当前券选择。
- 新增6项模块回归，连同支付页30项、营销API16项及插槽5项，共57项通过；vue-tsc修正订单类型边界后通过。改动核心文件ESLint通过，更新后的check门禁/审计通过。原始814路径账本不变，增加两个组件的提取去向及支付页防回流规则。
- 证据：.tmp/custom-extensions-stage12-final-targeted.log、custom-extensions-stage12-types-retry.log、custom-extensions-stage12-lint-retry.log、custom-extensions-stage12-check-final.log。
- 本阶段尚未完成：优惠券组件真实挂载交互测试、宿主券+手续费/换汇集成回归、订阅各支付方式限额与折后报价的一致性、全量前端测试和最终构建。新测试中的投影为端口测试替身，不作为真实支付链路证明。未部署、未更新上游、未写真实数据库。


### Stage13：折后支付限额与真实组件集成验证（2026-09-26）

- 通用结算端口增加可选目标币种，宿主保留原生无券报价；订阅支付方式列表现在使用该方式币种的折后报价判断限额，不再使用原价。没有把币种/手续费逻辑重新塞进营销模块。
- 新增真实 CouponSelection/Select 点击、CouponSummary、宿主支付按钮与CreateOrder请求集成回归。100本金、20券、7倍CNY汇率、10%手续费得到616，切换USD得到88；两种方式各自限额正确解锁，后端请求仍是本金100及券ID。99本金即使换汇后超过100也不能满足券门槛。
- 专项38项通过；前端全量337文件2458项通过；vue-tsc、改动文件ESLint、29门禁/审计及git diff --check通过。生产前端构建通过；复制此前已构建的未改动canvas资源后，Go embed构建成功，产物 .tmp/custom-extensions-stage13-server.exe 未部署。
- 证据：.tmp/custom-extensions-stage13-targeted-final.log、custom-extensions-stage13-all-frontend-tests.log、custom-extensions-stage13-types.log、custom-extensions-stage13-lint.log、custom-extensions-stage13-check-final.log、custom-extensions-stage13-frontend-build.log、custom-extensions-stage13-embed.log。
- 本轮没有重跑所有Go测试或重建canvas，不能称统一11步已再次全过。构建仍有大包及混合导入警告。组件/API模拟回归不代表真实支付、浏览器视觉或真实PostgreSQL并发验证。managed=false保持不变，未fetch/merge/提交/重启/部署/写数据库。
- 下一关键交易风险已从现源码确认：payment_order.go在invokeProvider失败时仍无条件更新Failed且忽略更新错误，可能覆盖并发支付状态。需要条件状态更新及失败/回调竞争回归，不能通过直接释放券处理网络超时；退款与关闭后历史权益规则仍待实现。

### Stage14：供应商创建响应失败与支付状态保护（2026-09-26）

- 源码确认：仅条件写Failed仍不能正确处理网络超时。Failed不进入现有Pending主动查单，而Failed又用于履约重试，不能将“创建响应失败”当成“确认未支付”。
- CreateOrder错误分支改为recordPaymentCreationFailure：保留现有订单状态、支付事实、时间戳及券占用，只记录ORDER_CREATE_RESPONSE_FAILED审计，原结构化错误原样返回。供应商原始错误消息不存入该审计；审计写失败仍记录日志，不覆盖原错误，也不改变业务状态。
- 新增九类状态保护、取消上下文导致审计失败、创建响应丢失后通过真实查单/余额履约完成、迟到错误不回退Completed且不重复入账的回归。专项及既有营销生命周期原子性测试重复10轮通过。测试使用隔离SQLite和模拟供应商，不是真实支付或PostgreSQL并发证明。
- 新增宿主错误处理文件的required检查及CreateOrder禁止回流SetStatus(OrderStatusFailed)门禁。历史SQL与原始814路径不变，未引入新数据库字段。
- 初次验证先暴露新测试依赖unit-tag测试助手、遗漏errors导入及应用时钟monotonic与数据库时间值比较差异；分别修正测试构建标记、导入和持久化基准后通过，没有跳过测试或放宽业务断言。失败日志保留；最终专项证据 .tmp/custom-extensions-stage14-targeted-repeat.log，门禁证据 .tmp/custom-extensions-stage14-check.log。
- 完整统一 --build 验证已完成：.tmp/custom-extensions-stage14-build-result.json 的 complete=true，全部11步通过；前端337文件2458项测试通过，含全量Go普通/unit、lint/types、前端与画布构建及最终embed。产物 .tmp/customization-release/sub2api.exe 未部署。画布仍有Node20.17低于Vite要求、动态/静态混合导入和大包警告，不能称无警告。营销仍managed=false。退款策略、关闭准入/旧权益、其他核心模块和实际升级验收均仍待完成。未fetch/merge/提交/部署/写真实数据库。


## Stage15：营销新增业务准入端口（2026-09-26，尚未接通管理开关）

模块新增NewBusinessAdmission及WithAdmission构造器，发券、券报价/预留、抽奖及默认机会发放在业务读写前检查；抽奖进入宿主事务后再次检查。显式传nil时拒绝准入，开关读取失败原样返回，不缓存开启状态。已有核销/释放接口不查此端口，关闭不能清空历史次数或阻断旧订单履约。

旧构造器暂显式采用pending兼容策略，宿主仍走原构造器，因此当前生产行为不变，marketing-tools仍managed=false。本阶段是服务边界准备，不代表已经可以关闭营销：宿主设置适配/注入、前端历史权益可见策略、退款和完整关闭回归仍待完成。准入不取消已在途事务，不承诺开关和业务提交具备跨请求线性一致性。

新增回归验证禁用/未配置在仓库调用前拒绝、事务内复查、每次请求实时检查以及历史核销/释放绕过开关。原有模块测试连同新增测试重复10轮通过；后续宿主回归与最终门禁结果另行追加。Stage14完整构建为本次新增端口之前的证据，不冒充Stage15完整构建。未部署、未写真实数据库、未操作Git提交或上游。


Stage15最终阶段证据：营销模块完整测试及宿主Coupon/Lottery/支付创建失败/原子生命周期专项均重复10轮通过（.tmp/custom-extensions-stage15-module-tests.log、.tmp/custom-extensions-stage15-host-tests.log）；29项门禁与814路径审计通过（.tmp/custom-extensions-stage15-check.log），git diff --check通过，仅已有换行提示。当前Go源码embed编译通过（.tmp/custom-extensions-stage15-embed.log），产物 .tmp/custom-extensions-stage15-server.exe 未部署。本阶段没有重跑全量Go/前端测试或重新构建前端/画布，复用了Stage14刚完成的静态产物；完整统一构建证据仍明确属于Stage14。下一步为宿主准入适配和依赖注入，不可直接把catalog改为managed=true。


## Stage16：营销准入接入真实依赖装配（2026-09-26）

扩展wiring新增ProvideMarketingAdmission/ProvideCouponService/ProvideLotteryService，Wire重新生成的启动代码将同一准入端口传入券和抽奖服务，不再使用legacy构造器装配生产服务。已知pending条目暂保留旧行为；unknown/缺失状态拒绝，managed分支每次读取共享设置、不缓存开启状态。catalog仍managed=false，不能据此称营销已支持关闭。

抽奖/订单事务内读设置使用tx.Client()，避免宿主设置仓库默认root连接导致小连接池等待或读不到事务状态；此适配限于扩展wiring，没有改写上游settingRepository。单连接SQLite回归验证真实Manager经适配器读取事务内未提交设置、回滚后不留数据。真实Manager测试使用已经managed的充值活动条目，不伪造营销已开放管理。营销managed分支由状态端口测试覆盖，两者证据范围分开。

新增装配回归覆盖：pending显式兼容、未知条目拒绝、开关读取错误传播、每次读取状态、生产provider实际传入两项服务、历史核销/释放不读取开关。首轮SQL测试漏导入Ent runtime导致validator为空，补正确测试初始化后，customize全部包重复10轮通过（.tmp/custom-extensions-stage16-customize-tests-retry.log），未跳过失败测试。后续宿主回归、门禁与编译结果另行追加。

Stage15的“宿主未注入”待办由本阶段完成；用户历史权益可见策略、退款券政策、管理写入口准入、前后端关闭联动、迁移接纳和完整关闭验收仍未完成。仍有其他pending模块、Ent隔离、真实PG并发及实际升级演练；不缩小最终目标。未fetch/merge/commit/push/部署/写真实数据库。


Stage16最终阶段验证：customize全部5个包重复10轮通过；宿主Coupon/Lottery/支付创建失败/原子生命周期专项重复10轮通过（.tmp/custom-extensions-stage16-host-tests.log）；最终check全部通过（.tmp/custom-extensions-stage16-check-final.log），git diff --check退出0。当前源码含重新生成的Wire完成embed构建（.tmp/custom-extensions-stage16-embed.log），.tmp/custom-extensions-stage16-server.exe未部署。没有重跑全量Go或前端测试，也没有重建未改动的前端/画布静态产物；Stage14仍是最近完整11步构建，不冒充本阶段全量证据。


## Stage17：营销管理写入口与历史只读接口（2026-09-26）

在营销service而非页面按钮增加7项管理写准入：券模板创建/更新、抽奖活动创建/更新/删除、奖品创建/更新；拒绝发生在参数解引用和任何仓库调用之前。显式disabled或状态读取失败不允许管理写入。生产仍使用Stage16注入，catalog继续managed=false，不提前提供不完整开关。

新增真实模块路由→handler→生产provider→service回归，分别验证7个写接口在disabled返回403、设置不可用返回503且不触碰仓库；不改变上游鉴权组，本测试只验证准入传播，不冒充完整鉴权测试。用户券、活动概览、用户抽奖记录、管理券模板/活动/记录6个读接口在disabled和状态不可用仍可查询；验证券71的unused历史数据未被清空，用户历史查询仍取原生认证UserID=42而不是query中的99。历史核销/释放沿用已有不查开关的服务边界。

本阶段没有让历史查询重新发放次数，也没有按关闭状态修改旧券、旧次数、奖品库存或订单。customize全部包及新增服务/HTTP回归重复10轮通过（.tmp/custom-extensions-stage17-customize-tests.log）。管理写入口待办已完成；前端只读展示/结算联动、退款策略、接纳迁移和完整开关验收仍待完成，其他pending模块与真实升级演练仍保留完整范围。后续宿主验证与编译证据另行追加。


Stage17最终阶段证据：扩展全部5个包重复10轮通过；宿主优惠券/抽奖及支付原子生命周期专项重复10轮通过（.tmp/custom-extensions-stage17-host-tests.log）；29门禁与原始814路径审计通过（.tmp/custom-extensions-stage17-check.log），git diff --check退出0。当前后端embed编译通过（.tmp/custom-extensions-stage17-embed.log），产物 .tmp/custom-extensions-stage17-server.exe未部署。HTTP回归为内存Gin与仓库替身，不代表真实PostgreSQL或浏览器验收；本轮没有重跑前端或全量Go测试，最近完整11步构建仍是Stage14。未写真实数据库，未拉取上游或提交代码。


## Stage18：支付券贡献的前端关闭联动（2026-09-26，营销仍pending）

营销模块新增useMarketingAdmission，明确区分pending兼容行为和managed共享状态。当前catalog仍pending，保持旧功能；以后完成管理接纳时，同一组件读取已有共享store，未知/刷新失败按关闭处理，不额外创造后台开关或复制公共设置接口。管理接纳必须前后端一起更新，当前测试中的managed目录为隔离替身，不代表已上线开关。

券结算controller接受只读响应式准入，关闭后同步清除当前选择、返回原生报价、隐藏CouponSelection与CouponSummary，新订单移除遗留user_coupon_id，不清空已加载的历史券；重新启用不会自动恢复之前的选券。恢复订单优先保留wechat_resume_token历史字段，不按今天的开关改历史券参数。关闭期间不请求支付选券列表，关闭后才返回的在途列表不覆盖当前状态。后端仍是最终下单准入权威。

新增回归覆盖managed状态缺失/共享刷新失败、原生金额回退、恢复订单保留、重新开启不自动选券、在途请求与真实选择/摘要组件显隐。券组件测试只替换Select下拉部件，已有PaymentView真实Select集成测试继续保留。专项3文件44项通过（.tmp/custom-extensions-stage18-targeted-final-retry.log）；首次新测试文件路径相对cwd错误已修正，随后修复i18n测试替身误覆盖createI18n，没有删除或跳过失败测试。全量前端/类型检查结果另行追加。

本阶段没有修改宿主PaymentView或营销目录可管理状态。抽奖页/管理页只读联动、退款策略、旧安装接纳和完整开关验收仍待完成；其他pending模块与真实升级演练仍在完整目标内。未部署、未修改真实数据库、未拉取或提交上游代码。


Stage18最终阶段证据：前端全量338文件2464项通过，vue-tsc与修改文件ESLint通过；29门禁/814路径审计及git diff --check通过。前端生产构建成功（.tmp/custom-extensions-stage18-frontend-build.log），复制未变更且Stage14已验证的canvas/web/dist后，当前源码embed构建成功（.tmp/custom-extensions-stage18-embed.log），产物.tmp/custom-extensions-stage18-server.exe未部署。本阶段未重建画布、未重跑全量Go，最近完整11步验证仍为Stage14；大包构建警告仍保留。


## Stage19：抽奖页只读联动（2026-09-26，营销仍pending）

LotteryView接入共享营销准入：关闭后禁用抽奖按钮，handleDrawClick/confirmDraw/executeDraw均在发请求前检查；同步关闭尚未提交的钱包确认弹层，重新开启不会自动提交。保留活动概览、历史次数及记录查询，不修改服务端历史权益，也不取消已经提交的抽奖请求或丢弃其结算结果。仅阻止新的抽奖；后端仍为最终准入权威。提示文案位于营销模块，未向上游全局语言文件新增耦合。

新增真实Vue组件回归：关闭下历史读取、三个入口拒绝、钱包确认期间关闭及过期回调、重新开启不重提、开启时正常发送与关闭不重复请求。初轮真实Transition离场动画导致DOM断言时机失败，改为Vue Test Utils的Transition桩后，专项3文件15项通过（.tmp/custom-extensions-stage19-targeted.log）；不是浏览器动画验收。vue-tsc及修改文件ESLint通过，门禁/原始814路径审计与git diff --check通过。生产构建结果另行追加。

本阶段未接纳营销managed、未改变路由关闭策略、未改管理页。管理页只读、退款策略/历史Failed订单审查、旧安装接纳、完整关闭验收以及其余pending模块仍未完成。未重跑全量Go；Stage18前端全量338文件2464项为新增本轮组件测试之前的证据，Stage14仍为最近完整11步验证。未部署、未写真实数据库、未拉取上游、未提交。


Stage19构建收尾：前端生产构建成功（.tmp/custom-extensions-stage19-frontend-build.log），保留大包警告；复制未改动、Stage14已验证的画布产物后，当前源码embed构建通过（.tmp/custom-extensions-stage19-embed.log），产物.tmp/custom-extensions-stage19-server.exe未部署。未重建canvas；本次是前端专项/类型/lint/门禁/构建证据，不冒充完整11步重跑。


## Stage20：营销管理页只读与无活动历史入口（2026-09-26）

优惠券模板页和抽奖管理页接入同一useMarketingAdmission。优惠券新建/编辑/提交/状态切换，以及活动/奖品新建/编辑/提交/状态切换/活动删除，在处理函数入口重新检查准入；删除确认返回后再次检查。按钮同步disabled，开关关闭同步关闭未提交写表单，重新开启不会自动提交。查询、刷新、表格数据和参与记录抽屉保持可用，不清空库存、次数、券或历史记录。已发出的写请求仍由服务端权威处理，不伪装为客户端撤销。

新增MarketingReadOnlyNotice模块内共享中英文提示，抽奖与管理页复用，不修改上游全局语言包。修复无活动时历史入口缺失：LotteryView无active activity仍可打开历史抽屉，以未指定activity_id查询全部本人历史，沿用原生认证接口。

新增admin-admission.spec.ts使用真实DataTable和页面函数，覆盖关闭后全部13个管理处理入口、已打开的4类活动/奖品表单与2类券表单、删除确认跨开关变化、启用后正常写请求、历史参与者可见；仅替换布局、Select和弹窗容器。新增无活动用户历史查询测试。首轮vue-tsc发现抽奖页抽取提示组件后残留旧computed，已删除残留并重新通过类型/修改文件ESLint；未忽略类型错误。最终全量测试与构建结果下附。

营销仍managed=false，以上测试注入准入状态验证关闭分支，不冒充真实管理开关已接纳。服务端准入见Stage16–17；退款/旧Failed支付事实策略、接纳迁移、路由历史访问策略和完整端到端验收仍待完成。其他pending模块、Ent隔离、真实PG和实际上游升级演练保留完整范围。未部署、未写真实数据库、未拉取上游、未提交。


Stage20退款证据补充：新增payment_marketing_refund_test.go，使用历史138迁移中前三张真实券表定义，仅将BIGSERIAL与NOW默认值转换为SQLite方言，未修改原SQL。宿主finishRefund→markRefundOk真实事务覆盖余额/订阅 × 全额/部分 × 正常/审计失败8组，重复10轮通过。逐列快照验证模板、已用券及折扣快照完全不变；审计失败回滚状态，重试仅一条成功审计；准入端口一旦被读取则测试失败。此为结算事务验证，不代表真实网关、余额/订阅扣回或PG验收，未增加no-op钩子。源码确认PrepareRefund当前不接受Failed，即使PaidAt存在；该历史异常退款路径仍待按实际发放证据设计，不能把正常退款测试解释为异常支付也已解决。


Stage20最终验证：最终前端全量340文件2478项通过（.tmp/custom-extensions-stage20-all-frontend-tests-final.log），vue-tsc与修改文件ESLint通过；宿主internal/service全部unit测试及customize全部5包通过（.tmp/custom-extensions-stage20-go-tests.log），其中新增退款8组专项重复10轮通过。29门禁、原始814路径审计与git diff --check通过；前端生产构建成功，复制未改动的Stage14画布产物后，当前源码embed构建成功（.tmp/custom-extensions-stage20-embed.log），.tmp/custom-extensions-stage20-server.exe未部署。保留大包警告；未重跑其他Go包、未重建canvas、没有真实浏览器/PG/网关验收，因此最近完整11步验证仍为Stage14。

### Stage21 追加：历史入口不随新业务开关消失

营销模块的历史三页面采用模块所有权+精确路径白名单，共享路由/运行时和侧栏保留只读入口；状态服务挂起不阻塞历史导航，状态失败不允许新增业务。原鉴权与管理权限保留，其他插件继续使用既有关闭重定向策略。营销仍managed=false，尚未正式接纳开关。新增异常退款/履约抢占安全回归，没有放宽Failed订单退款；剩余范围与验证边界见customizations/IMPLEMENTATION_PLAN.md Stage21。

## Stage22 当前接纳边界（2026-09-26）

优惠券/抽奖 marketing-tools 已在前后端真实目录中接纳为 managed，第七个可管理模块；另六组核心二开仍待隔离，整体目标尚未完成。旧阶段中的 pending/managed=false 是当时状态，不代表当前目录。

- 服务端每次通过实际共享设置准入；缺失、错误值或读取失败拒绝新增业务。旧构造器不再隐式放行，测试需要显式提供准入。前端报价、券选择、两个结算插槽及管理/抽奖写操作使用同一真实开关，不保留 pending 绕过。
- 关闭同步清除未提交选券并恢复原生报价；历史三个页面保留原有身份/管理员/支付权限，仍可读取。核销、释放、已支付订单履约重试不因开关中断，不撤销已经准入的请求。
- 新增独立迁移 marketing-tools/202609260002_adopt_marketing.sql；不扩写冻结的 v1 六项名单，也不改历史 SQL。legacy 缺失值补 true，native 缺失值补 false，显式 true/false/错误值均保留。登记后删除设置不因重启再次补写。
- Failed（即使已有 PaidAt）的退款增强仍未实现。已读取上游基线 aea725f2ea644d5592d0bbb1d63b607efa7e200a 的 payment_refund.go：PrepareRefund/ExecuteRefund 同样不接受 Failed。本次保留这一资金政策，不为提供开关擅自扩大退款状态；历史券冲突不能宣称可直接退款。异常权益事实与退款增强仍保留为待办。
- managed 表示真实开关接入，不等于发布验收。营销 Ent 仓储仍由宿主适配；真实 PostgreSQL 并发、浏览器联动、支付网关及上游升级演练均未完成。

### Stage22 验证结果与未覆盖项

- 前端全量 341 文件、2485 项测试通过；最终 vue-tsc、扩展目录 ESLint、Vite 生产构建通过。
- customize 全部包通过；service 全包 unit 通过；repository 首次因 Windows 同时继承 PATH/Path 而缺失 sh 失败，规范化后全包 unit 通过。统一验证器已加入同名 PATH 合并与 Windows/POSIX 两项回归，实际 repository 通过该环境函数重跑成功。不修改生产准入来绕过测试。
- 门禁 31 项、814 路径所有权审计通过；最终 embed 构建通过，产物 .tmp/custom-extensions-stage22-server.exe 未部署。画布复用未变 Stage14 dist，本轮未重建；前端仍有 Browserslist/混合导入/大包警告。
- 本阶段结果见 .tmp/custom-extensions-stage22-result.json；本轮未重新跑全部 Go 包、完整 11 步或真实 PG/浏览器/支付网关验收，最近完整 11 步仍是 Stage14。未 fetch/merge/提交/部署/写真实数据库。

## Stage23：多分组密钥的策略边界提取（2026-09-26）

开始推进剩余核心模块，而不是增加更多已完成模块的开关。多分组候选顺序、余额/订阅优先级、回退及规范化规则已迁入 backend/internal/customize/modules/multigroupbilling。共享 APIKeyService 不再内嵌选择算法，独立 api_key_group_selection.go 通过泛型宿主端口接回真实分组、订阅与余额，保持外部签名和错误码。模块无 service/Ent/HTTP 依赖；通用协议规范化仍由宿主提供。

保留原始 814 路径账本，追加两条实际提取记录；新增宿主算法防回流与模块依赖边界门禁。没有删除鉴权或扣费校验，没有替换持久化分组和缓存语义。

本模块仍 managed=false：Create/Update、仓储与鉴权缓存、两类网关鉴权入口、前端 KeysView 及新旧密钥策略归属必须完成后才能接纳。不能通过关闭后让既有多分组密钥失效来声称恢复上游。七项可管理、六组待隔离的总状态不变；其余模块、真实数据库/网关/浏览器与实际升级验收继续属于完整目标。具体后续接口见模块 README。

### Stage23 验证结果与边界

统一验证器 --full 的七步已全部通过：31 项门禁、所有权审计、后端全部普通测试、后端全部 unit 测试、前端 lint、前端 341 文件 2485 项测试与类型检查。最终 --check 和 git diff --check 亦通过。记录见 .tmp/custom-extensions-stage23-full-result.json。

独立选组策略专项重复十轮通过，模块语句覆盖率 100%；宿主桥接与原有选组回归重复十轮通过。该覆盖率只代表小型策略模块，不代表整个多分组业务验收。embed 构建通过，产物 .tmp/custom-extensions-stage23-server.exe 未部署；复用 Stage22 前端与 Stage14 画布静态产物，本阶段未重新构建前端或画布。最近完整十一项构建验证仍是 Stage14。

当前仍为七项 managed、六组 pending，多分组模块尚未开放运行时开关。没有真实 PostgreSQL、浏览器、网关或实际上游升级验收；没有 fetch/merge、提交、部署或修改真实数据库。整体目标未完成，也不承诺任意未来上游版本零冲突。阶段汇总见 .tmp/custom-extensions-stage23-result.json。
## Stage24：密钥创建与更新的绑定输入边界（2026-09-26）

新增 multigroupbilling/binding.go：PlanCreate/PlanUpdate 独立规划主分组、候选列表、平台推断输入及精确写字段意图；保留 GroupIDsSet 显式清空与字段缺省差异。计划复制输入指针和数组，不修改持久化快照。普通无绑定变更的更新不访问用户/分组仓储。

共享 APIKeyService 的创建和更新改为调用 api_key_group_binding.go 宿主适配器；平台定义、分组读取、有效订阅和用户绑定权限检查仍由宿主负责，全部成功后才应用绑定与字段掩码。平台单独更新不标记 GroupID/GroupIDs 写入；显式清空同时写两个分组字段。绑定失败不修改绑定或已有字段掩码，不影响计费热路径的配额/用量列。

原始814路径账本保留，追加真实提取记录与模块依赖/宿主规则防回流门禁。新增字段存在性、主分组优先、空列表、错误用户、禁止分组、平台不符、创建推断及无输入别名测试；模块与宿主专项重复十轮通过，--check 与 diff --check 通过。service 全包 unit 和 customize 全包测试另行执行，完成证据以阶段日志为准。

尚未实现新旧密钥策略持久化归属及开关准入，因此 multi-group-billing 仍 pending，七项 managed/六组 pending 不变。本次不改数据库或历史迁移，不改前端，不部署；Stage23全量结果不代表本次新源码已完成全量验证。
Stage24补充验证：此前后台 service 全包 unit 与 customize 全包测试已正常结束（exit 0）；service 用时215.729秒，日志 .tmp/custom-extensions-stage24-unit.log。不是仅专项测试通过。

## Stage25：分组生命周期与密钥仓储边界（2026-09-26）

从 api_key_repo.go 迁出分组删除/迁移的完整实现到宿主 api_key_group_binding.go。纯分组包含、移除和替换去重规则由 multigroupbilling/group_lifecycle.go 拥有，PostgreSQL原子批量SQL由 group_lifecycle_sql.go 拥有。两条SQL逐字与原HEAD实现核对一致，没有修改历史迁移、数据库字段或实际数据。

宿主适配器保留两种原执行路径：没有事务时使用原批量SQL；已有Ent事务时使用 clientFromContext，不绕过事务从根连接写入。保留软删除条件、用户范围、新分组平台默认值、主分组清空/保留、影响行数和错误传播。这是模块边界迁移，不是重新设计原SQL与Ent在异常历史数据上的差异。

新增纯策略9组用例、SQL执行/结果错误与影响行数回归、事务读失败回归，以及事务内真实Ent生成UPDATE的参数断言（主分组删除、次分组替换）；模块与仓储专项重复十轮通过。SQL mock验证SQL与适配行为，不冒充真实PostgreSQL事务或并发验收。门禁和所有权审计、diff --check通过。repository全包unit与customize全包另行验证，完成情况见阶段结果。

整体仍七项managed/六组pending。多分组仍缺新旧策略归属、关闭后的新增配置准入、缓存/网关与前端适配，不能仅因更多文件迁出就接纳开关；没有部署、上游合并或真实数据库操作。

Stage25最终验证：repository全包unit与customize全包均exit 0（.tmp/custom-extensions-stage25-unit.log）；专项重复十轮通过。未重跑前端、全后端、embed或完整11步构建；最近全量7步仍Stage23、完整11步仍Stage14。

### Stage26 补充（2026-09-26）

多分组计费新增密钥路由归属字段、模块Ent mixin、独立迁移及缓存版本25。历史密钥归属不随开关重解释；当前仍未开放该模块开关。仓储新增回归十轮通过，完整相关unit验证记录见customizations/IMPLEMENTATION_PLAN.md。7 managed / 6 pending未改变，未部署、未写真实数据库。

## Stage27：多分组密钥开关接纳（2026-09-29）

多分组密钥 `multi-group-billing` 正式接纳为第八个 managed 模块。开关**只决定新配置**：路由归属在密钥创建时持久化（Stage26），此后不随开关重解释。

- **创建**：服务端每次创建都读取共享开关。开启 → 归属 `multigroup-v1`（保持原二开行为）；关闭 → 归属 `upstream-v1`，且拒绝两个以上分组或订阅优先配置（`CUSTOM_EXTENSION_DISABLED`）。开关读取失败直接报错，不猜测永久归属。未接入扩展宿主时等同上游。
- **更新**：`upstream-v1` 密钥始终保持单分组、余额优先（`API_KEY_NATIVE_ROUTING`，需新建密钥）。`multigroup-v1` 密钥在关闭时可保持现有配置、缩减或改回余额优先，但不能加入原本没有的分组或新改为订阅优先。只做上游形态修改（改名、改单分组等）时不读取开关存储。
- **排序问题**：第一版规则把"重排已有分组"算作新配置；前端测试暴露出历史数据的主分组不一定排在列表首位，关闭后仅改名就会被拒绝。已改为按集合判断，只有原本没有的分组才算新增。
- **前端**：新增 `extensions/modules/multi-group-billing/key-routing.ts`；KeysView 的编辑框与表格内分组选择器按"多分组 / 保留 / 单分组"三种模式显示分组与计费选项及说明。共享状态缺失或读取失败时按关闭处理；服务端始终是最终权威。DTO 只读暴露 `routing_policy`，请求类型不含该字段。
- **目录拆分**：原 `multi-group-billing` 的所有权规则包含 billing/scheduler/composite 等共享计费调度路径，开关控制不了它们。已拆出新的 pending 条目 `billing-scheduling`（计费与调度增强），避免开关声称控制了实际不受控的代码。Manifest 新增 `gates` 字段标注只有后端准入点、没有页面或插槽的模块。
- **迁移**：新增独立 `multi-group-billing/202609290001_adopt_multi_group_billing.sql`，不修改冻结 v1 及之前文件。历史 fork 安装补 true，新装/纯上游补 false，显式值（含错误值）保留。
- **装配**：`wiring.ProvideKeyRoutingAdmission` 复用营销模块的事务感知设置读取器；Wire 已重新生成。

当前状态：**8 managed / 6 pending**（pending：计费与调度增强、订阅与兑换增强、视频与媒体、管理效率、站点定制、访问策略）。标准模式鉴权场景、真实 PostgreSQL 迁移演练、浏览器与网关验收仍未完成；没有部署、提交或修改真实数据库。

### Stage27 验证结果

- 统一 `--full` 七步全部通过（`FULL_EXIT=0`）：门禁、所有权审计、Go 全量、Go unit 全量、前端 lint、前端 342 文件 2495 项测试、vue-tsc。日志 `.tmp/custom-extensions-stage27-full.log`，结果 `.tmp/custom-extensions-stage27-full-result.json`。
- 首轮相关 unit 运行发现 `TestAPIContracts` 两处失败：API Key 响应新增只读 `routing_policy` 字段。已按新契约更新期望值（无扩展宿主时新建为 upstream-v1，未记录归属的旧行显示 multigroup-v1），未改变业务行为。
- 新增：模块准入 17 组更新用例与 9 组创建用例（模块语句覆盖率 100%）、宿主 Create/Update 端到端 14 组、真实 SQLite 设置 + 生产 provider 的开关读取、独立迁移显式值保留、KeysView 6 组模式测试、前端路由模式 4 组。
- 前端生产构建通过（保留既有大包警告）；复用未改动画布产物后 embed 构建通过，产物 `.tmp/custom-extensions-stage27-server.exe` 未部署。本阶段未重建画布，最近完整 11 步构建仍为 Stage14。
- 未完成：标准模式鉴权专项、真实 PostgreSQL 迁移演练、浏览器与网关验收。未 fetch/merge/提交/部署，未写真实数据库。

## 2026-10-01 Stage28：访问与注册策略规则拆分

本轮按管理页使用的后端 catalog 继续，当前为 **10 managed / 4 pending**。订阅/兑换、视频/媒体、管理效率、访问/注册策略仍是 pending；没有为安全模块增加可绕过既有策略的业务开关。

- 后端规则归属：`backend/internal/customize/modules/accesspolicy/`。提取签名注册凭证、邮箱白名单/主域归一化、别名去重、域名额度准入决策和区域页面访问判断；不依赖共享 Service、Ent 或扩展开关仓库。
- 前端归属：`frontend/src/extensions/modules/access-policy/`，包含邮箱规则、凭证求解器及 Worker、区域跳转规则。旧 utils/router 路径保留兼容转发；Worker 与求解器同目录。
- 宿主继续拥有安全设置、注册鉴权、限流、事务和原子别名/域名注册守卫。预检与实际插入共用独立决策规则，写入前仍重新检查策略，不能把预检当作授权。
- v1 凭证 JSON 字段、HMAC scope、邮箱/IP 绑定、有效期和错误码保留。新增旧版算法签发令牌的兼容性测试，确认普通插件状态 true/false/invalid 都不能跳过凭证验证。
- 区域限制仍只影响站点导航，不增加网关封禁；关闭行为设计尚未完成，所以页面仍明确显示“尚不可切换”。既有安全设置读取失败语义本轮未更改，须后续独立审查。
- 修正既有清单漂移：前端补登记后端已 managed 的 billing-scheduling；扩展测试从旧 8 项更新到 10 项；三个 site-customization 文件补齐所有权。没有新增迁移、修改真实数据库、重启、部署或提交。

本阶段验证结果见实施方案 Stage28 末尾；局部测试、全量回归、构建和线上验收分开记录。


## Stage29：管理效率增强——上游账号分组目录边界（2026-10-01）

### 已落地

- 新增独立 Go 模块 `adminefficiency`，拥有上游分组数据类型、名称归一化、目录查询/重命名/排序服务，以及三个可独立装配的仓储端口；不依赖宿主 Service、Repository 或 Ent。
- 宿主 `admin_account_upstream_groups.go` 仅做能力发现与委托；`account_service.go` 保留类型与错误别名，`account_upstream_group.go` 保留名称校验适配，因此普通创建/更新、管理员创建/更新和仓储路径仍复用同一规则。
- 重命名的目录与账号标签联动、唯一性检查/锁、排序全量事务继续由现有账号仓储负责，不另起事务、不改表、不删历史分组。管理路由及其权限中间件保持原样。
- 前端 `admin-efficiency` 拥有上游分组类型、目录 API、字段组件与批量设置交互。通用账号批量写入由宿主通过 `BulkUpstreamGroupUpdater` 注入；模块不引用整套 admin API，旧组件/接口路径保持兼容。
- 保持 100 字符后端 Unicode 校验、空名称与显式清空的差异、空目录组展示、部分成功/失败提示、API URL/JSON/错误码。独立测试和宿主兼容测试分别覆盖，未把仓储 mock 当作真实事务验收。
- 更新所有权、必需文件、禁止反向依赖、宿主接入标记、提取台账和管理页 catalog 进度。

### 未完成

`admin-efficiency` 仍为 **managed=false**；总数仍是 **10 managed / 4 pending**。本轮完成的是上游分组目录子边界，不是整个管理效率模块可卸载或可关闭。账号排序、批量用户操作、其余分组管理，以及全入口写入准入和关闭后的历史分组保留契约仍待处理。

没有修改真实数据库、部署、重启、提交或拉取上游；浏览器交互、真实 PostgreSQL 事务/并发与线上行为未验收。


### Stage29 验证记录（分项记录，不等同于全量通过）

- 新 Go 模块连续运行 10 轮通过，语句覆盖率 100%；原账号创建/更新、管理员分组管理专项通过。
- 前端生产构建通过，保留既有大包与 Browserslist 数据过期提示；未升级或安装依赖。
- 统一 `--full` 两次均在普通 Go 全量阶段停止，失败点位于本轮未改动的测试：首次 `TestRecordCyberPolicyEvent_RuntimeSnapshotRefreshFailureKeepsStaleScope` 的异步等待断言；第二次 `TestSanitizeOpenAIResponsesToolParameterTypes_RewriteCountIndependentOfHits` 的分配次数断言。两者各自独立重跑 20 轮通过。未修改业务代码、放宽断言或跳过失败测试；不能据此宣称全量回归通过。
- 失败日志：`.tmp/stage29-full.log`、`.tmp/stage29-full-retry.log`；保留两份结果 `.tmp/stage29-full-first-result.json`、`.tmp/stage29-full-result.json`。后续 Go unit 与前端 lint/tests/types 单独执行，不覆盖失败结论。

- 分项补验完成：Go `-tags=unit ./...` 全量通过；前端 lint、348 文件 2529 项测试、vue-tsc 全部通过。日志 `.tmp/stage29-backend-unit.log`、`.tmp/stage29-frontend-full.log`，前端结果 `.tmp/stage29-frontend-result.json`。单独执行 lint 首次缺少 pnpm 隐式依赖查找路径，后复用正式验证器的环境解析后通过；未安装依赖。
- 最终所有权/契约/台账 `--check` 与 `git diff --check` 通过。普通 Go 全量的两次失败仍保留，未声称统一 `--full` 已通过。


## Stage30：管理效率模块完整开关边界（2026-10-02）

本阶段完成 `admin-efficiency` 剩余关闭契约，源码清单为 **11 managed / 3 pending**，管理页使用真实 catalog 的组件测试断言 11 个开关和 3 项待解耦。剩余为订阅扩展、媒体网关、访问与注册策略增强。Stage28/29/30 数字均为当时记录，当前状态见文首和 Stage31–33。

- 模块拥有账号显示排序、上游分组目录、分组账号替换编排、批量用户操作归一化/逐项结果，以及对应 API/UI。宿主保留鉴权、原生平台/OAuth-only 校验和仓储事务，不复制数据库模型。
- 新写入准入覆盖账号排序、目录重命名/排序、账号创建/修改/批量更新中的上游分组字段（包括清空）、导入所用创建路径、影子账号继承分组（原生复制沿用不复制上游标签的既有行为）、分组账号替换、批量禁用/删除用户。关闭、缺失、无效或读取失败均不得继续写入；普通编辑携带完全相同的历史标签仍允许。
- 前端关闭入口并在提交时再次检查，包括已打开弹窗与二次确认；普通创建/编辑剔除上游分组字段而非阻止原生保存。批量删除改为真实批量端点，不能循环原生单项删除绕过开关；部分失败保留对应选择项。
- 历史分组目录、标签、顺序和成员读取不受实时开关限制，不清空业务数据。原生分组排序、原生账号/用户/分组 CRUD 不加入整页封锁。管理页开关是运行时准入，不是在线卸载代码。
- 独立接纳迁移 `admin-efficiency/202610020001_adopt_admin_efficiency.sql`：已识别旧 fork 默认开启、原生新装默认关闭，已有 true/false/无效值不覆盖；账本已记录后删除的开关不会重新补种。保留冻结旧 SQL 和原有按路径排序机制；每个迁移单独事务，后续失败不会撤回已成功的前序迁移。
- 复用现有 `SettingService.CustomExtensions()` 注入，不改 Wire 生成代码。所有权、必需文件、反向依赖限制、宿主接入标记和提取台账同步更新。

本阶段未重启服务、部署、提交、拉取上游或写真实数据库；真实 PostgreSQL 事务/并发和运行中浏览器尚未验收。已有进程仍可能返回旧 catalog，因此不能把源码组件测试描述成当前浏览器已经显示 3 项。


### Stage30 最终验证记录

- `go test ./...` 全量通过：`.tmp/stage30-go-full.log`。`go test -tags=unit ./...` 在补齐影子继承/原生复制边界测试后再次全量通过：`.tmp/stage30-go-unit-final.log`。复用已安装的 Git sh，仅在测试进程临时设置 PATH。
- `adminefficiency` 独立模块重复 10 轮通过，语句覆盖率 100%；迁移和批量用户处理器专项通过：`.tmp/stage30-module-repeat.log`、`.tmp/stage30-boundary-final.log`。
- 前端全量 **350 个文件 / 2541 项测试**通过：`.tmp/stage30-frontend-tests-3.log`。包括真实 catalog 的 11/3 断言、未知/关闭状态、对已打开弹窗的写入撤销、原生账号创建/编辑和批量删除失败保留。
- 前端全量 lint、vue-tsc 通过；最终改动文件 lint 和 vue-tsc 再次通过：`.tmp/stage30-lint-final.log`、`.tmp/stage30-types-final.log`。Vite 生产构建通过：`.tmp/stage30-build.log`，仍有既有大 chunk / Browserslist 数据过期提示，未升级依赖。
- `node tools/customization-verify.mjs --check`、`git diff --check` 通过；契约、所有权和台账检查无新增未归类/违规/缺失路径。日志 `.tmp/stage30-check-final.log`。
- 以上为本地源码、测试与构建证据；未重启或部署，未应用迁移到真实数据库，未声称已完成线上/浏览器/真实事务验收。


## Stage31：访问策略配置准入与历史规则保留（2026-10-02）

`access-policy` 完成独立配置准入、服务装配和前端访问边界，阶段清单为 **12 managed / 2 pending**。

- 开关控制新的二开安全配置修改，而不是关闭认证。关闭、缺失或读取异常时，禁止更改域名配额、注册证明及大陆页面限制；未提交/未变化字段和原生邮箱白名单仍可保存。后端两个设置保存入口都执行准入，不能靠旧表单或直接请求绕过。
- 已保存安全规则、旧签名凭证、邮箱别名唯一性和原子配额检查继续生效；读取异常或非法持久化规则返回错误，不隐式放行。若要停止既有规则，应先在扩展开启时明确关闭该规则，再关闭扩展。
- App 挂载页面前获取当前请求的公开设置；失败停留在可重试边界，不把服务端注入/本地缓存当作请求级地区判断。原生初始化 `/setup` 保留；区域限制仍只限制页面，不新增网关/API 地区封锁。
- 新接纳迁移 `access-policy/202610020001_adopt_access_policy.sql` 保留旧安装默认启用、原生默认关闭和显式选择，不修改历史规则或冻结 SQL。

## Stage32：媒体新增请求准入与历史任务保留（2026-10-02）

`media-gateway` 提取端点注册、操作分类和新增请求准入，阶段清单为 **13 managed / 1 pending**。

- 接入 Grok 图片/视频/语音、Seedance、Gemini Veo，以及 Grok Responses HTTP/WebSocket 服务端 `image_generation` 工具。新增请求在占用用户/账号名额、调度账号或访问上游前检查开关；关闭/缺失拒绝，读取失败/非法值返回不可用。
- 原有分组图片权限优先执行；WebSocket 首帧和后续生成轮次各有准入入口。已经准入的轮次不因开关变化被重复撤销。客户端同名 function、被动图片命名空间、纯文本、图片理解和原生 OpenAI 图片接口不误判为该扩展新增任务。
- 历史状态、下载、取消和计费结算不依赖实时开关或开关服务可用性；原有 API key 归属、鉴权和错误处理继续执行。关闭不是抹掉任务或向其他 key 开放下载。
- 独立接纳迁移 `media-gateway/202610020002_adopt_media_gateway.sql` 只处理模块开关，不改历史媒体数据。媒体传输、账号选择、任务归属和结算仍是宿主共享责任。

## Stage33：订阅发放归属与冻结履约策略（2026-10-02）

`subscription-extensions` 完成发放策略、不可变归属和支付/兑换接入，当前源码清单为 **14 managed / 0 pending**。

- 开启按 `independent-v1` 独立发放；关闭/缺失按 `upstream-v1` 原生归属单链路发放，管理分配保持幂等、购买/兑换续期。旧独立订阅不会被选择成原生续期目标。
- 新订单把策略写入支付快照，新兑换码持久化策略，批量生成只读取一次；支付回调、重试和实际兑换按创建时策略履约，不查询实时开关。历史无策略数据保留独立权益，非法显式策略拒绝而非猜测。
- 原生首次发放/续期在同一 Ent 事务内先锁用户行，再查询原生归属记录；不恢复会破坏历史重复订阅的全局唯一约束。支付/兑换复用外层事务，缓存失效在提交后处理。
- 新增归属字段由扩展 Ent mixin 提供，Wire 和相关 Ent 生成代码同步更新。迁移 `subscription-extensions/202610020003_subscription_ownership.sql` 保留历史多订阅/未兑换码，新增非唯一查找索引并接纳开关；账本失败回滚覆盖字段与开关。
- 历史订阅查询、到期/额度/扣费与删除缓存，以及仅供品牌首页展示的销量字段仍为宿主或既有模块兼容边界，不以实时发放开关改写历史权益或共享展示配置。

### Stage31–33 最终验证记录与未执行项

- 最终代码普通 Go 全量 `go test ./...` 与 unit 全量 `go test -tags=unit ./...` 均通过：`.tmp/final-plugin-go-verified.log`、`.tmp/final-plugin-unit-verified.log`。前次运行的媒体下载测试固定请求次数假设和图片权限校验顺序失败保留在原日志；分别按实际请求行为修正测试、恢复权限优先顺序后重新跑完全量，未跳过失败用例。
- 前端全量 **353 个文件 / 2549 项测试**、lint、vue-tsc 和生产构建通过：`.tmp/final-plugin-front.log`、`.tmp/final-plugin-lint.log`、`.tmp/final-plugin-types.log`、`.tmp/final-plugin-build.log`。构建仅保留既有 Browserslist 数据过期和大 chunk 提示，未安装/升级依赖。
- 所有权、宿主接入契约、反向依赖和库存台账门禁 `node tools/customization-verify.mjs --check`、`git diff --check` 通过：`.tmp/final-plugin-check-verified.log`、`.tmp/final-plugin-diff-check.log`。改造前 **814 个 files 条目逐项与 HEAD 相同**；只增加提取映射/必要所有权，没有修改任何已跟踪的历史 SQL。
- 新测试包含真实处理器的 HTTP/WS 首帧拒绝、名额/调度前拦截、下载归属、订单/兑换码冻结策略和实际履约、仓储锁顺序，以及 SQLite 迁移回滚。后续 WS 轮次与取消入口有源码/分类器契约，但未单独完成真实上游连续会话及取消的端到端验收。
- **未部署、未重启、未提交、未写真实数据库**；未声称真实 PostgreSQL 并发/升级回退、生产支付回调或运行中浏览器验收通过。运行中的旧进程可能仍返回旧数量；需审阅迁移并在获准的发布流程更新后端/前端后，运行页才会使用新 catalog。
- 14/0 表示清单内真实开关边界完成，不是所有共享宿主逻辑物理搬空、任意 ZIP 热卸载、生产验收完成或未来上游零冲突的承诺。
