# StarAI Seedance Mini 与 2.5 适配

## 范围

- 注册 `doubao-seedance-2-0-mini-260615` 与 `doubao-seedance-2-5-260628`。
- 使用公开刊例价配置 480p、720p、1080p 及是否包含视频输入的价格矩阵。
- Mini 支持 480p/720p、4–15 秒；2.5 支持 480p/720p/1080p、4–30 秒。
- 成功任务响应直接返回经过安全校验的火山 TOS 签名视频 URL。
- 解析真实 duration、ratio、resolution、FPS、seed 与 usage 数据。
- 将 StarAI 默认 Base URL 更新为高可用节点，并同步临时素材格式与 50 MB 视频限制。

## 约束

- 不公开、存储或推导展示上游采购折扣。
- 最终结算继续使用上游真实 `total_tokens`。
- 不修改其他渠道、价格、部署或 CI/CD。
- 保留现有项目标识和版权信息。
