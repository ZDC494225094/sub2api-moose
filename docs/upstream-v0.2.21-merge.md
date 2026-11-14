# 上游 v0.2.21 源码合并记录

## 范围与回退

- 日期：2026-11-14；分支：`codex/my`。
- 合并前提交：`04fac6d2fb4b18d5a238e54263857e18512d14e0`。
- 回退分支：`codex/backup-before-v0.2.21-20261114`，保留合并前提交。
- 上游：[Wei-Shaw/sub2api v0.2.21](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.21)，对应源码 commit `10ba8f9b4dc2e6023d4a9879ab77b21841babd10`。
- 共同基线：v0.2.18 源码 `ac1b8f1271be0d674fa6f074403c1410f30a3a81`。完整合入 265 个提交、759 个变动路径，包含 v0.2.19、v0.2.20 的全部历史，不追加发布之后的 main 提交。
- 合并使用解引用后的 commit，保留双父节点历史。发布源码内 VERSION 仍为 0.2.20，本地显示版本更新为 0.2.21。

## 功能取舍与冲突决策

30 个文件发生文本冲突，另审阅 175 个上游和二开交叉路径；重叠功能由上游实现，独立二开通过原接入点保留。

| 范围 | 决策 |
| --- | --- |
| Ent schema / 生成文件 | 保留本地多分组密钥、上游分组、视频价格等字段，纳入上游 `image_count_billing` 与 30m/12h token 字段。根据合并声明在临时目录生成，最终仅替换 `migrate/schema.go`、`mutation.go`、`runtime/runtime.go`，修正字段下标、容量和 runtime 描述绑定。 |
| Ent 的历史孤立声明 | 本地 `user_balance_snapshot.go` 只有 schema，现有余额快照用原生 SQL 实现且没有 ORM 接线。临时生成输入排除此历史孤立 schema，源码原样保留，不额外引入无关 ORM 模型。临时 schema 的 import 在输出中还原为正式 `ent/schema`，不保留临时目录依赖。 |
| Wire | 按合并后的 provider 与 cleanup 重新生成，接入 MCP OAuth、受限管理密钥仓储、通知 worker/会话缓存清理，同时保留开票、日志归档、机器人、手册、视频、画布等本地接线和生命周期。 |
| 公开设置 / 本地化 | MCP 开启状态完整贯穿 service、DTO、公开注入和菜单；本地品牌/页脚/客服/访问策略保留。中英文新增 `adminAccessKeys`、`mcp` 加入 base 后再与本地 custom 合并。 |
| 注册安全 | 上游精确邮箱黑名单与别名匹配保留；已有访问策略的严格配置读取和原子域名额度继续执行。上游新增调用需要的两个邮箱解析/校验 helper 通过原模块端口转发，不复制两套规则。 |
| 管理订阅响应 | 保留本地幂等发放/批量发放和订阅归属；用上游 `executeAdminIdempotentJSONWithResponse` 在首次执行和缓存重放之后统一做权限裁剪，不把完整用户、分组定价或无权读取的用量放进响应。新增真实 handler 回归检查重放不重复发放权益。 |
| 账号诊断 / probe | 上游 Grok 模型目录与 metadata 在行锁内取最新数据库值；本地探测 interval/auto-sync 保留。查询和 SQL mock 统一为 16 列，保留旧表单、乱序/失败观察不能覆盖新目录的回归。 |
| API Key 缓存 | 使用上游 v27 schema 版本、分组隐私要求、按张计费和 RPM override 分组标记，同时保留多分组路由归属、billing priority 与平台信息。 |
| Codex 模型列表 | 上游已实现 singleflight 内重新读缓存，删除等价的本地重复代码，保留原有并发回归。 |
| 图片 / 视频 | 完整接入专用图片端点标记、实际张数计费、1K/2K/4K 账号分流、GPT Image 2.6 原生参数及超时/内存约束；本地视频时长和模型级视频价格继续工作。模型广场采用上游固定图片价优先规则，同时保留独立视频展示。 |
| 图片部分成功 | 保留本地部分出图错误结果，补齐新上游 `ImageEndpoint: true` 标记，避免专用端点在该返回分支失去原生按张计费身份。 |
| Antigravity 多模态 | 上游并行工具结果、functionResponse 媒体与 Gemini 转换保留；本地按文件头修正 MIME 的代码与回归保留。旧回归调用补齐新增 `multimodalToolResults` 参数。 |
| 前端账号列表 / 表单 | 保留本地上游分组展示与排序；采用上游刷新保留数据和新增 Grok 诊断、尺寸档位、配额 UI。MCP 菜单与本地开票、日志归档、体验中心、手册菜单并存。 |
| 前端 Key 回归 | 上游异步用量加载与本地多分组权限回归全部保留，未用任一侧测试覆盖另一侧。 |
| `.gitignore` / 资源 | 保留本地文档例外及上游新增文档规则；接受上游删除赞助商 `apimart.jpg`。 |

### 自动合并没有标出的功能重叠

上游邮箱黑名单功能新增原生批量禁用用户接口，与本地同名 handler、URL、API facade 重叠：

