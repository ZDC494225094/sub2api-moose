# 上游 v0.2.14 合并记录

## 最终截止范围（优先于首批验证记录）

首批合并验证期间，上游继续集中合入修复。本次追加固定到第二次 fetch 的 `745ff1d3ca79a7f39520f627f6aa44d778a11580`（2026-10-08），不持续追逐随后可能变动的 main。相对首批目标追加 86 个 Git 提交、120 个文件；保留首批完整验证和正常 merge 历史。

追加范围包括：请求级输出 TPS 及运维百分位、长上下文计价标记、组合路由模型目录和远程 Codex 投影/ETag、WebSocket 分组计价及图片用量、Anthropic/Chat/Responses 协议兼容与心跳、Grok SSO/静默拒绝、OpenCode 路由/使用量退避、账号测试日志/模型/紧凑筛选、渠道探测路径、原生裸 API 静态路由旁路、支付离页处理及多处页面异步请求/输入草稿竞态。

- 第二次合并仅有两个测试文件文本冲突：模型目录缓存迟到请求回归与上游 ETag 回归都保留；画布开关回归与上游裸 API 路由回归都保留。
- 已审查 22 个上游/二开交叉路径。按共同基线比较改动区间，生产源码没有重叠改动块；实际重叠仅为上述两个测试文件末尾追加。对 120 个上游变动路径逐文件比较“基线→上游”与“本地首批→合并结果”的增删行序列，全部一致；不是只凭 Git 无冲突判断。
- 本地旧的 OpenCode Zen 账号创建测试缺少上游新增的 `qwen3.8-max → chat_completions` 规则，补齐一条精确断言；未回退或改写上游生产逻辑，GO 模式及其他 Qwen 路由保持各自原语义。

### 追加功能与二开交叉审查

| 范围 | 合并与审查结果 |
| --- | --- |
| 模型目录与账号模型测试 | 保留组合路由精确模型目录、Codex 模型投影及 ETag、Antigravity 配置映射请求名；保留本地画布模型平台信息及缓存迟到请求回归，不覆盖上游原生目录规则。 |
| WebSocket 与协议 | 上游逐 turn 分组计价/利润门、图片 usage、Anthropic/Chat/Responses 心跳和工具参数兼容原样纳入；WebSocket 生产文件与上游仅有既有空行差异。 |
| 探测和计费调度 | 保留上游智谱官方/中转路径判定和 Retry-After 24h 上限；保留独立二开探测错误标签、账号周期、倍率同步及扩展关闭语义，不绕过上游封顶。 |
| 静态前端与画布 | 采用上游全部裸 API 旁路路径，保留画布开关、独立入口和 CSP nonce 注入。两套回归均保留，并在最终嵌入资产生成后单独运行 embed 标签测试。 |
| 使用量/账号/自定义页面 | 输出 TPS 和长上下文提示使用上游逻辑；保留本地视频使用量和失败请求展示、账号分组/排序/批量动作、外链按钮拖动；采用上游紧凑筛选布局及 Markdown 请求版本防竞态。 |
| 翻译与其余更新 | 英中 TPS、长上下文、紧凑筛选及 OpenCode 403 文案全部纳入，独立 custom.ts 覆盖未遮蔽新增键；其余单侧上游路径逐字节一致。 |

### 本次整体完整性

- 从合并前共同上游基线到最终目标共 98 个 Git 提交、147 个变动路径（首批 27 + 追加 120）。最终树中 117 个单侧上游变动路径与上游逐字节一致；另外 30 个交叉路径保留独立二开差异。
- 上游目标跟踪文件缺失 0，本地独有跟踪文件缺失 0；367 个既有 SQL/迁移相关跟踪路径与合并前一致，包括迁移 runner 与 checksum 白名单。本次上游增量不包含 SQL 迁移。
- 额外变动只涉及 Codex discovery 与 OpenCode 默认路由回归断言、上游基线/归属/证据记录和本文；14 个 managed 模块清单及开关不变，没有扩大归属规则或重置历史清单。
- 完整性与增量证明：`.tmp/merge-v0.2.14-combined-integrity.json`、`.tmp/merge-v0.2.14-latest-integrity.json`、`.tmp/merge-v0.2.14-latest-delta-proof.json`。文件/增量检查不替代真实环境功能验收。

