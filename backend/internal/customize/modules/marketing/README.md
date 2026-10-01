# 优惠券 / 抽奖模块（开关已接纳，完整隔离与发布验收仍进行中）

本目录拥有券模型、仓储接口、发券/报价/预留/核销/释放规则以及抽奖资格、次数、奖品选择规则。模块不导入宿主 service、Ent 或开关目录；宿主通过窄端口提供事务、钱包和原生兑换码服务。兑换码以泛型保留宿主原生 DTO，避免复制旧版宿主字段。

## 宿主边界

- 旧 service 类型和构造器为兼容桥接；依赖注册集中在 customize/wiring，不把营销 providers 放回核心 Wire 集合。
- lotteryEntTransactions 负责 Ent 事务；marketingSQL 使营销仓储实际使用该事务连接，不只是接收 context 后仍走根连接池。
- 原生兑换码 Create 尊重事务 context；钱包事务内仅读取余额，扣款以余额条件更新防止不同活动重复花费。
- payment_marketing.go 是订单接入点：订单写入、券预留和折扣快照在同一事务内提交；报价变化时回滚并要求重新确认。普通无券订单不调用券服务。
- 历史核销/释放不依赖开关。宿主 consumePaymentCoupon 为核销建立真实事务，或复用调用者事务；完成状态与核销、取消/过期状态与释放分别同事务提交，外部供应商调用和通知不放入事务。履约失败仍允许重试，因此保留券占用。

- HTTP Handler 只依赖模块接口与宿主提供的用户 ID 解析器，不导入宿主 service/middleware。用户 ID 来自原生鉴权上下文，不读取请求中伪造的 user_id。宿主扩展助手在原有鉴权、限流、管理权限和审计分组内注册，未向 public/webhook 挂载。

## 当前证据及限制

marketing_tx_test.go 用真实仓储、真实 Ent 和 SQLmock，将连接池限制为一个连接，覆盖三类奖品提交/失败回滚、条件扣款失败、预留竞争失败、折扣插入失败及历史核销/释放。逃逸到根连接池会失败，不能以仅传递 context 的测试冒充事务验证。

payment_marketing_test.go 用 SQLite 实际验证订单和两个扩展写入共同提交/回滚，覆盖报价变化、服务/报价缺失及普通上游路径。另覆盖历史核销两次写入的共同提交、第二次写入失败回滚、外层事务回滚及重试。它不是 PostgreSQL 并发或真实支付验证。

新增 HTTP 回归覆盖身份、防伪造 ID、绑定失败、冲突状态码、泛型奖品 JSON、券过滤与分页；宿主真实注册助手回归覆盖所有 14 路由的鉴权/管理员/限流/审计继承及无活动模块时的独立注册。

## 当前接纳边界（Stage20）

- 规则、HTTP、三个页面、14个API方法、结算选择器/报价/订单参数已隔离；生产装配已注入新增业务准入，七项管理服务写入口也已受保护。前端支付、抽奖、管理页在准入关闭时停止新业务并保留历史查询。目录仍managed=false，当前部署保持pending兼容行为，不展示假开关。
- 取消/过期释放和正常完成核销与宿主状态同事务；履约失败不释放券。迟到支付仅按历史快照恢复归属，不抢占他单券；Completed修复不重新发权益。创建响应失败保留可查单状态。
- 已完成订单的退款延续现有政策：不自动返还已用券、不删除原折扣快照。Stage20验证宿主实际退款结算事务的全额/部分退款、审计失败回滚及重试，未新增空操作退款钩子或改变资金政策。
- **已支付但履约失败仍是明确待办**：PrepareRefund目前不接纳Failed状态，即使存在PaidAt。券归属冲突因此不能被文档描述为“管理员已可直接退款”。需要按实际发放证据区分是否扣回权益，再补支持与回归；不得简单放宽状态或凭PaidAt判定已经发放。
- Overview为纯SELECT；默认次数仅预览，发放在Draw事务内。无活动时仍可查询历史记录。正式managed接纳时仍需维护只读路由/导航，避免共享关闭重定向挡住历史。
- 尚待独立接纳迁移、完整开关验收、真实PostgreSQL并发、实际支付/退款和上游隔离合并演练；其他pending模块仍在整体目标内。

## 已支付订单的券恢复边界

