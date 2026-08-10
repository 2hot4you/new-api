---
sidebar_position: 2
---

# 平台与账户

Molii 平台把账户、API 凭证、模型调用、临时素材、生成记录和账单放在同一套工作区中。先从当前目标进入对应页面，再回到这里检查完整使用链路。

## 平台能力地图

| 目标 | 入口 | 你可以完成的操作 |
| --- | --- | --- |
| 访问账户 | [注册、登录与找回密码](/platform/register-and-sign-in) | 注册、登录、验证和重置密码 |
| 查看运行情况 | [看板](/platform/dashboard) | 查看额度、调用趋势、模型分析和渠道分流 |
| 管理凭证 | [API 密钥](/platform/api-keys) | 创建、查看、停用和批量管理 Key |
| 准备媒体 | [临时素材](/platform/temporary-assets) | 上传或提交 URL、查询状态、预览和删除素材 |
| 核对结果 | [使用与生成记录](/platform/usage-and-generation-records) | 查找任务、Token、费用和生成状态 |
| 管理资金 | [钱包与账单](/platform/wallet-and-billing) | 充值、兑换和查看订单历史 |

## 从注册到生产使用

推荐顺序是：完成账户验证，创建权限受控的 API Key，发送测试请求，确认生成记录与费用，再为生产环境设置独立 Key。不要让多个环境共享同一个长期凭证。

## 看板与模型分析

[看板](/platform/dashboard)用于查看账户概览、模型调用趋势和分流情况。图表没有数据时先检查时间范围、Key 状态和是否确实产生过请求；看板统计不能替代单次任务日志中的最终结算信息。

## API Key 管理

为开发、测试和生产分别创建 Key，并按模型、额度和有效期限制使用范围。Key 泄露时立即停用，而不是只从代码仓库删除。具体操作见[API 密钥](/platform/api-keys)。

## 管理临时素材

[临时素材](/platform/temporary-assets)支持本地上传和 URL 来源。素材提交后需要等待上游状态变为可用，过期或删除后不能继续在生成请求中引用。API 工作流见[临时素材指南](/guides/temporary-assets)。

## 核对生成记录与费用

[使用与生成记录](/platform/usage-and-generation-records)展示任务状态、关键参数、预计 Token、实际 Token 和最终费用。异步任务应以成功或失败后的最终结算为准，不要把提交阶段的预估当作最终扣费。

## 保护账户

在[个人资料与安全](/platform/profile-and-security)维护登录方式和安全设置。不要向支持人员发送密码、完整 API Key、Authorization 请求头或包含敏感媒体的公开链接。

## 下一步

- 了解模型：[模型广场与 Playground](/platform/model-square-and-playground)
- 开始调用：[快速开始](/quick-start)
- 查看请求规则：[开发指南](/api-basics)
- 排查问题：[帮助与更新](/help)
