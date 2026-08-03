# Molii Seedance 2.0 API 接口文档
> Base URL：`https://aigc.claudeye.com`
> API 版本：`v1`
> 文档版本：2026-08-03

本文档只描述 Molii 提供的 Seedance 2.0 异步视频生成接口，不包含注册、登录、控制台操作、余额充值或其他模型接口。

---
## 一、接入概览
### 1. 支持模型
|模型 ID|定位|支持分辨率|
|---|---|---|
|`doubao-seedance-2-0-260128`|标准版，优先追求生成质量|`480p`、`720p`、`1080p`、`4k`|
|`doubao-seedance-2-0-fast-260128`|Fast 版，优先追求速度和成本|`480p`、`720p`|

两个模型使用相同的请求结构，均支持：

- 文生视频；
- 首帧图生视频；
- 首尾帧图生视频；
- 参考图片、参考视频、参考音频的多模态生成；
- 视频编辑、视频延长等基于参考视频的生成方式；
- 有声或无声视频；
- 联网搜索工具；
- `4`–`15` 秒指定时长，或 `-1` 智能时长。
### 2. 鉴权
所有接口都使用 Molii API Key：
```http
Authorization: Bearer sk-xxxxxxxxxxxxxxxx
```
JSON 请求还需发送：
```http
Content-Type: application/json
Accept: application/json
```
本文示例使用环境变量，避免把密钥写入代码：
```bash
export MOLII_BASE_URL="https://aigc.claudeye.com"
export MOLII_API_KEY="sk-替换为你的MoliiAPIKey"
```
安全要求：

- API Key 只能保存在服务端；
- 不要把 API Key 放入 URL、浏览器代码、移动端安装包或 Git 仓库；
- 不要在日志中记录完整 `Authorization` 请求头；
- 不要把 Molii API Key 转发给素材 CDN 或视频结果地址。
### 3. 异步调用流程
Seedance 视频生成是异步任务：

1. 调用 `POST /v1/video/generations` 创建任务；
2. 保存响应中的 Molii 公共任务 ID，例如 `task_xxx`；
3. 每隔 3–5 秒查询任务状态；
4. 状态变为成功后，通过 Molii 视频内容接口下载或播放；
5. 状态变为失败后停止轮询，读取失败原因。

创建接口不会向调用方暴露上游任务 ID。查询和下载只能使用 Molii 返回的 `task_...` 公共任务 ID。

---
## 二、创建视频生成任务
### 1. 接口
```http
POST https://aigc.claudeye.com/v1/video/generations
```
兼容别名：
```http
POST https://aigc.claudeye.com/v1/videos
```
两个创建路径使用相同的鉴权、请求体和响应结构。本文统一使用 `/v1/video/generations`。
### 2. 请求参数总表
|参数|类型|必填|默认值|说明|
|---|---|---|---|---|
|`model`|string|是|无|Seedance 模型 ID|
|`content`|object[]|条件必填|`[]`|文本和多模态输入；与 `prompt` 至少提供一种有效输入|
|`prompt`|string|条件必填|无|纯文本提示词的便捷字段|
|`generate_audio`|boolean|否|`true`|是否生成与画面同步的音频|
|`resolution`|string|否|`720p`|输出分辨率|
|`ratio`|string|否|`adaptive`|输出宽高比|
|`duration`|integer|否|`5`|输出时长，单位秒；支持 `4`–`15` 或 `-1`|
|`watermark`|boolean|否|`false`|是否添加水印|
|`tools`|object[]|否|`[]`|当前只支持联网搜索工具|

如果同时提供 `prompt` 和 `content` 中的 `text`，两段文本都会发送给模型。通常选择其中一种即可，避免提示词重复。

显式的 `false` 和合法的数字不会被忽略。例如 `generate_audio:false`、`watermark:false` 会按原值发送。
### 3. `model`
必填，只能填写以下值之一：
```text
doubao-seedance-2-0-260128
doubao-seedance-2-0-fast-260128
```
Fast 模型不接受 `1080p` 或 `4k`。不符合模型能力的请求会在创建阶段返回 `400`。
### 4. `content`
`content` 是有顺序的多模态输入数组。每个元素必须包含 `type`，并携带该类型对应的载荷。

允许的组合：

