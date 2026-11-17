# 上游 v0.2.22 源码合并记录

## 合并范围

- 日期：2026-11-17；分支：`codex/my`。
- 合并前提交：`e41cc98fec1ad6221cce7a67c694835b9602891f`；备份分支：`codex/backup-before-v0.2.22-20261117`。
- 上游：[Wei-Shaw/sub2api v0.2.22](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.22)，源码提交 `b0fa9f3e6eeab1968a53e1289b69daa4bb40ca2b`。
- 共同基线为 v0.2.21 源码 `10ba8f9b4dc2e6023d4a9879ab77b21841babd10`；纳入 187 个提交、340 个变动路径，不追加发布之后的 main 提交。
- 使用标签解引用后的源码 commit 合并，保留双父节点。上游发布源码的 VERSION 仍为 0.2.21，本地更新为 0.2.22。
- 合并前工作区干净。保留 v0.2.21 合并之后的六个本地提交，包括首页加载、验证码和插件轮询优化。

## 冲突和重叠功能决策

共审阅 87 个交叉变动路径，解决 8 个文本冲突文件。重叠功能采用上游实现，独立二开保留原有功能和接入点。

| 范围 | 决策 |
| --- | --- |
| `wire_gen.go` | 根据合并后的声明重新运行 Wire。接入上游批量账务仓储、按用户结算队列及 Stop 清理；保留二开模块、画布、开票、日志归档、机器人等接线。 |
| `ent/mutation.go`、`ent/runtime/runtime.go` | 用合并后的 schema 在临时目录重新生成，纳入 `force_openai_ultrafast` 并保留二开字段；最终只有这两个生成文件需要重新应用，其余生成结果已经匹配。历史 `user_balance_snapshot.go` 仅从临时输入排除，原文件保持不变，因为当前实现使用独立 SQL。 |
| `repository/scheduler_cache.go` | 使用原生快照值读取、按 bucket 合并加载、冷却账号成员保留与能力投影；保留本地启动时 Redis Ping 健康检查。 |
| `usage_log_repo_stats.go`、`usage_log_repo_trend.go` | 使用上游的独立 `actual_cost` 与 `account_cost` 查询/扫描；保留画布托管密钥筛选，删除原有按账号成本替换实际扣费的行为。 |
| `gateway_usage_billing.go` | 采用上游结算与缓存同步各自独立的超时；保留画布账本和具体订阅 ID 的缓存归属。计费命令先加入画布账本 ID，再计算幂等指纹并量化金额，避免历史重试因先舍入再重算指纹而冲突。 |
| `web/embed_on.go` | 设置注入 HTML 统一采用上游 no-store/CDN no-store 策略，覆盖二开画布宿主 HTML。保留首页验证和 CSP nonce，移除被上游替代的本地首页缓存头 helper。静态资源仍走原生缓存规则。 |

### 自动合并之外的语义修复

1. **批量结算与画布账本**：上游新 `ApplyBatch` 不识别二开 `CanvasChargeID`。画布命令仍留在同一用户队列中，按顺序单独调用已有账本感知的 `Apply`；普通请求段继续批量结算。仓储在执行 SQL 前拒绝误入批次的画布命令。回归验证普通钱包只扣普通请求、画布账本不二次扣费、已退款迟到请求不重新收费以及幂等重放。
2. **公共错误解析采用上游**：旧二开在公共解析器加入视频 `error` 字符串/数组，影响新版 Grok 的优先级和隔离要求。删除公共扩展，恢复与上游文件一致；将视频解析放回 OpenAI 兼容视频和 Gemini 视频入口。视频兼容 handler 只替换显示消息解析函数，原始错误体、透传规则、账号状态和失败转移仍由共享原生流程处理。
3. **生成代码保持一致**：不手工拼接 Ent 字段下标或 Wire 局部片段。实际生成输出已与合并树逐文件比对。
4. **测试前提修正**：Grok 映射用例显式设置并恢复全局映射选项，分别验证跨客户端映射开启和关闭；设置接口契约补上合并前就存在的 `homepage_captcha_enabled`，保留严格 JSON 比对。
5. **Stripe 懒加载回归**：合并前首页优化已经移除 `vendor-misc`，旧测试仍搜索该字符串。改为在 Node 环境加载真实 Vite 配置并调用分包函数，检查 npm/pnpm 两种 Stripe 路径、普通 HTTP 依赖和 Vue 分包，保留无副作用按需导入断言。生产分包配置不变。

## 已纳入的主要上游更新

- 分组强制 Ultra Fast 按模型门控，可与强制 Fast 并存；贯穿迁移、Ent、DTO、仓储、API Key 缓存 v28、表单和 HTTP/WebSocket 转发。多分组密钥归属继续保留。
- 按用户合并账务结算、分组倍率事务同步、RPM override 分组范围修复。
- 调度快照并发加载/数据库回源合并、冷却过期恢复、`openai_capabilities` 与显式透传模型过滤投影。
- Gemini 4 Carbon/Hydrogen、Vertex us/eu 多区域端点及 global 回退；默认价格的多区域 10% 加价、显式定价不重复加价；并行同名工具和粘性会话用量修复。
- Codex HTTP 上下文规范化、`additional_tools` 声明 ID 保留与 UUID 规范化、sealed 参数/引用保留、工具 Schema 正则兼容、fork prompt-cache 命名空间。
- 保留既有 `image_gen.imagegen` namespace、多分组缓存和跨协议回传回归；客户端工具声明不会被二次注入 hosted `image_generation` 桥接提示。
- OpenAI SSE 心跳/首输出/错误处理、兼容路径账号状态与失败转移、Bedrock 错误分类及渠道开关。
- OpenAI 配额刷新失败保持原快照新鲜度，Kimi 月度配额与 Fable 周期恢复，Gemini 刷新期间用量展示。
- 用量统计实际扣费与账号成本分离、组合路由客户端模型名恢复、启动价格检查后台执行、支付宝稳定退款请求 ID。
- Go 工具链更新至上游要求的 1.27.4。

