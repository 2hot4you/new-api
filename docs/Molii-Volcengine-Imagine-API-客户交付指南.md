# Molii Volcengine Imagine API 客户交付指南

> API 版本：`v1`
>
> 文档版本：2026-09-01
>
> 适用范围：Seedance 异步视频生成与临时素材 API

本文档面向服务端开发者，说明如何通过 Molii 调用 Seedance 标准版、Fast、Mini 与 2.5。示例只使用 Molii 公共任务 ID 和公共素材 ID；客户端不需要了解底层供应商或渠道实现。

---

## 1. 接入准备

### 1.1 Base URL

在 Molii 控制台的“API 接入地址”中选择距离业务部署区域较近的节点，并保存为环境变量：

```bash
export MOLII_API_BASE_URL="https://替换为控制台展示的接入地址"
export MOLII_API_KEY="sk-替换为你的MoliiAPIKey"
```

本文示例使用 `${MOLII_API_BASE_URL%/}` 移除 Base URL 末尾多余的 `/`。

### 1.2 鉴权

所有接口均使用 Bearer 鉴权：

```http
Authorization: Bearer YOUR_MOLII_API_KEY
```

JSON 请求同时发送 `Content-Type: application/json` 和 `Accept: application/json`。

安全要求：

- API Key 只保存在服务端环境变量或密钥管理系统中；
- 不要把 API Key 写入浏览器前端、移动端安装包、公开仓库或日志；
- 不要把完整 `Authorization` 请求头放入工单、聊天记录或截图；
- 不要把 Molii API Key 转发给素材 CDN 或结果下载地址。

### 1.3 异步调用流程

1. 调用 `POST /v1/videos` 创建任务；
2. 保存响应中的 Molii 公共任务 ID；
3. 每隔 3–5 秒调用 `GET /v1/videos/{task_id}` 查询状态；
4. `completed` 后读取 `metadata.url`，或调用 Molii 内容接口下载；
5. `failed` 后停止轮询并读取 `error`。

付费 POST 不承诺幂等。发生连接中断或客户端超时时，不要自动重复提交；应先通过已保存的任务 ID 或平台生成记录确认任务是否已经受理。

---

## 2. 模型与能力

| 版本 | 模型 ID | 输出分辨率 | 时长范围 | 适用场景 |
| --- | --- | --- | --- | --- |
| Seedance 2.0 | `doubao-seedance-2-0-260128` | `480p`、`720p`、`1080p`、`4k` | `4`–`15` 秒或 `-1` | 优先追求生成质量和更高分辨率 |
| Seedance 2.0 Fast | `doubao-seedance-2-0-fast-260128` | `480p`、`720p` | `4`–`15` 秒或 `-1` | 优先追求速度和成本 |
| Seedance 2.0 Mini | `doubao-seedance-2-0-mini-260615` | `480p`、`720p` | `4`–`15` 秒或 `-1` | 高性价比、大规模视频生成 |
| Seedance 2.5 | `doubao-seedance-2-5-260628` | `480p`、`720p`、`1080p` | `4`–`30` 秒或 `-1` | 更长时长、多模态生成与视频编辑 |

`-1` 表示由模型自动选择合法的整数时长。Fast 和 Mini 不支持 `1080p` 或 `4k`；2.5 不支持 `4k`。

四个模型使用相同的 Molii 请求结构，支持：

- 文生视频、首帧图生视频、首尾帧图生视频；
- 参考图片、参考视频、参考音频组合生成；
- 基于参考视频的视频编辑、延长和镜头续写；
- 有声或无声视频；
- 固定比例或 `adaptive` 自适应比例；
- 联网搜索工具；
- 公网媒体 URL、受支持的 Data URL 和 Molii `asset://` 临时素材。

音频不能作为唯一媒体输入。使用参考音频时，至少还要提供一张参考图片或一个参考视频。

---

## 3. 创建视频任务

### 3.1 接口

推荐路径：

```http
POST /v1/videos
```

兼容路径：

```http
POST /v1/video/generations
```

两条路径接受相同的 Seedance 请求体。新接入建议使用 `/v1/videos`，使创建、查询和下载位于同一组路径下。

