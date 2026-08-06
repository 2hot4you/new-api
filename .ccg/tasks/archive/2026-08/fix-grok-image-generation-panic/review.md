# 审查记录

## 根因

请求 ID `202608060248435714830008268d9d6voDuQdqL` 的恢复堆栈指向 `controller/relay.go:140`。图片直价预估逻辑在 `relay.ImageHelper` 之前执行，但 `RelayInfo.ChannelMeta` 原本只在各协议 handler 内初始化。访问内嵌指针提升字段 `relayInfo.ApiType` 因而触发 nil pointer panic。

## 修复

- 在图片直价预估选择 adaptor 前调用 `relayInfo.InitChannelMeta(c)`，使用分发中间件已经选定并写入 Gin context 的渠道信息。
- 保留 `ImageHelper` 内原有的渠道元数据刷新，避免扩大重试流程的行为变更。
- 新增控制器回归测试，验证 Molii Grok 图片计费错误返回 HTTP 400 而不是 panic。

## 审查结论

- Critical：无。
- Warning：无。
- Info：未调用外部模型审查，因为用户明确禁止 antigravity 和 Claude；已完成源码堆栈审计、红绿回归测试和本地 diff 自审。

## 验证证据

- RED：`go test ./controller -run '^TestRelayMoliiGrokImagePricingErrorDoesNotPanic$' -count=1` 在修复前稳定复现 `controller/relay.go:140` panic。
- GREEN：同一命令在修复后通过。
- `go test ./controller ./relay ./relay/channel/moliigrok ./relay/helper ./router -count=1`：通过。
- `go test ./service ./setting/ratio_setting -count=1`：通过。
- `go vet ./controller ./relay ./relay/channel/moliigrok`：通过。
- `go build ./...`：通过。
- `go test ./... -count=1`：通过。
- `git diff --check`：通过。
- launchd `com.molii.new-api` 已替换为新二进制并重启，PID `34095`，TCP 3000 正常监听，`GET /api/status` 返回 200。

## 回滚

旧二进制备份：`/Users/naf/Library/Application Support/Molii/new-api/new-api.previous-20260806-before-grok-image-panic-fix`。

