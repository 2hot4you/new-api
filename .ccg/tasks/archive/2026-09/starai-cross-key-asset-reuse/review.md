# Review: StarAI 跨 Key 素材复用

## 结论

- Critical：无。
- Warning：无。
- Info：素材创建仍只选择一个启用的 StarAI 渠道，且不自动重试有副作用的创建请求；上游素材 ID 可跨 Key 使用，因此不影响四个 Seedance 模型复用。

## 正确性与安全

- 生成请求仍先使用 `user_id + asset_id` 读取本地绑定，未绑定素材不会请求上游。
- 不同用户即使知道同一个上游素材 ID，也无法查询或引用对方的本地绑定。
- 状态验证改用当前生成渠道的 Key，成功后原样传递 `asset://<upstream_id>`。
- 原创建渠道被删除或禁用时，状态刷新可使用其他启用的 StarAI 渠道。
- 旧 `asset-molii-*` 映射、168 小时有效期、失败状态与错误信息持久化保持不变。
- 未发现硬编码密钥、计费变更或数据库迁移。

## 验证

- TDD 红灯：旧实现将跨 Key 素材解析为源 URL；旧记录不刷新；禁用渠道仍被使用。
- TDD 绿灯：新增跨 Key、Key 轮换、旧记录刷新、禁用渠道回退和上游调用前所有权校验覆盖。
- `go test ./service ./controller ./relay/channel/task/starai -count=1`
- `bun install --frozen-lockfile && bun run build`（生成 Go embed 所需的 `web/dist`）
- `go test ./... -count=1`
- `go vet ./service ./controller ./relay/channel/task/starai`
- `go test -race ./service ./controller ./relay/channel/task/starai -count=1`
- `git diff --check`

## 审查限制

CCG 指定的 antigravity 与 Claude 外部分析、审查命令均已并行尝试，但本机不存在 `~/.claude/bin/codeagent-wrapper`，退出码均为 127。本次未声称完成外部双模型审查。
