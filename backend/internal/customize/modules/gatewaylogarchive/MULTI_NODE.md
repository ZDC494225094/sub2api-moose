# 多节点共享数据库：会话完整性与部署方案

## 结论与适用范围

当前实现的 PostgreSQL 共享的是归档索引、配置及上传状态，正文保存在各节点自己的 `spool/` 和 JSONL 文件中。一个客户端会话被负载均衡分配给 A、B 两个节点时，A/B 分别记录自己的片段。数据库唯一约束只分配文件 part 号，不合并正文。

以下结论来自当前源码与本地测试；没有读取生产负载均衡、Redis 或线上异常日志，不能据此断言五类问题全部由多节点引起。本次修改针对日志归档、导出与诊断，不改网关实际转发协议。

| 现象 | 多节点的影响 | 还需要检查的独立原因 |
| --- | --- | --- |
| 执行链路不完整 | 同一 session 的请求分布到多个节点；只查看一个 JSONL 行得到片段 | 没有稳定 session ID、按模型分组、30 分钟空闲结束、512 MB 会话分片、仅保留成功请求、队列丢弃、节点磁盘未持久化、索引过期 |
| 缺思考/推理签名 | 签名所在轮次可能在其他节点；某些网关内存状态也不随数据库共享 | 模型/协议没有返回签名、未开启思考、SSE `signature_delta` 未拼接、协议转换、账号切换、日志截断；OpenAI reasoning 文本不等于 Anthropic signature |
| assistant 轮数不足 | 只统计单节点片段会少算 | 响应原本是 SSE/WS 增量，没有还原为最终 assistant；请求中的历史 assistant 不应重复算成新生成轮次；客户端只提交摘要/增量历史 |
| 缺合法 tool_use | 调用与结果可分散在不同节点 | Anthropic `tool_use` / OpenAI `tool_calls`、`function_call` / Gemini `functionCall` 格式不同；参数分块未拼接、调用 ID 错配、上游实际没调用工具 |
| 上下文截断 | 片段缺失看起来像上下文变短；节点本地续接缓存切换可另有影响 | 日志单体上限（默认 64 MB，最高 256 MB）、预览只显示前 1 MB、模型 token 上限、客户端压缩/摘要、断流；共享数据库不能恢复这些内容 |

## 本次已落地

### 1. 可关联的节点片段

新 JSONL 行额外包含：

- `scope: "node_fragment"`：明确这是一段节点归档。
- `node_id`：节点来源。
- `segment_id`：每次新建片段的唯一标识。
- `correlation_id`：同一个 API Key + 会话 ID 的稳定 SHA-256 标识，跨模型/节点一致，供离线聚合使用；不含 API Key 密钥。

保持原始 `request` / `response` 结构；不因归档需求重写线上请求。没有客户端会话 ID 的请求仍是独立会话，不按 IP、用户或模型猜测合并。

### 2. 跨节点合并导出

后台「会话数据」每行的 **合并会话导出**，或管理员接口：

```text
GET /api/v1/admin/gateway-log-archive/entries/{id}/session
```

以所选条目为入口，按 `(user_id, api_key_id, session_id)` 从共享数据库检索所有仍在索引内的片段，不限制节点、日期或模型。一个 SQL 查询取得索引与文件状态快照，避免上传提交期间把旧偏移配到新对象。只读取各存储桶中的已上传版本，因此不会重新脱敏，也不会用本地原文替代上传的脱敏版本。

- 有已入库但未上传的片段：拒绝导出并返回数量，避免悄悄丢掉其他节点的数据。
- 排序：请求开始时间、请求 ID、条目 ID、片段内顺序。相邻请求时间重叠时提示并发，排序不冒充严格因果顺序。
- 去重：只去掉字节相同的 exchange。同一个 request ID 但正文不同仍保留，并标记重复 ID。
- 大小限制：输入和最终结果各限 2 GB，最多 10,000 个片段 / 100,000 轮；超限报错，不截断输出。逐轮处理，临时文件落盘，内存不承载整个会话；每节点同时仅准备一个导出。
- `scope: "uploaded_indexed_fragments"`：表示当前可查询的已上传片段快照。活跃 spool、从未采集/被过滤的请求、已清理的索引及后续新轮次不在其中。不会声称“会话一定完整”。