- 删除本地重复的批量禁用 handler、第二次路由注册和插件-owned HTTP facade，完整使用上游校验、人数上限、普通用户筛选和缓存失效语义。
- 原生批量禁用不受 `admin-efficiency` 开关影响；用户管理页按钮、调用和结果按上游 `{affected}` 契约处理。
- 批量删除用户、上游分组维护、账号排序和分组成员替换继续属于二开准入；相应失败详情和关闭后拒绝规则继续保留。
- 同步模块目录与 `CUSTOM_EXTENSIONS.md`，更新回归为“关闭插件保留原生禁用、拒绝二开删除”，不保留冲突的两套禁用实现。

合并前原生 `HomeView.vue` 因本地主题接入已不满足既有 native-equivalent 守卫。本次按用户要求恢复为上游源码，保留独立品牌首页、文档中心及其 Glass 样式，不放宽 native-equivalent 契约。

## 上游关键更新

- MCP：OAuth/PKCE 授权、用户/管理员授权范围、连接与授权页面、撤销授权和真实宿主权限/审计；只有显式 `MCP_PUBLIC_URL` 才启用，默认关闭。
- 管理 API Key：独立不可逆摘要存储、按操作/账号平台/类型/分组授权、IP 与有效期限制、全权兼容模式和敏感响应裁剪。
- 生图：OpenAI/Composite 可选实际出图张数计费（默认关闭、只影响专用图片端点），1K/2K/4K 分流，GPT Image 2.6 原生链路，超大 multipart 明确拒绝，异步任务请求体预算归属修复。
- Codex：原生图片请求携带 `X-Codex-Image-Turn-Id` 时等待最终 HTTP 状态，避免心跳提前提交 200 掩盖错误；页面上下文规范化、元数据不提前提交、sealed function args/namespace、稳定会话种子、WebSocket 重试与搜索能力等修复纳入。
- 继续保留客户端 `image_gen.imagegen` 与附带 hosted `image_generation` 的差别，只在确有 hosted 工具时添加其桥接指令和默认 tool_choice。
- OpenAI 30m/12h 缓存写入 token 分桶和 12h 独立价卡贯穿存储、计费、管理员详情；已配置通用写入价卡保留原语义。
- Claude Fable/Mythos 5.2、GPT-6.1 Terra、模型目录/effort/定价更新，DeepSeek Pro 与 Sonnet 缓存读取价格修正。
- 请求体预算、上游/下游读写超时、SSE 字节队列和行/事件上限、会话缓存/后台通知 worker 生命周期、刷新 token 原子消费等稳定性更新全部纳入。
- 易支付实例可配置退款成功码，余额充值倍率最多 6 位小数；本地 BEpusdt、历史支付实例快照、营销/订阅履约与退款边界保留。
- 前端采用上游 vue-i18n 11.4.16 及锁定依赖，保留独立主题、jszip、three 和画布接入。

## 数据库升级边界

原样新增五个上游 SQL 文件：

1. `247_scoped_admin_api_keys.sql`
2. `248_group_image_count_billing.sql`
3. `249_add_usage_logs_api_key_cost_index_notx.sql`
4. `249_mcp_oauth.sql`
5. `250_openai_cache_write_ttl.sql`

Runner 按完整文件名记账，同前缀的两个 249 均保留。CONCURRENTLY 用量索引的无事务执行/无效索引重试准备采用上游代码。既有 340 个 SQL 文件未改动或删除。本次不运行业务数据库迁移；首次部署大表索引可能耗时，须备份后在数据库副本或预发布验证。

## 完整性证据

- 上游 4649 个文件缺失 0；本地独有新增文件缺失 0；上游主动资源删除保留。
- 759 个上游变动路径均进入合并审阅。与上游增删序列不同的路径逐项登记原因，包括生成文件下标、版本标识、权限投影、原生功能替换、附加回归、字段排版及独立二开接入，不将这些差异隐瞒为字节相同。
- 最终有 45 个变动路径的增删序列不能逐字等同上游，全部有精确路径决策；另一个未发生上游变动的堆内存测试适配也登记在内，共 46 条决策。其中 9 个“仅上游变动”的差异全部为测试桩、额外回归或测量夹具适配，无未审阅项。
- 仅上游变动而需要适配的路径均为测试文件，按本地 handler/repository 接口补齐桩或增加回归；生产实现保留上游更新。
- 结构证据：`.tmp/merge-v0.2.21-integrity.json`、`.tmp/v021-delta-decisions.json` 和 `.tmp/v021-review-*.patch`。字节/路径清单不能替代业务回归或部署验收。

## 验证记录

最终 `node tools/customization-verify.mjs --full --upstream upstream/v0.2.21` 于 2026-11-14 20:21（Asia/Shanghai）完整通过 7 个阶段：

