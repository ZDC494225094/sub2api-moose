# 上游 v0.2.15 源码合并记录

## 范围

- 日期：2026-10-12；分支：codex/my。
- 合并前：9a1131e7eb1636af417b2d1fcc03df65ea72b4db；回滚分支：codex/backup-before-v0.2.15-20261012。
- 用户指定 Wei-Shaw/sub2api 的 v0.2.15 发布源码，目标 f2669c8cf62555cd92389b3f55920e9e6e7c6ff2。未追加 main 后续提交。
- 本地同名旧标签属于其他历史，保留原标签；上游发布标签保存为 upstream/v0.2.15。
- 上游发布源码的 VERSION 仍为 0.2.14，本地仅将该标识同步为 0.2.15。
- 相对共同基线 745ff1d3c，共 55 个提交、174 个变动路径。

## 合并处理

- 上游 provider profile、平台目录、Command Code/Cline、按模型协议选择、Web Search 历史修复、安全依赖和前端竞态修复全部纳入。
- 6 个文本冲突逐项解决：计费探测同时保留二开模块和上游 domain 引用；平台配额测试完整采用上游目录；账号表单保留 Gitee AI 并采用上游 TypeSafe 和新增平台入口；UseKeyModal 保留 discovery 断言并采用上游独立特征断言。
- 公告采用上游已读失败语义：失败保持未读、显示错误并允许重试；本地共享公告面板保留，使用其选择事件测试失败重试，不恢复已替换的旧弹窗 DOM。
- 133 个单侧上游路径与目标逐字节一致；41 个交叉路径中，169/174 个路径的上游增删序列原样纳入。5 个非同序列路径均为公告适配或测试断言：平台配额测试采用上游全文、账号 Qwen 规则已在本地存在、UseKeyModal 双方断言合并、公告组件及其回归适配本地面板。
- 上游跟踪文件缺失 0，本地独有跟踪文件缺失 0。完整性证据：.tmp/merge-v0.2.15-integrity.json。
- 已有 SQL 及 migration runner 未修改。新增上游 242_drop_platform_check_constraints.sql；迁移按完整文件名记账，与本地 242_add_user_balance_snapshots.sql 名称不冲突。未连接或迁移业务数据库。
- 保留画布预扣费、计费上下文、Gitee、账号排序/分组和独立二开。原先已跟踪的 .canvas-current-state.json 保持原样，仅补精确归属登记，未放宽门禁。

## 分组生图

- image_generation_intent、Responses namespace、OpenAI 转发及新增协议路由核心源码与本次上游一致，没有以二开替换上游生图规则。
- 客户端 image_gen.imagegen 与服务端 hosted image_generation 是不同路径。分组允许生图不等于所有账号自动注入 hosted 工具；账号策略高于渠道/全局，Responses Lite 不自动注入 hosted 工具。
- 专项回归涵盖分组权限、桥接策略、跨协议 namespace、模型协议目录/路由；重复 3 次通过。新增 TestCodexImagegenCrossProtocolRoundTrip 对普通 Responses 与 Lite 的 imagegen 声明、历史调用及 SSE/JSON 输出名称还原连续 10 次通过。
- 尚无用户实际失败请求、账号配置和真实上游调用证据，不能宣称 Codex 实际生图问题已修复。此次先完成指定版本源码合并。

### 2026-10-13 生图故障跟进

