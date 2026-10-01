# 二开迁移与默认状态契约

## 所有权与不可变历史

- 不移动或改写 `backend/migrations` 中已存在的二开 SQL；原文件名、执行记录与校验和继续由宿主管理。
- 从本阶段开始，新二开 SQL 只放 `backend/internal/customize/migrations/sql/<module-id>/<YYYYMMDDHHmm>_<name>.sql`。模块目录分离，登记主键为 `(module_id, version)`，不争用上游递增编号。
- 当前 runner 只执行事务内 SQL，不支持 `_notx.sql`、并发索引或依赖别的模块执行顺序的迁移。需要这些能力时先设计端口和恢复测试，不能直接塞入现有 runner。
- 发布过的 SQL 不得修改；更改要追加新版本。checksum 对 CRLF/LF 规范化，避免 Windows/Linux 检出导致伪变化。
- 关闭开关不跳过迁移，更不删除历史数据、订单、配置或订阅权益。

## 唯一组合入口

`repository.ApplyMigrations` 为安装向导、自动安装、普通启动共同入口。它保留宿主原迁移实现，只注入 `Prepare` / `Apply` 生命周期。二者在同一个宿主 PostgreSQL advisory lock 内，复用已固定的连接，不再申请第二条连接（连接池上限为 1 也不会因扩展挂起）。宿主迁移失败时不执行扩展 Apply；任何扩展失败阻止启动，不能以缺少状态继续服务。

1. 宿主创建自己的 `schema_migrations` 记录表。
2. `Prepare` 在宿主本轮 SQL 执行前，读取固定的历史 fork 迁移文件名；存在则记 `legacy`，否则记 `native`。该来源记录在独立事务中持久化，重复启动不得覆盖。
3. 宿主正常执行现有 SQL 和 checksum 校验。
4. `Apply` 执行独立命名空间 SQL，并在同一事务记录独立账本；不写入或篡改宿主历史记录。

如果第 3 步中途失败，新装库中可能已经出现 fork SQL；第 2 步的来源记录会防止重试时误判为老部署。

## v1 接纳名单固定

仅包含 premium-home、playground、infinite-canvas、operations-analytics、recharge-campaigns、customer-support。legacy 实例补充缺失 true；native 实例补充缺失 false。所有已有值均 `ON CONFLICT DO NOTHING`，保持管理员选择，错误值仍由运行时 fail closed。

后续新插件不扩充这个历史文件，缺失值默认 false。完成接纳后删除某个设置键不会再次接纳；此时按关闭处理。待隔离模块没有假开关，不受六模块接纳影响。

若导入数据库时丢失了原 `schema_migrations`，不能凭现有表自动推断来源；应先恢复可验证的迁移账本和备份，不要手改来源表以强制开启功能。

## 本地证据与上线门槛

隔离 SQLite 事务测试覆盖：新装/纯上游/两个历史 fork、宿主半途失败后重试、显式 true/false/错误值保留、整批回滚/重试、已删除配置不重新开启、checksum 变化拒绝、Windows/Linux 行尾一致。宿主 SQL mock 覆盖同连接、锁内顺序以及 Prepare/host/Apply 各阶段失败后解锁与阻断。

这些不是实际 PostgreSQL 升级演练。部署前需在恢复出的隔离副本上验证新装、老库升级、重复启动、多实例并发、失败重试与旧订单履约；先验证备份恢复，再运行迁移。不要以回退版本为理由删除扩展表或账本。

## Stage22 当前接纳边界（2026-09-26）

优惠券/抽奖 marketing-tools 已在前后端真实目录中接纳为 managed，第七个可管理模块；另六组核心二开仍待隔离，整体目标尚未完成。旧阶段中的 pending/managed=false 是当时状态，不代表当前目录。

- 服务端每次通过实际共享设置准入；缺失、错误值或读取失败拒绝新增业务。旧构造器不再隐式放行，测试需要显式提供准入。前端报价、券选择、两个结算插槽及管理/抽奖写操作使用同一真实开关，不保留 pending 绕过。
- 关闭同步清除未提交选券并恢复原生报价；历史三个页面保留原有身份/管理员/支付权限，仍可读取。核销、释放、已支付订单履约重试不因开关中断，不撤销已经准入的请求。
- 新增独立迁移 marketing-tools/202609260002_adopt_marketing.sql；不扩写冻结的 v1 六项名单，也不改历史 SQL。legacy 缺失值补 true，native 缺失值补 false，显式 true/false/错误值均保留。登记后删除设置不因重启再次补写。
- Failed（即使已有 PaidAt）的退款增强仍未实现。已读取上游基线 aea725f2ea644d5592d0bbb1d63b607efa7e200a 的 payment_refund.go：PrepareRefund/ExecuteRefund 同样不接受 Failed。本次保留这一资金政策，不为提供开关擅自扩大退款状态；历史券冲突不能宣称可直接退款。异常权益事实与退款增强仍保留为待办。
- managed 表示真实开关接入，不等于发布验收。营销 Ent 仓储仍由宿主适配；真实 PostgreSQL 并发、浏览器联动、支付网关及上游升级演练均未完成。

### 独立迁移回归范围

隔离 SQLite 已覆盖仅 v1 安装后追加营销接纳、native/legacy、显式值和错误值保留、删除设置不重种、营销账本写入失败仅回滚本次迁移并成功重试。原 v1 六项及账本保持不变。这不是 PostgreSQL 副本升级证明。

### Stage27 多分组接纳

新增独立 `multi-group-billing/202609290001_adopt_multi_group_billing.sql`：legacy 缺失值补 true，native 补 false，显式值保留。该值只影响新密钥配置，既有密钥归属由 202609260003 记录。隔离 SQLite 回归覆盖 native/legacy 与显式值；不是 PostgreSQL 副本升级证明。
