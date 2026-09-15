# 审查结果

## 根因

异步任务原子结算把仅适用于单次请求的 `common.MaxQuota`（int32）用于校验用户钱包、令牌余额和累计统计。账户余额 `4,997,697,697` 超过该范围后，即使最终结算仅退款 14 quota，也会在事务提交前失败。

## 修复边界

- 用户钱包、令牌余额及累计统计改用项目既有的 `common.MaxWalletQuota`（JavaScript 安全整数上限）。
- 单次任务 `targetQuota`、预扣 `fromQuota` 继续受 `common.MaxQuota` 约束。
- 未修改计费公式、倍率、数据库结构、API 契约、任务轮询或同步计费路径。

## 审查

- 自审：未发现 Critical 或 Warning 问题。
- 外部双模型审查：已并行尝试 antigravity 与 Claude；两者均因本机不存在 `/Users/naf/.claude/bin/codeagent-wrapper` 而以 127 退出，未获得外部审查结果，也未声称完成外部双模型审查。

## 验证

- 新增生产故障值 `4,997,697,697` 的回归测试；修复前稳定复现 `current quota out of range`，修复后通过。
- `go test -race ./model -run '^TestTaskBillingDelta' -count=1`
- `go test -race ./service -run 'TaskBilling' -count=1`
- `go test ./model ./service -count=1`
- `go vet ./model ./service`
- `bun install --frozen-lockfile`、`bun run build`
- `go test ./... -count=1`
- `git diff --check`

以上验证均通过；本地未配置 PostgreSQL/Redis 集成测试 DSN，未连接或修改任何线上数据库。
