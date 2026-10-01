# 多分组密钥与计费优先级（开关已接纳）

## 当前归属

policy.go 拥有分组 ID 去重/主分组顺序、计费优先级规范化、可用分组选择、余额与订阅排序及故障回退规则。模块只依赖 Go 标准库，不依赖 service、repository、Ent、HTTP 或共享扩展设置。

宿主 api_key_group_selection.go 只负责实体视图、协议规范化、缓存分组复用、分组读取、可用订阅查询、实时余额端口及宿主错误映射。Group 和订阅使用泛型携带原有宿主对象，不重新定义一份可能过时的完整 DTO。服务对外方法签名、调用方和错误码不变。

## 必须保持的既有契约

- 主分组优先加入候选，去重并忽略非正 ID，不改写持久化 GroupIDs 数组。
- 使用已缓存的匹配分组；其他候选走 GetByIDLite，不新增重型查询。
- 平台默认值与支持的平台仍由宿主决定；有效的原始平台字符串不因规范化而改写。未知平台沿用首个成功读取分组的推断规则，包括不可用分组。
- 仅存在余额与订阅竞争时读取实时余额；单余额分组由既有 billing preflight 检查，不额外查询。实时读取失败仍使用原用户余额回退，不伪造实时成功。
- 可用订阅由宿主 ListUsableSubscriptionsForGroup 决定，保留原有多份订阅、配额与排序规则；模块选取返回的第一份完整对象。
- 没有可用候选时保留原来的余额/订阅/不可用分组回退顺序，以便宿主继续返回原有状态、余额及订阅错误。返回候选不等于授权，不进行资金扣减。
- 无分组返回空选择；找不到匹配候选仍映射为宿主 NO_USABLE_API_KEY_GROUP。鉴权、管理员、IP ACL、订阅和扣费检查未移除。

## 尚未接纳运行时开关

catalog 中 multi-group-billing 仍 managed=false。不能仅在鉴权选择处绕过本模块就称恢复上游：既有密钥和订阅可能依赖多分组，创建/更新及缓存也携带这些语义。

实际源码已核对的后续边界：

1. APIKeyService.Create/Update 仍含多分组输入和平台一致性验证。需把二开输入与上游单 GroupID 输入区分，关闭后拒绝新扩展配置，普通上游操作不依赖扩展设置服务。
2. repository/api_key_repo.go 写入与读取 GroupIDs/BillingPriority；auth-cache snapshot 与 restore 同样保留两字段，不能仅在请求结构移除字段或在前端隐藏控件。
3. api_key_auth 与 api_key_auth_google 均调用选择策略；两种协议入口必须一起处理历史绑定兼容，不通过停掉历史密钥来实现开关。
4. 尚需定义并持久化新旧密钥策略归属，覆盖创建、更新、删除、缓存失效、关闭后历史读取与新增操作；不得只按当前全局开关重新解释旧计费意图。
5. KeysView 的多选、平台、计费优先级需要独立 UI/输入端口；计费调度的其他增强仍须按固定清单逐项核对，不把本次分组选择提取等同整个模块完成。

没有新增迁移、没有修改历史 SQL、没有写真实数据库。后续接纳必须使用新模块迁移，不复写旧 v1 名单。


## Stage24 输入隔离进度

binding.go 已拥有创建/更新绑定输入计划；宿主 api_key_group_binding.go 保留实际权限检查、协议规范化与字段掩码应用。上述后续边界第1项的输入隔离已完成，但新旧策略归属及新增配置准入仍未实现，不因此开放开关。


## Stage25 分组生命周期隔离

group_lifecycle.go 和 group_lifecycle_sql.go 拥有分组移除/替换规则与原PostgreSQL批量语句；repository/api_key_group_binding.go 保留事务选择及Ent实体适配。历史密钥所属分组删除/迁移不读取插件开关；新增业务准入和策略归属仍待实现。SQL mock不代表真实PG验收。

## Stage26 路由归属

策略归属已通过模块Ent mixin和独立新增迁移持久化，覆盖仓储及鉴权缓存版本25。旧行与当前pending阶段新建默认multigroup-v1；显式upstream-v1旁路自定义选组但仍经过宿主鉴权检查；未知归属拒绝。普通更新与软删除不改变归属。此前章节“未新增迁移/归属待实现”是当时阶段记录，不再代表当前实现。

本模块仍pending：新增准入、更新输入分派、UI隔离和真实PG演练未完成，不可把上述归属字段等同已支持全局开关。

## Stage27 开关接纳

admission.go 拥有创建归属决策（DecideCreate）与按归属的更新准入（CheckUpdate），只依赖标准库。宿主 api_key_group_binding.go 通过 APIKeyRoutingAdmission 端口读取开关并映射错误码；wiring/key_routing_admission.go 提供生产实现。上文"尚未接纳运行时开关"一节是 Stage26 前的记录。

- 开关只影响新配置；归属一经创建不可变。
- upstream-v1 密钥永不获得多分组配置；multigroup-v1 密钥关闭时可使用原有分组的任意子集（任意顺序），不能新增分组或新改为订阅优先。
- 计费/调度等共享增强已拆到 pending 条目 billing-scheduling，本开关不控制它们。
- 尚未完成：标准模式鉴权专项、真实 PostgreSQL 迁移演练、浏览器与网关验收。