### 最终追加验证

- 最终全量 `customization-verify --build` 于 2026-10-08 12:41:03（UTC+08:00）完成，11 个步骤全部通过：34 项边界守卫、二开审计、Go 普通/unit 测试、前端 lint/测试/类型/生产构建、画布构建与嵌入、后端 embed 构建。Node.js 24.19.0；后端按 go.mod 自动使用 Go 1.27.0，没有改动工具链要求。
- 前端 406 个测试文件、3027 项测试全部通过。账号创建/UseKeyModal/紧凑筛选定向 3 文件、73 测试通过；CustomPageView 竞态与使用量表定向 3 文件、46 测试通过。
- 二开审计：未归属 0、归属违规 0、宿主接缝缺失 0、契约违规 0、历史清单缺失 0；14 个 managed 模块及其开关不变。
- 模型目录缓存/ETag、WS 分组计价与利润门/换组/刷新失败、探测 Retry-After 封顶/智谱路径及二开开关与倍率等 15 个顶层后端回归连续 20 次通过。记录在 `.tmp/merge-v0.2.14-latest-repeat-result.json` 及对应 service/handler 日志。
- 最终前端及画布资产生成后，`go test -mod=readonly -tags=embed ./internal/web -count=1` 通过；画布开关与裸 API 静态路由旁路两个冲突相关回归另行连续 20 次通过。新构建二进制仅执行 `-version`，确认 `Sub2API 0.2.14`；该分支在安装/配置/应用初始化前返回，不启动服务或迁移。构建未注入发布 commit/date，输出中的两项元数据为 unknown，不作为发行包。记录在 `.tmp/merge-v0.2.14-latest-embed-verification.log`。
- 4 份 Compose 文件的 `config --no-interpolate --quiet` 以及 `bash -n deploy/docker-deploy.sh` 通过；只做静态验证，没有启动容器或执行部署。仍有 Browserslist/分块体积等非阻断警告。
- 追加首轮全量中，Go 普通/unit 与 lint 通过，前端 405 文件通过、1 文件失败（3026/3027 测试通过）：旧账号创建断言缺少上游新增 `qwen3.8-max → chat_completions` 默认规则。仅补齐测试断言，未回退上游生产逻辑。失败日志 `.tmp/merge-v0.2.14-latest-full.log` 和结果 `.tmp/merge-v0.2.14-latest-result.json` 保留。
- 下一轮全量曾在上游未改动的 `TestInflightEstimate_AccountMappingNoDBAndBoundedMemory` 内存断言偶发失败：进程级 HeapAlloc 增量 8,605,192 字节，略超 8,388,608 字节上限。未修改生产代码、测试代码或阈值；同一测试独立连续 50 次通过，随后完整 11 步复跑通过。失败日志 `.tmp/merge-v0.2.14-latest-final.log`、结果 `.tmp/merge-v0.2.14-latest-final-result.json` 与诊断 `.tmp/merge-v0.2.14-memory-diagnostic.log` 保留，不把失败日志名称中的 final 当作成功状态。
- 最终成功全量日志为 `.tmp/merge-v0.2.14-latest-verified.log`，结构化结果为 `.tmp/merge-v0.2.14-latest-verified-result.json`（complete=true，11 步均 ok）。本文补齐后还执行最终暂存树完整性/增量证明及 check 门禁，再提交正常双亲 merge；上述本地验证不替代下文真实环境验收。

## 首批范围与回滚点

- 合并日期：2026-10-08。
- 工作分支：`codex/my`；合并前工作区干净。
- 合并前提交：`a7b655654571f8f9aef65abce33e0abe081f05ca`。
- 回滚分支：`codex/backup-before-v0.2.14-20261008`。
- 上游仓库：`Wei-Shaw/sub2api`；本次核验的发布标签 `v0.2.14` 对应 `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d`。
- 首批实际合入当次 fetch 获得的 `origin/main`：`db29b3a7738e26cfbc30025f033f481ac365ab7a`，包括发布后的 VERSION 修正及 Codex `service_tiers` 空值修复。
- 相对已合并基线 `b8dece9000c68815a5b867ca5a1e6f236e173905` 共 12 个 Git 提交、27 个变动文件；使用正常双亲 merge，不 squash，不整体选择 ours/theirs。

