# 计费与调度增强：待隔离

2026-10-03 撤销旧的“标记优先、默认透传”计划；无运行时拒绝分支不得算作 managed。

- 分时定价和高峰倍率是上游功能，不挂二开开关。
- 倍率同步应在同步任务侧隔离，保留 Group 的 BillingRateSyncAccountID / BillingRateMarkup 数据。
- 组合调度另行梳理准入、在途行为和历史兼容。
- 正式 wiring 中的旧 provider 已删除。
- admission.go 暂留仅因误提交的 backend/backend 旧副本仍引用它；清理副本后再删除，不代表运行时接入。
- 在真实调用、关闭拒绝测试、上游归属证据齐备之前保持 pending。
