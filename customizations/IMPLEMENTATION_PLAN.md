# Sub2API 二次开发插件化实施方案

> 当前状态更新：2026-10-02；依据当前工作区源码，而不是假定上游存在某个插件 API。
> 状态：实施中。本文是目标、边界和验收标准，不是“全部已完成”的声明。

## 一、把需求转换成可验收的工程目标

**保留上游 Sub2API 作为默认宿主，把本地业务能力拆分为拥有独立归属、可编译注册、可运行时开关的内置扩展。**

1. 未启用的新增能力不注册可用入口、不参与新业务决策；替换型能力未启用时执行宿主的上游策略。
2. 开启后，通过稳定的类型化接口、路由注册和 UI 插槽接入本地实现，不能覆盖整份上游文件。
3. 关闭是“停止承接新的扩展业务”，不是卸载：历史订单、资金、权益、券核销、退款、异步任务仍按创建时的契约处理。
4. 插件代码与宿主接口分离，新增插件主要修改扩展自身目录；宿主仅保留少量明确、受契约测试保护的接入点。
5. 上游更新后，常规路径是“合并更新 → 一条验证/打包命令 → 发布”；接口不兼容时必须在构建或测试阶段明确拦截，不能静默丢失本地功能。
6. “不遗漏、不冲突、不报错”以固定清单、边界契约、测试矩阵和实际构建证据验收，不能等同于对任意未来上游版本的无限保证。

### Git 层面的必要约束

`git pull` 是获取并合并提交，不是插件兼容性协议。只要上游没有主动维护宿主接入点，上游仍可能修改相同位置。因此不能靠把文件放进 plugins 目录就承诺以后永远无需适配。

本项目采用**最小宿主补丁 + 独立扩展实现 + 升级门禁**。不使用全量文件覆盖、`ours/theirs` 批量解决冲突、复制旧版上游页面、编译后文本替换等会隐藏丢失或冻结上游行为的方案。未全部完成下面的阶段前，不能宣布“拉取后即可放心部署”。

## 二、已核对的代码事实

- 已合并上游基线：v0.2.7，`aea725f2ea644d5592d0bbb1d63b607efa7e200a`。
- 改造前本地快照：`d754fe1dc`；与基线有 814 个差异路径。数字包含测试、资源、构建和生成文件，不能直接解释为 814 项功能。
- 后端是 Go/Wire/Ent；前端为 Vue/Vite；另有独立画布前端。已有 `/admin/plugins` 属于上游 OAuth 传输插件，不能挪用成业务开关。
- 数据库迁移按**文件名和校验和**记录；旧迁移已经可能在生产执行，不能为了目录整洁改名或重写。
- 多订阅、API Key 多分组、媒体计费等二开仍贯穿共享实体和核心服务，不能只隐藏菜单后就声称关闭。
- 当前清单十一项模块可管理，另有三组明确处于待隔离状态（2026-10-02 核对）。待隔离项返回 `enabled: null`，不会伪装成已关闭。
- 尚未对当前改造进行真实数据库、真实支付供应商或生产部署验证；当前缓存的上游 ref 不等于刚获取的最新上游。

## 三、目标架构

```text
上游宿主（鉴权、限流、标准路由、标准交易/计费、生命周期）
 ├─ 组合根：extensionwiring.ProviderSet
 ├─ 路由宿主：registerCustom*Routes（沿用上游安全中间件）
 ├─ UI 宿主：ExtensionSlot（不直接引用某个插件组件）
 ├─ 业务端口：类型化策略、只读查询、交易扩展生命周期
 └─ 扩展管理：独立持久化开关、公开只读状态、管理员写入
        ↓
内置扩展（独立清单 / 页面 / 服务 / 仓储 / 测试 / 迁移归属）
        ↓
固定差异清单 + 宿主边界契约 + 开/关/故障/历史数据测试 + 构建门禁
```

选择编译期注册、运行时开关，而非在线上传任意代码或进程内动态 `.so`：现有服务依赖注入、实体和发布方式需要类型检查与整体版本一致性。开关不等于沙箱、不降低代码信任，也不停止已经初始化的后台对象。