- 文本；
- 文本（可选）+ 首帧图片；
- 文本（可选）+ 首帧图片 + 尾帧图片；
- 文本（可选）+ 参考图片；
- 文本（可选）+ 参考视频；
- 文本（可选）+ 参考图片 + 参考视频；
- 文本（可选）+ 参考图片 + 参考音频；
- 文本（可选）+ 参考视频 + 参考音频；
- 文本（可选）+ 参考图片 + 参考视频 + 参考音频。

音频不能单独作为媒体输入。使用参考音频时，至少还要提供一张 `reference_image` 或一个 `reference_video`。

媒体地址与请求体要求：

- 公网 URL 必须能由上游服务直接访问，不能依赖 Molii API Key 或调用方自定义请求头；
- 带时效签名的素材 URL 应覆盖排队和生成所需时间；
- 为兼容上游媒体限制，完整 JSON 请求体应控制在 64 MB 以内；
- 大图片或音频不要使用 Base64，优先使用公网 URL 或 Molii 临时素材；
- 视频输入使用公网 URL 或 Molii 临时素材，不使用 Base64；
- 素材 URL 失效、返回 HTML、重定向到登录页或 Content-Type 错误都会导致任务失败。
#### 4.1 文本
```json
{
  "type": "text",
  "text": "一艘白色帆船穿过清晨的薄雾，电影感航拍镜头"
}
```
字段：

|字段|类型|必填|固定值/说明|
|---|---|---|---|
|`type`|string|是|`text`|
|`text`|string|是|非空提示词，支持中英文|

提示词建议：

- 中文尽量不超过 500 字；
- 英文尽量不超过 1000 词；
- 按时间顺序描述镜头、主体动作、环境和声音；
- 生成对白时，可用引号标记角色台词；
- 过长提示词可能导致模型忽略次要细节。
#### 4.2 图片
```json
{
  "type": "image_url",
  "image_url": {
    "url": "https://example.com/reference.jpg"
  },
  "role": "reference_image"
}
```
字段：

|字段|类型|必填|说明|
|---|---|---|---|
|`type`|string|是|固定为 `image_url`|
|`image_url`|object|是|图片对象|
|`image_url.url`|string|是|公网 URL、Data URL 或 Molii 临时素材 URI|
|`role`|string|条件必填|图片用途|

`role` 取值：

|`role`|用途|数量规则|
|---|---|---|
|`first_frame`|首帧|必须正好 1 张；单首帧或首尾帧模式|
|`last_frame`|尾帧|最多 1 张；必须与首帧同时使用|
|`reference_image`|多模态参考图|1–9 张|
|不填|按 `first_frame` 处理|只建议单首帧场景使用|

首尾帧模式与多模态参考模式互斥。`first_frame`/`last_frame` 不能和 `reference_image`、`reference_video`、`reference_audio` 混用。

支持的图片地址形式：

公网 URL：
```json
{
  "url": "https://cdn.example.com/reference.png"
}
```
Base64 Data URL：
```json
{
  "url": "data:image/png;base64,iVBORw0KGgoAAA..."
}
```
Molii 临时素材：
```json
{
  "url": "asset://asset-molii-xxxxxxxx"
}
```
图片媒体要求：

|项目|要求|
|---|---|
|格式|JPEG、PNG、WebP、BMP、TIFF、GIF|
|宽高比|建议满足 `0.4 < 宽/高 < 2.5`|
|边长|建议在 300–6000 px 范围内|
|单张大小|小于 30 MB|
|数量|首帧 1 张；首尾帧 2 张；参考图最多 9 张|

大文件不建议使用 Base64，因为 Base64 会增加请求体大小。
#### 4.3 视频
```json
{
  "type": "video_url",
  "video_url": {
    "url": "https://example.com/reference.mp4"
  },
  "role": "reference_video"
}
```
字段：

|字段|类型|必填|固定值/说明|
|---|---|---|---|
|`type`|string|是|`video_url`|
|`video_url`|object|是|视频对象|
|`video_url.url`|string|是|公网 URL 或 `asset://asset-molii-...`|
|`role`|string|是|`reference_video`|

视频媒体要求：

|项目|要求|
|---|---|
|格式|MP4、MOV|
|分辨率|参考视频建议为 480p 或 720p|
|单个时长|2–15 秒|
|总时长|所有参考视频合计不超过 15 秒|
|数量|最多 3 个|
|宽高比|`0.4`–`2.5`|
|边长|300–6000 px|
|画面像素|409600–927408|
|单个大小|不超过 50 MB|
|帧率|24–60 FPS|
#### 4.4 音频
```json
{
  "type": "audio_url",
  "audio_url": {
    "url": "https://example.com/reference.mp3"
  },
  "role": "reference_audio"
}
```
字段：

