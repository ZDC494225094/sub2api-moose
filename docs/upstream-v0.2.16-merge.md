# 上游 v0.2.16 源码合并记录

## 范围

- 日期：2026-10-14；分支：`codex/my`。
- 合并前：`3dc9c11324d0d31bb6c2559f7d08ce684020d441`。
- 回退分支：`codex/backup-before-v0.2.16-20261014`。
- 目标：Wei-Shaw/sub2api 发布 v0.2.16 的源码提交 `01842576b3d5c1e2545b5a99ad555d36e28ed306`，不是仅更新标签或版本号，也不追加发布之后的 main 提交。
- 共同基线：v0.2.15 源码 `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`。完整合并上游 79 个提交与 213 个变动路径。
- 发布源码的 VERSION 仍为 0.2.15，本地仅将显示版本同步到 0.2.16。

## 冲突与功能边界

- 12 个冲突文件逐项处理；重叠功能以上游为准，非重叠二开保留。
- 分组多语言完整采用上游 domain、schema、仓储、DTO、本地化接口与前端编辑行为；保留独立的自动倍率同步字段。Ent 从合并后的 schema 在隔离目录重新生成，只有分组及相关公共生成文件变化，避免错误的字段下标。现有以原生 SQL 实现的余额快照不引入额外 ORM 模型。
- Wire 从合并后的 provider 集与 cleanup 声明重新生成；同时注册 Gemini 显式缓存服务与本地视频、余额快照、Playground/画布等服务，保留生命周期清理。
- Gemini 显式缓存采用上游完整 CRUD、绑定账号、错误透传、存储定价、预留准入与结算规则。模型发现使用上游新的 AI Studio/Agent Platform (Vertex) 实现，不恢复已替换的旧模型列表策略。
- 本地 Veo 异步操作轮询和视频内容下载移至 `gemini_video_forward.go`，与上游模型发现分开；保留原视频路由、项目归属/路径/下载安全护栏，Vertex 请求复用上游 API Key/服务账号认证。统一 action 校验额外保留本地 `predictLongRunning` 支持。
- 新增视频共存回归发现旧视频轮询对 Vertex 的十段 operation name 错套通用八段后缀上限，导致合法操作返回 `invalid path`。仅在视频 helper 内按资源类型检查长度、闭集路径字符、末尾 `operations/id` 与项目归属，不放宽共用 Responses/模型路径护栏；覆盖 API Key 轮询/分段下载以及编码穿越、查询串、额外路径、跨项目和不可信下载域名的拒绝。
- 计费冲突采用上游 `recordUsageMultipliers`、`recordUsageBillingModel` 与显式缓存存储费拆分；上游抽取后的 helper 已含本地 composite 具体模型计费和无价兜底行为，不重复保留旧实现。
- 上游 Haiku 5.5、GPT-6.1 Sol 500k 默认上下文与订阅长上下文附加费豁免、结构化 Responses→Chat 安全回退、模型级临时不可调度、Antigravity 图片限流隔离及前端修复均纳入。
- 旧迁移及 migration runner 不改；新增上游 `242_group_i18n.sql`、`243_gemini_cached_contents.sql`、`244_gemini_cached_contents_channel_usage.sql`、`245_gemini_cached_contents_mutation_lease.sql`。runner 按完整文件名记账，不与同编号的二开迁移混淆。

## Codex 生图

- 纳入上游仅 OpenAI 平台账号允许 hosted 生图自动桥接的修复，保持账号→渠道→全局的配置优先级。
- 客户端 `image_gen.imagegen` 与 hosted `image_generation` 是两条不同路径；保留此前“确实附带 hosted 工具才添加 hosted 指令及默认 tool_choice”的回归修复。
- 分组生图权限、strip 策略、普通/Lite、透传/非透传、跨协议 namespace 和历史工具调用回归均须验证。
- 源码合并和本地回归不等于已解决用户桌面端真实生图失败；本次不修改 Codex/Magpie 配置，不执行收费远程生图。

## 验证记录

- 原始失败日志保留，不用重跑结果掩盖失败阶段。
- 完整性清单：上游 4322 个跟踪文件缺失 0；本地独有跟踪文件缺失 0；147 个仅上游变更的路径与目标逐字节一致；66 个交叉路径逐项审阅。213 个上游增删序列中 202 个原样纳入，剩余 11 个是版本标识、生成代码下标/容量、字段排版、计费 helper 抽取、视频 helper 拆分与附加回归，不代表丢弃功能。证据：`.tmp/merge-v0.2.16-integrity.json`、`.tmp/v016-delta-decisions.json`。
- 上游 296 个 SQL 迁移纳入；已有迁移改动 0。清单/字节相等只是结构证据，业务语义由下面的回归补充，不宣称所有功能已端到端验收。
- 首次完整验证停在后端普通测试：备份模拟进程需要 `sh`，当前验证进程 PATH 未包含本机 Git Bash。三条备份测试因此失败；未改备份源码，仅给重跑进程补 `D:\WorkApp\Git\Install\Git\bin`。日志：`.tmp/v016-full-verification.log`。
- 补齐测试环境后的中间全量仍停在视频失败先行用例（当次已编译旧轮询代码），未进入 unit/前端构建阶段：`.tmp/v016-full-verification-retry.log`。后续以修复后的源码重新执行完整门禁，不将中间失败描述为已通过。
- 增加视频认证共存回归时，失败先行日志为 `.tmp/v016-video-regressions.log`；修复视频专属路径校验后，生图、Vertex/视频、显式缓存、Haiku 与订阅长上下文专项连续 10 次通过：`.tmp/v016-focused-final.log`。
- 独立前端验证：lint、427 个测试文件 / 3210 个测试、类型检查通过。日志：`.tmp/v016-frontend-lint.log`、`.tmp/v016-frontend-tests.log`、`.tmp/v016-frontend-types.log`。
- 最终完整门禁 11 步全部通过：34 个边界守卫测试、归属/契约/库存审计、Go 普通全量（189.694 秒）、Go unit 全量（285.584 秒）、前端 lint、427 文件 / 3210 测试、类型检查、前端生产构建、画布构建、画布嵌入、后端 embed 构建。执行时使用支持的 Node 24.19.0、上游 Go 1.27.2、项目本地 GOCACHE 与 `GOFLAGS=-p=2`。日志/结构结果：`.tmp/v016-full-verification-final.log`、`.tmp/v016-full-verification-final-result.json`。
- 审计：未分类路径、归属违规、缺失 host seam、契约违规、历史库存缺失均为 0；未放宽原有门禁或修改历史保留清单。
- 补充 `go test -mod=readonly -tags=embed ./internal/web` 通过；最终二进制 `.tmp/customization-release/sub2api.exe -version` 返回 `Sub2API 0.2.16`。未注入发行 commit/date 元数据，输出仍为 unknown。日志：`.tmp/v016-embed-tests.log`、`.tmp/v016-binary-version.log`。
- 构建有非阻断的 chunk-size、动态/静态重复导入与 Browserslist 数据过期提示；未为消除提示更新额外依赖或改变二开。

本次不 push、不部署、不重启业务服务，也不连接或迁移业务数据库。本地验证不替代 PostgreSQL 历史库迁移、真实支付/模型调用及浏览器端到端验收。
