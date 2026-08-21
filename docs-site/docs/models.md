---
sidebar_position: 4
---

# 模型与能力

先按兼容协议、任务类型、输入媒体和输出规格选择模型，再进入具体模型页核对参数。当前检入的 Development 公开快照包含 **10 个 Provider** 和 **35 个模型**；完整目录见 [Provider 与模型](/providers)。

## 先按任务选择模型

- 使用 OpenAI 消息格式时，从 [Chat Completions](/api-reference/chat-completions) 开始。
- 使用 OpenAI Responses 格式时，阅读 [Responses](/api-reference/responses)。
- 使用 Anthropic Messages 格式时，阅读 [Anthropic Messages](/api-reference/anthropic-messages)。
- 使用 Gemini 原生格式时，阅读 [Gemini GenerateContent](/api-reference/gemini-generate-content)。
- 图片生成与编辑使用[图片 API](/api-reference/images)；异步视频创作使用[视频 API](/api-reference/videos)。

需要通用文本、多模态理解或 Agent 能力时，先在 [Provider 与模型](/providers)中按兼容协议和输入模态筛选。需要图片、视频或多参考媒体创作时，再核对对应模型的分辨率、时长和输入限制。

## 模型选择矩阵

| 任务或协议 | 入口 | 需要重点核对 |
| --- | --- | --- |
| 完整公开目录 | [Provider 与模型](/providers) | Provider、模型 ID、兼容协议、输入输出模态 |
| OpenAI 消息格式 | [Chat Completions](/api-reference/chat-completions) | 消息内容、流式响应、模型支持参数 |
| OpenAI Responses 格式 | [Responses](/api-reference/responses) | 输入项、工具、流式事件 |
| Anthropic 消息格式 | [Anthropic Messages](/api-reference/anthropic-messages) | 专用鉴权头、消息块、最大输出 |
| Gemini 原生格式 | [Gemini GenerateContent](/api-reference/gemini-generate-content) | 路径模型 ID、内容块、专用鉴权头 |
| 图片生成与编辑 | [Grok Imagine 图片](/models/grok-imagine-image) | 图片数量、质量、输入 URL 和编辑限制 |
| 异步视频任务 | [视频 API](/api-reference/videos) | 创建、轮询、下载和模型特有限制 |

## Seedance 视频生成

Seedance 2.0 支持文本、图片、视频和音频参考；Fast 版本面向较低分辨率和更快生成路径。完整模型边界见 [Seedance 2.0](/models/seedance-2)，有效输入组合见[Seedance 多模态输入](/guides/seedance-multimodal)。

## Grok Imagine 图片

Grok Imagine 图片模型覆盖生成与受支持的图片编辑。输入媒体、数量、质量和结果保存规则见[模型说明](/models/grok-imagine-image)与[图片 API](/api-reference/images)。

## Grok Imagine 视频

Grok Imagine 视频覆盖文生视频、图生视频及受支持的编辑、延长路径。模型差异见[Grok Imagine 视频](/models/grok-imagine-video)，通用异步流程见[视频 API](/api-reference/videos)。

## 核对输入与输出能力

每个生成的模型页都会列出快照声明的协议、输入输出模态、能力、参数和限制。选择后依次核对上下文或时长、媒体数量、输出规格与异步行为，不要把一个模型的默认值套用到另一个模型。

## 查看权威参数与价格

账户实际可用模型以带当前 API Key 调用 [`GET /v1/models`](/api-reference/models) 的结果为准。当前模型价格与权限以平台模型广场和账户状态为准；目录页不复制可能变化的价格。付费 POST 结果不确定时不要自动重发，并按[错误与重试](/api-basics/errors-retries)处理。