|字段|类型|必填|固定值/说明|
|---|---|---|---|
|`type`|string|是|`audio_url`|
|`audio_url`|object|是|音频对象|
|`audio_url.url`|string|是|公网 URL、Data URL 或 `asset://asset-molii-...`|
|`role`|string|是|`reference_audio`|

音频媒体要求：

|项目|要求|
|---|---|
|格式|WAV、MP3|
|单个时长|2–15 秒|
|总时长|所有参考音频合计不超过 15 秒|
|数量|最多 3 个|
|单个大小|不超过 15 MB|

音频 Data URL 格式：
```text
data:audio/wav;base64,<BASE64_DATA>
```
### 5. `generate_audio`
类型：`boolean`，默认：`true`。

- `true`：生成与画面同步的人声、环境声、音效或背景音乐；
- `false`：生成无声视频。

有声视频输出为单声道，与参考音频的声道数无关。
### 6. `resolution`
类型：`string`，默认：`720p`。

|分辨率|标准版|Fast 版|
|---|---|---|
|`480p`|支持|支持|
|`720p`|支持|支持|
|`1080p`|支持|不支持|
|`4k`|支持|不支持|

参数值区分大小写，应使用表中的小写形式。
### 7. `ratio`
类型：`string`，默认：`adaptive`。

支持：
```text
16:9
4:3
1:1
3:4
9:16
21:9
adaptive
```
`adaptive` 表示由模型根据提示词或输入媒体自动选择比例：

- 文生视频：根据提示词场景选择；
- 首帧/首尾帧：优先参考首帧比例；
- 多模态参考：优先按生成意图判断，否则参考第一个视频或图片。

固定比例对应像素：

|分辨率|16:9|4:3|1:1|3:4|9:16|21:9|
|---|---|---|---|---|---|---|
|`480p`|864×496|752×560|640×640|560×752|496×864|992×432|
|`720p`|1280×720|1112×834|960×960|834×1112|720×1280|1470×630|
|`1080p`|1920×1080|1440×1080|1080×1080|1080×1440|1080×1920|2520×1080|
|`4k`|3840×2160|2880×2160|2160×2160|2160×2880|2160×3840|5040×2160|

`adaptive` 的最终像素由模型决定，不应根据上述表格预判。
### 8. `duration`
类型：`integer`，单位：秒，默认：`5`。

允许值：

- `4`–`15` 中的任意整数；
- `-1`：智能时长，由模型在有效范围内选择。

以下值均不合法：`0`、负数（`-1` 除外）、小数、字符串形式的小数、超过 `15` 的整数。

时长参与费用估算和最终结算。不要用 `duration:0` 表示默认值；如需默认 5 秒，应省略字段或显式传 `5`。
### 9. `watermark`
类型：`boolean`，默认：`false`。

- `true`：请求添加水印；
- `false`：请求不添加水印。
### 10. `tools`
当前只支持联网搜索：
```json
{
  "tools": [
    {"type": "web_search"}
  ]
}
```
模型会根据提示词自行判断是否实际搜索。查询成功任务时，可通过 `usage.tool_usage.web_search` 查看实际调用次数。值为 `0` 表示未执行搜索。

其他工具类型会返回 `400 invalid_request`。
### 11. 不支持的字段
以下上游字段不属于 Molii Seedance 2.0 稳定契约，请不要发送：

- `service_tier`；
- `draft`；
- `frames`；
- `camera_fixed`。