ReconcilePaidOrderCoupon 只使用原订单快照与券所有权，不按当前模板/有效期重新报价，不抢占另一订单的券。宿主先持久化支付事实，再在取得履约租约后开启独立短事务恢复券占用和 released 快照；任一步失败整体回滚，充值/订阅写入不执行。正常无折扣订单不改变权益。冲突保留已支付事实和原券归属，需要退款或消除归属冲突后重试，不自动补发一张券或重复优惠。

回归覆盖12种归属状态、实际SQLite的恢复失败回滚/重试及两类履约前置阻断；单连接SQLmock覆盖条件恢复命中/未命中和事务连接。未替代真实支付、PostgreSQL并发验证。

## 历史已完成订单和只读抽奖概览

Completed 的回调、余额/订阅重入和租约重试统一在一个事务内核对券归属并修复核销，不重复发放余额/订阅。支持旧版先更新折扣记录后用户券失败的半写入；他单归属冲突保留 Completed 并返回错误，不抢占权益。宿主外层事务可复用，不提前提交。

Overview 不再 GetOrCreate，也不 best-effort 发放次数；纯读取不存在的状态返回未发放零值，资格响应单独表示 pending_default_draw_times，Draw 仍在活动/用户状态锁内发放和扣减。前端独立 marketing/lottery-state 助手读取可用次数与预览，Draw 返回 default_granted 后忽略旧预览，避免重复计数。


## Stage11 前端页面/API/路由边界

用户抽奖、管理优惠券和管理抽奖页面完整迁移；URL、路由名、requiresAuth/requiresAdmin/requiresPayment 元数据及国际化 key 不变。公共 payment API 不再含营销方法，公共 router 不再声明营销路径，由 extensions/routes.ts 统一接入模块 routes.ts。14项API契约回归检查方法、URL、参数和响应 envelope；路由回归检查原有权限和唯一注册。

PaymentView 仍有券选择和计算逻辑，本阶段只通过现有 checkout 扩展入口隔离券加载的传输细节。该过渡端口不等于完整报价插件或可安全关闭的营销插件，managed=false 不变。原始814路径账本未改，另记三条实际页面搬迁和API/路由提取，边界门禁禁止迁出内容回流公共API/router。


## Stage12–13 结算与真实组件回归

PaymentView 的券选择、摘要、资格规则、折扣和请求参数已经进入模块，通过通用 checkout 端口和 pending 插槽接入。门槛/扣券先作用于订单本金，宿主再进行换汇与手续费计算。端口接受目标币种，订阅支付方式列表不再使用未折扣金额判断限额；没有券时仍返回宿主原生报价。

真实 Select 选项点击、CouponSelection/CouponSummary、宿主按钮/限额及 CreateOrder 请求已有集成回归；100本金减20券、7倍CNY汇率和10%手续费报价616，切换USD报价88，传给后端的本金仍为100。99本金不能因换汇或手续费达到100门槛。前端全量337文件2458项通过；这是模拟DOM/API环境，不是真实支付证明。

managed=false保持不变。关闭准入、旧权益保留、供应商失败和退款、真实PG与升级演练仍是完成前置条件。

## Stage14 创建响应失败不改变交易状态

供应商创建请求失败/超时或成功后的本地详情保存失败都不能证明供应商未接受订单。宿主现已移除无条件Failed更新，统一只记录ORDER_CREATE_RESPONSE_FAILED审计并返回原结构化错误；不更新订单状态/支付事实/时间戳，不释放券，不写入原始供应商错误详情。

Pending继续由查单、回调或正常过期/取消处理，已有Paid/Completed/退款等状态不回退。九类状态与审计失败的隔离SQLite测试、真实查单及余额履约测试（创建响应丢失后查单完成，再收到迟到失败也不重复入账）重复10轮通过。此处不把网络不确定性当成可释放券的确定失败，也未修改旧Failed订单修复政策。

关闭准入、退款时券权益政策和历史权益保留仍未完成，模块保持managed=false。完整组合构建结果见根文档Stage14。


## Stage15：营销新增业务准入端口（2026-09-26，尚未接通管理开关）

模块新增NewBusinessAdmission及WithAdmission构造器，发券、券报价/预留、抽奖及默认机会发放在业务读写前检查；抽奖进入宿主事务后再次检查。显式传nil时拒绝准入，开关读取失败原样返回，不缓存开启状态。已有核销/释放接口不查此端口，关闭不能清空历史次数或阻断旧订单履约。