| 检查 | 最终结果 |
| --- | --- |
| 守卫与结构审计 | 34 个守卫通过；unclassified、ownership violations、missing host seams、contract violations、missing inventory paths 全部为 0。 |
| 普通 Go 全量 | `go test -mod=readonly ./...` 通过，约 234 秒。 |
| unit Go 全量 | `go test -mod=readonly -tags=unit ./...` 通过，约 334 秒。 |
| 前端 lint | 全部通过，约 33 秒。 |
| 前端组件 / API / 路由 | 455 文件、3540 测试全部通过，约 108 秒。 |
| 前端类型 | `vue-tsc -b` 通过，约 25 秒。 |
| 专项重复回归 | Codex namespace/多分组、生图状态/尺寸/按张计费、缓存 TTL、受限订阅重放、黑名单、BEpusdt 两轮共 394 个含子用例结果通过；其中 repository 初次过滤未命中，不将空包计作覆盖。 |
| Grok / 禁用补充专项 | 按精确名称补跑 Grok CAS、16 列 probe 合并与原生禁用/缓存失效，两轮共 44 个含子用例结果通过。 |
| 最终修改后的专项 | TTFT 请求同步、两项内存隔离、Codex/原生图片/按张计费三轮共 375 个含子用例结果通过。 |
| Linux cgroup | 交叉编译后在 WSL Ubuntu 执行，9 个子用例全部通过，包括符号链接逃逸。 |
| 生产构建 | 主站前端构建通过；最终后端更新后重新运行 embed web 测试与 `go build -tags=embed -trimpath` 通过。可执行文件版本为 0.2.21。 |
| 画布资源 | 未修改/重建画布源码；复制已有 25 个 dist 文件并逐文件核对 SHA-256 一致。 |
| Git whitespace | 除上游 SQL 自带 EOF 空行外其余路径全部通过，迁移保持上游原字节。 |

最终二进制：`.tmp/v021-release/sub2api.exe`，SHA-256 `B03433FA7D18986DA4C1AB8D5E3B9FC1FF48FF92DD8C4D82310B285E98EE1935`。版本输出的发行 commit/date 未注入，仍为 unknown。工具链为 Go 1.27.3、Node 24.22.1、pnpm 10.12.1；使用上游 go.mod 和锁定依赖。完整结果保存在 `.tmp/v021-verification-full-complete-result.json`，构建与专项均有独立任务结果，不依赖会被下一次审计覆盖的共享临时 JSON。

以下中间失败日志保留，不用通过的重跑覆盖原始失败：

- 合并前审计：原生 HomeView 差异；恢复上游后契约通过。
- 初次普通测试：黑名单调用的 helper 已迁至二开模块，补齐 facade 后通过编译。
- 首次 unit：新增测试桩缺本地接口、MIME 回归参数不齐、MCP 桩缺 GetMultiple、用量桩缺 ListAdminWithFilters；逐项适配，无需削弱生产权限。
- Windows 符号链接需要权限：仅该环境子断言跳过，路径穿越断言仍执行；另交叉编译 Linux 测试并在 WSL 执行全部 cgroup 用例，包括符号链接逃逸，全部通过。
- 一次并发全量的 thinking 分配断言记录 23 次而阈值为 16；原生产源码与上游一致，单独连续三轮通过，未放宽阈值或删除断言。
- 后续 unit 全量的未定价模型内存用例记录 8,543,016 字节，而阈值为 8,388,608；原用例独立连续三轮通过。两个用例均统计整个进程，采用上游 orphan-precheck 已有的有超时子进程模式隔离其他 service worker，原始 16 次分配 / 8 MiB 阈值、随机 20,000 模型、零直接 DB 查询和快照读取断言全部保留。生产内存策略与代码不变。
- WebSocket 首 token 测试第二轮出现 499 ms / 最低 500 ms：原夹具从上一轮完成后就开始下一轮固定延时，未等新 `response.create` 到达。改为等待真实转发请求再回放事件，保留 semantic/visible 两种模式、两轮及原时间上下界；不修改生产 TTFT 语义。
- Grok 受管额外字段测试 mock 仍是 14 列，按正式查询补成 16 列后重跑。
- 前端初次全量 453 文件通过，剩余 MCP 路由测试没有 Pinia 和旧插件 facade 身份断言不再适用；补齐测试上下文、调整为上游原生接口边界后，相关五文件 24 测试通过。
- 前端生产构建、嵌入静态资源测试和后端 embed 构建通过；二进制 `-version` 显示 0.2.21。未注入发行 commit/date，仍显示 unknown。
- 遵循画布目录约束，不修改或重建画布源码；主站构建后复制已有 25 个画布 dist 文件，SHA-256 逐文件一致。该结果不是画布重新构建证明。
- 上游 SQL `250_openai_cache_write_ttl.sql` 自带一个 EOF 空行；保持迁移字节与上游一致，单独记录 diff-check 提示，不改发布迁移消除提示。

## 运行验收限制

本次完成本地源码合并、回归与构建，不 push、不部署、不重启业务服务，不修改 Codex/Magpie 配置，不连接或修改业务数据库。未进行实际收费生图、真实支付、生产 MCP 授权或浏览器端到端验收。本地模拟/SQLite 和编译结果不能替代 PostgreSQL 历史迁移、历史权益、外部网关或真实客户端验收。