---
## 三、创建请求示例
### 1. 文生视频
```bash
curl "$MOLII_BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "雨后的未来城市，一辆红色跑车驶过霓虹街道，低机位跟拍，电影感灯光"
      }
    ],
    "generate_audio": false,
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 5,
    "watermark": false
  }'
```
也可使用 `prompt` 便捷字段：
```json
{
  "model": "doubao-seedance-2-0-fast-260128",
  "prompt": "一只橘猫戴着宇航头盔，在月球表面缓慢行走",
  "generate_audio": false,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5
}
```
### 2. 首帧图生视频
```bash
curl "$MOLII_BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "镜头缓慢向前推进，云层从山峰之间流过，保持自然光照"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://cdn.example.com/mountain-first-frame.jpg"
        },
        "role": "first_frame"
      }
    ],
    "resolution": "720p",
    "ratio": "adaptive",
    "duration": 5
  }'
```
### 3. 首尾帧图生视频
```bash
curl "$MOLII_BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "从白天平滑过渡到夜晚，镜头位置保持稳定"
      },
      {
        "type": "image_url",
        "image_url": {"url": "https://cdn.example.com/day.jpg"},
        "role": "first_frame"
      },
      {
        "type": "image_url",
        "image_url": {"url": "https://cdn.example.com/night.jpg"},
        "role": "last_frame"
      }
    ],
    "generate_audio": false,
    "resolution": "720p",
    "ratio": "16:9",
    "duration": 6,
    "watermark": false
  }'
```
### 4. 多模态参考生成
```bash
curl "$MOLII_BASE_URL/v1/video/generations" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "content": [
      {
        "type": "text",
        "text": "参考图片中的角色外观、视频中的动作节奏和音频的音乐风格，生成一段新的广告视频"
      },
      {
        "type": "image_url",
        "image_url": {"url": "https://cdn.example.com/character.jpg"},
        "role": "reference_image"
      },
      {
        "type": "video_url",
        "video_url": {"url": "https://cdn.example.com/motion.mp4"},
        "role": "reference_video"
      },
      {
        "type": "audio_url",
        "audio_url": {"url": "https://cdn.example.com/music.mp3"},
        "role": "reference_audio"
      }
    ],
    "generate_audio": true,
    "resolution": "720p",
    "ratio": "adaptive",
    "duration": 8,
    "watermark": false
  }'
```
### 5. 编辑参考视频
```json
{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    {
      "type": "text",
      "text": "将视频中的白色礼盒替换成参考图片里的蓝色礼盒，保持原有运镜和光照"
    },
    {
      "type": "image_url",
      "image_url": {"url": "https://cdn.example.com/blue-box.jpg"},
      "role": "reference_image"
    },
    {
      "type": "video_url",
      "video_url": {"url": "https://cdn.example.com/source.mp4"},
      "role": "reference_video"
    }
  ],
  "generate_audio": true,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5
}
```
### 6. 延长或串联参考视频
```json
{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    {
      "type": "text",
      "text": "延续视频1的镜头运动，穿过拱门后自然衔接视频2的室内场景"
    },
    {
      "type": "video_url",
      "video_url": {"url": "https://cdn.example.com/part-1.mp4"},
      "role": "reference_video"
    },
    {
      "type": "video_url",
      "video_url": {"url": "https://cdn.example.com/part-2.mp4"},
      "role": "reference_video"
    }
  ],
  "generate_audio": true,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 8
}
```
### 7. 联网搜索
```json
{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    {
      "type": "text",
      "text": "制作一段符合今天上海天气氛围的城市短片"
    }
  ],
  "tools": [
    {"type": "web_search"}
  ],
  "generate_audio": true,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 6
}
```
---
## 四、创建响应
### 1. 成功响应
HTTP 状态码：`200`
```json
{
  "id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "task_id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "queued",
  "progress": 0,
  "created_at": 1785744000
}
```
字段：

|字段|类型|说明|
|---|---|---|
|`id`|string|Molii 公共任务 ID|
|`task_id`|string|兼容字段，与 `id` 相同|
|`object`|string|固定为 `video`|
|`model`|string|请求使用的模型|
|`status`|string|创建成功时通常为 `queued`|
|`progress`|integer|进度百分比，创建时通常为 `0`|
|`created_at`|integer|Unix 秒时间戳|

客户端必须保存 `id`。不要从其他字段推断或构造任务 ID。
### 2. 创建任务的幂等性
创建接口没有对外承诺 `Idempotency-Key` 幂等语义。网络超时不等于任务创建失败，自动重试可能创建两个视频并产生两次费用。

建议：

- 为每次业务请求生成自己的本地请求编号；
- 在发送前记录请求编号和时间；
- 收到响应后立即保存 Molii 任务 ID；
- 创建阶段超时或连接中断时，不要自动重复提交，先由人工或业务补偿流程确认。

---
## 五、查询视频生成任务
Molii 提供两种查询视图。两者查询同一个任务，不会触发第二次生成或收费。
### 1. 通用任务视图
```http
GET https://aigc.claudeye.com/v1/video/generations/{task_id}
```
示例：
```bash
curl "$MOLII_BASE_URL/v1/video/generations/task_xxx" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Accept: application/json"
```
响应外层：
```json
{
  "code": "success",
  "message": "",
  "data": {
    "task_id": "task_xxx",
    "status": "IN_PROGRESS",
    "progress": "42%",
    "fail_reason": "",
    "result_url": ""
  }
}
```
稳定业务字段：