旧构造器暂显式采用pending兼容策略，宿主仍走原构造器，因此当前生产行为不变，marketing-tools仍managed=false。本阶段是服务边界准备，不代表已经可以关闭营销：宿主设置适配/注入、前端历史权益可见策略、退款和完整关闭回归仍待完成。准入不取消已在途事务，不承诺开关和业务提交具备跨请求线性一致性。

新增回归验证禁用/未配置在仓库调用前拒绝、事务内复查、每次请求实时检查以及历史核销/释放绕过开关。原有模块测试连同新增测试重复10轮通过；后续宿主回归与最终门禁结果另行追加。Stage14完整构建为本次新增端口之前的证据，不冒充Stage15完整构建。未部署、未写真实数据库、未操作Git提交或上游。


Stage15最终阶段证据：营销模块完整测试及宿主Coupon/Lottery/支付创建失败/原子生命周期专项均重复10轮通过（.tmp/custom-extensions-stage15-module-tests.log、.tmp/custom-extensions-stage15-host-tests.log）；29项门禁与814路径审计通过（.tmp/custom-extensions-stage15-check.log），git diff --check通过，仅已有换行提示。当前Go源码embed编译通过（.tmp/custom-extensions-stage15-embed.log），产物 .tmp/custom-extensions-stage15-server.exe 未部署。本阶段没有重跑全量Go/前端测试或重新构建前端/画布，复用了Stage14刚完成的静态产物；完整统一构建证据仍明确属于Stage14。下一步为宿主准入适配和依赖注入，不可直接把catalog改为managed=true。


## Stage16：营销准入接入真实依赖装配（2026-09-26）

扩展wiring新增ProvideMarketingAdmission/ProvideCouponService/ProvideLotteryService，Wire重新生成的启动代码将同一准入端口传入券和抽奖服务，不再使用legacy构造器装配生产服务。已知pending条目暂保留旧行为；unknown/缺失状态拒绝，managed分支每次读取共享设置、不缓存开启状态。catalog仍managed=false，不能据此称营销已支持关闭。

抽奖/订单事务内读设置使用tx.Client()，避免宿主设置仓库默认root连接导致小连接池等待或读不到事务状态；此适配限于扩展wiring，没有改写上游settingRepository。单连接SQLite回归验证真实Manager经适配器读取事务内未提交设置、回滚后不留数据。真实Manager测试使用已经managed的充值活动条目，不伪造营销已开放管理。营销managed分支由状态端口测试覆盖，两者证据范围分开。

新增装配回归覆盖：pending显式兼容、未知条目拒绝、开关读取错误传播、每次读取状态、生产provider实际传入两项服务、历史核销/释放不读取开关。首轮SQL测试漏导入Ent runtime导致validator为空，补正确测试初始化后，customize全部包重复10轮通过（.tmp/custom-extensions-stage16-customize-tests-retry.log），未跳过失败测试。后续宿主回归、门禁与编译结果另行追加。

Stage15的“宿主未注入”待办由本阶段完成；用户历史权益可见策略、退款券政策、管理写入口准入、前后端关闭联动、迁移接纳和完整关闭验收仍未完成。仍有其他pending模块、Ent隔离、真实PG并发及实际升级演练；不缩小最终目标。未fetch/merge/commit/push/部署/写真实数据库。


Stage16最终阶段验证：customize全部5个包重复10轮通过；宿主Coupon/Lottery/支付创建失败/原子生命周期专项重复10轮通过（.tmp/custom-extensions-stage16-host-tests.log）；最终check全部通过（.tmp/custom-extensions-stage16-check-final.log），git diff --check退出0。当前源码含重新生成的Wire完成embed构建（.tmp/custom-extensions-stage16-embed.log），.tmp/custom-extensions-stage16-server.exe未部署。没有重跑全量Go或前端测试，也没有重建未改动的前端/画布静态产物；Stage14仍是最近完整11步构建，不冒充本阶段全量证据。


## Stage17：营销管理写入口与历史只读接口（2026-09-26）