### 后端规则

- 上游 Handler/Service 不再随插件增加字段和方法；独立 Handler 通过 `ExtensionHandlers` 暴露给扩展路由。
- 插件查询通过最小仓储端口；运营分析已使用独立 `OperationsRepository`，不再依附 `DashboardService` / `UsageLogRepository`。
- 充值活动目录已迁入 `customize/modules/rechargecampaigns`，用最小 Queryer/Repository 和返利可用性端口，不依赖共享 PaymentService 或 Ent schema；历史订单保持 JSON/摘要兼容，交易侧仍有桥接。
- Wire 只在扩展组合根注册插件；必须真实运行 Wire 生成，不手改 `wire_gen.go`。
- 门禁位于鉴权、限流、审计之后；管理员权限和原业务开关仍然有效。
- 普通上游请求不应依赖无关插件状态。单模块读取故障不得破坏其他模块；涉及价格或权限的可选请求不能在状态异常时悄悄按另一价格/权限执行。
- 交易扩展拆成 **准入/报价 → 预留 → 成交确认 → 取消释放 → 退款补偿**。前两步由实时开关控制，后几步由已存订单或预留记录控制。
- 安全策略保持独立审查和 fail-closed；不能用普通业务开关绕过既有访问控制。

### 前端规则

- 路由集中在 `frontend/src/extensions/routes.ts`；模块资产放入 `extensions/modules/<id>/` 或已独立的功能目录。
- 上游页面只引用宿主，不直接导入插件 API、组件、算法。UI 插槽选择组件；插件异常只隔离自身，不让上游整页崩溃。
- 交易页面通过类型化 checkout 扩展端口取得报价、限制和请求补充字段；不把活动计算和异步加载逻辑继续塞进 PaymentView。
- 默认渲染上游内容，扩展状态未知时不显示可选扩展。开关状态请求不得阻塞普通核心页面导航。
- 关闭时清理插件的临时选择、弹层、计时和在途读取；迟到结果不能在关闭后把插件重新显示出来。
- 扩展之间显式声明依赖与互斥，不依赖注册先后顺序决定支付价格或权限。

## 四、逐模块清单与关闭语义

| 模块 | 当前实施状态 | 关闭时的新业务 | 必须保留 / 仍待提取 |
| --- | --- | --- | --- |
| 品牌首页/文档 | 已接入路由开关；页面已有独立目录 | 回到上游 `/home`，保留查询参数 | 上游 HomeView 已恢复基线实现；站点设置不丢失 |
| 体验中心 | 路由门禁、独立处理器接入 | 不接受新任务，不显示入口 | 查询、取消、媒体下载和后台在途任务继续；媒体底层仍共享 |
| 无限画布 | Go 宿主页/配置/新建接口门禁 | 不接受新的宿主任务 | 旧任务生命周期继续；独立 CDN/Vite 画布壳仍需单独接状态 |
| 运营分析 | 已拆独立处理器/服务/只读仓储端口及前端模块 | 关闭分析和营销邮件入口 | 上游仪表盘不变；分析口径不改、已发送邮件不撤回 |
| 充值活动 | 独立目录模型/服务/仓储/处理器；前端统一 checkout 端口和四个 UI 插槽；新订单领域门禁 | 标准充值；明确旧活动参数拒绝，要求重新确认 | 旧订单活动快照、返佣、退款；活动快照交易接入继续拆分；共享支付不再承载配置 CRUD |
| 客服悬浮窗 | 独立组件 + 三处 UI 插槽/入口 | 移除悬浮窗及品牌首页联系按钮 | 联系配置、上游页面和原帮助链接保留 |
| 优惠券/抽奖 | 已接真实共享开关、独立迁移、前端报价与插槽；宿主保留事务适配 | 禁止新发券/抽奖/券预留及管理写入；普通支付不受影响 | 历史查询、核销/释放、履约重试及宿主支持的退款；Failed退款增强未实现 |
| 多分组密钥/计费优先级 | 已接纳（Stage27）：归属持久化、创建/更新准入、KeysView 模式 | 新密钥和新配置采用上游策略 | 既有多分组密钥按原归属继续；可保持或缩减，不能新增分组 |
| 计费与调度增强 | 当前清单已 managed；后端四项门禁，Stage28 补齐前端 ID | 见 catalog 的基础计费回退契约；Stage28 未重验全部关闭场景 | 已发生用量和账单不能随开关重算 |
| 订阅/兑换增强 | 待权益策略和数据兼容方案 | 新发放采用上游策略 | 历史同用户/分组多份订阅仍有效；不得重新加唯一性约束损害权益 |
| 视频/媒体网关 | 待适配器注册与任务端口 | 停止新增自定义协议任务 | 查询、回调、结算、下载和退款；与上游已有协议重叠时优先上游 |
| 管理效率增强 | 已管理：独立规则/API/UI，宿主窄端口适配 | 拒绝显示排序、上游分组、成员替换及批量用户写入 | 历史排序/标签/成员可读；不封锁原生 CRUD 或原生分组排序 |
| 站点定制 | 当前清单已 managed；配置过滤与公告创建准入 | 隐藏自定义 UI 配置、停止创建公告 | 已发布公告继续可读；设置与站点名称保留 |
| 访问/注册策略 | Stage28 已提取前后端纯规则和凭证协议；仍待安全关闭契约及设置故障策略审查 | 不暴露业务开关，原安全设置继续生效 | 旧凭证兼容、事务守卫及页面级区域限制保留 |