## 上游功能与二开衔接

| 范围 | 处理结果 |
| --- | --- |
| 安装初始化 | 完整采用上游随机管理员邮箱、8–72 字节密码校验及邮箱格式校验；已有用户/管理员时保留上游跳过创建和校验的语义。CLI、Web、自动安装及部署示例同步更新。 |
| EasyPay 回调 | 完整采用上游回调参数白名单和 return_url 查询参数清理，保留上游有效通知和订单对账路径，不为二开放宽安全校验。 |
| 充值活动 | `WeChatPaymentResumeClaims` 的 CampaignID、CampaignRevision 保留；服务端生成订单/恢复参数的调用顺序不变；营销及历史履约源码不改写。 |
| Codex 使用密钥 | 四类配置生成路径采用上游远程目录发现配置；本地补充回归断言，验证远程模式开启 discovery、文件模式不注入 discovery，覆盖 OpenAI HTTP/WS、Grok/Composite 和操作系统切换。 |
| Codex 模型目录 | 采用上游 `service_tiers` 非 null 输出与多账号能力交集 fallback，保留新增上游测试。 |
| 前端依赖 | 唯一文本冲突位于 package.json：Vue 采用上游 `^3.5.43`，保留本地 three、jszip、画布构建脚本；冻结锁文件安装成功。审计例外记录和其余依赖更新全部保留。 |
| 部署配置 | 采用上游随机邮箱默认值；保留本地数据库环境变量覆盖、图片响应体上限及既有持久化密钥说明。 |
| 二开隔离 | 14 个 managed 模块清单及实际开关不变；已合并基线、归属审查和证据表同步到本次目标，未放宽门禁、未重置历史 inventory。 |

## 完整性检查

- 上游变动的 27 个路径全部纳入；其中 19 个文件与目标上游逐字节一致，其余 8 个为保留本地独立差异的合并结果（支付恢复、5 份部署配置、package.json 和锁文件）。
- 上游目标跟踪文件缺失 0，本地独有跟踪文件缺失 0；合并本身未修改上游增量以外的本地文件。额外变更仅为回归断言、基线/证据记录和本文。
- 本地全部 SQL 迁移、扩展迁移及 migration runner 与合并前一致，包括原有 checksum 白名单；本次上游增量未带来 SQL 迁移，不连接或变更业务数据库。
- 完整性明细保存在本地 `.tmp/merge-v0.2.14-integrity.json`。

## 验证

- Node.js 24.19.0 下完整 `customization-verify --build` 的 11 个步骤全部通过：边界守卫、二开审计、Go 普通测试及 unit 测试、前端 lint/测试/类型/生产构建、画布构建与嵌入、后端嵌入构建。
- 前端 392 个文件、2928 项测试通过；补充 9 个 discovery 断言，并修正 3 个旧配置文本断言，保留 goals 与 WebSocket 断言，生产源码不偏离上游。
- 安装初始化、EasyPay 通知、return_url 与 Codex service_tiers 定向测试连续 20 次通过。
- 二开审计：未归属 0、归属违规 0、宿主接缝缺失 0、契约违规 0、历史清单缺失 0。
- 4 份 Compose 文件静态解析和部署脚本 `bash -n` 通过；没有启动容器或执行部署脚本。
- 仍有前端分块体积警告，不阻断构建。
- 最终日志 `.tmp/merge-v0.2.14-final.log`、结构化结果 `.tmp/merge-v0.2.14-final-result.json`；首轮旧断言失败日志另行保留，不当作成功结果。

## 验证边界

- 本次仅合并和本地验证，不 push、不发版、不部署、不启动应用服务。
- 单元测试、mock/SQLite 和本地嵌入构建不等于 PostgreSQL 历史库、真实支付回调、第三方模型、浏览器端到端或 Docker 多架构发布验收。
- 上线前仍需备份配置和数据库，验证真实环境；若上线后回退，需要同时评估数据库状态，不能只切回代码分支。