在营销service而非页面按钮增加7项管理写准入：券模板创建/更新、抽奖活动创建/更新/删除、奖品创建/更新；拒绝发生在参数解引用和任何仓库调用之前。显式disabled或状态读取失败不允许管理写入。生产仍使用Stage16注入，catalog继续managed=false，不提前提供不完整开关。

新增真实模块路由→handler→生产provider→service回归，分别验证7个写接口在disabled返回403、设置不可用返回503且不触碰仓库；不改变上游鉴权组，本测试只验证准入传播，不冒充完整鉴权测试。用户券、活动概览、用户抽奖记录、管理券模板/活动/记录6个读接口在disabled和状态不可用仍可查询；验证券71的unused历史数据未被清空，用户历史查询仍取原生认证UserID=42而不是query中的99。历史核销/释放沿用已有不查开关的服务边界。

本阶段没有让历史查询重新发放次数，也没有按关闭状态修改旧券、旧次数、奖品库存或订单。customize全部包及新增服务/HTTP回归重复10轮通过（.tmp/custom-extensions-stage17-customize-tests.log）。管理写入口待办已完成；前端只读展示/结算联动、退款策略、接纳迁移和完整开关验收仍待完成，其他pending模块与真实升级演练仍保留完整范围。后续宿主验证与编译证据另行追加。


Stage17最终阶段证据：扩展全部5个包重复10轮通过；宿主优惠券/抽奖及支付原子生命周期专项重复10轮通过（.tmp/custom-extensions-stage17-host-tests.log）；29门禁与原始814路径审计通过（.tmp/custom-extensions-stage17-check.log），git diff --check退出0。当前后端embed编译通过（.tmp/custom-extensions-stage17-embed.log），产物 .tmp/custom-extensions-stage17-server.exe未部署。HTTP回归为内存Gin与仓库替身，不代表真实PostgreSQL或浏览器验收；本轮没有重跑前端或全量Go测试，最近完整11步构建仍是Stage14。未写真实数据库，未拉取上游或提交代码。


## Stage18：支付券贡献的前端关闭联动（2026-09-26，营销仍pending）

营销模块新增useMarketingAdmission，明确区分pending兼容行为和managed共享状态。当前catalog仍pending，保持旧功能；以后完成管理接纳时，同一组件读取已有共享store，未知/刷新失败按关闭处理，不额外创造后台开关或复制公共设置接口。管理接纳必须前后端一起更新，当前测试中的managed目录为隔离替身，不代表已上线开关。

券结算controller接受只读响应式准入，关闭后同步清除当前选择、返回原生报价、隐藏CouponSelection与CouponSummary，新订单移除遗留user_coupon_id，不清空已加载的历史券；重新启用不会自动恢复之前的选券。恢复订单优先保留wechat_resume_token历史字段，不按今天的开关改历史券参数。关闭期间不请求支付选券列表，关闭后才返回的在途列表不覆盖当前状态。后端仍是最终下单准入权威。

新增回归覆盖managed状态缺失/共享刷新失败、原生金额回退、恢复订单保留、重新开启不自动选券、在途请求与真实选择/摘要组件显隐。券组件测试只替换Select下拉部件，已有PaymentView真实Select集成测试继续保留。专项3文件44项通过（.tmp/custom-extensions-stage18-targeted-final-retry.log）；首次新测试文件路径相对cwd错误已修正，随后修复i18n测试替身误覆盖createI18n，没有删除或跳过失败测试。全量前端/类型检查结果另行追加。

本阶段没有修改宿主PaymentView或营销目录可管理状态。抽奖页/管理页只读联动、退款策略、旧安装接纳和完整开关验收仍待完成；其他pending模块与真实升级演练仍在完整目标内。未部署、未修改真实数据库、未拉取或提交上游代码。


Stage18最终阶段证据：前端全量338文件2464项通过，vue-tsc与修改文件ESLint通过；29门禁/814路径审计及git diff --check通过。前端生产构建成功（.tmp/custom-extensions-stage18-frontend-build.log），复制未变更且Stage14已验证的canvas/web/dist后，当前源码embed构建成功（.tmp/custom-extensions-stage18-embed.log），产物.tmp/custom-extensions-stage18-server.exe未部署。本阶段未重建画布、未重跑全量Go，最近完整11步验证仍为Stage14；大包构建警告仍保留。


## Stage19：抽奖页只读联动（2026-09-26，营销仍pending）