### 3.2 顶层参数

| 参数 | 类型 | 必填 | 默认值 | 可选值/范围 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `model` | string | 是 | 无 | 四个受支持模型 ID | 生成模型 |
| `content` | object[] | 条件必填 | `[]` | 见第 4 节 | 有顺序的文本和媒体输入 |
| `prompt` | string | 条件必填 | 无 | 非空字符串 | 纯文本便捷字段 |
| `generate_audio` | boolean | 否 | `true` | `true`、`false` | 是否生成同步音频 |
| `resolution` | string | 否 | `720p` | `480p`、`720p`、`1080p`、`4k` | 受模型能力限制 |
| `ratio` | string | 否 | `adaptive` | `16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`21:9`、`adaptive` | 输出宽高比 |
| `duration` | integer | 否 | `5` | `4`–模型上限或 `-1` | 输出时长，单位秒 |
| `watermark` | boolean | 否 | `false` | `true`、`false` | 是否请求添加水印 |
| `tools` | object[] | 否 | `[]` | 当前仅 `web_search` | 模型可调用的工具 |

`content` 与 `prompt` 至少提供一种有效输入。若同时提供 `prompt` 和 `content` 中的 `text`，两段文字都会进入模型，通常不建议重复填写。显式的 `false` 会被保留。

`generate_audio:true` 输出带同步音频的视频，`false` 输出无声视频。有声输出为单声道。

`ratio` 支持 `16:9`、`4:3`、`1:1`、`3:4`、`9:16`、`21:9` 和 `adaptive`。自适应模式的实际比例只有任务完成后才能确定。

联网搜索配置：

```json
{"tools": [{"type": "web_search"}]}
```

查询响应中的 `usage.tool_usage.web_search` 表示实际搜索次数，`0` 表示未使用。

---

## 4. `content` 多模态输入

### 4.1 文本

```json
{"type": "text", "text": "清晨的海岸线，镜头贴近海面平稳向前推进"}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | string | 是 | 固定为 `text` |
| `text` | string | 是 | 非空提示词，支持中文和英文 |

建议中文不超过 500 字、英文不超过 1000 词。可按时间顺序描述镜头、主体动作、环境和声音。

### 4.2 图片

```json
{
  "type": "image_url",
  "image_url": {"url": "https://cdn.example.com/reference.png"},
  "role": "reference_image"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | string | 是 | 固定为 `image_url` |
| `image_url.url` | string | 是 | 公网 URL、图片 Data URL 或 `asset://` URI |
| `role` | string | 条件必填 | 图片用途 |

| `role` | 用途 | 数量规则 |
| --- | --- | --- |
| `first_frame` | 首帧 | 必须正好 1 张 |
| `last_frame` | 尾帧 | 最多 1 张，必须与首帧同时使用 |
| `reference_image` | 多模态参考图片 | 图片总数最多 9 张 |
| 不填 | 按 `first_frame` 处理 | 仅建议单首帧场景使用 |

图片支持 JPEG、PNG、WebP、BMP、TIFF、GIF；建议宽高比满足 `0.4 < 宽/高 < 2.5`，边长 300–6000 px，单张小于 30 MB，完整请求体不超过 64 MB。

图片 Data URL 格式：`data:image/png;base64,BASE64_DATA`。大文件优先使用公网 URL 或 Molii 临时素材。

### 4.3 视频

```json
{
  "type": "video_url",
  "video_url": {"url": "https://cdn.example.com/reference.mp4"},
  "role": "reference_video"
}
```

| 项目 | 要求 |
| --- | --- |
| `type` / `role` | `video_url` / `reference_video` |
| URL | 公网 URL 或 `asset://` URI，不支持 Base64 |
| 格式 | MP4、MOV |
| 分辨率 | 参考视频建议为 480p 或 720p |
| 时长 | 单个 2–15 秒，所有参考视频合计不超过 15 秒 |
| 数量 | 最多 3 个 |
| 画面 | 宽高比 0.4–2.5、边长 300–6000 px、409600–927408 像素 |
| 大小 / 帧率 | 单个不超过 50 MB；24–60 FPS |

### 4.4 音频

