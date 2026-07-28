# 体验中心调用 gpt-image-2 接口规范

> 文档状态：按当前仓库实现整理
>
> 更新日期：2026-07-23
>
> 适用范围：用户端体验中心的图片生成、图片编辑及其后端任务调用链

## 1. 接口结论

体验中心采用“后台任务 + 轮询 + 图片下载”的调用方式，不让浏览器直接长时间等待图片接口。

| 层级 | 接口 | 鉴权 | 用途 |
| --- | --- | --- | --- |
| 体验中心任务层 | `POST /api/v1/playground/runs` | 用户 JWT | 提交生成或编辑任务 |
| 体验中心任务层 | `GET /api/v1/playground/runs/{id}` | 用户 JWT | 轮询任务状态 |
| 体验中心任务层 | `GET /api/v1/playground/runs/{id}/images/{index}` | 用户 JWT | 下载任务生成的图片 |
| 体验中心任务层 | `DELETE /api/v1/playground/runs/{id}` | 用户 JWT | 取消任务 |
| OpenAI 兼容网关层 | `POST /v1/images/generations` | API Key | 无输入图时生成图片 |
| OpenAI 兼容网关层 | `POST /v1/images/edits` | API Key | 携带输入图时编辑图片 |

当前体验中心不调用 `/v1/images/generations/async`。该接口是另一套依赖对象存储的公共异步图片任务协议，不能与 `/api/v1/playground/runs` 混用。

## 2. 调用链

```text
浏览器
  -> POST /api/v1/playground/runs
  -> 后端立即返回 202 和 run id
  -> 后端后台任务通过本机监听地址调用网关
       无输入图 -> POST /v1/images/generations
       有输入图 -> POST /v1/images/edits
  -> 浏览器约每 900 ms 轮询 GET /api/v1/playground/runs/{id}
  -> succeeded 后按 assetIndex 下载图片二进制
```

后台任务调用网关时使用服务实际监听地址，不经过公网域名或 CDN，避免长时间生图请求产生回源绕行和连接重置。

## 3. 调用前提

1. 用户已登录，任务层请求携带用户访问令牌：`Authorization: Bearer <USER_JWT>`。
2. 请求体中的 `apiKey` 是用户在体验中心选中的可用 API Key，例如 `sk-...`。
3. API Key 必须处于启用状态，所属分组必须启用 `allow_image_generation`。
4. 分组下必须存在支持 `gpt-image-2` 的可用 OpenAI 账号，并满足余额、并发和限流要求。
5. 推荐显式传递模型名 `gpt-image-2`；当前还支持固定版本名 `gpt-image-2-2026-04-21`。

## 4. 提交体验中心任务

### 4.1 请求

```http
POST /api/v1/playground/runs HTTP/1.1
Authorization: Bearer <USER_JWT>
Content-Type: application/json
```

纯文本生成示例：

```json
{
  "id": "run-7a976ce9-8ea2-4bba-a5ae-2f26c7a104ea",
  "mode": "image",
  "apiKey": "sk-user-api-key",
  "platform": "openai",
  "model": "gpt-image-2",
  "prompt": "一张现代中式客厅的室内设计效果图，白天自然光",
  "size": "2048x2048",
  "n": 2,
  "quality": "high",
  "outputFormat": "png",
  "images": []
}
```

图片编辑示例：

```json
{
  "id": "run-819ad5ae-b1ef-48c8-ac1e-b4066d8ecba8",
  "mode": "image",
  "apiKey": "sk-user-api-key",
  "platform": "openai",
  "model": "gpt-image-2",
  "prompt": "保留主体，将背景替换为纯白摄影棚背景",
  "size": "1536x1024",
  "n": 1,
  "quality": "high",
  "outputFormat": "png",
  "images": [
    {
      "name": "source.png",
      "type": "image/png",
      "dataUrl": "data:image/png;base64,<BASE64_DATA>"
    }
  ]
}
```

### 4.2 请求字段

