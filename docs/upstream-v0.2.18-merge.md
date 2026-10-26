# 上游 v0.2.18 源码合并记录

## 合并范围与回退点

- 日期：2026-10-26；本地分支：`codex/my`。
- 合并前：`a35088d46533ce261f51127c96f77da6acc661c3`。
- 回退分支：`codex/backup-before-v0.2.18-20261026`，指向合并前提交。
- 上游：Wei-Shaw/sub2api，发布 [v0.2.18](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.18) 对应源码提交 `ac1b8f1271be0d674fa6f074403c1410f30a3a81`。
- 共同基线：v0.2.16 源码 `01842576b3d5c1e2545b5a99ad555d36e28ed306`。
- 完整合入区间内 93 个提交、245 个变动路径，包含 v0.2.17；不追加发布之后的 main 提交。实际合并的是解引用后的 commit，不是仅合并标签说明或修改版本号。
- 发布源码内 `backend/cmd/server/VERSION` 仍为 0.2.17，本地显示版本同步为 0.2.18。

## 冲突处理

共 6 处冲突；重叠功能使用上游实现，非重叠二开按最小增量保留：

| 文件 | 处理 |
| --- | --- |
| `backend/cmd/server/VERSION` | 显示目标版本 0.2.18。 |
| `backend/internal/repository/scheduler_cache.go` | 保留二开健康检查 `Ping`；完整采用上游解码快照常驻内存与共享只读数据约束。 |
| `backend/internal/service/gemini_messages_compat_service.go` | 采用上游断开连接后继续收集计费及流结束处理；保留 Veo 异步结果使用的 `responseBody`。 |
| `frontend/src/components/account/CreateAccountModal.vue` | 上游 Vertex 配额能力判断与本地 Gitee 导入共存。 |
| `frontend/src/components/account/EditAccountModal.vue` | 同上，避免漏掉 `supportsAccountQuota`。 |
| `frontend/src/views/admin/AccountsView.vue` | 刷新时保留本地上游分组折叠，同时调用上游筛选条件持久化。 |

本次上游没有改动 Ent schema 或 Wire provider 参数，不重新生成不相关文件。完整测试发现本地此前给 `provideCleanup` 增加扩展参数但漏改测试调用；仅补齐测试的 `extensions` 入参，运行时 Wire 接线与清理逻辑不变。

## 重点功能和行为变化

- Gemini 4 Argon、thinking level/summary、工具 schema 常量清理及兼容协议转换采用上游实现。
- Vertex 服务账号支持总额/日/周账号配额，包含调度、扣费、告警与前端配置/展示；不再误按 API Key 类型限制配额能力。
- Gemini、Antigravity、Composite 分组的图片生成开关在选号前、模型映射后及兼容转发层均按上游规则执行。
- 新增原样上游迁移 `246_enable_gemini_image_generation_groups.sql`：只自动开启存量 Gemini/Antigravity 分组的生图权限。**Composite 分组不会自动开启**，因为同一开关还影响 OpenAI 图片生成；如需生图，部署后应由管理员明确开启。
- OpenAI overload 身份识别、输出前安全空流重试、模型退役和 Responses `created_at` 修复全部保留。
- Anthropic 兼容流和 Gemini 客户端断开连接后的用量收集、异常结束/安全切号采用上游行为，输入 token 差值不允许为负。
- Grok 4.7 Fast 使用上游官方 Fast 价格，为标准版的两倍；不保留旧低价。
- 调度快照进程内缓存、模型映射无分配签名、限流状态 copy-on-write、账号配额按已配置周期选择 SQL 均纳入；保留本地顶层分组/排序字段和健康检查。
- Cloudflare 1010 拦截不错误惩罚账号；异步图片下载执行公网地址、重定向及内容类型校验，不能为兼容本地对象存储而绕过上游安全边界。用户归属通过上传 context 独立保留。
- 支付完整纳入易支付主动确认、支付宝退款查询、pending 退款扣减保留与原子终态处理、订单类型白名单、Stripe 历史订单密钥快照和返回 URL 安全检查。
- 本地订阅归属和履约租约、营销优惠券核销/修复、推广奖励退款冲回继续使用既有事务边界，与上游支付终态处理共存。营销支付回归改用真实 balance 订单测试 balance 入口，不放宽上游新增的订单类型校验。
- 采用上游 Go 1.27.3、DOMPurify 3.4.17、follow-redirects 1.16.1 及其锁文件/CI/镜像更新。

## 二开保留与校验中修复

- 保留通知机器人及 Telegram/QQ、AI 支持、模板/调度、安全护栏，用户手册及插图、公众号绑定/抽奖，存储素材列表/用户归属，画布任务/诊断与视频服务等已有二开。
- 未修改画布源码；遵循画布目录约束，不运行画布语法检查、测试或重建。主站生产构建后把已有 `canvas/web/dist` 的 25 个文件原样复制到后端嵌入目录，逐文件 SHA-256 比对无差异。这不是画布源码重新构建的证明。
- 43 个近期二开路径缺少精确责任登记，已按真实模块边界登记；补齐机器人管理入口和手册公开/管理入口的可执行回归证据。不删除历史保留清单、不放宽归属/契约守卫。
- 新增手册真实路由回归：关闭/非法开关先于数据库读取拒绝；开启时返回已保存内容；管理入口先认证再检查开关。
- 修复既有营销选项组件直接修改 prop 导致的 lint 失败：通过事件回写父级状态，保留“仅新用户”限制与原生成流程，并增加冻结 prop 回归。
- 公众号抽奖页补齐前后端相同目录登记与精确历史页面白名单，沿用其他抽奖页的共享准入和只读提示；关闭/缺失开关保留已有内容读取但禁止新抽奖，配套组件回归覆盖关闭、开启和再次关闭。不改变普通抽奖与公众号抽奖的渠道隔离。