```json
{
  "type": "audio_url",
  "audio_url": {"url": "https://cdn.example.com/reference.mp3"},
  "role": "reference_audio"
}
```

| 项目 | 要求 |
| --- | --- |
| `type` / `role` | `audio_url` / `reference_audio` |
| URL | 公网 URL、音频 Data URL 或 `asset://` URI |
| 格式 | WAV、MP3 |
| 时长 | 单个 2–15 秒，所有参考音频合计不超过 15 秒 |
| 数量 | 最多 3 个 |
| 大小 | 单个不超过 15 MB；完整请求体不超过 64 MB |

音频 Data URL 格式：`data:audio/wav;base64,BASE64_DATA`。

### 4.5 组合与互斥规则

- 帧模式包含一个 `first_frame`，可再加一个 `last_frame`；
- 多模态模式使用 `reference_image`、`reference_video`、`reference_audio`；
- `first_frame` 或 `last_frame` 不能与任何 `reference_*` 项混用；
- 多模态图片必须使用 `reference_image`；
- 图片最多 9 张、视频最多 3 个、音频最多 3 个；
- 参考音频必须与至少一张参考图片或一个参考视频同时使用；
- 纯文生视频只需要一个非空文本输入。

---

## 5. 完整创建示例

### 5.1 文生视频

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-0-260128",
    "content": [{"type": "text", "text": "清晨的海边木栈道，镜头平稳向前移动，环境声音自然同步"}],
    "generate_audio": true,
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5,
    "watermark": false
  }'
```

### 5.2 首帧图生视频

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-0-fast-260128",
    "content": [
      {"type": "text", "text": "镜头缓慢拉远，云层从山谷中流过"},
      {
        "type": "image_url",
        "image_url": {"url": "https://cdn.example.com/first-frame.jpg"},
        "role": "first_frame"
      }
    ],
    "resolution": "720p",
    "ratio": "adaptive",
    "duration": 6,
    "generate_audio": true,
    "watermark": false
  }'
```

### 5.3 首尾帧生视频

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-0-mini-260615",
    "content": [
      {"type": "text", "text": "从白天自然过渡到灯火初上的夜景，运镜连续平稳"},
      {"type": "image_url", "image_url": {"url": "https://cdn.example.com/first.jpg"}, "role": "first_frame"},
      {"type": "image_url", "image_url": {"url": "https://cdn.example.com/last.jpg"}, "role": "last_frame"}
    ],
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 8,
    "generate_audio": false,
    "watermark": false
  }'
```

### 5.4 多模态参考生成

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-5-260628",
    "content": [
      {"type": "text", "text": "使用图片中的产品外观和视频中的镜头节奏，全程使用参考音频作为背景音乐"},
      {"type": "image_url", "image_url": {"url": "https://cdn.example.com/product.png"}, "role": "reference_image"},
      {"type": "video_url", "video_url": {"url": "https://cdn.example.com/camera.mp4"}, "role": "reference_video"},
      {"type": "audio_url", "audio_url": {"url": "https://cdn.example.com/music.mp3"}, "role": "reference_audio"}
    ],
    "generate_audio": true,
    "resolution": "1080p",
    "ratio": "16:9",
    "duration": 12,
    "watermark": false
  }'
```

### 5.5 视频编辑

编辑没有额外的专用参数。通过参考视频、参考图片和文本指令描述要保留与修改的内容：

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "将视频中的白色礼盒替换成参考图片中的蓝色礼盒，保持原有运镜和光线"},
      {"type": "image_url", "image_url": {"url": "https://cdn.example.com/blue-box.png"}, "role": "reference_image"},
      {"type": "video_url", "video_url": {"url": "https://cdn.example.com/source.mp4"}, "role": "reference_video"}
    ],
    "generate_audio": true,
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5,
    "watermark": false
  }'
```

### 5.6 视频延长与镜头续写

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-5-260628",
    "content": [
      {"type": "text", "text": "延续视频1的运动方向进入室内，再自然衔接视频2中的展厅镜头"},
      {"type": "video_url", "video_url": {"url": "https://cdn.example.com/segment-1.mp4"}, "role": "reference_video"},
      {"type": "video_url", "video_url": {"url": "https://cdn.example.com/segment-2.mp4"}, "role": "reference_video"}
    ],
    "generate_audio": true,
    "resolution": "1080p",
    "ratio": "16:9",
    "duration": 20,
    "watermark": false
  }'
```