| 字段 | 类型 | 必填 | 约束与说明 |
| --- | --- | --- | --- |
| `id` | string | 否 | 幂等任务 ID；不传时由后端生成，最大 128 个字符。同一用户重复提交同一 ID 会返回已有任务 |
| `mode` | string | 是 | 图片任务固定为 `image` |
| `apiKey` | string | 是 | 体验中心选中的 API Key；用于后台调用兼容网关 |
| `platform` | string | 否 | `gpt-image-2` 建议传 `openai` |
| `endpointBase` | string | 否 | 网关路径前缀，默认 `/v1`；只表示路径，不用于指定远程主机 |
| `displayEndpoint` | string | 否 | 当前图片任务链路不使用，调用方不要依赖该字段 |
| `model` | string | 是 | `gpt-image-2` 或其固定版本名 |
| `prompt` | string | 是 | 图片生成或编辑指令；任务层不主动裁剪文本，最终由上游校验 |
| `size` | string | 否 | `auto` 或 `<width>x<height>`；默认/非法格式最终按 `auto` 处理 |
| `n` | integer | 否 | 单任务图片数，体验中心范围为 `1` 到 `4`；小于等于 0 按 1，大于 4 按 4 |
| `quality` | string | 否 | `auto`、`low`、`medium`、`high`；`auto` 不向上游显式传递，默认 `auto` |
| `background` | string | 否 | 背景参数；空值或 `auto` 不向上游显式传递。当前体验中心 UI 未开放此项 |
| `outputFormat` | string | 否 | `png`、`webp`、`jpeg`，默认 `png` |
| `images` | array | 否 | 输入图片数组。非空时任务自动切换到 `/v1/images/edits` |
| `images[].name` | string | 是 | 上传文件名，如 `source.png` |
| `images[].type` | string | 否 | MIME 类型，如 `image/png` |
| `images[].dataUrl` | string | 是 | 必须是 `data:image/...;base64,...` 格式 |

注意：体验中心的 `outputFormat` 使用驼峰命名；OpenAI 兼容网关使用下划线命名 `output_format`。

### 4.3 提交响应

HTTP 状态为 `202 Accepted`，原始响应带统一管理面包裹：

```json
{
  "code": 0,
  "message": "accepted",
  "data": {
    "id": "run-7a976ce9-8ea2-4bba-a5ae-2f26c7a104ea",
    "mode": "image",
    "status": "queued",
    "model": "gpt-image-2",
    "createdAt": "2026-07-23T10:00:00+08:00",
    "updatedAt": "2026-07-23T10:00:00+08:00"
  }
}
```

前端 `apiClient` 会自动去掉 `{ code, message, data }` 包裹，因此业务代码拿到的是 `data` 对象本身。

## 5. 轮询任务

```http
GET /api/v1/playground/runs/{id} HTTP/1.1
Authorization: Bearer <USER_JWT>
```

任务状态：

| 状态 | 是否终态 | 说明 |
| --- | --- | --- |
| `queued` | 否 | 已创建，等待后台协程执行 |
| `running` | 否 | 正在请求上游 |
| `succeeded` | 是 | 生成完成，可读取 `images` |
| `failed` | 是 | 生成失败，原因在 `error` |
| `canceled` | 是 | 已被调用方取消 |

成功示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "run-7a976ce9-8ea2-4bba-a5ae-2f26c7a104ea",
    "mode": "image",
    "status": "succeeded",
    "model": "gpt-image-2",
    "images": [
      {
        "assetIndex": 0,
        "mimeType": "image/png",
        "width": 2048,
        "height": 2048
      },
      {
        "assetIndex": 1,
        "mimeType": "image/png",
        "width": 2048,
        "height": 2048
      }
    ],
    "createdAt": "2026-07-23T10:00:00+08:00",
    "updatedAt": "2026-07-23T10:01:12+08:00",
    "completedAt": "2026-07-23T10:01:12+08:00",
    "durationMs": 72134
  }
}
```

如果上游直接返回图片 URL，图片项可能包含 `url` 而不是 `assetIndex`。调用方应优先使用 `url`；没有 `url` 时再通过 `assetIndex` 下载二进制。

失败示例：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "run-7a976ce9-8ea2-4bba-a5ae-2f26c7a104ea",
    "mode": "image",
    "status": "failed",
    "model": "gpt-image-2",
    "error": "No available compatible accounts (HTTP 503)",
    "durationMs": 1284,
    "createdAt": "2026-07-23T10:00:00+08:00",
    "updatedAt": "2026-07-23T10:00:01+08:00",
    "completedAt": "2026-07-23T10:00:01+08:00"
  }
}
```

