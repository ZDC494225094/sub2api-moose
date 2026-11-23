# 模型脱敏服务（privacy-detector）

给「网关日志存储」上传脱敏用的独立容器。内部运行 [AstrLink](https://github.com/Calcium-Ion/AstrLink) 的 `astrlink-privacy-worker`（Rust + ONNX Runtime）和 AstrLink Guard CPU INT8 中英文隐私模型，外面包一层带密钥的 HTTP 接口。它只识别并返回敏感片段的位置，替换由 sub2api 完成。

## 部署

1. 在 `deploy/.env` 中设置密钥（至少 16 个字符）：

   ```bash
   PRIVACY_DETECTOR_API_KEY=$(openssl rand -hex 32)
   ```

2. 与主编排一起启动（首次会构建镜像，需要编译 Rust 程序）：

   ```bash
   docker compose -f docker-compose.yml -f docker-compose.privacy-detector.yml up -d --build
   ```

   使用 `docker-compose.local.yml` 时替换第一个 `-f`。服务只加入 `sub2api-network`，不暴露端口。

3. 后台 → 网关日志存储 → 采集配置 → 「模型脱敏服务」：开启，地址填 `http://privacy-detector:8090`，密钥填同一个值，点「测试脱敏服务」确认后保存。

多节点部署时，可以在一台内网机器上运行该服务：取消 compose 中 `ports` 的注释，并让各节点填写内网地址。不要暴露到公网。

## 构建输入

`Dockerfile` 固定所有外部输入：AstrLink 源码 commit（`ASTRLINK_COMMIT`）、ONNX Runtime 1.23.2（按 SHA-256 校验）、Hugging Face 上 `QuantumNous/astrlink-guard` 的 revision 和每个模型文件的 SHA-256。`astrlink-model.json` 是 AstrLink Core 安装该模型时写出的清单，worker 启动时据此校验文件。

构建需要访问 GitHub、crates.io、NuGet 和 Hugging Face。无法访问 Hugging Face 时设置 `PRIVACY_DETECTOR_HF_ENDPOINT`（例如 `https://hf-mirror.com`），文件仍按 SHA-256 校验，不一致会使构建失败。

升级 AstrLink 或模型时，同时修改 `ASTRLINK_COMMIT`、`GUARD_REVISION`、Dockerfile 中的校验值和 `astrlink-model.json`。

## 资源与性能

- 模型文件约 39 MB；AstrLink 为该模型估计每个 worker 进程约需 256 MB 内存。
- `PRIVACY_DETECTOR_WORKERS`（默认 CPU 核数的一半，最多 4）控制并行进程数，`PRIVACY_DETECTOR_INTRA_THREADS` 控制每个进程的线程数。compose 默认限制 4 CPU / 2 GB。
- 吞吐取决于 CPU。上线前请用真实归档观察「运行状态」里的送检量和归档上传耗时；归档上传会比只用规则时慢。sub2api 按文本哈希缓存结果，会话中重复的历史消息不会重复检测。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DETECTOR_API_KEY` | 必填 | Bearer 密钥，至少 16 个字符 |
| `DETECTOR_ALLOW_NO_AUTH` | `false` | 设为 `true` 时允许不设密钥，仅限隔离的内网 |
| `DETECTOR_LISTEN` | `:8090` | 监听地址 |
| `DETECTOR_WORKERS` | CPU/2，1–4 | worker 进程数 |
| `DETECTOR_INTRA_THREADS` | CPU/进程数 | 每个进程的推理线程数 |
| `DETECTOR_MAX_BODY_MB` | `16` | 单次请求体上限 |
| `DETECTOR_MAX_TEXTS` | `4096` | 单次请求的文本段数上限 |
| `DETECTOR_FRAME_TIMEOUT_SECONDS` | `120` | 单帧推理超时；超时的 worker 会被重启 |

## 接口

```
POST /v1/detect
Authorization: Bearer <key>
{"texts": ["我叫王小明，住在北京市朝阳区"]}

200 {"model": "QuantumNous/astrlink-guard@…#cpu_int8",
     "results": [[{"start": 6, "end": 15, "label": "private_person", "score": 0.97}, …]]}
```

`start`/`end` 是 UTF-8 字节偏移。`label` 取值为 `email`、`phone`、`account`、`payment_card`、`ip_address`、`url`、`common_secret`、`private_address`、`private_date`、`private_person`。长文本按 8 KB、重叠 512 字节切片，与 AstrLink Core 的做法相同。

`GET /healthz` 在至少一个 worker 就绪时返回 200。

## 许可

AstrLink 的 privacy-worker 和 AstrLink Guard 模型采用 Apache-2.0，ONNX Runtime 采用 MIT，许可文件复制在镜像的 `/usr/share/doc/privacy-detector/` 下。本服务不包含 AstrLink Core 依赖的 AGPL 组件 RelayKit。