这些分组不是最终插件粒度。共享文件会匹配多个候选归属；仍未归类的路径必须继续人工审计，不能用兜底标签宣称清点完毕。

## 五、开关默认值与部署接纳

最终要求：**全新安装未启用时默认上游；已有部署升级时不得突然失去正在使用的业务。** 这需要一次性、幂等的老部署接纳机制，不能仅修改一个布尔默认值。

当前实现：缺失设置键默认关闭；已显式写入 true/false 则按其值处理；错误值或存储不可用均不按开启处理。一次性接纳通过独立迁移完成，不在普通读取时推断或补写开关。

已编码并有隔离测试覆盖的接纳契约：

1. 在执行本轮宿主 SQL 之前，以固定的历史二开迁移文件名识别已有 fork；持久化部署来源，宿主中途失败后重试不重新分类。
2. 只对接纳 v1 固定的六个已管理模块补齐缺失开关：已有 fork 为 true，新装/纯上游为 false；已有显式值（含错误值）永不覆盖。
3. 同宿主迁移锁、同一连接执行，六项接纳与独立账本登记同事务；失败回滚，重启重试不会覆盖管理员选择。
4. 接纳完成后不再补写；删除设置键即按缺失规则关闭。以后新增插件不扩大 v1 名单，默认关闭。
5. 已通过内存 SQL 事务回归和宿主生命周期 SQL mock 回归，涵盖新旧来源、失败重试、整批回滚、重复执行、校验和与已有设置保留。

**上线验收仍未完成：**须在数据库备份恢复出的 PostgreSQL 副本上验证老版本升级、新库安装、重复/并发启动和回滚；隔离内存测试不等于真实 PostgreSQL 演练。详见 `MIGRATIONS.md`。

## 六、数据库、生成文件与构建资产

