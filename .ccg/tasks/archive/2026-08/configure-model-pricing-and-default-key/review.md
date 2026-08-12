# 最终审查

结论：通过，无未解决的 Critical 或 Important 问题。

## 范围

- 仅为 `minimax-m3`、`qwen3.5-flash`、`qwen3.5-plus` 配置 CNY 分段表达式、模型目录元数据与模型广场详情。
- Token 表新增 `is_default`；密码、OAuth、微信自助注册与默认 Key 在同一事务中创建。
- 默认 Key 可编辑、启停和确认式轮换；禁止单删和批量删除，存量用户不回填。
- Playground 与其他模型未修改。

## 核心审查结果

- 计费档位使用完整输入长度 `len`，输入、输出、缓存命中分别使用 `p`、`c`、`cr`，无缓存 Token 重复计价。
- 人民币价格在模型广场不经美元汇率二次转换。
- 三个模型详情均具备概览、分档价格表、性能空状态/指标和真实 `/v1/chat/completions` API 示例与参数表。
- 默认 Key 删除返回稳定码 `default_token_delete_forbidden`；批量删除在执行前检查并整批拒绝。
- PostgreSQL 部署迁移是幂等的，部分唯一索引保证每个用户至多一条未删除的默认 Key。

## 验证

- `go test ./... -count=1`
- 默认 Key 关键测试 `-race -count=3`
- `go vet` 相关 Go 包
- Web `bun test`：258 通过，0 失败
- Web `bun run typecheck`
- PostgreSQL 15 迁移契约测试与本地数据库迁移
- 本地 `127.0.0.1:3000` 的 `/api/status`、`/api/pricing` 以及三个模型详情页浏览器走查
- `git diff --check`

## 保留说明

- 本地 `ServerAddress` 当前仍由系统配置为 `https://aigc.molii.co`，因此泛用 API 示例会读取该值；本任务未擅自修改站点全局域名配置。
- 未创建真实用户或持久化测试凭据。
