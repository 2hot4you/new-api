# Review: StarAI 上游素材 ID 透传与用户隔离

## 结论

- Critical: 无。
- Warning: 无阻塞项。
- Info: 管理员按单个 ID 操作时，如果多个用户恰好持有完全相同的上游 ID，会安全返回歧义错误，不会选择任意用户记录；普通用户接口不受影响。

## 安全审查

- 新记录以 `user_id + upstream_id` 组成 Redis 隔离键，不同用户不会互相覆盖。
- 普通查询、刷新、删除和视频请求解析均从认证上下文取得当前 `user_id`。
- 未绑定的上游 ID 不会直接发送给上游验证，避免把平台变成跨用户素材探测器。
- 同一用户的不同 Token 共用同一个用户作用域，符合已确认需求。
- 旧 `asset-molii-*` 仅在旧 Redis 记录仍存在且 `user_id` 匹配时兼容。
- 渠道 ID、渠道 Key 指纹、7 天 TTL 与 COS 清理逻辑保持不变。
- 差异中未发现 API Key、密码或其他硬编码密钥。

## 验证

- `go test ./... -count=1`
- `go test ./relay/channel/task/starai -run TestTemporaryAssetIsVerifiedBeforeBuildingUpstreamRequest -count=1 -v`
- `bun run typecheck`（`web`）
- `bun test src/features/temporary-assets`（`web`）
- `bun run build`（`web`）
- `bun test scripts/seedance-content-contract.test.ts`（`docs-site`）
- `bun run api:lint`（`docs-site`）
- `bun run build`（`docs-site`）
- `git diff --check`

## 审查限制

CCG 要求的 antigravity 与 Claude 外部审查命令无法运行，因为本机不存在 `~/.claude/bin/codeagent-wrapper`。本次仅完成了本地代码、安全边界和测试审查，没有声称完成外部双模型审查。