|字段|类型|说明|
|---|---|---|
|`code`|string|查询成功为 `success`|
|`message`|string|提示信息，成功时通常为空|
|`data.task_id`|string|Molii 公共任务 ID|
|`data.status`|string|通用任务状态|
|`data.progress`|string|进度，通常为百分比字符串|
|`data.fail_reason`|string|失败原因，非失败状态通常为空|
|`data.result_url`|string|成功后提供 Molii 签名播放地址|

响应可能包含额外的计费、时间和任务诊断字段。客户端应忽略不认识的字段，不应依赖未在上表列出的内部字段。

通用状态：

|状态|含义|是否继续轮询|
|---|---|---|
|`SUBMITTED`|已提交|是|
|`QUEUED`|排队中|是|
|`IN_PROGRESS`|生成中|是|
|`SUCCESS`|已成功|否|
|`FAILURE`|已失败|否|
### 2. OpenAI 视频视图
```http
GET https://aigc.claudeye.com/v1/videos/{task_id}
```
该视图结构更精简，推荐新接入方使用。
```bash
curl "$MOLII_BASE_URL/v1/videos/task_xxx" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Accept: application/json"
```
进行中：
```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "in_progress",
  "progress": 42,
  "created_at": 1785744000
}
```
成功：
```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "completed",
  "progress": 100,
  "created_at": 1785744000,
  "completed_at": 1785744055,
  "usage": {
    "completion_tokens": 731025,
    "total_tokens": 731025,
    "tool_usage": {
      "web_search": 0
    }
  },
  "metadata": {
    "url": "https://aigc.claudeye.com/v1/videos/task_xxx/content?expires=...&user_id=...&signature=..."
  }
}
```
失败：
```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "doubao-seedance-2-0-260128",
  "status": "failed",
  "progress": 100,
  "created_at": 1785744000,
  "completed_at": 1785744055,
  "error": {
    "code": "starai_task_failed",
    "message": "视频生成失败"
  }
}
```
OpenAI 视频状态：

|状态|含义|是否继续轮询|
|---|---|---|
|`queued`|已提交或排队中|是|
|`in_progress`|生成中|是|
|`completed`|已成功|否|
|`failed`|已失败|否|
|`unknown`|未识别状态|是，但应受客户端总超时限制|
### 3. `usage`
|字段|类型|说明|
|---|---|---|
|`completion_tokens`|integer|视频生成消耗的输出 Token 数|
|`total_tokens`|integer|本任务总 Token 数|
|`tool_usage.web_search`|integer|实际联网搜索次数|

`usage` 通常在任务成功并完成结算后出现。不要使用客户端估算值替代服务端最终用量。
### 4. 轮询建议
- 每个任务每 3–5 秒查询一次；
- `429`、`502`、`503` 时使用指数退避；
- 推荐退避间隔为 2、4、8、16 秒，并加入随机抖动；
- 为客户端设置合理的总等待时间；
- 只有 `completed`/`SUCCESS` 或 `failed`/`FAILURE` 才是终态；
- 未知状态不能当作成功或失败。
### 5. 计费与失败处理
- 创建任务时，Molii 会根据模型、分辨率、输入类型和预计生成量预扣额度；
- 任务成功后，根据服务端最终用量和计费配置完成结算；
- `usage.completion_tokens` 和 `usage.total_tokens` 是模型用量，不是人民币或美元金额；
- 创建请求在本地校验阶段失败，不会形成有效视频任务；
- 已创建任务最终生成失败时，由 Molii 的异步任务流程统一处理退款；
- 成功任务因上游结果地址异常而暂时无法下载，不会自动改成失败或自动退款；
- 重复提交会被视为两个独立任务，因此创建请求不能自动重试。

具体单价可能随商业配置调整，不属于本接口响应契约。调用方应以 Molii 提供的当前价格说明为准，不要从 Token 数自行反推固定金额。

---
## 六、下载或播放视频
### 1. 使用 API Key 下载
```http
GET https://aigc.claudeye.com/v1/videos/{task_id}/content
```
```bash
curl "$MOLII_BASE_URL/v1/videos/task_xxx/content" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  --output result.mp4
```
只有成功任务可以下载。任务仍在生成时调用会返回 `400`。

接口支持：

- `Range`；
- `If-Range`；
- HTTP `206 Partial Content`；
- 浏览器/播放器的拖动与断点续传。

