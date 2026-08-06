# 审查记录

## 结果

- 后端品牌范围规格与质量审查：通过，无 Critical/Warning。
- Grok 管理接口规格审查：通过；质量初审发现的缺失 quota 防误归零、管理根校验和请求超时问题均已修复，复审通过，无 Critical/Warning。
- 前端品牌、可达性动作与 i18n 规格和质量审查：通过，无 Critical/Warning。
- 依用户要求使用子智能体协同；未调用 antigravity 或 Claude。

## 实现结论

- 渠道类型 61 的用户可见品牌统一为 `Molii Volcengine Imagine API`，内部类型编号、Go 标识符、数据库记录和 `molii-aigc` owner 保持兼容。
- 渠道类型 62 的测试动作改为仅建立 TCP 连接的可达性测试，不进行 TLS、HTTP 或鉴权，不产生上游费用。
- Grok 余额通过环境配置的系统 PAT 与 `User-id` 调用管理根 `/api/status` 和 `/api/user/self`，按 `quota / quota_per_unit` 换算。
- Grok 模型列表通过保存的 enabled channel Key 调用管理根 `/v1/models`，不走生成前缀 `/xai`，不使用系统 PAT。
- 管理根仅允许 HTTP(S) 根 URL；余额和模型请求都有 15 秒整体超时；缺失/null quota 不会写零或禁用渠道。
- `.env` 示例仅包含占位配置，真实系统 PAT 未写入工作区。

## 验证证据

- `go test ./...`：通过。
- `go vet ./...`：通过。
- 变更 Go 文件 `gofmt -d`：无输出。
- Grok 管理接口/模型发现/可达性定向测试与 `-race`：通过。
- 前端 `bun test`：239/239 通过。
- 前端 `bun run typecheck`：通过。
- 前端变更文件 lint：通过。
- 前端 `bun run format:check`：通过。
- 前端 `bun run build`：通过。
- Compose 示例配置解析：通过。
- 真实上游只读验证：PAT + `User-id: 2205` 可读用户 quota；channel Key 请求管理根 `/v1/models` 成功，生成前缀下的模型/余额路径为 404。
- 本地页面验证：type 62 可达性测试成功（20ms）；模型发现返回 11 个模型；未配置管理 PAT 时余额操作显示明确配置错误且不更新余额。
- LaunchAgent `com.molii.new-api` 已用新二进制重启，状态 running，`/api/status` 返回 HTTP 200。
- `git diff --check`：通过；真实 PAT 工作区扫描未发现命中。

## 已知基线项

- 全库前端 lint 仍有本任务之外的既存错误；本次所有变更 TypeScript 文件 lint 均通过。
- 本地常驻服务按“不持久化真实测试凭据”的约束未写入 Grok 管理 PAT；部署时需配置三个 `MOLII_GROK_NEW_API_*` 环境变量后才能在页面更新余额。
