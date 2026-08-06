# Grok 视频播放失败修复需求

## 根因

本地任务日志页面运行在 `http://localhost:3000`，数据库 `ServerAddress` 配置为 `https://aigc.molii.co`。后台任务列表把该全局地址写入预览 URL，导致浏览器把本地任务请求发送到不可用的远端域名，本地 `/content` 代理没有收到请求。

## 修复范围

- 后台任务列表中的 StarAI/Molii Grok 成功任务返回同源相对签名 URL。
- 用户侧视频任务 API 继续返回基于 `ServerAddress` 的绝对 URL，不改变外部 API 契约。
- 保留签名、用户归属、24 小时有效期及上游 URL 隐私保护。
- 不修改数据库 `ServerAddress`，避免影响生产域名配置。

## 验收

- 本地任务日志预览请求命中当前站点 `/v1/videos/:task_id/content`。
- 指定任务可在 Chrome 预览。
- 签名校验、视频代理和任务列表测试通过。