- `138_marketing_lottery_coupon.sql`、`145_subscription_multi_instance_api_key_groups.sql`、`146_api_key_platform.sql`、`147_payment_order_subscription_id.sql`、`159_operations_marketing_email_records.sql`、`174/175/176/177` 账号迁移、`185_group_billing_rate_sync.sql`、`191_playground_video_asset_urls.sql`、`194_marketing_redeem_codes.sql`、`239_custom_api_key_platform_sync.sql`、`240_recharge_campaigns.sql` 均属于需保留的历史兼容链。
- 不改写/重命名这些已执行文件，不随关闭插件 drop 表/列，不让数据回退依赖重新开启插件。
- 新扩展迁移登记器已在 `backend/internal/customize/migrations/` 实现，按模块/日期命名并使用独立账本；普通启动与安装向导复用宿主锁的同一连接，支持事务失败回滚、重试和不可变校验和。隔离 SQL 与生命周期回归已通过，真实 PostgreSQL 演练未完成；历史编号碰撞和共享 Ent/schema 耦合仍须继续处理，不能据此声称数据层已全部解耦。
- Ent 生成差异不是可以手动丢弃的噪声：先隔离 schema 扩展/兼容投影，按统一工具生成并检查结果，不能机械覆盖上游生成目录。
- Wire、前端产物与后端 embed 形成同一版本的产物链。前端最终变更之后必须重建 embed，旧构建通过不证明当前源码通过。
- 发布自己的组合产物；直接使用纯上游镜像不会包含本地扩展。

## 七、不遗漏与升级门禁

### 固定账本

`inventory-baseline.json` 固定改造前 814 个差异路径及 SHA。迁移文件时追加来源→目标记录；提取共享代码时记录目标实现。不能不断重建基线来掩盖删除。检查只能证明路径存在，业务语义仍靠测试与人工核对。

### 边界契约

`contracts.json` 声明：

- 必須与所选上游一致的文件（当前为 HomeView）；
- 必须保留的独立实现文件、宿主接入点；
- 上游共享文件中禁止重新引入的插件依赖。

审计区分精确人工记录（reviewed）、规则候选（rule）、未归类（unclassified），保留多责任归属和全部规则候选，并输出上游同文件变更、边界违规、丢失路径。精确记录必须包含合法相对路径、已知所有者、来源证据、后续动作并绑定固定基线；不允许兜底归入运行产物。未归类或无效记录会阻止检查通过。退出码 2 表示需要审查，不等于已经证明 Git 文本冲突；退出码 0 也不等于端到端通过。

### 测试矩阵

每个可切换模块都应覆盖：开启、关闭、状态不可用、权限不足、关闭时有在途请求、旧数据继续处理、再次开启恢复。交易模块另需并发/重复通知/退款幂等与报价一致性；计费模块需金额对账；安全模块需授权负例。

本地快速门禁：Node 契约测试 + 基线审计 + 针对变更的 Go/Vitest 测试。
发布门禁：上述全部 + 全量 Go/带 unit 标签测试、前端全量测试/类型/静态检查/生产构建、最终 embed 构建，及隔离测试环境中的真实数据库和业务回归。

独立 CI workflow（`.github/workflows/custom-extensions.yml`）和本地校验入口（`tools/customization-verify.mjs`，check/full/build）已放在扩展拥有的文件，不修改上游 CI。新增文件不等于远程 CI 已运行通过。若使用浅克隆且缺失固定 SHA，必须明确失败提示，不能跳过账本。

## 八、实施顺序与完成标准

| 阶段 | 交付 | 完成条件 |
| --- | --- | --- |
| A 清点与宿主 | 固定清单、管理页、路由/状态 API、基本开关 | 已部分实现；新装/接纳已编码并回归，70 个原未归类路径已记录；继续审查规则候选及 PostgreSQL 演练 |
| B 展示/只读模块 | 首页、客服、运营、活动展示、画布入口 | 原生页面无插件实现泄漏，开关与 UI 故障隔离可测 |
| C 交易扩展 | 活动/券/抽奖的报价与生命周期端口 | 关闭停止新业务，旧订单回调/取消/退款可证明 |
| D 核心策略 | 密钥计费、订阅权益、媒体、访问策略 | 开启/关闭/历史兼容各有策略实现和回归；不能一刀切 |
| E 数据与构建 | 新迁移命名空间、生成链、独立验证/发布入口 | 新装/升级/回滚演练可重复 |
| F 上游演练 | 在隔离检出上合并一个明确 SHA | 账本无遗漏、无未解决冲突、测试/构建/真实回归全部通过 |

每完成一项更新模块状态和证据，不提前把待隔离项变成开关。**当前具备部分 A/B/C/E，不具备宣布全目标完成的条件。**

