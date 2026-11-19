# 网关日志存储（gateway-log-archive）

把网关对话接口的请求体与返回体按会话归档，同模型按天合并为未压缩 JSONL，每行一个完整会话 JSON，按优先级上传到一个或多个 S3 兼容存储桶。扩展与采集默认关闭；上传时个人信息脱敏默认开启。

## 数据流

1. **采集**：`CaptureMiddleware` 挂在每条网关链的 API Key 鉴权与模型白名单之后、合成路由改写之前，所以记录的是客户端原始请求字节。HTTP 响应通过 writer tee 复制；Responses WebSocket 通过 Hijack 后的连接旁路解析 RFC 6455 帧（支持 permessage-deflate 上下文接管），不改动任何字节。采集范围：Messages、Chat Completions、Responses（含 `/backend-api/codex`）、Gemini `generateContent`/`streamGenerateContent`，以及 Antigravity 对应入口。`count_tokens`、`input_tokens` 不采集。
2. **会话**：会话 ID 取客户端会话头（`ExtractClientSessionID`）、Claude `metadata.user_id` 中的 session、OpenAI `prompt_cache_key`，按 (API Key, 会话, 模型归档文件名) 聚合。没有会话 ID 的请求单独成为一个会话。会话空闲超过配置时间（默认 30 分钟）后结束；单个会话文档超过 512 MB 时切分。
3. **落盘**：进行中的会话写入 `DATA_DIR/gateway-log-archive/spool/*.spool`（带 CRC 的追加记录），重启后恢复。
4. **合并**：会话结束后追加为当天该模型 JSONL 中的一行，行尾为 LF，不压缩。`数据_000001.json` 等名称只是后台单条记录的显示/下载名称，文件内不放目录或额外元数据行。默认每文件 10 GB，可配置 64 MB～1 TB；同模型继续追加直到达到上限、跨天、手动结束写入或停用扩展。单行不拆开，最后一行可能使文件略超上限。崩溃后截去未提交尾部，继续追加。相同规则别名可合并多个模型变体以减少小文件。
5. **上传**：`<前缀>/<YYYY-MM-DD>/<模型>-<YYYY-MM-DD>.jsonl`，达到上限时滚动为 `-part002.jsonl`。后台直接读取本地原文件，逐条转换、脱敏并发送，不另写整份临时文件。小文件一次上传，大文件在对象存储端分片组装为**同一个未压缩 JSONL**（`application/x-ndjson; charset=utf-8`）。上传时计算各行的偏移、长度和 CRC；整份对象成功后才在同一事务中更新索引、文件大小、状态和统计。失败时保留本地原文件，已开始的分片上传会请求取消。脱敏/转换失败不会回退上传原文。检查到对象已存在时顺延为下一个 part 名；只写凭证无法检查远端时使用已分配的名称。上传成功后默认删除本地文件。
6. **多存储桶**：存储桶保存在 `gateway_log_archive_buckets`，按「优先级」从小到大选择已启用的存储桶；某个存储桶失败会自动切换到下一个（同一轮上传中跳过刚失败的存储桶），全部失败才按退避重试。每个归档文件记录 `bucket_id`。已上传数据的预览、下载从对应存储桶读取，即使本地还保留原始文件，也不会用原始副本覆盖脱敏预览。删除存储桶配置后，其中的文件不能再在后台读取，但对象和统计保留。
7. **上传统计**：`gateway_log_archive_upload_stats` 按（存储桶, 模型）累计文件数、会话数和上传后的实际字节数，与 `MarkUploaded` 在同一事务里更新，重复标记不重复计数。后台索引按保留期清理时这张表不清理，所以「已上传总量」和按模型下钻始终完整。

## JSONL 格式

```json
{"format":"sub2api.gateway-log.v1","session_id":"...","model":"...","model_key":"Opus5",
 "platform":"anthropic","started_at":"...","ended_at":"...","exchange_count":2,
 "exchanges":[{"index":1,"request_id":"...","transport":"http","method":"POST","path":"/v1/messages",
   "status":200,"stream":true,"request":{...原始 JSON...},"response":"event: ...原始 SSE 文本..."}]}
```

以上为便于阅读换行的单条示意，实际文件每条占一行。请求/响应若是 JSON，仅移除字符串外的无意义空白以满足 JSONL，保留键顺序、数字精度、重复键与原始转义。其他文本（SSE）存为 JSON 字符串，关闭脱敏时解码后与原始字节一致。二进制数据用 base64 存储并带 `*_encoding` 标记。响应被 gzip/zstd 压缩时先解压。超过单体上限会截断并带 `*_truncated` 标记。WebSocket 的 `response` 是服务端消息数组。

## 上传时个人信息脱敏

