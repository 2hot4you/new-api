# Grok 图片结果持久化 502 排查与修复

## 范围

- 仅修改 Grok 图片响应持久化、必要测试和脱敏诊断日志。
- 不修改 main、生产配置或生产环境。
- Development 分支为 `develop`，部署目标为 `https://dev.molii.co`。

## 行为要求

- 先确认所有 Grok 图片模型共享链路中的真实失败阶段，不预设一定是 `mime_type`。
- 为持久化链路提供带 request ID 的结构化、脱敏阶段日志。
- 对外继续返回通用安全错误。
- 第一阶段仅增加诊断日志，不改变 HTTPS、MIME、SSRF、远程响应类型/大小、COS、Redis 锁与清理队列行为。
- 成功结果只返回 Molii COS 签名地址，不返回上游临时地址。
- 保持幂等对象键与 24 小时保留期。

## 验证

- 第一阶段覆盖所有规定 stage 的错误分类、日志脱敏、客户端通用错误、计费日志以及三个模型回归。
- 部署日志版后只执行一次 `grok-imagine-image` 请求，根据真实 stage 再实施第二阶段最小修复。
- 运行 service、moliigrok、controller 与全量 Go 测试。
- 双模型分析和审查后提交并推送 `origin/develop`，观察 Development 部署。
- 没有用户提供的测试 API Key 时，不执行真实付费请求。