## 九、标准升级与回退流程

1. 保存当前工作，记录部署 SHA，创建新的回退分支；涉及数据时先备份并验证恢复。
2. 核对 remote 后获取明确上游 ref；审计该 ref，不把旧缓存称为最新。
3. 在隔离的检出/测试环境合并，不在运行目录覆盖文件；重叠功能优先上游，本地独有能力按账本重新接入。
4. 运行门禁与真实业务回归。失败则不发布，列明需要适配的端口；不能为了通过而删除契约或历史清单。
5. 通过后记录已合并 SHA，产出前后端一致的版本，再部署。
6. 行为回退优先调整明确的模块开关；代码回退使用已验证组合产物。数据库使用前向兼容或经过验证的恢复，不直接删除扩展业务表。

## 十、本阶段验证记录的边界

执行日志及当前源码证据在仓库根目录 `CUSTOM_EXTENSIONS.md` 中分阶段记录。旧测试通过不覆盖后续改动，模拟浏览器不是生产支付证明，结构审计不是完整业务验证。尚未执行的操作必须继续标为未验证。

## 2026-09-26 补充进展：默认关闭与独立迁移

新安装/纯上游实例默认关闭、历史 fork 六模块一次性接纳已实现；管理员已有选择不覆盖，未来模块缺失值关闭。新扩展迁移已使用模块命名空间和独立 checksum 账本；历史 SQL 仍原位保留。安装和正常启动已接共同生命周期，隔离 SQL/连接顺序回归通过。详见 `MIGRATIONS.md`。仍未完成 PostgreSQL 副本演练、七组核心模块隔离与实际上游合并验收。

## 2026-09-26 补充进展：归属门禁与交易边界

- 精确人工记录覆盖原先 70 个未匹配路径，不把其余规则候选当作人工已审核；保留原始 814 路径账本，新增归属/路径安全/产物排除门禁，总计 29 项 Node 回归。
- 充值活动的报价、快照、邀请资格和奖励冲回 SQL 已进入独立模块，不导入共享 service 或 Ent。原支付服务保留准入/原生金额适配、订单保存、事务提交与幂等判断；快照 wire shape 和计算精度保持不变。
- 关闭活动只停止新报价，旧订单处理不重查开关或活动配置；SQLmock 与隔离 SQLite 回归覆盖宿主重复退款拦截和失败回滚，仍需真实 PostgreSQL 证明锁和并发语义。
- 上一阶段完整 build 与前端跨标签/failure mock 浏览器验证已完成；本阶段代码变更后的全量结果记录在根文档中，不能复用旧二进制充当当前版本。
- 未移走历史迁移、未清理用户运行数据、未 fetch/merge/commit/push/部署。优惠券/抽奖、其余七组核心模块、Ent 生成隔离和实际升级演练仍待实施。


## 2026-09-26 Stage11：营销前端路由和传输隔离

三个营销页面及14个HTTP方法已移入 frontend/src/extensions/modules/marketing；原路由身份和权限不变，由统一扩展 routes 接入。支付页经既有 checkout 扩展端口加载券，保留原响应/失败语义，不在宿主直接导入模块实现。固定814路径不变，新增实际搬迁/提取记录及防回流门禁。支付内联券UI、选择/折扣计算、订单参数、关闭准入/历史权益、供应商失败与退款策略仍待推进，不把这次提取当作交易模块完成。验证证据见根文档Stage11。


## 2026-09-26 Stage12–13：营销结算边界

已提取支付券UI、规则和请求参数，补齐手续费/换汇前门槛及折扣、每支付币种报价与限额一致性。真实模块组件到宿主请求的集成回归和前端全量337文件2458项通过。Stage11所列支付内联逻辑待办已由本阶段完成；关闭准入/历史权益、供应商失败与退款、其他核心模块及实际升级验收仍保留原范围，不把结算提取视为整个营销开关完成。最新构建与证据见根文档。

## 2026-09-26 Stage14：供应商创建响应不确定性