- 找到一处可复现的源码缺陷：非透传分支开启 Codex 生图桥接时，只有客户端 `image_gen` namespace 的请求也会收到“本次请求附带原生 `image_generation` 工具”的指令，但实际没有该 hosted 工具。原版 v0.2.15 同样包含该缺陷，并非已经证明的二开独有回归。
- 修复 `openai_codex_transform.go`：仅在顶层确实附带 hosted `image_generation` 时追加对应指令和默认 `tool_choice`；namespace、客户端函数和 `additional_tools` 继续保持自己的调用方式。未修改分组权限、账号 strip 策略、Lite 不注入 hosted 工具的规则和计费。
- 失败先行证据：`.tmp/codex-imagegen-bridge-red.log`；修复后专项连续 10 次通过：`.tmp/codex-imagegen-bridge-green.log`。新增测试覆盖 namespace/carrier 的指令边界，以及 24 组缓存序列化、原生/多组选组、普通/Lite、透传/非透传、allow/strip 组合；既有 hosted 工具注入和 WebSocket 回归一起执行。
- 服务层全量首次失败于原版 `TestCommandCodeGatewayPassesThroughCatalogProtocols`：Windows 连续时间戳重复，使成功目录与“目录不可用”fixture 共用缓存。用 Go overlay 恢复修改前生产源码仍可复现（50 次中失败 32 次）；诊断日志直接记录了两种 fixture 的重复地址。仅给测试地址追加用例序号，连续 100 次通过，再跑 service 全量 unit 通过（233.909 秒）。失败日志保留，未修改生产目录或协议路由规则。
- 普通标签生图专项、handler/middleware/routes/media-gateway/multi-group-billing 相关回归、归属和契约 audit、`git diff --check` 通过；后端源码构建成功，验证二进制 `-version` 为 0.2.15。该二进制未替换业务服务，也未包含远程真实验收结论。日志：`.tmp/codex-imagegen-production-tests.log`、`.tmp/codex-imagegen-gateway-regressions.log`、`.tmp/codex-imagegen-service-full-retry.log`、`.tmp/codex-imagegen-audit.log`、`.tmp/codex-imagegen-build.log`。
- 实时查询两个服务公开版本均为 0.2.15；直接获取的 `gpt-6.1-sol` 工具能力字段一致，包括 `tool_mode=code_mode_only`、`use_responses_lite=true` 和 `supports_search_tool=false`。公开版本号不证明远程构建提交或账号配置与本地一致。
- 使用同一份最小工具声明，直接请求两端的普通 Responses 和 Lite 共 4 次，全部 HTTP 200，并返回 `function_call(namespace=image_gen,name=imagegen)`。该测试未执行客户端函数，不能替代桌面端出图验收，也不证明故障时选中的账号一定相同。
- 再通过本机 Magpie 请求相同的两组公开模型：号池的普通/Lite 仍返回客户端 imagegen 调用；麋鹿云普通请求返回 hosted `image_generation_call`，Lite 请求只返回 reasoning/message。这是已观察到的调用路径差异，不足以确定由代理改写、请求头差异还是账号选择引起。普通请求实际收到托管生图事件，因此这些代理探针不能描述成“完全没有触发生图”。
- 本机 Magpie 的 `/v1/models?client_version=...` 返回标准 `data` 列表，不含直接服务提供的完整 Codex 工具能力字段；`/backend-api/codex/models` 返回自己的 manifest，工具字段也不同。该结果仅是当前代理响应，不是历史失败轮的请求工具定义。
- 已核对用户贴出的失败会话：2026-10-12 17:38 的工具搜索返回 `tools=[]`，随后没有 imagegen 调用。该会话记录未保存完整的实际请求工具声明，因此仍缺“桌面端原始请求 → Magpie 出站请求 → 远程实际选中账号”对照；不能宣称本次局部源码修复已解决全部实际故障。
- 未更改 Codex/Magpie 配置、数据库和业务进程，未 push 或部署。需要部署修复后，用失败时的同一调用路径进行真实验收。

## 验证

- 冲突相关前端 4 文件 / 89 测试通过；独立前端全量 416 文件 / 3132 测试、lint 和类型检查通过。
- 首次完整验证因 proxy.golang.org 下载 Go 1.27.2 超时中止；改用 goproxy.cn 后下载成功，保留上游 go.mod 版本。
- 生图专项日志：.tmp/v015-image-protocol-tests.log、.tmp/v015-imagegen-roundtrip.log。
- 全量验证前 10 步全部通过：边界守卫、审计、Go 普通/unit、前端 lint/416 文件 3132 测试/类型/生产构建、画布构建及嵌入。第 11 步 backend-embed 首次因 Windows compile.exe Access is denied 失败；相同源码和参数增加 -p 2 降低并行度后成功，无生产代码修改。最终 embed 标签 web 测试通过，新二进制 -version 为 0.2.15（未注入发行 commit/date 元数据）。成功构建不代表原 11 步记录全部为绿，原失败记录保留。
- 全量日志/结果：.tmp/merge-v0.2.15-verified.log、.tmp/merge-v0.2.15-verified-result.json；成功补跑：.tmp/v015-embed-retry.log、.tmp/v015-embed-tests.log、.tmp/v015-binary-version.log。4 份 Compose 静态解析、bash -n 通过；新增回归和合并文档有精确归属记录，最终 check 门禁通过。
- 不 push、不部署、不启动业务服务；本地测试不替代 PostgreSQL 历史库、真实支付/模型调用和浏览器端到端验收。
