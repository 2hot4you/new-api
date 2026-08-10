---
sidebar_position: 3
---

# 开发指南

这里汇总所有模型共享的公共调用规则。先理解环境、鉴权、媒体输入和异步任务，再进入具体模型或端点页面。

## 公共请求约定

所有请求使用 HTTPS、JSON 或端点明确要求的媒体格式。为每次请求设置连接和总超时，保存公开 Request ID，并把付费 POST 与安全 GET 查询区分处理。完整基础地址见[Base URL 与环境](/api-basics/base-url)。

## 环境与身份验证

API Key 只放在服务端 `Authorization: Bearer ...` 请求头中。开发、测试和生产使用不同 Key；日志、前端包、截图和工单不得包含完整密钥。详见[身份验证](/api-basics/authentication)。

## 选择媒体输入

根据模型选择公网 URL、Data URL 或 `asset://` 临时素材。提交前检查协议、Content-Type、文件可达性和有效期；Seedance 多参考输入还需要遵守角色和数量约束。阅读[媒体输入](/api-basics/media-inputs)、[Seedance 多模态输入](/guides/seedance-multimodal)和[临时素材](/guides/temporary-assets)。

## 处理异步任务

视频任务通常先返回任务 ID。付费创建请求只发送一次，查询请求按建议间隔有限轮询，成功后再读取媒体和最终计费。状态定义和查询响应见[异步任务](/api-basics/async-tasks)。

## 错误、超时与重试

只对明确安全且可恢复的查询操作重试。创建请求超时时先使用 Request ID 或任务记录确认是否已受理，不能直接重复提交。状态码矩阵见[错误与重试](/api-basics/errors-retries)，标准结构见[错误 API](/api-reference/errors)。

## 预计费用与最终结算

预估用于提交前展示，最终费用以任务完成后的实际 Token 和结算结果为准。不要自行猜测上游缺失的用量。详见[计费与用量](/api-basics/billing-and-usage)和[使用与生成记录](/platform/usage-and-generation-records)。

## 选择语言示例

- [Seedance curl](/examples/seedance-curl)
- [Seedance Python](/examples/seedance-python)
- [Seedance TypeScript](/examples/seedance-typescript)
- [Grok 图片 curl](/examples/grok-image-curl)
- [Grok 视频 curl](/examples/grok-video-curl)
- [Grok 查询与下载](/examples/grok-poll-download)

## 生产上线清单

- 使用独立生产 Key 和可信基础地址。
- 为连接、响应和异步任务分别设置超时。
- 不自动重试付费 POST。
- 验证媒体 Content-Type、大小、可达性和有效期。
- 记录公开 Request ID、任务 ID、模型、状态和最终费用。
- 对日志、错误原因和用户输入进行脱敏。
- 在上线前完成[快速开始](/quick-start)和目标模型的[API 参考](/api-reference)。