- 配置 `redact_pii`，默认 `true`。只由后台上传任务执行；请求转发、响应输出、采集队列、会话缓存和本地原始 JSONL 不进行脱敏。更改开关影响随后开始准备的上传，不追溯修改已上传对象。
- 明确的个人字段：姓名（`full_name`、`real_name`、`first_name` 等）、邮箱、电话、身份证/护照号码、银行卡号、家庭/收货地址、出生日期；`name`、`address` 只在 `person`、`profile`、`customer` 等个人对象下按个人信息处理。字符串替换为 `[REDACTED]`，明确个人字段中的数字替换为 `0`，保留字段结构。
- 正文识别邮箱、中国大陆手机号（含 `+86` 前缀）、通过日期与校验码验证的 18 位身份证号，以及带明确标签的姓名、住址、电话、证件/银行卡号。嵌套 JSON 文本和 SSE 的 `data:` JSON 使用相同规则，保留事件与换行；WebSocket JSON 消息逐条处理。
- 普通模型名、工具名、请求 ID、数字、URL、代码等内容不会被整段清空；工具 schema 中的字段声明不当作真实个人资料。仅含个人信息的字符串需要重新转义，未改变的字符串 token 原样写回。
- 这是确定性规则脱敏，不使用外部模型。无法保证识别没有标签的姓名/住址、被拆到多个流式片段里的信息、图片/音频/base64 中的信息或所有语言/国家的格式。后台页面明确显示该范围。

## ZIP 升级兼容

- 新归档始终为 JSONL。旧 `max_zip_size_mb` 配置按原值读取为 `max_file_size_mb`，再次保存时写入新字段，不需要新增数据库迁移。
- 未结束写入的旧 ZIP 在重启恢复时结束写入，不再追加新会话；待上传和失败重试的 ZIP 在上传阶段转换为 JSONL，并按当前开关脱敏。
- 已上传的 ZIP 保持不变，仍可预览与下载。既有表的 `header_offset`、`compressed_size`、`uncompressed_size` 沿用；JSONL 的 offset 为行起点，两种 size 均为该行 JSON 的字节数（不含 LF）。

## 处理旧版 `.upload-tmp: no space left on device`

这个报错发生在持有归档的网关节点：旧版要在本地原文件旁再写一份脱敏后的完整文件。升级时保留该节点的数据卷及 `node-id`，**不要删除待上传的 `.jsonl`、`.zip` 或 `spool/`**。新版本不需要这份额外空间；重试时会尝试移除该归档对应的旧临时文件。无需关闭脱敏或切分旧文件。

1. 在报错的节点/容器内运行 `df -h /app/data/gateway-log-archive` 和 `df -i /app/data/gateway-log-archive`，检查可用空间与 inode。若数据卷本身仍满，先扩容或释放其他已确认可清理的空间。新归档和待上传积压仍占用磁盘，低于配置阈值会暂停采集。
2. 更新持有该原文件的节点，再到「网关日志存储 → 归档文件」选择该文件的「重试上传」，或等待自动退避重试。未使用新版本的节点仍走旧上传方式。
3. 在文件列表确认状态为「已上传」，并从「会话数据」预览其中一条记录。「最近错误」保留历史报错时间，判断该次重试以文件状态为准。

## 隐私与安全

- 归档 JSON 中不包含请求头、API Key 或用户 ID。用户 ID 与 Key ID 只写入后台索引表，用于检索。Gemini `?key=` 等查询参数中的凭证会被移除。
- 请求/响应正文本身就是用户对话内容，开启前请确认符合站点的用户协议与隐私政策。
- 每个存储桶的 AK/SK/Session Token 用宿主 `SecretEncryptor` 加密存储，接口只返回脱敏 AK 与“是否已设置”。
- 升级时，旧版单存储桶配置在首次读取存储桶时自动迁移为「默认存储桶」：凭证按密文原样复制，配置行用比较并交换标记已迁移，多实例不会重复迁移。
- 管理接口挂在 admin 鉴权、限流、审计链之下，并受扩展开关保护。

## 多实例

每个实例在数据目录保存一个 `node-id`，只上传自己本地的归档文件。多个实例同时写同一天同一模型时，会通过唯一约束分配不同的 part 号，因此可能出现多个 part 文件。

新 JSONL 行带 `scope=node_fragment`、`node_id`、`segment_id`、`correlation_id`，明确节点片段及跨节点会话关联。共享数据库不自动合并本地正文；后台「会话数据 → 合并会话导出」按同一用户、API Key 和会话 ID 汇总已上传、仍保留索引的片段，还原 SSE/WS 原生消息并附上完整性诊断。有未上传片段时拒绝部分导出；活跃或未入库请求无法证明已收齐。

详见 [多节点会话完整性与部署方案](MULTI_NODE.md)，包括五类问题归因、粘性路由/Redis/数据卷建议，以及需要全自动聚合时的后续架构。