已去除CreateOrder供应商错误分支的无条件Failed更新。错误只记不含原始供应商消息的审计，保留当前订单状态和券占用，让Pending继续进入现有查单/回调/过期链路，避免覆盖已支付状态或把未确认支付误送入Failed履约重试。九类状态、审计失败及响应丢失后真实查单/履约回归重复10轮通过。退款、关闭准入、历史权益和其他待迁移模块仍保留在目标范围内；验证结果见根文档。


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

## Stage21：关闭后历史页面与异常退款安全边界（2026-09-26）

新增 modules/marketing/route-access.ts 声明三个已实现只读准入的历史页面；共享 route-access.ts 按模块所有权与精确路径判定，不用前缀或外部 query/meta 放行。runtime 导航守卫与开页监听、store.pathEnabled（侧栏既有接入点）共同保留历史入口。设置读取失败/挂起不阻塞历史导航，但营销新增业务准入仍 fail closed；原宿主认证、管理员、支付功能限制不改变。其他插件关闭后仍按原策略隐藏/重定向。营销生产目录仍 managed=false，新增测试用目录替身验证将来接纳后的行为，不代表本次已正式开放营销开关。

historical-access.spec.ts 覆盖精确路径与越界子路径、原认证元数据、读取失败/挂起、侧栏可见性、共享状态从启用到读取失败后不驱离当前历史页，并核对新增业务准入已关闭。已有页面/服务端写保护继续作为写入权威，历史访问白名单不是写权限。

payment_marketing_refund_safety_test.go 固化当前宿主限制：余额/订阅 × 普通/force 的已支付Failed订单在 PrepareRefund 和绕过预检的 ExecuteRefund 中均拒绝，预留券/折扣/支付事实不改变；六类退款状态同时拒绝 RetryFulfillment 与持有旧Failed快照的租约申请。10组专项重复10轮通过；第一轮夹具误写locked_order_id，已按真实138迁移修正为reserved_order_id和reserved状态，最终日志另存。测试采用SQLite顺序交错，不冒充PG并发。没有放宽Failed退款、没有修改资金或返券政策。

进一步源码确认不能按缺失成功审计推断“未发放”：余额兑换发生在最终RECHARGE_SUCCESS之前；订阅分配及关联/审计事务提交后，缓存失效仍可能报错并进入Failed。未来异常退款需核对订单绑定的兑换事实/订阅分配事实，区分已发、未发、不确定，且同时保护PrepareRefund、ExecuteRefund、异步退款重建计划与履约租约竞争。未判明权益的订单继续拒绝自动退款，不误扣其他余额或订阅。

本阶段仍未完成独立营销接纳迁移、异常PaidAt+Failed退款策略、真实PG/网关/浏览器验收、其他pending模块、Ent隔离和实际上游升级演练；不缩小整体目标。未拉取上游、未改历史SQL、未提交、未部署或写真实数据库。验证结果收尾后另附。

Stage21最终验证：前端全量341文件2483项通过（.tmp/custom-extensions-stage21-all-frontend-tests-final.log），最终vue-tsc及修改文件ESLint通过；新增退款安全10组及Stage20退款结算8组合计18组重复10轮通过（.tmp/custom-extensions-stage21-refund-safety-final.log）。29门禁及原始路径账本审计通过，git diff --check通过。前端生产构建成功，保留大包警告；复制未改动的Stage14画布产物后embed构建成功，.tmp/custom-extensions-stage21-server.exe未部署。本轮未跑全部Go包/全部service unit、未重建canvas、未进行浏览器/PG/实际支付验收；最近完整统一11步验证仍为Stage14。本阶段完成不等于整体插件化完成。

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

## Stage26：密钥策略归属持久化（2026-09-26）

新增模块 ownership.go、独立 Ent Routing mixin 和 202609260003_key_routing_ownership.sql；历史 SQL 未改。迁移把现有密钥标记 multigroup-v1，不能通过 installation 类型猜测历史密钥意图。字段不可普通更新；当前 pending 阶段创建仍默认 legacy，不开放全局开关。

