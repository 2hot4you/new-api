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

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `model` | string | 是 | 已开通的模型标识，由服务方提供。 |
| `prompt` | string | 是 | 图片内容、风格、构图等描述。 |
| `n` | integer | 否 | 本文示例使用 `1`，表示生成一张图片。 |
| `size` | string | 否 | 本文示例使用 `1024x1024`。其他尺寸以所选模型开通的能力为准。 |
| `quality` | string | 否 | 本文示例使用 `medium`。其他质量档位以所选模型开通的能力为准。 |
| `response_format` | string | 否 | 请求图片链接使用 `url`；请求 Base64 使用 `b64_json`。客户端应检查实际返回字段。 |
| `stream` | boolean | 否 | 本文使用 `false`，一次性返回完整结果。 |

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
      "url": "https://YOUR_IMAGE_HOST/v1/images/proxy/OPAQUE_TOKEN",
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
