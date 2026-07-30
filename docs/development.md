# New API 本地二开环境

New API 的前端和 Go API 都直接运行在本机。Docker Compose 只启动 PostgreSQL
与 Redis，并且两个端口都只绑定到 `127.0.0.1`。

## 依赖

- Go 1.25.1（以 `go.mod` 为准）
- Bun
- Docker Desktop，或带有 Compose v2 的 Docker Engine
- GNU Make 或兼容实现
- OpenSSL（仅用于生成本地会话密钥）

## 首次启动

在仓库根目录准备本地环境变量：

```sh
cp .env.local.example .env
openssl rand -hex 32
```

将第二条命令的输出写入 `.env` 的 `SESSION_SECRET`。`.env` 已被 Git 忽略；
不要提交本地密钥。`SESSION_COOKIE_SECURE=false` 只适用于可信的本地 HTTP
环境，不得用于生产部署，也不要在该模式下设置
`SESSION_COOKIE_TRUSTED_URL`。

启动基础设施：

```sh
make infra-up
```

然后分别打开两个终端，在本机启动 API 和前端：

```sh
make dev-api
```

```sh
make dev-web
```

`make dev-api` 会在首次启动时生成 Go `embed` 所需的 `web/dist`，但不会运行
前端容器或后端容器。`make dev-web` 启动 Rsbuild 开发服务器，并将 `/api`、
`/mj` 和 `/pg` 代理到本机 API。运行 `make dev` 可以再次查看三进程启动说明。

## 首次配置与登录注册

第一次使用全新的 PostgreSQL 数据卷时，打开
<http://localhost:5173/setup/> 创建本地管理员并完成初始化。不要在生产环境复用
本地管理员密码。

初始化后的用户名、密码、注册开关和站点信息均由你在 New API 初始化向导与管理后台
自行配置。本次环境搭建不修改上游登录注册前端。

## 本地 URL

| 服务 | 地址 |
| --- | --- |
| Rsbuild 前端 | <http://localhost:5173> |
| Go API | <http://localhost:3000> |
| PostgreSQL | `127.0.0.1:5432` |
| Redis | `127.0.0.1:6379` |

如果修改 `MOLII_POSTGRES_PORT` 或 `MOLII_REDIS_PORT`，也要同步修改 `.env`
中的连接字符串。

## 日志、停止与重置

查看 PostgreSQL 和 Redis 日志：

```sh
make infra-logs
```

停止容器并保留数据：

```sh
make infra-down
```

删除容器及所有本地 PostgreSQL/Redis 数据后重新开始：

```sh
make infra-reset
make infra-up
```

`infra-reset` 会永久删除两个具名开发卷。仅重置初始化向导状态时，在 API
停止或准备重启的情况下运行：

```sh
make reset-setup
```

## 测试

运行 Go 测试：

```sh
make test
```

运行前端检查与生产构建：

```sh
cd web
bun install --frozen-lockfile
bun run typecheck
bun run lint
bun run build
```

可用以下命令检查 Compose 文件，而无需启动服务：

```sh
docker compose -f docker-compose.dev.yml config
```

## 上游与许可

本项目基于 [QuantumNous/New API](https://github.com/QuantumNous/new-api)
进行二次开发。分发修改版本时必须保留上游署名、原项目链接、受保护的版权
及许可声明，并随分发产物保留仓库中的 `LICENSE` 和
`THIRD-PARTY-LICENSES.md`。本地开发流程不会删除或替换这些信息。