仓储创建校验归属、鉴权精简查询保留字段；缓存版本25拒收旧24快照，快照保留归属而不按当前开关推断。显式 upstream-v1 密钥旁路自定义选组，保留原 Group/GroupID；未知非空归属失败，不静默回退。该旁路不等于新增密钥准入和两种协议完整网关验收。

迁移SQLite回归覆盖旧行回填、ledger失败DDL回滚/重试、native行重跑保留。新增仓储测试十轮通过，覆盖默认/显式归属、GetByID/GetByKeyForAuth、普通更新/软删除保留归属和未知策略不落库。软删除验证使用宿主 SkipSoftDelete 查询历史行，不修改生产删除语义。

Ent直接生成两次被Windows文件映射锁中断；修复生成文件imports后，在独立 .tmp/ent-stage26-generated 完整生成成功，并逐文件比较与backend/ent一致（0差异）。未改依赖解决锁问题。源码专项及统一check通过；完整相关unit结果以 .tmp/custom-extensions-stage26-unit.log 为准。

剩余工作：创建准入按开关持久化策略、更新输入按归属隔离、Keys UI端口、两种鉴权入口的标准模式专项场景以及真实PostgreSQL迁移演练。multi-group-billing仍pending；总状态7 managed / 6 pending。没有部署、重启或写真实数据库。
Stage26鉴权补充：新增 middleware 双入口路由归属回归，OpenAI与Google入口均覆盖历史归属（含空值）的停用主组回退、原生归属保持已加载分组、原生停用/缺失分组不回退，以及未知归属拒绝。断言最终分组上下文和候选仓储读取次数；专项重复十轮及 middleware 全包 unit 通过，统一 --check 和 git diff --check 通过。这只是简易模式的鉴权边界验证，标准模式、创建/更新准入、前端、真实PG迁移及实际网关验收仍未完成；multi-group-billing 保持 pending，不开放运行时开关。

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


## Stage28：访问/注册策略规则物理隔离（2026-10-01）

### 已落地边界

1. 独立 Go 模块 `accesspolicy`：注册凭证 v1 协议、邮箱白名单/主域归一化、别名去重、域名额度准入决策、区域页面访问判断。
2. 独立前端模块 `access-policy`：注册凭证 Worker/求解器、邮箱规则、区域路由规则；utils/router 只留兼容导出，原调用方无须整体重写。
3. 宿主接入：AuthService/SettingService 薄适配；邮箱预检和写入前复查均调用同一准入决策；真实数据库写入仍由 `CreateWithEmailAliasGuardAndDomainLimit` 原子守卫负责，不在插件另开事务。
4. 兼容：旧 JSON/HMAC/TTL/绑定/错误码保留；用冻结的旧签名算法验证历史令牌仍可接受，同时扩展开关 false/invalid 不能跳过验证。无迁移，不改 JWT 秘钥或用户数据。
5. 升级门禁：新增模块必需文件、禁止 Service/Ent/普通开关依赖、宿主接入标记和提取台账；补齐已有 site-customization 所有权。前后端 catalog 对齐 10 managed / 4 pending，旧测试数量与 pending 目标同步。

### 未完成与下一步

- access-policy **仍为 managed=false**。规则迁移不等同于安全模块可随意关闭；状态键缺失/false/错误均不得通过业务开关绕过验证。
- 既有设置读取错误可能回落为关闭或空策略的行为保持原样；下一阶段必须设计可审计的安全配置故障策略及关闭契约，再考虑纳入管理开关。
- 数据库并发防重、真实部署升级、浏览器端交互和线上流量未验收；不会把 Go/前端测试当作这些验收的替代。
- 另外三组 pending（订阅、媒体、管理效率）未在本轮改动，后续分别围绕历史权益、异步任务收尾、独立管理端口拆分。

### Stage28 验证记录