断点续传示例：
```bash
curl "$MOLII_BASE_URL/v1/videos/task_xxx/content" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Range: bytes=1048576-" \
  --output partial.mp4
```
### 2. 使用签名播放地址
成功任务的 `metadata.url` 或通用视图中的 `result_url` 是 Molii 生成的短期签名地址。该地址：

- 默认有效期为 24 小时；
- 在有效期内可不携带 API Key 直接访问；
- 重新查询成功任务可获得新的签名地址；
- 包含访问签名，应像临时凭据一样保护；
- 不应写入公开日志、前端埋点或第三方错误上报。
### 3. 结果地址异常
如果上游返回未签名的私有结果地址，Molii 不会把渠道密钥发送给对象存储，也不会自行伪造签名。此时：

- 生成任务仍可能保持成功并完成计费；
- 视频内容接口返回 HTTP `502`；
- 稳定错误码为 `upstream_invalid_result_url`；
- 客户端应保存 Molii 公共任务 ID 并联系支持；
- 不要把此错误当作可以重新创建任务的信号。

示例：
```json
{
  "error": {
    "code": "upstream_invalid_result_url",
    "message": "Upstream returned an unsigned private result URL",
    "type": "server_error"
  }
}
```
`message` 可能随服务版本调整，客户端逻辑应依赖 `error.code`。

---
## 七、临时素材 API
临时素材用于把公网图片、视频或音频注册为可在 Seedance 请求中引用的 `asset://` URI。

特点：

- 素材与创建它的 Molii 用户绑定；
- 其他用户不能查询或使用；
- 默认有效期为 24 小时，实际以响应 `expires_at` 为准；
- 素材适合即传即用，不应长期保存素材 ID；
- 本接口接收公开 HTTP/HTTPS URL，不接收本地文件路径。
### 1. 创建临时素材
```http
POST https://aigc.claudeye.com/v1/assets
```
参数：

|字段|类型|必填|说明|
|---|---|---|---|
|`url`|string|是|可公网访问的 HTTP/HTTPS URL|
|`asset_type`|string|是|`image`、`video` 或 `audio`|
|`name`|string|是|素材名称，最多 80 个字符|

限制：

- URL 不能包含用户名或密码；
- `localhost`、环回 IP、私网 IP、链路本地地址会被拒绝；
- URL 应在素材有效期内保持可访问；
- URL 返回的媒体类型和 `asset_type` 必须一致。

示例：
```bash
curl "$MOLII_BASE_URL/v1/assets" \
  -H "Authorization: Bearer $MOLII_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://cdn.example.com/reference.mp4",
    "asset_type": "video",
    "name": "动作参考视频"
  }'
```
成功：
```json
{
  "id": "asset-molii-xxxxxxxxxxxxxxxxxxxxxxxx"
}
```
### 2. 查询临时素材
```http
GET https://aigc.claudeye.com/v1/assets/{id}
```
```bash
curl "$MOLII_BASE_URL/v1/assets/asset-molii-xxx" \
  -H "Authorization: Bearer $MOLII_API_KEY"
```
响应示例：
```json
{
  "id": "asset-molii-xxx",
  "asset_type": "video",
  "name": "动作参考视频",
  "source_url": "https://cdn.example.com/reference.mp4",
  "source_kind": "url",
  "status": "SUCCESS",
  "created_at": 1785744000,
  "expires_at": 1785830400,
  "verified_at": 1785744000
}
```
状态：

