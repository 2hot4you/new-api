---
sidebar_position: 4
---

# 模型与能力

先按任务类型、输入媒体、输出规格和延迟要求选择模型，再进入模型页核对完整参数。模型能力和可用价格以当前模型页与平台模型广场为准。

## 先按任务选择模型

- 需要多参考图片、参考视频或参考音频的视频创作：选择 Seedance 2.0。
- 需要 480p 或 720p 快速视频生成：选择 Seedance 2.0 Fast。
- 需要图片生成或图片编辑：选择 Grok Imagine 图片模型。
- 需要文生视频、图生视频或受支持的视频编辑：选择 Grok Imagine 视频模型。

## 模型选择矩阵

| 任务 | 推荐模型页 | 需要重点核对 |
| --- | --- | --- |
| Seedance 标准视频 | [Seedance 2.0](/models/seedance-2) | 参考媒体、分辨率、比例、时长、音频 |
| Seedance 快速视频 | [Seedance 2.0](/models/seedance-2) | Fast 分辨率边界和输入是否包含视频 |
| 图片生成与编辑 | [Grok Imagine 图片](/models/grok-imagine-image) | 图片数量、质量、输入 URL 和编辑限制 |
| Grok 视频任务 | [Grok Imagine 视频](/models/grok-imagine-video) | 生成、图生视频、编辑和异步状态 |

## Seedance 视频生成

Seedance 2.0 支持文本、首尾帧、多参考图片、参考视频和参考音频组合。Fast 版本面向较低分辨率和更快生成路径。完整内容结构与有效组合见[Seedance 多模态输入](/guides/seedance-multimodal)。

## Grok Imagine 图片

Grok Imagine 图片模型覆盖标准生成、高质量生成、单图编辑和多图编辑。输入媒体、数量和质量字段按[模型说明](/models/grok-imagine-image)与[图片 API](/api-reference/images)执行。

## Grok Imagine 视频

Grok 视频接口覆盖文生视频、受支持的图生视频和视频编辑路径，并通过异步任务返回结果。模型边界见[Grok Imagine 视频](/models/grok-imagine-video)和[视频 API](/api-reference/videos)。

## 核对输入与输出能力

选择模型后依次核对：输入类型和数量、输出分辨率、宽高比、时长、音频开关、媒体有效期和模型特有限制。不要把一个模型的参数默认值套用到另一个模型。

## 查看权威参数与价格

接口参数以对应[API 参考](/api-reference)为准，当前模型价格以模型说明和平台模型广场为准。入口页不复制可能变化的价格，避免多个页面产生不一致。