- 统一 `--full` 七步全部通过：门禁、所有权/台账审计、Go 全量、Go unit 全量、前端 lint、345 文件 2516 项测试、vue-tsc。日志 `.tmp/access-policy-full-retry.log`，固定结果 `.tmp/access-policy-full-result.json`。
- 新模块测试重复 10 轮通过，语句覆盖率 89.2%；注册/别名/区域访问/扩展状态专项通过。管理页实际 catalog 组件测试确认 10 个开关和 4 个无开关待解耦项。
- `pnpm run build` 通过，迁移后的注册凭证 Worker 正常独立打包；保留既有大包警告。日志 `.tmp/access-policy-build.log`。未重建画布或生成嵌入式服务端产物。
- 首次全量运行因 Go 缓存访问权限和 PATH 缺少 Git sh 失败，未跳过失败测试；经授权使用现有缓存、临时加入已安装的 Git bin 后重新完整运行并通过。未安装依赖、改全局 PATH 或修改测试来掩盖环境失败。
- 文档更新后再次执行 `--check` 和 `git diff --check`。未 fetch/merge/commit/push/部署/重启，未写真实数据库；浏览器与线上流量验收仍未完成。


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


## Stage30：管理效率剩余边界与真实运行时开关（2026-10-02）

### 完成范围

1. `adminefficiency` 新增窄设置准入端口、账号显示排序端口、分组成员替换编排和用户批量操作规则。原生鉴权、用户保护、平台/OAuth-only 校验、仓储原子写入仍由宿主负责。
2. 管理服务的创建/编辑/批量字段/影子继承，以及排序、目录和成员替换入口全部检查真实设置。关闭或未知拒绝二开写入；不变的历史字段和普通 CRUD 继续。
3. 分组账号弹窗迁入模块目录，宿主只提供账号列表/修改的窄端口；API 兼容导出保留原调用路径。账号/分组/用户页保持可进入，新增写控件和弹窗提交检查同一开关。
4. 用户批量删除使用批量后端准入，不再用原生单项删除循环绕过开关。新测覆盖关闭弹窗的延迟确认、历史列表、原生保存、部分失败和 API 封装。
5. 独立一次性接纳迁移保留旧 fork 默认开启/原生默认关闭与明确选择；单独事务及账本失败回滚测试保留原迁移排序，不更改冻结 SQL。
6. `managed=true`，清单 **11/3**。继续 pending：`subscription-extensions`、`media-gateway`、`access-policy`。Stage29 的 10/4 是此前子阶段记录。

### 未执行的运行态验收

未重启/部署、未应用迁移到真实数据库、未执行真实 PostgreSQL 并发/升级回退、未验证运行中浏览器。源码开关/迁移测试不等于这些检查已经完成；现有进程需运行新构建才会返回新 catalog。


### Stage30 最终验证记录

- `go test ./...` 全量通过：`.tmp/stage30-go-full.log`。`go test -tags=unit ./...` 在补齐影子继承/原生复制边界测试后再次全量通过：`.tmp/stage30-go-unit-final.log`。复用已安装的 Git sh，仅在测试进程临时设置 PATH。
- `adminefficiency` 独立模块重复 10 轮通过，语句覆盖率 100%；迁移和批量用户处理器专项通过：`.tmp/stage30-module-repeat.log`、`.tmp/stage30-boundary-final.log`。
- 前端全量 **350 个文件 / 2541 项测试**通过：`.tmp/stage30-frontend-tests-3.log`。包括真实 catalog 的 11/3 断言、未知/关闭状态、对已打开弹窗的写入撤销、原生账号创建/编辑和批量删除失败保留。
- 前端全量 lint、vue-tsc 通过；最终改动文件 lint 和 vue-tsc 再次通过：`.tmp/stage30-lint-final.log`、`.tmp/stage30-types-final.log`。Vite 生产构建通过：`.tmp/stage30-build.log`，仍有既有大 chunk / Browserslist 数据过期提示，未升级依赖。
- `node tools/customization-verify.mjs --check`、`git diff --check` 通过；契约、所有权和台账检查无新增未归类/违规/缺失路径。日志 `.tmp/stage30-check-final.log`。
- 以上为本地源码、测试与构建证据；未重启或部署，未应用迁移到真实数据库，未声称已完成线上/浏览器/真实事务验收。