## Codex 生图边界

- 保留客户端 `image_gen.imagegen` namespace 与 hosted `image_generation` 的区别，以及已有“确实附带 hosted 工具才添加 hosted 指令及默认 tool_choice”的回归修复；本次上游未替换该兼容点。
- 同时验证 namespace/历史工具调用、原生/多分组选择、普通/Lite、透传/非透传、allow/strip 与 Gemini 分组门禁。
- 源码合并和本地回归不是 Codex 桌面真实生图验收；未执行收费模型请求，也未修改 Codex/Magpie 或客户端工具配置。

## 完整性核对

- 上游 4362 个跟踪文件缺失 0；只在上游变动的路径与目标字节差异 0；本地独有新增文件缺失 0。
- 上游 245 个变动路径的增删序列均原样纳入，唯一例外是把 VERSION 从发布源码内的 0.2.17 改成 0.2.18。
- 上游主动删除的两个赞助商图片和旧账号筛选测试未恢复；它们不属于被遗漏的本地新增功能。
- 合并前 334 个 SQL 文件没有修改或删除，上游新增迁移原样保留。未运行业务数据库迁移。
- 结构清单不能证明所有业务语义等价，需结合回归与部署验收。临时证据在 `.tmp/merge-v0.2.18-integrity.json` 和 `.tmp/v018-review-*.patch`。

## 验证结果

- 最终执行 `node tools/customization-verify.mjs --full --upstream upstream/v0.2.18`，7 步全部通过：
  - 34 个边界/库存/归属守卫测试；归属与契约审计。
  - 后端普通全量 `go test -mod=readonly ./...`（212.457 秒）。
  - 后端 unit 全量 `go test -mod=readonly -tags=unit ./...`（300.441 秒）。
  - 前端只读 lint、431 个测试文件 / 3224 个测试、`vue-tsc -b` 类型检查。
  - 日志与结构结果：`.tmp/v018-verification-full.log`、`.tmp/v018-verification-full-result.json`。执行环境为 Node 24.19.0、Go 1.27.3、项目本地 GOCACHE、`GOFLAGS=-p=2`，并为备份进程模拟测试补入已有 Git Bash PATH。
- 最终含合并文档的独立审计也通过：未分类路径、归属违规、缺失 host seam、契约违规、历史库存缺失均为 0。证据：`.tmp/v018-final-audit.json`、`.tmp/v018-audit-with-doc.log`。
- 生图工具/分组权限、Vertex 配额、流中断、调度快照、退款/支付确认及履约租约等匹配专项连续 3 轮通过，日志：`.tmp/v018-regressions-final.log`。其中 apicompat 包没有匹配到该专项名称筛选；其完整测试已由普通/unit 全量覆盖。
- 手册公开/管理路由和机器人门禁测试包含在后端全量；公众号抽奖及营销选项新增回归和插件目录/历史访问专项单独通过（4 文件 / 11 测试），同时纳入最终前端全量。
- 通过 `pnpm 10.12.1 install --frozen-lockfile` 安装锁定依赖；实际安装 DOMPurify 3.4.17、follow-redirects 1.16.1。未重写/放宽锁文件。
- 主站 `pnpm run build`（包含 i18n 完整性、类型检查及生产 Vite 构建）通过，日志：`.tmp/v018-frontend-build.log`。画布沿用已有构建而非重建，复制校验：`.tmp/v018-canvas-asset-copy.json`。
- `go test -mod=readonly -tags=embed ./internal/web` 和 `go build -mod=readonly -tags=embed -trimpath` 均通过。二进制 `.tmp/v018-release/sub2api.exe -version` 返回 `Sub2API 0.2.18`；没有注入发行 commit/date，二者仍显示 unknown。日志：`.tmp/v018-embed-tests.log`、`.tmp/v018-backend-build.log`、`.tmp/v018-binary-version.log`。
- 保留中间失败：首次普通测试停在 Wire 测试参数缺失（`.tmp/v018-backend-ordinary.log`）；首次前端 lint 停在营销 prop 修改（`.tmp/v018-frontend-lint-check.log`）；中间 unit/前端全量暴露错误订单测试数据和公众号页面登记缺失（`.tmp/v018-backend-unit-final.log`、`.tmp/v018-frontend-test-run-final.log`）。修复后才执行上述最终完整门禁，不能把这些旧失败日志误认为最终结果。
- 初次安装误用了 pnpm 11，因其不采用现有 `pnpm.overrides` 而触发 frozen-lockfile 配置不匹配（`.tmp/v018-frontend-install.log`）；改用现有 pnpm 10 后冻结安装成功（`.tmp/v018-frontend-install-pnpm10.log`）。公众号组件测试最初的 mock 缺少实际依赖导出也保留在 `.tmp/v018-frontend-focus.log`，隔离真实依赖后通过，不更改生产 API 行为来适配测试。
- 构建仍有非阻断的 chunk-size、静态/动态重复导入与 Browserslist 数据过期提示；未为消除提示升级无关依赖或重构业务。

## 未执行与部署前要求

本次只完成本地源码合并与验证，不 push、不部署、不重启服务，不连接/修改业务数据库。部署前应备份数据库，在数据库副本或预发布环境执行迁移，并验收历史订单/退款、订阅归属、二开开关、Composite 生图权限以及实际 Codex 生图。不要把本地模拟支付或 SQL/SQLite 回归当作真实支付网关和 PostgreSQL 历史库验收。
