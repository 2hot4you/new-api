# 上游补丁处置台账

状态说明：`equivalent` 表示 Molii 已有经过测试的改写实现；`adopt` 表示需要纳入；
`rewrite` 表示需结合 Molii 定制重写；`reject` 表示明确不适用于本 fork。

## v1.0.0-rc.24

| Commit | 状态 | 依据 |
| --- | --- | --- |
| `d6b5ce99d` HTTP/2 请求体重放 | equivalent | Molii 已有 `ReplayableBody`、独立 reader、`GetBody`、禁止跟随重定向及 HTTP/2 回归测试。 |
| `ea4f02101` replay metadata 重构 | equivalent | Molii 将 replay metadata 封装在 body 类型中，避免跨渠道共享陈旧 metadata。 |
| `0cd9dc85e` 用户并发更新加固 | equivalent | 已有 `UpdateUserAccessToken`、邀请额度原子更新、用户更新排除并发计费与 token 字段及测试。 |
| `c9bc03864` 渠道抓取模型分类 | equivalent | Git patch-ID 等价。 |
| `b941253ae` Claude/Gemini 原生测试格式 | equivalent | Git patch-ID 等价。 |
| `1da23d6b3` 用户敏感操作限流 | equivalent | 已有按用户和 scope 的 `UserCriticalRateLimit`，access-token 与 aff-transfer 路由已启用并有 Redis key 测试。 |
| `e926e5cac` 兑换码额度精度 | equivalent | Git patch-ID 等价。 |
| `5c3abffe8` GitCode 发布同步 | reject | Molii 使用独立 origin/deploy workflow，不同步 QuantumNous 的 GitCode release。 |

定向验证：

```text
go test ./common -run 'Test.*BodyStorage' -count=1
go test ./relay/common -run 'TestNewOutboundJSONBody' -count=1
go test ./relay/channel -run 'TestApplyUpstream|TestDoTaskApiRequest|TestUpstreamGetBody|TestDoRequest_DoesNotFollowRedirects' -count=1
go test ./middleware -run 'TestUserCriticalRateLimit' -count=1
go test ./model -run 'TestUserUpdate|TestUpdateUserAccessToken' -count=1
```

结果：全部通过。

