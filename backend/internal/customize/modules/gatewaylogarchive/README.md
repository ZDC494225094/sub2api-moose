# 网关日志存储（gateway-log-archive）

把网关对话接口的完整请求体与返回体按会话归档，同模型按天打包为 ZIP，滚动上传到 S3 兼容存储桶。默认关闭。

## 数据流

1. **采集**：`CaptureMiddleware` 挂在每条网关链的 API Key 鉴权与模型白名单之后、合成路由改写之前，所以记录的是客户端原始请求字节。HTTP 响应通过 writer tee 复制；Responses WebSocket 通过 Hijack 后的连接旁路解析 RFC 6455 帧（支持 permessage-deflate 上下文接管），不改动任何字节。采集范围：Messages、Chat Completions、Responses（含 `/backend-api/codex`）、Gemini `generateContent`/`streamGenerateContent`，以及 Antigravity 对应入口。`count_tokens`、`input_tokens` 不采集。
2. **会话**：会话 ID 取客户端会话头（`ExtractClientSessionID`）、Claude `metadata.user_id` 中的 session、OpenAI `prompt_cache_key`，按 (API Key, 会话, 模型压缩包名) 聚合。没有会话 ID 的请求单独成为一个会话。会话空闲超过配置时间（默认 30 分钟）后结束；单个会话文档超过 512 MB 时切分。
3. **落盘**：进行中的会话写入 `DATA_DIR/gateway-log-archive/spool/*.spool`（带 CRC 的追加记录），重启后恢复。
4. **打包**：会话结束后写成一个 JSON（`数据_000001.json`）追加到当天该模型的 ZIP。ZIP 支持追加、支持 ZIP64，崩溃后按数据库里已提交的长度继续写。达到上限（默认 10 GB）滚动为 `-part002`，跨天或手动「立即封包」时封包。
5. **上传**：`<前缀>/<YYYY-MM-DD>/<模型>-<YYYY-MM-DD>.zip`，大文件走分片上传。对象已存在时顺延为下一个 part 名，不覆盖。失败按退避重试；上传成功后默认删除本地文件。

## JSON 格式

```json
{"format":"sub2api.gateway-log.v1","session_id":"...","model":"...","model_key":"Opus5",
 "platform":"anthropic","started_at":"...","ended_at":"...","exchange_count":2,
 "exchanges":[{"index":1,"request_id":"...","transport":"http","method":"POST","path":"/v1/messages",
   "status":200,"stream":true,"request":{...原始 JSON...},"response":"event: ...原始 SSE 文本..."}]}
```

请求/响应若是 JSON，原样嵌入（不重新编码）。其他文本（SSE）存为 JSON 字符串，解码后与原始字节一致。二进制数据用 base64 存储并带 `*_encoding` 标记。响应被 gzip/zstd 压缩时先解压。超过单体上限会截断并带 `*_truncated` 标记。WebSocket 的 `response` 是服务端消息数组。

## 隐私与安全

- 归档 JSON 中不包含请求头、API Key 或用户 ID。用户 ID 与 Key ID 只写入后台索引表，用于检索。Gemini `?key=` 等查询参数中的凭证会被移除。
- 请求/响应正文本身就是用户对话内容，开启前请确认符合站点的用户协议与隐私政策。
- AK/SK/Session Token 用宿主 `SecretEncryptor` 加密存储，接口只返回脱敏值。
- 管理接口挂在 admin 鉴权、限流、审计链之下，并受扩展开关保护。

## 多实例

每个实例在数据目录保存一个 `node-id`，只上传自己本地的压缩包。多个实例同时写同一天同一模型时，会通过唯一约束分配不同的 part 号，因此可能出现多个 part 文件。
