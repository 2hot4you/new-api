# 图像生成 API 接口文档

## 接入信息

服务地址、模型标识和 API Key 由服务方单独提供。本文使用 `BASE_URL`、`MODEL_ID`、`API_KEY` 作为占位符；调用时必须替换为实际值。

所有生成请求使用 HTTPS，并携带以下请求头：

```http
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

API Key 只用于调用生成接口，不要放入图片链接或前端公开代码。

## 生成图片

```http
POST /v1/images/generations
```

同步请求：服务完成生成后返回 JSON。首次接入建议使用单张图片、非流式请求。

### 请求参数

以下为本文所对接图像生成能力的参数。`model` 的实际值由服务方单独提供，`YOUR_MODEL_ID` 仅为占位符。示例值不代表默认值；仅在说明中明确标注时才表示默认行为。

| 参数 | 类型 | 必填 | 示例值 | 可选值 / 取值范围 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `model` | string | 是 | `"YOUR_MODEL_ID"` | 当前 API Key 已开通的模型标识；可通过 `GET /v1/models` 查询。 | 必须使用服务方提供的实际标识，不能直接发送占位符。 |
| `prompt` | string | 是 | `"一只橘猫坐在窗边，写实摄影风格"` | 非空文本，无固定枚举；接口定义未公布统一长度上限。 | 描述图片内容、风格、构图及限制条件。 |
| `n` | integer | 否 | `1` | `1`、`2`、`3`、`4`、`5`、`6`、`7`、`8`。 | 默认 `1`。首次接入建议 `1`；多张生成能力按所选模型支持情况使用，返回张数可能少于请求张数。 |
| `size` | string | 否 | `"1024x1024"` | `"1k"`、`"2k"`、`"4k"`，或 `"宽x高"` 像素尺寸；明确像素尺寸的每边范围为 `64–4096`。 | 支持档位和尺寸的模型可使用下方尺寸表。默认 `"1024x1024"`；使用英文小写 `x`，不要使用 `×`。实际输出像素可能略有差异。 |
| `quality` | string | 否 | `"medium"` | 标准档位：`"low"`、`"medium"`、`"high"`；自动档位：`"auto"`。 | 本文所对接能力默认 `"medium"`；`"auto"` 按中等质量处理。兼容别名 `"standard"` 按 `"medium"`、`"xhigh"` 按 `"high"` 处理；建议使用标准档位。 |
| `response_format` | string | 否 | `"url"` | `"url"`、`"b64_json"`。 | 默认 `"url"`，返回可直接访问的图片链接；`"b64_json"` 返回 Base64 字符串。客户端应检查实际返回字段。 |
| `stream` | boolean | 否 | `false` | `false`、`true`，不要使用带引号的字符串。 | 默认 `false`，返回完整 JSON；`true` 返回 SSE，不能按一次性 JSON 解析。 |
| `seed` | integer | 否 | `12345` | 整数，无固定枚举；接口定义未规定上下限或默认值。 | 可选随机种子，是否生效取决于模型；相同值不保证生成完全相同的图片。 |
| `style` | string | 否 | `"natural"` | `"vivid"`（鲜明风格）、`"natural"`（自然风格）。 | 模型可选能力，使用前确认支持；接口通用定义的默认值为 `"vivid"`，不能据此认定所有模型都会应用该风格。 |
| `user` | string | 否 | `"client_001"` | 自定义字符串，无固定枚举。 | 可选的终端用户标识；接口定义未公布长度限制，不要填写 API Key 或其他敏感凭据。 |

### 尺寸示例

| 用途 | `size` 示例值 | 说明 |
| --- | --- | --- |
| 1K 方图 | `"1k"` 或 `"1024x1024"` | `"1k"` 默认对应 `1024x1024`。 |
| 2K 方图 | `"2k"` 或 `"2048x2048"` | `"2k"` 默认对应 `2048x2048`。 |
| 2K 横图 | `"2048x1152"` | 明确指定宽和高。 |
| 4K 横图 | `"4k"` 或 `"3840x2160"` | `"4k"` 默认对应 `3840x2160`，不是方图。 |
| 4K 竖图 | `"2304x4096"` | 明确指定竖向尺寸。 |
| 4K 方图 | `"4096x4096"` | 明确指定方图尺寸。 |

尺寸范围并不表示其中每一种宽高组合都已实测。首次接入可使用下方已验证的单张、`1024x1024`、`medium`、`url`、非流式组合；其他组合按模型能力选择。

### 参考图与尺寸扩展参数

以下均为选填参数，仅在所选模型支持时使用。普通文生图可以全部省略。URL 示例中的域名和图片路径需要替换为实际可读取的素材地址。

| 参数 | 类型 | 示例值 | 可选值 / 取值范围 | 说明 |
| --- | --- | --- | --- | --- |
| `image` | string | `"https://YOUR_ASSET_HOST/reference.png"` | 图片 URL 或 Base64 Data URL，例如 `data:image/png;base64,...`。 | 单张参考图；Base64 示例中的省略号不是有效图片数据。 |
| `images` | array of strings | `["https://YOUR_ASSET_HOST/reference.png"]` | 图片 URL 或 Base64 Data URL 组成的数组；接口定义未公布统一数量上限。 | 多张参考图，是否支持及数量限制取决于模型；不要与 `image` 重复提交相同素材。 |
| `resolution` / `image_size` / `imageSize` | string | `"2k"` | 分辨率档位 `"1k"`、`"2k"`、`"4k"`。 | 同类参数的不同名称，选择一个使用；不得与 `size` 指定的尺寸矛盾。 |
| `aspect_ratio` / `aspectRatio` | string | `"16:9"` | 宽高比字符串，例如 `"1:1"`、`"16:9"`、`"9:16"`；接口定义未列出完整支持比例。 | 选择一个参数名使用；若同时指定明确的 `size`，比例必须一致。 |

`output_format`、`background`、`reference_images` 等其他扩展字段没有在本接口定义中给出完整类型、枚举和模型支持范围，因此不作为本文承诺的通用参数；需要时请先向服务方确认。不支持的可选参数可能被忽略或返回错误。

当 `stream=true` 时，客户端需按 SSE 读取 `image_generation.completed` 事件，并以 `data: [DONE]` 作为结束标志。流式连接中断后，已生成的图片仍可能产生费用；不要自动重新生成。

### curl 示例

以下命令适用于 Bash、Zsh 等终端，需要安装 `jq`。

```bash
BASE_URL='https://YOUR_API_HOST'
API_KEY='YOUR_API_KEY'
MODEL_ID='YOUR_MODEL_ID'

