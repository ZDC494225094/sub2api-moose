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

## 验证

- 冲突相关前端 4 文件 / 89 测试通过；独立前端全量 416 文件 / 3132 测试、lint 和类型检查通过。
- 首次完整验证因 proxy.golang.org 下载 Go 1.27.2 超时中止；改用 goproxy.cn 后下载成功，保留上游 go.mod 版本。
- 生图专项日志：.tmp/v015-image-protocol-tests.log、.tmp/v015-imagegen-roundtrip.log。
- 全量验证前 10 步全部通过：边界守卫、审计、Go 普通/unit、前端 lint/416 文件 3132 测试/类型/生产构建、画布构建及嵌入。第 11 步 backend-embed 首次因 Windows compile.exe Access is denied 失败；相同源码和参数增加 -p 2 降低并行度后成功，无生产代码修改。最终 embed 标签 web 测试通过，新二进制 -version 为 0.2.15（未注入发行 commit/date 元数据）。成功构建不代表原 11 步记录全部为绿，原失败记录保留。
- 全量日志/结果：.tmp/merge-v0.2.15-verified.log、.tmp/merge-v0.2.15-verified-result.json；成功补跑：.tmp/v015-embed-retry.log、.tmp/v015-embed-tests.log、.tmp/v015-binary-version.log。4 份 Compose 静态解析、bash -n 通过；新增回归和合并文档有精确归属记录，最终 check 门禁通过。
- 不 push、不部署、不启动业务服务；本地测试不替代 PostgreSQL 历史库、真实支付/模型调用和浏览器端到端验收。