推荐轮询间隔为 1 秒左右。当前前端使用 900 ms，最长等待 45 分钟。

## 6. 下载图片

当图片项包含 `assetIndex` 时：

```http
GET /api/v1/playground/runs/{id}/images/{assetIndex} HTTP/1.1
Authorization: Bearer <USER_JWT>
```

成功时直接返回图片二进制，不使用 JSON 包裹：

```http
HTTP/1.1 200 OK
Content-Type: image/png
Cache-Control: private, no-store
X-Content-Type-Options: nosniff
```

`id` 不存在、任务未完成或索引越界时返回 `404`。

## 7. 取消任务

```http
DELETE /api/v1/playground/runs/{id} HTTP/1.1
Authorization: Bearer <USER_JWT>
```

非终态任务会被标记为 `canceled`，`error` 为 `request canceled`。已经进入终态的任务保持原状态。

## 8. gpt-image-2 实际上游请求

### 8.1 无输入图：图片生成

体验中心将单张请求转换为：

```http
POST /v1/images/generations HTTP/1.1
Authorization: Bearer <API_KEY>
Content-Type: application/json
```

```json
{
  "model": "gpt-image-2",
  "prompt": "一张现代中式客厅的室内设计效果图，白天自然光",
  "size": "2048x2048",
  "n": 1,
  "quality": "high",
  "output_format": "png",
  "response_format": "b64_json"
}
```

### 8.2 有输入图：图片编辑

```http
POST /v1/images/edits HTTP/1.1
Authorization: Bearer <API_KEY>
Content-Type: multipart/form-data; boundary=...
```

表单字段为：

| 字段 | 内容 |
| --- | --- |
| `model` | `gpt-image-2` |
| `prompt` | 编辑指令 |
| `size` | 归一化后的尺寸 |
| `n` | 固定为 `1` |
| `response_format` | 固定为 `b64_json` |
| `quality` | 非 `auto` 时传递 |
| `background` | 非 `auto` 时传递 |
| `output_format` | `png`、`webp` 或 `jpeg` |
| `image` | 一个或多个输入图片文件 |

### 8.3 多图并发规则

体验中心的 `n` 不会直接作为一个大请求传给上游。任务服务会把它拆成 `n` 个并行请求，每个上游请求固定 `n: 1`，再按顺序合并图片结果。

- 单任务最多拆成 4 个并行请求。
- 同一页面可以同时存在多个独立 run，各 run 之间也可并行。
- 任一并行请求失败时，其他同 run 请求会被取消，整个 run 进入 `failed`。
- 实际并发仍受系统图片并发、用户并发、账号并发和上游限流约束。

## 9. 尺寸规范

体验中心支持自动、预设比例和自定义宽高。`gpt-image-2` 的最终数值尺寸必须符合：

| 规则 | 限制 |
| --- | --- |
| 单边最大值 | 3840 像素 |
| 最小总像素 | 655360 |
| 最大总像素 | 8294400 |
| 最大宽高比 | 3:1 |
| 宽高步长 | 均为 16 的倍数 |

不符合约束的数值尺寸会保持目标比例并自动缩放、补足或取整。例如：

| 输入 | 实际请求尺寸 |
| --- | --- |
| `4096x4096` | `2880x2880` |
| `4096x2304` | `3840x2160` |
| `4096x3072` | `3312x2480` |
| `256x256` | `816x816` |
| `4096x256` | `3840x1280` |

`auto` 保持为 `auto`。体验中心自定义输入框本身将宽高限制在 256 到 4096 之间，但最终仍以后端归一化结果为准。

## 10. 直接调用兼容网关

非体验中心客户端如果不需要刷新恢复，可直接调用兼容网关：

```bash
curl https://api.example.com/v1/images/generations \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "A clean product photo of a white ceramic mug",
    "size": "1536x1024",
    "n": 1,
    "quality": "high",
    "output_format": "png",
    "response_format": "b64_json"
  }'
```

典型非流式响应遵循 OpenAI 图片接口格式：

