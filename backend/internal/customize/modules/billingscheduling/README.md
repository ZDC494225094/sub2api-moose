# 计费探测增强（2026-10-04）

开关：`custom_extensions.billing-scheduling.enabled`。仅管理二开新增的账号探测间隔/自动同步选项、定时探测后的分组倍率写回。

- `upstream_billing_probe.go` 在单次探测准入读取开关；关闭、缺失、非法值、数据库错误都忽略二开选项，继续上游探测。
- 关闭时使用上游全局间隔（至少五分钟），不主动重写已有倍率/配置。
- 分组同步在写回前再次读取开关。关闭发生在 HTTP 探测中途时，不再写分组倍率。
- 已准入的账号探测可以完成；上游原生 `upstream_billing_rate_sync_enabled` 仍独立生效。
- 不接入每请求计费热路径，不引入空壳 Wire provider。

## 上游归属纠正

`aea725f2` 已有 `NewCompositeRouteResolver`、`composite_platform.go`、分时定价、高峰倍率。组合调度并非本模块待隔离能力，不再计划给它添加开关。两份组合调度文件纳入上游等价门禁。

## 接纳迁移

新增独立 `202610040001_adopt_probe_extensions.sql`：legacy 默认开启，native 默认关闭，保留显式值，删除后的键不重复补回。历史宿主 500 号迁移不删除、不改写。

## 验证边界

真实服务测试覆盖缺失/关闭/非法值时不写分组倍率、不执行二开自动同步、保留原生同步、全局间隔回退，以及探测过程中关闭；策略测试覆盖读取失败。迁移测试覆盖来源、显式选择、账本失败回滚重试。

精确读开关/调用/测试位置见 `customizations/RUNTIME_EVIDENCE.md`。本地 Go 测试不代表已部署或真实 PostgreSQL/上游网关验收。