LotteryView接入共享营销准入：关闭后禁用抽奖按钮，handleDrawClick/confirmDraw/executeDraw均在发请求前检查；同步关闭尚未提交的钱包确认弹层，重新开启不会自动提交。保留活动概览、历史次数及记录查询，不修改服务端历史权益，也不取消已经提交的抽奖请求或丢弃其结算结果。仅阻止新的抽奖；后端仍为最终准入权威。提示文案位于营销模块，未向上游全局语言文件新增耦合。

新增真实Vue组件回归：关闭下历史读取、三个入口拒绝、钱包确认期间关闭及过期回调、重新开启不重提、开启时正常发送与关闭不重复请求。初轮真实Transition离场动画导致DOM断言时机失败，改为Vue Test Utils的Transition桩后，专项3文件15项通过（.tmp/custom-extensions-stage19-targeted.log）；不是浏览器动画验收。vue-tsc及修改文件ESLint通过，门禁/原始814路径审计与git diff --check通过。生产构建结果另行追加。

本阶段未接纳营销managed、未改变路由关闭策略、未改管理页。管理页只读、退款策略/历史Failed订单审查、旧安装接纳、完整关闭验收以及其余pending模块仍未完成。未重跑全量Go；Stage18前端全量338文件2464项为新增本轮组件测试之前的证据，Stage14仍为最近完整11步验证。未部署、未写真实数据库、未拉取上游、未提交。


Stage19构建收尾：前端生产构建成功（.tmp/custom-extensions-stage19-frontend-build.log），保留大包警告；复制未改动、Stage14已验证的画布产物后，当前源码embed构建通过（.tmp/custom-extensions-stage19-embed.log），产物.tmp/custom-extensions-stage19-server.exe未部署。未重建canvas；本次是前端专项/类型/lint/门禁/构建证据，不冒充完整11步重跑。


## Stage20：营销管理页只读与无活动历史入口（2026-09-26）

优惠券模板页和抽奖管理页接入同一useMarketingAdmission。优惠券新建/编辑/提交/状态切换，以及活动/奖品新建/编辑/提交/状态切换/活动删除，在处理函数入口重新检查准入；删除确认返回后再次检查。按钮同步disabled，开关关闭同步关闭未提交写表单，重新开启不会自动提交。查询、刷新、表格数据和参与记录抽屉保持可用，不清空库存、次数、券或历史记录。已发出的写请求仍由服务端权威处理，不伪装为客户端撤销。

新增MarketingReadOnlyNotice模块内共享中英文提示，抽奖与管理页复用，不修改上游全局语言包。修复无活动时历史入口缺失：LotteryView无active activity仍可打开历史抽屉，以未指定activity_id查询全部本人历史，沿用原生认证接口。

新增admin-admission.spec.ts使用真实DataTable和页面函数，覆盖关闭后全部13个管理处理入口、已打开的4类活动/奖品表单与2类券表单、删除确认跨开关变化、启用后正常写请求、历史参与者可见；仅替换布局、Select和弹窗容器。新增无活动用户历史查询测试。首轮vue-tsc发现抽奖页抽取提示组件后残留旧computed，已删除残留并重新通过类型/修改文件ESLint；未忽略类型错误。最终全量测试与构建结果下附。

营销仍managed=false，以上测试注入准入状态验证关闭分支，不冒充真实管理开关已接纳。服务端准入见Stage16–17；退款/旧Failed支付事实策略、接纳迁移、路由历史访问策略和完整端到端验收仍待完成。其他pending模块、Ent隔离、真实PG和实际上游升级演练保留完整范围。未部署、未写真实数据库、未拉取上游、未提交。


Stage20退款证据补充：新增payment_marketing_refund_test.go，使用历史138迁移中前三张真实券表定义，仅将BIGSERIAL与NOW默认值转换为SQLite方言，未修改原SQL。宿主finishRefund→markRefundOk真实事务覆盖余额/订阅 × 全额/部分 × 正常/审计失败8组，重复10轮通过。逐列快照验证模板、已用券及折扣快照完全不变；审计失败回滚状态，重试仅一条成功审计；准入端口一旦被读取则测试失败。此为结算事务验证，不代表真实网关、余额/订阅扣回或PG验收，未增加no-op钩子。源码确认PrepareRefund当前不接受Failed，即使PaidAt存在；该历史异常退款路径仍待按实际发放证据设计，不能把正常退款测试解释为异常支付也已解决。