|状态|含义|能否用于创建任务|
|---|---|---|
|`PROCESSING`|正在处理|否，稍后查询|
|`ACTIVE`|可用|是|
|`SUCCESS`|可用|是|
|`FAILED`|处理失败|否|
|`EXPIRED`|已过期或上游已删除|否|
### 3. 在视频请求中使用
将 Molii 素材 ID 加上 `asset://` 前缀：
```json
{
  "type": "video_url",
  "video_url": {
    "url": "asset://asset-molii-xxx"
  },
  "role": "reference_video"
}
```
创建视频任务时，Molii 会再次检查素材归属、有效期和上游状态。未就绪、过期或不属于当前用户的素材会被拒绝。
### 4. 删除临时素材
```http
DELETE https://aigc.claudeye.com/v1/assets/{id}
```
```bash
curl -X DELETE "$MOLII_BASE_URL/v1/assets/asset-molii-xxx" \
  -H "Authorization: Bearer $MOLII_API_KEY"
```
成功：
```json
{
  "success": true
}
```
---
## 八、错误响应
Molii Seedance 涉及模型网关、异步任务和视频流三类响应，因此错误外层可能不同。客户端应优先使用 HTTP 状态码和稳定错误码。
### 1. 任务错误
```json
{
  "code": "invalid_request",
  "message": "resolution must be one of 480p, 720p, 1080p, or 4k",
  "data": null
}
```
### 2. OpenAI 风格错误
```json
{
  "error": {
    "message": "Invalid token",
    "type": "new_api_error",
    "code": ""
  }
}
```
### 3. 素材错误
```json
{
  "success": false,
  "message": "temporary asset not found or expired"
}
```
### 4. 常见错误码
|HTTP|错误码|含义|是否建议重试|
|---|---|---|---|
|`400`|`invalid_request`|请求体、媒体组合、比例、工具或模型参数错误|否，修正请求|
|`400`|`invalid_seconds`|`duration` 不在允许范围|否|
|`400`|`task_not_exist`|任务不存在或不属于当前用户|否|
|`400`|`temporary_asset_expired`|临时素材已过期|否，重新创建素材|
|`400`|`temporary_asset_not_ready`|临时素材尚未就绪|查询素材状态后再试|
|`401`|—|API Key 缺失或无效|否，检查鉴权|
|`403`|`access_denied` 或空|用户、令牌、模型、分组或 IP 无权访问|否|
|`413`|`read_request_body_failed`|请求体过大|否，改用公网 URL/临时素材|
|`429`|—|请求过快或上游负载饱和|是，指数退避|
|`400`|`model_price_error`|模型尚未配置可用计费|否，联系支持|
|`502`|`temporary_asset_verification_failed`|临时素材上游验证失败|稍后重试查询，持续失败则联系支持|
|`502`|`starai_api_error`|视频上游拒绝请求或返回异常|视错误信息决定|
|`502`|`invalid_response`|上游响应缺少有效任务 ID|不要自动重建任务，联系支持|
|`502`|`upstream_invalid_result_url`|成功任务的结果地址不可下载|否，联系支持|
|`503`|—|暂无可用渠道或服务暂不可用|是，指数退避|

错误 `message` 主要用于诊断，不应作为程序分支的唯一依据。

---
## 九、完整 Python 示例
依赖：
```bash
pip install requests
```
```python
import os
import random
import time
from pathlib import Path

import requests

BASE_URL = "https://aigc.claudeye.com"
API_KEY = os.environ["MOLII_API_KEY"]
HEADERS = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json",
    "Accept": "application/json",
}


def create_video() -> str:
    payload = {
        "model": "doubao-seedance-2-0-260128",
        "content": [
            {
                "type": "text",
                "text": "海边日落，一只金毛犬沿着潮水奔跑，电影感跟拍镜头",
            }
        ],
        "generate_audio": False,
        "resolution": "720p",
        "ratio": "16:9",
        "duration": 5,
        "watermark": False,
    }
    response = requests.post(
        f"{BASE_URL}/v1/video/generations",
        headers=HEADERS,
        json=payload,
        timeout=(10, 60),
    )
    response.raise_for_status()
    result = response.json()
    return result["id"]


def wait_for_video(task_id: str, max_wait_seconds: int = 1800) -> dict:
    deadline = time.monotonic() + max_wait_seconds
    delay = 3.0

    while time.monotonic() < deadline:
        response = requests.get(
            f"{BASE_URL}/v1/videos/{task_id}",
            headers=HEADERS,
            timeout=(10, 30),
        )

        if response.status_code in {429, 502, 503}:
            time.sleep(delay + random.uniform(0, 1))
            delay = min(delay * 2, 30)
            continue

        response.raise_for_status()
        video = response.json()
        status = video.get("status")

        if status == "completed":
            return video
        if status == "failed":
            error = video.get("error") or {}
            raise RuntimeError(
                f"video failed: {error.get('code')} {error.get('message')}"
            )

        time.sleep(3)
        delay = 3.0

    raise TimeoutError(f"video polling timed out: {task_id}")


def download_video(task_id: str, target: Path) -> None:
    with requests.get(
        f"{BASE_URL}/v1/videos/{task_id}/content",
        headers={"Authorization": f"Bearer {API_KEY}"},
        stream=True,
        timeout=(10, 300),
    ) as response:
        response.raise_for_status()
        with target.open("wb") as output:
            for chunk in response.iter_content(chunk_size=1024 * 1024):
                if chunk:
                    output.write(chunk)


task_id = create_video()
print("task:", task_id)
video = wait_for_video(task_id)
print("completed:", video)
download_video(task_id, Path("result.mp4"))
print("saved: result.mp4")
```
创建函数故意不自动重试。只有状态查询执行有上限的退避重试。