在线大文件仍按节点、模型聚合到大 JSONL；这个接口不会把存储桶原对象合并或改写。需要所有上传文件自动成为完整跨节点会话时，应实施下方集中聚合方案。

### 3. 流式消息还原与诊断

导出的每个 exchange 保留原始字段，额外增加 `analysis.normalized_response`：

- Anthropic：拼接 `text_delta`、`thinking_delta`、`signature_delta`、`input_json_delta`，还原原生 content blocks。参数不合法时保留其字符串并报告错误，不用 `{}` 伪装成有效调用。
- Chat Completions：按 choice/tool index 拼接正文、reasoning 与并行工具调用参数。
- Responses / WebSocket：保留最终 `response.output`（含 `encrypted_content`）；没有完整快照时使用已观察到的 item/content/summary 事件。
- Gemini：保留各个 part、`thoughtSignature`、`functionCall` 原生字段；不强制转换为 Anthropic tool_use。

主要诊断：

| 字段/警告 | 含义 |
| --- | --- |
| `assistant_turns` | 观察到 assistant 输出的 exchange 数；单 exchange 多 choice 按一轮，不把历史重放叠加 |
| `request_assistant_messages` | 当前请求携带的历史 assistant 数，单独记录 |
| `signatures` | 观察到的签名/加密推理字段数量；不代表密码学验证成功，历史重放可能重复计数 |
| `valid_tool_calls` | 结构正确的工具调用数量；有 ID 的按 ID 去重，不代表执行成功或通过工具 schema 验证 |
| `capture_truncated` | 采集副本被截断 |
| `output_token_limit` | 上游 finish/stop reason 表示输出 token 用尽 |
| `stream_terminal_missing` | 没看到预期的流结束信号 |
| `thinking_without_signature` | Anthropic thinking block 没有 signature |
| `reasoning_signature_not_observed` | 有推理内容但未观察到签名；不是所有协议都要求签名 |
| `tool_result_without_matching_call` | 当前快照找不到工具返回对应的调用 |
| `tool_call_without_result_in_snapshot` | 当前快照找不到调用的工具结果，可能仍在执行 |
| `previous_response_not_in_snapshot` | Responses 增量续接的前一响应不在当前快照 |
| `context_compaction_observed` | 观察到了客户端/上游的上下文压缩 item |
| `signed_thinking_content_redacted` | 带签名的 thinking 文本含脱敏标记，不能保证原签名可用于重放 |

无签名、无工具调用不一定是错误。不会生成假的签名、assistant 轮次或 tool_use 来满足校验。

签名值原样保留不等于签名仍可验证：若签名绑定的 thinking 正文因个人信息被脱敏，原签名可能不再匹配。脱敏归档用于分析，不承诺直接重放到上游；不能用伪造签名同时满足脱敏与原始签名有效性。

### 4. 截断与归档保护

- `signature`、`thoughtSignature`、`encrypted_content`、协议 ID、工具关联 ID 不参与邮箱/手机正则替换，防止破坏协议关联或签名。签名是不可读协议值，个人字段与正文仍按上传规则脱敏。
- 修复采集队列字节预算的并发预留：先原子预留再入队，失败退回，避免并发请求绕过 512 MB 预算而导致更高内存压力。
- WebSocket 中途的客户端 follow-up 同样受日志副本预算限制，超限带截断标记；原连接字节照常通过。
- 解压超过 512 MB 时保留原压缩数据与编码标记，避免以前把静默截断的解压前缀误当完整正文。

## 生产建议：先做这一组

