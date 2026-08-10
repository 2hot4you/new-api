---
sidebar_position: 1
---

# 快速开始

这条路径帮助你安全地创建凭证、完成第一个图片请求，并理解如何继续接入异步视频任务。所有示例都使用占位 Key，不会自动执行请求。

## 你将完成什么

- 创建并安全保存 Molii API Key。
- 选择一个已开放模型并发送图片生成请求。
- 读取结果、Request ID 和用量信息。
- 理解视频任务为什么需要提交与轮询两个阶段。

## 开始前准备

你需要一个可登录的 Molii 账户、可用额度，以及能够发送 HTTPS 请求的 curl、Python 或 TypeScript 环境。先阅读[注册、登录与找回密码](/platform/register-and-sign-in)，再确认运行环境不会把密钥写入日志或前端代码。

## 获取 API Key

在平台的 API Key 页面创建只授予所需模型权限的 Key。创建后立即复制并安全保存在服务端环境变量中；再次展示或复制时，先确认当前环境和操作人员权限安全。完整管理方式见[API 密钥](/platform/api-keys)和[身份验证](/api-basics/authentication)。

```bash
export MOLII_API_KEY='replace-with-your-key'
```

## 发送第一个请求

先使用图片生成接口验证鉴权、模型权限和响应处理。付费 POST 只提交一次，不要配置自动重试。

```bash
curl --fail-with-body --request POST 'http://127.0.0.1:3000/v1/images/generations' \
  --header "Authorization: Bearer $MOLII_API_KEY" \
  --header 'Content-Type: application/json' \
  --data '{
    "model": "grok-imagine-image",
    "prompt": "清晨薄雾中的竹林，自然光，电影感构图",
    "n": 1
  }'
```

生产环境请把基础地址替换为你的 Molii API 地址，并保持协议、主机和密钥来自可信配置。

## 理解响应

成功响应包含生成结果和请求相关信息；失败时记录公开 Request ID，再按状态码决定是否重试。不要在工单、日志或截图中附带完整 Authorization 请求头。接口字段见[图片 API](/api-reference/images)，错误处理见[错误与重试](/api-basics/errors-retries)。

## 继续完成视频工作流

视频生成通常先返回任务 ID，再通过查询接口获取进度、最终媒体和结算信息。按[视频生成工作流](/getting-started/video-workflow)完成一次提交、有限轮询和安全下载；使用参考媒体前先阅读[媒体输入](/api-basics/media-inputs)。

## 接下来

- 按任务选择模型：[模型与能力](/models)
- 了解异步状态：[异步任务](/api-basics/async-tasks)
- 查看完整语言示例：[示例与工具](/examples)
- 进入端点定义：[API 参考](/api-reference)
