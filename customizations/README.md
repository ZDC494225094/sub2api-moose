# 二开扩展维护区

- `IMPLEMENTATION_PLAN.md`：与当前 Go/Wire/Ent/Vue 结构一致的目标架构、模块拆分、历史业务兼容、实施顺序和完成标准。
- `MIGRATIONS.md`：新装默认关闭、已有部署一次性接纳、独立迁移账本与 PostgreSQL 上线演练边界。
- `../backend/internal/customize/migrations/`：模块命名空间迁移登记器、接纳 SQL 与隔离事务回归。
- `upstream.json`：人工确认的已合并上游基线（不是最新版本）。
- `inventory-baseline.json`：固定的改造前 814 个差异路径，以及后续迁移/提取账本；不得重建基线掩盖丢失。
- `ownership.json`：规则候选归属，全部匹配项保留；`local-artifacts` 没有兜底规则。
- `ownership-reviewed.json`：70 个原先未匹配路径的精确人工归属、证据与下一步；优先于规则，但不代表运行时已隔离或其余路径已人工审核。
- `contracts.json`：独立实现、禁止回流的依赖、上游等价文件及最小宿主接入点。
- `../backend/internal/customize/catalog.json`：运行时功能清单、关闭语义、迁移状态。
- `../frontend/src/extensions/`：独立模块、路由、UI 插槽、开关状态和管理页。
- `../tools/customization-audit.mjs`：只读差异、归属来源、多责任归属、契约和固定账本检查；`--check` 阻止新增未归类路径或无效精确归属。
- `../tools/lib/customization-ownership.mjs`：纯函数精确路径校验与归属计算，不访问运行数据内容。
- `../tools/customization-verify.mjs`：统一 check/full/build 入口，合成前端、画布和后端 embed 产物。
- `../tools/__tests__/customization-*.test.mjs`：门禁、精确归属、构建产物保护及执行计划自身的 29 项回归测试。
- `../.github/workflows/custom-extensions.yml`：独立契约和全量回归 CI（尚未远端运行）。
- `../backend/internal/customize/modules/rechargecampaigns/`：独立活动配置、报价、历史快照、邀请奖励/退款规则与回归；不导入宿主 service/Ent，事务与订单幂等仍由宿主拥有。

- `../backend/internal/customize/modules/marketing/`：独立券/抽奖规则、仓储接口、事务/钱包/原生奖品端口；交易连接修正与订单券预留回归见模块 README。整体仍待隔离，未开放营销开关。

部署边界、验证记录、升级与回退步骤见仓库根目录 `CUSTOM_EXTENSIONS.md`。不存放凭据或本地配置。