1. **稳定会话标识贯穿所有轮次。** 所有客户端统一传 `X-Session-Id` 或对应原生 session header，同一会话不能每轮换 ID。Claude Code 可用 `X-Claude-Code-Session-Id`。LB 必须保留 header。
2. **负载均衡按 API Key + 会话 ID 做一致性哈希/粘性。** 会话 ID 的读取优先级要与应用一致；不要只按 IP，也不要把缺失 ID 的所有请求 hash 到同一节点。原生不同客户端 header 可由受控入口规范为同一个会话字段。保留原始鉴权信息的正确转发，不把 API Key 写入访问日志。
3. **同一 Redis 集群/逻辑库与命名空间。** 共享 PostgreSQL 不等于共享 Redis；已有账号粘性/网关缓存依赖共享 Redis。另外仓库仍有进程内 `DigestSessionStore`，所以 Redis 共享也不能保证全部续接状态跨节点可用，入口粘性仍有价值。
4. **每节点独立、持久化的数据卷及唯一 node-id。** 不要克隆正在使用的数据目录到另一个并行节点，不要让多个进程并发写同一 spool 目录。节点替换先排空请求、结束写入并完成上传，再移走旧卷。保证节点时钟同步。
5. **所有节点使用同一版本、归档配置与可用存储桶凭证。** 需要读到其他节点上传对象；原始文件仍按大 JSONL 聚合，所以刚结束的会话可能等待跨天/容量阈值才上传。后台「立即结束写入并上传」只作用于接收该请求的节点，不是集群广播。
6. **完整链路排查时关闭“只存储成功的请求”。** 按需要将日志单体上限从 64 MB 调到 128/256 MB，并查看 `dropped`、`skipped_status`、磁盘剩余空间。不要无界提高内存；日志原文已被截掉的部分无法事后恢复。
7. **区分日志与模型问题。** 日志 `request_truncated=false` 但请求本身只带摘要，说明缺失发生在客户端；`finish_reason=length`/`stop_reason=max_tokens` 是上游输出限制；日志完整却没有签名，应检查实际请求协议、模型能力、思考开关和协议转换。

粘性在节点故障/扩缩容时会失效，所以它减少拆分，但不能提供无丢失的完整链路保证。

## 需要全自动完整会话时的后续架构

建议增加独立归档聚合服务，不让数据库存每次的大请求正文，也不让请求线程等待归档上传：

```text
网关 A/B/C → 本地持久化 WAL/outbox → 后台发送（会话 key 分区）
                               ↓
                       持久化消息队列/对象暂存
                               ↓
                 单会话归属的聚合 worker（租约 + 幂等）
                               ↓
               会话结束/水位到达 → 个人信息脱敏 → 大 JSONL → S3
```

- 每轮独立 `event_id`，重试至少一次投递；消费者幂等去重；只有持久化确认后才清本地 outbox。
- 按会话 key 分区；聚合 worker 用租约/围栏令牌避免两个 worker 同时写同一会话；恢复后重放未确认事件。
- 记录 `node_id`、请求开始/结束、`request_id`、`previous_response_id`、`call_id`。客户端若能提供递增 turn index/parent event ID，可支持因果顺序；没有这些信息不能把并发分支强行串成线性对话。
- 明确会话结束、空闲窗口、迟到数据与修订版本；只有收齐水位/结束确认才标记 complete，其余是 partial。
- 持久化队列可用 Redis Streams + 持久化/复制/确认/重试机制，也可选 Kafka 等。只有共享 Redis 或增加 `SETNX` 锁并不能合并散在本地磁盘的正文。
- 集中聚合后再按模型/日期合并成大 JSONL，脱敏仍仅在上传任务进行。当前网关快路径保持异步采集；若要求进程崩溃也绝不丢最后一轮，需要额外选择持久化 ACK 的延迟与吞吐成本。

本次未部署该集中聚合基础设施，也未更改生产 LB、Redis、数据卷或模型配置。