```json
{
  "created": 1784772000,
  "data": [
    {
      "b64_json": "<BASE64_IMAGE>",
      "revised_prompt": "..."
    }
  ],
  "usage": {
    "input_tokens": 120,
    "output_tokens": 4096
  }
}
```

公共网关还接受 `stream`、`moderation`、`input_fidelity`、`output_compression`、`partial_images` 等原生图片参数，但体验中心当前不发送这些字段。调用这些扩展参数时应按所选上游账号能力处理，不能假定所有账号都支持。

## 11. 错误处理

### 11.1 任务层同步错误

任务提交、查询或下载本身失败时使用 HTTP 状态码，并返回管理面错误格式：

```json
{
  "code": 400,
  "message": "api key is required"
}
```

常见状态码：

| HTTP 状态 | 场景 |
| --- | --- |
| `400` | JSON 无效、`mode`/`model`/`apiKey` 缺失、任务 ID 超长 |
| `401` | 用户 JWT 缺失或失效 |
| `404` | 任务或图片不存在 |
| `500` | 任务服务不可用或图片读取失败 |

### 11.2 后台上游错误

任务已经返回 `202` 后发生的网关或上游错误不会改变轮询接口的 HTTP 200，而是体现在：

```json
{
  "status": "failed",
  "error": "<上游错误信息> (HTTP <状态码>)"
}
```

网关层常见失败包括：API Key 无效、分组未开启生图、余额不足、没有兼容账号、并发已满、上游 429 限流和上游 5xx。

## 12. 安全与运行约束

1. `POST /api/v1/playground/runs` 的请求体同时包含用户 JWT 对应会话和所选 API Key，不得记录完整请求体、浏览器控制台输出或错误上报内容。
2. 输入图使用 base64 Data URL，必须限制前端上传体积；网关单个 multipart 文件当前最多读取 20 MiB。
3. 图片任务最长执行 45 分钟；终态任务在内存中保留 6 小时。
4. 任务和图片当前保存在后端进程内存中，不是持久化任务。后端重启后任务会丢失；多实例部署需要会话粘滞或把任务状态迁移到共享存储。
5. 图片响应最大读取 128 MiB。高分辨率、多图并发会明显增加后端内存占用。
6. 图片任务响应不会返回原始上游 `raw` 数据，调用方以 `status`、`images`、`error` 和 `durationMs` 为准。

## 13. 前端参考实现

```ts
type RunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled'

async function createAndWaitForImageRun(payload: Record<string, unknown>) {
  const created = await apiClient.post('/playground/runs', payload)
  const runId = created.data.id as string

  while (true) {
    await new Promise((resolve) => setTimeout(resolve, 1000))
    const response = await apiClient.get(`/playground/runs/${encodeURIComponent(runId)}`)
    const run = response.data as { status: RunStatus; error?: string; images?: Array<{ url?: string; assetIndex?: number }> }

    if (run.status === 'failed' || run.status === 'canceled') {
      throw new Error(run.error || `Image run ${run.status}`)
    }
    if (run.status !== 'succeeded') continue

    return Promise.all((run.images || []).map(async (image) => {
      if (image.url) return image.url
      const binary = await apiClient.get(
        `/playground/runs/${encodeURIComponent(runId)}/images/${image.assetIndex}`,
        { responseType: 'blob' }
      )
      return URL.createObjectURL(binary.data)
    }))
  }
}
```

仓库自带 `apiClient` 已经解包统一响应；如果使用原生 `fetch`，需要自行读取响应中的 `data` 字段。

## 14. 实现依据

- 前端任务请求与轮询：`frontend/src/views/user/PlaygroundView.vue`
- 前端接口类型：`frontend/src/api/playground.ts`
- 用户任务路由：`backend/internal/server/routes/user.go`
- 任务控制器：`backend/internal/handler/playground_handler.go`
- 任务执行与图片转发：`backend/internal/service/playground_run_service.go`
- OpenAI 图片网关：`backend/internal/handler/openai_images.go`
- 图片请求解析和转发：`backend/internal/service/openai_images.go`
- gpt-image-2 尺寸适配：`frontend/src/utils/playgroundImageTools.ts`
