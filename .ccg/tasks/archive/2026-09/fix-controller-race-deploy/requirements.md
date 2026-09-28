# Requirements

- 修复 `TestPreConsumePolicyDatabaseMatrix` 在 Go Race Detector 下的稳定竞态失败。
- Redis 未启用时不投递无意义的用户额度缓存异步任务。
- 同时覆盖用户额度增加和扣减路径。
- Redis 启用时保持现有异步缓存更新语义。
- Seedance 计费测试不得因默认零余额误触发与测试目标无关的低余额异步通知。
- 通过定向 controller race 测试、CI 同款 race 命令及完整 Go 测试。
- 提交并推送到远端 `develop`，跟踪自动部署结果。
