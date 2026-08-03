# 客户测试交付就绪度审计

## 结论

当前版本可继续用于本机内部演示，但不应直接作为公网客户测试环境交付。修复安全、部署配置和发布一致性阻点后，再进行受控客户测试。

## 已通过

- `go test ./...`：通过。
- `go vet ./...`：通过。
- Go 格式检查：0 个未格式化文件。
- `bun run typecheck`：通过。
- `bun test`：173 passed，0 failed（38 个测试文件）。
- `bun run build`：通过。
- PostgreSQL、Redis：容器健康。
- 常驻 Go 服务：launchd running/active，`GET /api/status` 返回 HTTP 200。
- `GET /api/pricing` 返回 HTTP 200，公开目录可见 2 个 Seedance 模型。
- 未鉴权调用视频生成路由返回 HTTP 401，路由存在且鉴权生效。
- `.env` 被 Git 忽略且未被跟踪。

## 阻点

### Critical

1. `govulncheck` 确认当前代码可达路径受 4 个漏洞影响：
   - `GO-2026-5970`：`golang.org/x/text@v0.37.0`，修复版本 `v0.39.0`。
   - `GO-2026-5856`：Go `crypto/tls@go1.26.4`，修复版本 Go `1.26.5`。
   - `GO-2026-5061`：`golang.org/x/image@v0.41.0` WebP 解码 panic，修复版本 `v0.43.0`。
   - `GO-2026-4961`：同模块的 32 位 WebP 大图 panic，修复版本 `v0.42.0`。
2. 当前运行环境是本地开发配置，但进程监听 `*:3000`：
   - `SESSION_COOKIE_SECURE=false`。
   - 未配置 `SESSION_COOKIE_TRUSTED_URL`，OriginGuard 未启用。
   - `GIN_MODE` 不是 release，`DEBUG` 未关闭。
   - PostgreSQL 未要求 TLS；Redis 未配置认证或 TLS。
   该实例不得直接暴露到公网。

### Warning

1. `bun audit` 报告 15 个依赖漏洞（6 high、8 moderate、1 low）。多数高危项来自开发依赖 `shadcn` 的传递依赖；生产依赖 `dompurify` 存在 low 漏洞，仍需分层升级并复查生产依赖集合。
2. 全量前端 lint 未通过：374 errors、81 warnings。
3. 前端格式检查未通过，涉及 5 个文件。
4. 本地分支比 `origin/feat/molii-auth` 多 6 个提交，远端客户无法获得性能 500 修复和 API 示例改进。
5. 尚未用有效 API Key 对 StarAI Seedance 创建任务、轮询状态、资产访问做真实端到端验证；本次只验证了公开目录、路由和未授权响应。
6. 当前没有面向客户的、可复现的生产发布版本/tag 与完整交付运行手册；现有 launchd 常驻配置是本机部署状态。

## 交付前最低门槛

1. 升级 Go 工具链和受影响 Go 模块，重新运行 `govulncheck`、测试和 vet。
2. 处理前端运行时依赖漏洞；对开发依赖漏洞记录风险接受或升级方案。
3. 建立 HTTPS 反向代理和生产 `.env`：Secure Cookie、精确 Origin、精确可信代理、release/debug 配置、数据库与 Redis 生产凭据/TLS。
4. 清理 lint 和格式门禁，或明确建立经批准的基线，而不是带着 374 个 error 交付。
5. 将准备交付的提交推送到客户可访问的分支并固定版本/tag。
6. 由用户自行提供测试账号/API Key，完成一次 StarAI 创建、轮询、成功资产访问和失败场景验证；不在仓库中持久化凭据。
7. 随交付保留 `LICENSE`、`THIRD-PARTY-LICENSES.md` 及上游署名；AGPL-3.0 义务应由交付方结合实际托管/分发方式做法律确认。