### 5.7 联网搜索

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-0-260128",
    "content": [{"type": "text", "text": "生成一段介绍今天上海天气特点的城市短片"}],
    "generate_audio": true,
    "resolution": "720p",
    "ratio": "9:16",
    "duration": 8,
    "watermark": false,
    "tools": [{"type": "web_search"}]
  }'
```

### 5.8 创建成功响应

HTTP `200`：

```json
{
  "id": "task_public_789",
  "task_id": "task_public_789",
  "object": "video",
  "model": "doubao-seedance-2-5-260628",
  "status": "queued",
  "progress": 0,
  "created_at": 1788192000
}
```

保存 `id`。`task_id` 是兼容字段，值与 `id` 相同；新代码建议读取 `id`。

---

## 6. 查询任务

### 6.1 接口与请求

```http
GET /v1/videos/{task_id}
```

```bash
TASK_ID="task_public_789"

curl --fail-with-body --silent --show-error \
  --url "${MOLII_API_BASE_URL%/}/v1/videos/${TASK_ID}" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Accept: application/json"
```

该接口没有请求体，任务必须属于当前用户。

| `status` | 含义 | 是否继续轮询 |
| --- | --- | --- |
| `queued` | 已受理或排队中 | 是 |
| `in_progress` | 正在生成 | 是 |
| `completed` | 成功终态 | 否 |
| `failed` | 失败终态 | 否 |

### 6.2 处理中响应

```json
{
  "id": "task_public_789",
  "task_id": "task_public_789",
  "object": "video",
  "model": "doubao-seedance-2-5-260628",
  "status": "in_progress",
  "progress": 42,
  "created_at": 1788192000
}
```

### 6.3 成功响应

```json
{
  "id": "task_public_789",
  "task_id": "task_public_789",
  "object": "video",
  "model": "doubao-seedance-2-5-260628",
  "status": "completed",
  "progress": 100,
  "created_at": 1788192000,
  "completed_at": 1788192068,
  "usage": {
    "completion_tokens": 432000,
    "total_tokens": 432000,
    "tool_usage": {"web_search": 0}
  },
  "metadata": {
    "url": "https://signed-media.example.com/result.mp4?signature=temporary"
  }
}
```

`metadata.url` 是经过服务端安全校验的短期签名结果地址。请在任务完成后及时保存文件，不要把它当成永久资源 URL。

### 6.4 失败响应

```json
{
  "id": "task_public_789",
  "task_id": "task_public_789",
  "object": "video",
  "model": "doubao-seedance-2-5-260628",
  "status": "failed",
  "progress": 100,
  "created_at": 1788192000,
  "completed_at": 1788192012,
  "error": {
    "code": "video_task_failed",
    "message": "Video generation task failed"
  }
}
```

上例中的错误码和文案仅展示结构，实际值以响应为准。错误文本会经过脱敏处理；客户端应主要依据 HTTP 状态码与 `status` 分支处理，不要硬编码供应商特有的错误字符串。

### 6.5 有界轮询示例

下面的脚本最多查询 120 次，每次间隔 5 秒：

```bash
TASK_ID="task_public_789"

for attempt in $(seq 1 120); do
  RESPONSE=$(curl --fail-with-body --silent --show-error \
    --url "${MOLII_API_BASE_URL%/}/v1/videos/${TASK_ID}" \
    --header "Authorization: Bearer $MOLII_API_KEY" \
    --header "Accept: application/json") || exit 1

  printf '%s\n' "$RESPONSE"
  STATUS=$(printf '%s' "$RESPONSE" | jq -r '.status')

  case "$STATUS" in
    completed) break ;;
    failed) exit 1 ;;
  esac

  sleep 5