---
## 十、完整 Node.js 示例
Node.js 18 及以上可直接使用 `fetch`：
```javascript
import { createWriteStream } from 'node:fs';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';

const baseURL = 'https://aigc.claudeye.com';
const apiKey = process.env.MOLII_API_KEY;

if (!apiKey) {
  throw new Error('MOLII_API_KEY is required');
}

const headers = {
  Authorization: `Bearer ${apiKey}`,
  'Content-Type': 'application/json',
  Accept: 'application/json',
};

async function createVideo() {
  const response = await fetch(`${baseURL}/v1/video/generations`, {
    method: 'POST',
    headers,
    body: JSON.stringify({
      model: 'doubao-seedance-2-0-fast-260128',
      content: [
        {
          type: 'text',
          text: '清晨的森林里，一只梅花鹿穿过薄雾，镜头平稳跟随',
        },
      ],
      generate_audio: false,
      resolution: '720p',
      ratio: '16:9',
      duration: 5,
      watermark: false,
    }),
    signal: AbortSignal.timeout(60_000),
  });

  if (!response.ok) {
    throw new Error(`create failed ${response.status}: ${await response.text()}`);
  }
  return response.json();
}

async function waitForVideo(taskId, maxWaitMs = 30 * 60 * 1000) {
  const deadline = Date.now() + maxWaitMs;

  while (Date.now() < deadline) {
    const response = await fetch(`${baseURL}/v1/videos/${taskId}`, {
      headers,
      signal: AbortSignal.timeout(30_000),
    });

    if ([429, 502, 503].includes(response.status)) {
      await new Promise((resolve) => setTimeout(resolve, 5000));
      continue;
    }
    if (!response.ok) {
      throw new Error(`query failed ${response.status}: ${await response.text()}`);
    }

    const video = await response.json();
    if (video.status === 'completed') return video;
    if (video.status === 'failed') {
      throw new Error(`video failed: ${JSON.stringify(video.error)}`);
    }

    await new Promise((resolve) => setTimeout(resolve, 3000));
  }

  throw new Error(`video polling timed out: ${taskId}`);
}

async function downloadVideo(taskId, filename) {
  const response = await fetch(`${baseURL}/v1/videos/${taskId}/content`, {
    headers: { Authorization: `Bearer ${apiKey}` },
    signal: AbortSignal.timeout(5 * 60 * 1000),
  });
  if (!response.ok || !response.body) {
    throw new Error(`download failed ${response.status}: ${await response.text()}`);
  }
  await pipeline(Readable.fromWeb(response.body), createWriteStream(filename));
}

const created = await createVideo();
console.log('task:', created.id);
const completed = await waitForVideo(created.id);
console.log('completed:', completed);
await downloadVideo(created.id, 'result.mp4');
```
---
## 十一、接入检查清单
- Base URL 使用 `https://aigc.claudeye.com`；
- 每次请求携带 `Authorization: Bearer <Molii API Key>`；
- `model` 只使用本文列出的两个模型 ID；
- Fast 模型不请求 `1080p` 或 `4k`；
- 首尾帧模式和多模态参考模式不混用；
- 参考音频至少搭配一张参考图片或一个参考视频；
- 时长只使用 `4`–`15` 的整数或 `-1`；
- 保存 Molii 返回的 `task_...` ID；
- 创建请求不自动重试；
- 查询间隔不低于 3 秒，并对临时错误指数退避；
- 只在任务成功后请求视频内容；
- 不记录 API Key、签名播放 URL 或敏感素材地址；
- 程序分支依赖 HTTP 状态码和错误码，不依赖错误文案；
- 客户端能忽略响应中新增加的字段。

---
## 十二、技术支持所需信息
排障时请提供：

- 请求时间和时区；
- 请求方法和路径；
- 使用的模型 ID；
- HTTP 状态码；
- 响应错误码；
- Molii 公共任务 ID（`task_...`）；
- 是否包含图片、视频、音频、临时素材或联网搜索；
- 已脱敏的请求参数。

请勿提供：

- 完整 API Key；
- `Authorization` 请求头；
- 上游任务 ID；
- 完整签名播放 URL；
- 包含用户隐私的未脱敏素材或提示词。
