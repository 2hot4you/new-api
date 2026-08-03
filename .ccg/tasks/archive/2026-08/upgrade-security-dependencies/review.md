# Review

## 变更范围

- Go 工具链要求：`1.25.1` → `1.26.5`。
- `golang.org/x/image`：`0.41.0` → `0.43.0`。
- `golang.org/x/text`：`0.37.0` → `0.39.0`。
- `golang.org/x/sync` 由依赖解析升级到 `0.21.0`。
- 前端升级 DOMPurify、Hono、Hono Node Server、fast-uri、brace-expansion、PostCSS，并更新 Bun 锁文件。
- 未修改 HTTPS、反向代理、Cookie 或本地 `.env` 配置。

## 安全验证

- `govulncheck ./...`：0 个可达漏洞；扫描前为 4 个。
- `bun audit`：No vulnerabilities found；扫描前为 15 个。
- `bun install --frozen-lockfile`：通过。
- Shadcn CLI 与 AJV schema 编译兼容性检查：通过。

## 回归验证

- `go mod verify`：通过。
- `go test ./...`：通过。
- `go vet ./...`：通过。
- `bun test`：173 passed，0 failed。
- `bun run typecheck`：通过。
- `bun run build`：通过。
- `git diff --check`：通过。
- 常驻后端使用 Go 1.26.5 重新构建并启动，`GET /api/status` 返回 HTTP 200。

## 既有质量基线

- 全量前端 lint 仍为 374 errors / 81 warnings。
- 全量前端格式检查仍有 5 个文件不通过。
- 数量与升级前审计一致，涉及文件不在本次依赖变更范围内；本次未进行大范围历史代码格式化或 lint 重写。

## Findings

未发现本次依赖升级引入的 Critical 或 Warning 问题。第一次并行执行 Go 测试与前端构建时，Rsbuild 清理 `web/dist` 导致 Go embed 瞬时缺少 `index.html`；在前端构建完成后顺序重跑 Go 测试通过，确认是验证编排竞态。
