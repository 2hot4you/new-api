# 交付审查

## 结果

- Git 功能提交：`088771cd feat: add Docker deployment bundle`
- Git 分支：`origin/feat/molii-auth`
- Docker Hub：`pangmaom1/new-api`
- 标签：`molii-2026.08.03`、`sha-088771cd`、`latest`
- OCI 清单摘要：`sha256:fadbde77ae38a1b7e486085ed112ab16447c56ee06428002da7b252097496e55`
- 架构：`linux/amd64`、`linux/arm64`
- 供应链元数据：BuildKit SBOM 和 provenance 已发布为 attestation manifest。

## 交付文件

- `Dockerfile`：生产多阶段构建、版本注入、OCI 标签。
- `deploy/docker-compose.yml`：New API、PostgreSQL、Redis 和一次性迁移服务。
- `deploy/.env.example`：仅包含占位配置，不包含真实密钥。
- `deploy/migrations/20260803_molii_postgres.sql`：幂等 PostgreSQL 迁移。
- `deploy/README.md`：部署、备份、迁移重跑、升级及 HTTPS 后续配置说明。

## 验证

- `go test ./...`：通过。
- `go vet ./...`：通过。
- `govulncheck ./...`：无可达漏洞。
- 前端测试：173 通过，0 失败。
- `bun run typecheck`：通过。
- `bun audit`：无漏洞。
- `docker build --check .`：通过。
- 本地生产镜像构建：通过。
- Compose 配置解析：通过。
- 迁移在既有 PostgreSQL 上连续执行两次：通过，保持幂等。
- 全新本地 Compose 环境：迁移退出码 0，应用健康，`/api/status` 返回 200。
- 从 Docker Hub 回拉 `sha-088771cd` 后的全新 Compose 环境：迁移退出码 0，应用健康，版本为 `molii-2026.08.03`，`tokens.auto_groups` 为可空 `text`。
- 三个 Docker Hub 标签指向同一 OCI 清单摘要，并同时包含 amd64/arm64。

## 已知基线问题

- 全局前端 lint 仍有仓库既有的 374 个错误和 81 个警告。
- 全局格式检查仍报告 5 个仓库既有文件。
- 本次新增部署文件未引入上述问题。

## 部署边界

- 当前 Compose 默认只将应用绑定到 `127.0.0.1:3000`，符合后续由反向代理提供 HTTPS 的部署方式。
- HTTPS 上线时需将 `SESSION_COOKIE_SECURE` 改为 `true`，并配置生产来源域名。
- 未提交或持久化任何真实密码、会话密钥或测试凭据。
- 按用户明确要求，没有调用 antigravity 或 Claude；审查依据本地测试、静态检查、镜像清单检查及远端回拉验收。
