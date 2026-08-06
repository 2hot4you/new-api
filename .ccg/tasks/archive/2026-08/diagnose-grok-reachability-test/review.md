# Molii Grok Imagine API 可达性测试排查结论

## 根因

- 前端 `getChannelTestAction(62)` 明确返回 `Configuration Check`，因此页面不会显示“可达性测试”。
- 后端 `testMoliiGrokChannel` 只验证 Key 与固定 Base URL 是否存在，不执行 DNS、TCP 或 HTTPS 请求。
- 现有测试 `TestMoliiGrokChannelTestPerformsConfigurationCheckOnly` 与前端渠道测试均锁定了该行为。

## 原始设计目的

- 避免通过图片或视频生成接口进行测试而产生上游费用。
- 但当前实现也没有采用 StarAI 渠道已有的无付费 DNS/TCP 可达性检查，因此无法验证上游网络是否真正可达。

## 建议

- 保留 Key 配置校验。
- 增加针对固定 Base URL 的 DNS/TCP 可达性检查，不进行 TLS 握手、不发送 HTTP 请求、不触发生成任务。
- 前端动作名称改为“可达性测试”，后端成功消息明确说明仅验证配置与网络连通性。

## 处置

- 本任务只完成诊断，未修改代码或渠道配置。
- 按用户既有要求未调用 antigravity 或 Claude。
