# Review

## Critical

- 无。

## Warning

- 无。

## Info

- `IncreaseUserQuota` 与 `DecreaseUserQuota` 仅在 Redis 启用时投递异步缓存更新；Redis 启用时的行为保持不变，禁用时移除无意义的后台任务。
- Seedance 提交计费测试将用户余额设在提醒阈值以上，避免无关的低余额通知后台任务污染测试清理。
- 定向 Race Detector：预扣费测试连续 10 次通过，Seedance 测试连续 20 次通过。
- 完整 `controller` Race Detector 通过。
- `go vet ./...` 与 `go test ./...` 通过。