Stage20最终验证：最终前端全量340文件2478项通过（.tmp/custom-extensions-stage20-all-frontend-tests-final.log），vue-tsc与修改文件ESLint通过；宿主internal/service全部unit测试及customize全部5包通过（.tmp/custom-extensions-stage20-go-tests.log），其中新增退款8组专项重复10轮通过。29门禁、原始814路径审计与git diff --check通过；前端生产构建成功，复制未改动的Stage14画布产物后，当前源码embed构建成功（.tmp/custom-extensions-stage20-embed.log），.tmp/custom-extensions-stage20-server.exe未部署。保留大包警告；未重跑其他Go包、未重建canvas、没有真实浏览器/PG/网关验收，因此最近完整11步验证仍为Stage14。

### Stage21 历史访问接入

营销历史三页面增加模块内精确路径声明，共享路由守卫、运行时关停监听和侧栏路径过滤统一保留历史访问。仅保留页面读取，不绕过原认证/管理员/支付限制或服务端新增业务准入；新子路径不会自动继承白名单。生产marketing-tools仍pending，测试通过目录替身验证未来managed时关闭/状态失败的行为。

新增异常退款安全回归保护现有拒绝策略及退款/履约的条件抢占边界，没有实现Failed退款。PaidAt或成功审计的有无都不能单独证明权益是否已发；余额兑换先于最终成功审计，订阅分配事务提交后仍有缓存失败点。此项需进一步事实核对及并发验证，不能标记完成。

## Stage22 当前接纳边界（2026-09-26）

优惠券/抽奖 marketing-tools 已在前后端真实目录中接纳为 managed，第七个可管理模块；另六组核心二开仍待隔离，整体目标尚未完成。旧阶段中的 pending/managed=false 是当时状态，不代表当前目录。

- 服务端每次通过实际共享设置准入；缺失、错误值或读取失败拒绝新增业务。旧构造器不再隐式放行，测试需要显式提供准入。前端报价、券选择、两个结算插槽及管理/抽奖写操作使用同一真实开关，不保留 pending 绕过。
- 关闭同步清除未提交选券并恢复原生报价；历史三个页面保留原有身份/管理员/支付权限，仍可读取。核销、释放、已支付订单履约重试不因开关中断，不撤销已经准入的请求。
- 新增独立迁移 marketing-tools/202609260002_adopt_marketing.sql；不扩写冻结的 v1 六项名单，也不改历史 SQL。legacy 缺失值补 true，native 缺失值补 false，显式 true/false/错误值均保留。登记后删除设置不因重启再次补写。
- Failed（即使已有 PaidAt）的退款增强仍未实现。已读取上游基线 aea725f2ea644d5592d0bbb1d63b607efa7e200a 的 payment_refund.go：PrepareRefund/ExecuteRefund 同样不接受 Failed。本次保留这一资金政策，不为提供开关擅自扩大退款状态；历史券冲突不能宣称可直接退款。异常权益事实与退款增强仍保留为待办。
- managed 表示真实开关接入，不等于发布验收。营销 Ent 仓储仍由宿主适配；真实 PostgreSQL 并发、浏览器联动、支付网关及上游升级演练均未完成。

### Stage22 验证结果与未覆盖项

- 前端全量 341 文件、2485 项测试通过；最终 vue-tsc、扩展目录 ESLint、Vite 生产构建通过。
- customize 全部包通过；service 全包 unit 通过；repository 首次因 Windows 同时继承 PATH/Path 而缺失 sh 失败，规范化后全包 unit 通过。统一验证器已加入同名 PATH 合并与 Windows/POSIX 两项回归，实际 repository 通过该环境函数重跑成功。不修改生产准入来绕过测试。
- 门禁 31 项、814 路径所有权审计通过；最终 embed 构建通过，产物 .tmp/custom-extensions-stage22-server.exe 未部署。画布复用未变 Stage14 dist，本轮未重建；前端仍有 Browserslist/混合导入/大包警告。
- 本阶段结果见 .tmp/custom-extensions-stage22-result.json；本轮未重新跑全部 Go 包、完整 11 步或真实 PG/浏览器/支付网关验收，最近完整 11 步仍是 Stage14。未 fetch/merge/提交/部署/写真实数据库。