## 数据库文件和完整性

新增两个原生 SQL，完整文件名均保留，内容与上游一致：

1. `251_group_force_openai_ultrafast.sql`
2. `251_group_usage_rollup_nonblocking_build.sql`

Runner 按完整文件名记账，同为 251 的前缀不构成覆盖。合并前已有 345 个 SQL 文件未修改或删除；本次没有连接业务数据库或执行迁移。

按 Git blob/mode 和逐文件增删序列检查：上游 4721 个文件缺失 0，本地独有新增文件缺失 0。接受上游主动删除 `assets/partners/logos/lanox.jpg`。13 个上游变动路径的增删序列需要解释，已在 `.tmp/v022-delta-decisions.json` 按精确路径记录；其中三个仅上游变动文件的额外修改用于画布账本与原生批量结算兼容。另记录未发生上游变动的本地兼容修复和附加回归。

结构证据为 `.tmp/v022-merge-integrity.json`、`.tmp/v022-overlap.json` 及 `.tmp/v022-delta-decisions.json`。路径存在和字节核对不能代替运行验收。

## 验证记录

最终 `node tools/customization-verify.mjs --full --upstream upstream/v0.2.22` 于 2026-11-17 12:51（Asia/Shanghai）完整通过七个阶段。独立结果保存在 `.tmp/v022-verification-full-complete-result.json`；失败的先前轮次另行保留，不覆盖其证据。

| 检查 | 最终结果 |
| --- | --- |
| 守卫和结构审计 | 34 个守卫通过；未分类、归属违规、缺失接入点、契约违规和库存文件缺失均为 0。 |
| 普通后端全量 | `go test -mod=readonly ./...` 通过。最终生产源码上一轮非缓存执行约 242 秒；最后一轮复用 Go 有效缓存。 |
| unit 后端全量 | `go test -mod=readonly -tags=unit ./...` 通过。最终生产源码上一轮非缓存执行约 332 秒；最后一轮复用 Go 有效缓存。 |
| 前端 lint | 全量通过。 |
| 前端测试 | 460 个文件、3596 个测试全部通过。 |
| 前端类型 | `vue-tsc -b` 通过。 |
| 生产构建 | 主站前端、最终 embed web 测试和后端内嵌构建均通过。 |
| Git | 暂存与工作区 whitespace 检查通过，源码无未解决冲突。提交后再核验父节点、目标祖先、审阅 tree 与工作区清洁状态。 |

最终二进制为 `.tmp/v022-release/sub2api.exe`，显示版本 0.2.22，SHA-256 为 `395FCDA84686DF9D8FA8F673D4BF137929F6AF58AA394AD7F64EB288DA5734DD`。发行 commit/date 未注入，仍显示 unknown。工具链为 Go 1.27.4、Node 24.22.1 和 pnpm 10.12.1；使用上游依赖声明和现有锁定安装。主站 Vite 构建仍有大于 500 kB 的 chunk 提示，构建成功。

已完成的专项：

- 画布账本/指纹三轮：30 个含子用例结果通过，15 个顶层执行。
- 合并风险两轮：308 个含子用例结果通过，90 个顶层执行；覆盖 Codex imagegen、多分组缓存、additional_tools、Ultra Fast、调度、首页验证及画布统计。
- Grok/视频错误与设置契约两轮：94 个含子用例结果通过，12 个顶层执行。
- 主站前端生产构建和首页 bundle 静态检查通过。沿依赖图核对首页 15 个 JS/CSS 文件，没有将控制台字典和重型可选组件提前载入。
- 画布源码未修改、未重建；主站构建后复制已有 25 个画布 dist 文件，逐文件 SHA-256 全部一致。
- integration 仓储测试编译通过，未执行测试二进制、未启动容器或访问数据库。

中间失败保留：视频错误 writer 签名适配、Grok 映射测试全局状态、设置契约缺字段、公共错误解析与原生 Grok 隔离冲突、源码变更后证据表行号过期、Stripe 测试依赖已移除的分包名称。Stripe 配置执行需 Node 环境，首次放在 jsdom 中触发 esbuild TextEncoder 跨 realm 断言，改用 Node 测试环境后四项通过。修复不降低原生断言或放宽模块守卫。

## 验收边界

本次完成本地源码合并、测试和构建，不 push、不部署、不重启业务服务，不修改 Codex/Magpie 配置。未进行真实收费生图、真实支付、浏览器端到端或 PostgreSQL 历史迁移验收。构建产物和模拟回归不能证明已运行服务已升级，或真实 Codex 客户端已成功出图。