done
```

生产环境还应处理 `429`、`Retry-After`、暂时性 `5xx`、总截止时间和进程重启后的任务恢复。

---

## 7. 获取与下载结果

任务成功后通常可直接读取 `metadata.url`。这是短期签名 URL，不需要也不应该携带 Molii API Key；若响应未包含该字段，请使用下方的 Molii 内容接口：

```bash
curl --fail --location \
  --output result.mp4 \
  "从 metadata.url 读取的完整地址"
```

也可以通过 Molii 内容接口下载：

```http
GET /v1/videos/{task_id}/content
```

```bash
TASK_ID="task_public_789"

curl --fail-with-body --location \
  --url "${MOLII_API_BASE_URL%/}/v1/videos/${TASK_ID}/content?download=1" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --output "${TASK_ID}.mp4"
```

只有任务所有者可以访问结果。不要用用户输入拼接任意媒体 URL，也不要把 Molii API Key 发送给重定向后的第三方地址。

---

## 8. 临时素材 API

临时素材把图片、视频或音频转换成 Seedance 请求可引用的 `asset://` URI。素材属于当前用户并有过期时间，不是永久文件存储。

### 8.1 创建临时素材

```http
POST /v1/assets
```

| 参数 | 类型 | 必填 | 范围 | 说明 |
| --- | --- | --- | --- | --- |
| `url` | string | 是 | 公网 `http://` 或 `https://` | 服务端可直接访问的素材 URL |
| `asset_type` | string | 是 | `image`、`video`、`audio` | 素材类型 |
| `name` | string | 是 | 1–80 个 Unicode 字符 | 用户可识别名称 |

源 URL 不能指向 localhost、私有网络地址，也不能包含 URL 用户名或密码。源文件不能依赖 Molii API Key、Cookie 或调用方自定义请求头；带签名的 URL 必须在素材处理期间保持有效。

本地文件应先上传到可由服务端访问的对象存储，再把其 HTTP(S) 地址提交给该接口。

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/assets" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "url": "https://cdn.example.com/reference.mp4",
    "asset_type": "video",
    "name": "reference-video-01"
  }'
```

成功响应：

```json
{"id": "asset-molii-example123"}
```

### 8.2 查询素材状态

```http
GET /v1/assets/{id}
```

```bash
ASSET_ID="asset-molii-example123"

curl --fail-with-body --silent --show-error \
  --url "${MOLII_API_BASE_URL%/}/v1/assets/${ASSET_ID}" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Accept: application/json"
```

成功响应示例：

```json
{
  "id": "asset-molii-example123",
  "asset_type": "video",
  "name": "reference-video-01",
  "source_url": "https://cdn.example.com/reference.mp4",
  "source_kind": "url",
  "status": "SUCCESS",
  "created_at": 1788192000,
  "expires_at": 1788278400,
  "verified_at": 1788192002
}
```

只有就绪/成功状态的素材才能用于生成。以实际返回的 `expires_at` 为准，不要假设素材会永久存在。

### 8.3 引用临时素材

在媒体对象的 `url` 中添加 `asset://` 前缀：

```json
{
  "type": "video_url",
  "video_url": {"url": "asset://asset-molii-example123"},
  "role": "reference_video"
}
```

完整请求：

```bash
curl --fail-with-body --silent --show-error \
  --request POST \
  --url "${MOLII_API_BASE_URL%/}/v1/videos" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{
    "model": "doubao-seedance-2-5-260628",
    "content": [
      {"type": "text", "text": "延续参考视频的镜头运动，进入夜晚的城市街道"},
      {
        "type": "video_url",
        "video_url": {"url": "asset://asset-molii-example123"},
        "role": "reference_video"
      }
    ],
    "generate_audio": true,
    "resolution": "1080p",
    "ratio": "adaptive",
    "duration": 10,
    "watermark": false
  }'
```

公共素材 ID 与创建它的用户绑定。不要跨账户共享 `asset://` URI，也不要根据 ID 格式自行构造素材 ID。

### 8.4 删除临时素材

```http
DELETE /v1/assets/{id}
```

```bash
ASSET_ID="asset-molii-example123"

curl --fail-with-body --silent --show-error \
  --request DELETE \
  --url "${MOLII_API_BASE_URL%/}/v1/assets/${ASSET_ID}" \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header "Accept: application/json"
```

