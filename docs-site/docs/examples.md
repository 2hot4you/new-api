---
sidebar_position: 6
---

# 示例与工具

示例展示安全、可复制的调用模式。先按任务选择语言和模型，再进入 API 参考核对当前字段；不要把示例中的占位地址或 Key 直接用于生产。

## 按任务选择示例

| 任务 | 示例 | 对应参考 |
| --- | --- | --- |
| Seedance 完整视频流程 | [curl](/examples/seedance-curl) · [Python](/examples/seedance-python) · [TypeScript](/examples/seedance-typescript) | [Seedance API](/api-reference/seedance) |
| Grok 图片生成与编辑 | [Grok 图片 curl](/examples/grok-image-curl) | [图片 API](/api-reference/images) |
| Grok 视频创建与编辑 | [Grok 视频 curl](/examples/grok-video-curl) | [视频 API](/api-reference/videos) |
| Grok 任务查询与下载 | [查询与下载](/examples/grok-poll-download) | [异步任务](/api-basics/async-tasks) |

## Seedance 示例

Seedance 示例覆盖一次付费提交、有限轮询、状态处理和安全下载。Python 与 TypeScript 版本适合直接拆分到服务端任务模块，curl 版本适合验证接口和参数。

## Grok Imagine 示例

图片示例覆盖标准生成、高质量生成和编辑；视频示例区分文生视频、图生视频和编辑路径。查询与下载示例负责处理最终媒体，不应携带 API Authorization 跨域跟随重定向。

## 安全运行示例

- 使用 `$MOLII_API_KEY` 等环境变量占位符。
- 付费 POST 恰好执行一次，不自动重试。
- GET 查询设置请求超时、轮询间隔和总截止时间。
- 下载前验证状态码和视频 Content-Type。
- 错误日志只记录脱敏 Request ID、任务 ID 和公开原因。

## 从示例进入 API 参考

示例用于展示调用顺序，不替代参数定义。提交前核对[模型列表](/api-reference/models)、[图片 API](/api-reference/images)、[视频 API](/api-reference/videos)、[Seedance API](/api-reference/seedance)、[临时素材 API](/api-reference/assets)和[错误 API](/api-reference/errors)。