jq -n --arg model "$MODEL_ID" '{
  model: $model,
  prompt: "一只橘猫坐在窗边，温暖的自然光，写实摄影风格，无文字，无水印",
  n: 1,
  size: "1024x1024",
  quality: "medium",
  response_format: "url",
  stream: false
}' | curl --silent --show-error --fail-with-body \
  --connect-timeout 15 \
  --max-time 300 \
  --request POST "${BASE_URL%/}/v1/images/generations" \
  --header "Authorization: Bearer ${API_KEY}" \
  --header 'Content-Type: application/json' \
  --data-binary @- \
  --output image-response.json
```

生成完成后提取图片链接：

```bash
jq -er '
  if .error then error(.error.message // "生成失败")
  elif (.data[0].url // "" | test("^https://")) then .data[0].url
  else error("响应中没有 HTTPS 图片链接，请检查返回字段")
  end
' image-response.json
```

## 响应结构

以下为字段结构示例，示例域名、链接令牌和数值不代表真实资源：

```json
{
  "created": 1792392806,
  "data": [
    {
      "url": "https://YOUR_API_HOST/v1/images/proxy/OPAQUE_TOKEN",
      "revised_prompt": "一只橘猫坐在窗边，温暖的自然光，写实摄影风格，无文字，无水印"
    }
  ],
  "usage": {
    "input_tokens": 26,
    "output_tokens": 1056,
    "total_tokens": 1082,
    "input_tokens_details": {
      "text_tokens": 26,
      "image_tokens": 0,
      "cached_tokens": 0
    },
    "output_tokens_details": {
      "text_tokens": 0,
      "image_tokens": 1056,
      "reasoning_tokens": 0
    }
  }
}
```

| 字段 | 说明 |
| --- | --- |
| `created` | Unix 时间戳，单位为秒。 |
| `data` | 图片结果数组。成功接入需确认数组非空且图片可读取。 |
| `data[].url` | 图片地址。HTTPS 地址可在浏览器中直接打开，无需附加 API Key。 |
| `data[].b64_json` | 可选的 Base64 图片数据，与链接形式分别处理；不要直接作为 HTTP 地址请求。 |
| `data[].revised_prompt` | 可选的提示词说明。 |
| `usage` | 可选的用量信息；具体子字段以实际响应为准。Token 数值不等同于费用金额。 |

如果 `url` 的内容以 `data:image/` 开头，它属于内嵌图片数据，不是 HTTPS 下载链接；客户端应按照 Data URL 格式解析。

## 图片链接

本站临时图片链接的格式为 `/v1/images/proxy/{token}`。访问时直接返回图片内容，不需要再次发起生成请求。

- 链接签发后最长有效期为一小时；图片资源提前不可用时，访问也可能提前失败。
- 链接持有者可以读取图片，请按需分享并及时下载保存。
- 不要修改、截断或重新拼接链接中的令牌。
- 下载响应按图片内容设置 `Content-Type`，并使用 `Cache-Control: no-store`。
- 链接过期后不能通过刷新恢复；不要为了下载失败而自动重复生成。

## 错误处理

生成失败时检查 HTTP 状态码以及 JSON 中的 `error.message`。发生超时或 `5xx` 时，先核对该次请求是否已经完成，再决定是否重试，避免重复生成。

图片下载接口可能返回：

| HTTP 状态码 | 含义 | 建议 |
| --- | --- | --- |
| `404` | 链接无效或不可用。 | 检查链接是否完整。 |
| `410` | 临时链接已过期。 | 使用之前保存的图片，或联系服务方确认后续处理。 |
| `429` | 下载请求过于频繁。 | 降低频率，稍后重试下载。 |
| `502` | 暂时无法取得有效图片内容。 | 稍后重试下载；持续失败时联系服务方。 |
| `503` | 服务暂时繁忙。 | 稍后重试下载。 |

图片下载失败的响应示例：

```json
{
  "error": {
    "code": "image_unavailable",
    "message": "Image is temporarily unavailable"
  }
}
```