成功响应：

```json
{"success": true}
```

删除不可恢复。已创建的视频任务不会因删除素材自动取消，但新任务不能继续引用已删除或已过期的素材。

---

## 9. 用量与计费字段

成功任务可能返回：

```json
{
  "usage": {
    "completion_tokens": 432000,
    "total_tokens": 432000,
    "tool_usage": {"web_search": 1}
  }
}
```

| 字段 | 说明 |
| --- | --- |
| `completion_tokens` | 输出视频消耗的 Token 数量 |
| `total_tokens` | 本次任务最终计费用量 |
| `tool_usage.web_search` | 实际联网搜索次数 |

视频费用按实际 Token 用量、当前模型/分辨率价格和 API Key 所选分组倍率结算：

```text
最终费用 = 实际 total_tokens ÷ 1,000,000 × 当前 Token 单价 × 分组倍率
```

不同模型、分辨率以及是否包含参考视频可能使用不同单价。价格会随产品配置和客户合同调整，应以 Molii 模型广场、控制台账单或合同中的当前公开价格为准，不要把示例金额写死在业务代码中。

创建时的预计用量只用于预扣或预算提示；任务终态后以实际 `total_tokens` 和服务端最终账单为准。查询任务本身不会再次收取视频生成费用。

---

## 10. 错误处理与重试

| HTTP/错误码 | 常见原因 | 处理建议 |
| --- | --- | --- |
| `400 invalid_request` | 输入组合、`role`、比例、分辨率、工具或模型不合法 | 修正请求，不要原样重试 |
| `400 invalid_seconds` | 时长超出模型范围且不是 `-1` | 按模型时长上限修正 |
| `400 temporary_asset_not_ready` | 临时素材仍在处理 | 查询素材状态后再提交 |
| `400 temporary_asset_expired` | 素材已过期 | 重新创建素材并替换 URI |
| `401`、`403` | API Key 无效、已禁用或无模型权限 | 修复鉴权或权限 |
| `404` | 任务/素材不存在或不属于当前用户 | 核对公共 ID 与账户 |
| `429` | 请求过快或服务繁忙 | 遵守 `Retry-After` 并退避 |
| 暂时性 `5xx` | 服务端或依赖服务暂时异常 | 仅对安全的 GET 有界重试 |

重试边界：

- `POST /v1/videos`：不要自动重试；先确认是否已经创建任务；
- `POST /v1/assets`：不要在未知结果时自动重复创建；
- 任务/素材 GET：可以按间隔有界重试；
- DELETE 超时后：先 GET 确认资源是否仍存在；
- 每个轮询流程都必须设置最大次数和总截止时间。

---

## 11. 上线检查清单

- [ ] Base URL 来自 Molii 控制台配置，不在代码中散落多份；
- [ ] API Key 只存在于服务端安全配置中；
- [ ] 付费 POST 已关闭自动重试；
- [ ] 创建响应中的公共任务 ID 已持久化；
- [ ] 轮询支持 `429`、`Retry-After`、退避、最大次数和总超时；
- [ ] 只把 `completed` 和 `failed` 视为终态；
- [ ] 临时素材在提交前已处于成功/就绪状态；
- [ ] 媒体 URL 在排队和生成期间保持可访问；
- [ ] 结果签名 URL 已及时下载，不作为永久 URL 保存；
- [ ] 日志不包含完整 API Key、输入媒体的私有签名和敏感提示词；
- [ ] 费用核对使用最终账单，不使用客户端估算替代服务端结算。

---

## 12. 快速参考

| 操作 | 方法与路径 |
| --- | --- |
| 创建视频任务 | `POST /v1/videos` |
| 查询视频任务 | `GET /v1/videos/{task_id}` |
| 获取/下载视频 | `GET /v1/videos/{task_id}/content` |
| 创建临时素材 | `POST /v1/assets` |
| 查询临时素材 | `GET /v1/assets/{id}` |
| 删除临时素材 | `DELETE /v1/assets/{id}` |

所有 ID 都必须使用 Molii 接口返回的公共 ID。客户端应忽略响应中未来新增的未知字段，以保持向前兼容。
