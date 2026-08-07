# Molii AIGC 测试 Demo 实施计划

## 架构

在 `tools/molii-aigc-demo/` 创建独立 Go module。Demo 默认监听 `127.0.0.1:8787`，使用标准库 HTTP 服务和内嵌原生 HTML/CSS/JS，不修改 New API 的 `web/`。

浏览器只访问 Demo；Demo 解密所选环境的 API Key 后调用目标 New API。浏览器不接触明文 Key，也不使用 localStorage、sessionStorage、IndexedDB 或 Service Worker。

## Layer 1：可并行模块

### A. SQLite 与密钥安全

文件范围：

- `tools/molii-aigc-demo/internal/secure/*`
- `tools/molii-aigc-demo/internal/store/*`

职责：

- AES-256-GCM 加解密，随机 nonce，环境 ID + key version 作为 AAD。
- SQLite 打开、权限、WAL、busy timeout、显式迁移。
- environments、ui_sessions、runs、exchanges 表与 CRUD。
- API DTO 不返回密文或明文 Key。
- 运行记录支持待轮询任务恢复、账单更新和完整交换时间线。

### B. 模型目录、请求与计费

文件范围：

- `tools/molii-aigc-demo/internal/catalog/*`
- `tools/molii-aigc-demo/internal/upstream/*`
- `tools/molii-aigc-demo/internal/billing/*`
- `tools/molii-aigc-demo/internal/jobs/*`

职责：

- Seedance/Grok 模型、操作、参数元数据。
- 构造并校验真实 New API 请求，不自动重试生成 POST。
- HTTP 调用、响应大小限制、超时、重定向安全、日志脱敏。
- 动态读取 `/api/pricing` 并计算预估费用。
- 调用 `/api/log/token`，按 request_id 或 other.task_id 匹配实际结算。
- 异步视频和素材服务端轮询，支持启动恢复和取消。

### C. 独立测试界面

文件范围：

- `tools/molii-aigc-demo/static/*`

职责：

- 环境保存、编辑、删除、连通性测试、自由切换。
- Provider/模型/操作切换，完整参数表单和 Seedance content 动态数组。
- 实时 JSON/curl 预览，预估/实际/差额展示。
- 请求与响应时间线、轮询进度、图片/视频结果预览与下载。
- 所有浏览器状态仅保存在页面内存，通过 Demo API 持久化。

## Layer 2：集成

主代理负责：

- `tools/molii-aigc-demo/go.mod`
- `tools/molii-aigc-demo/cmd/server/main.go`
- `tools/molii-aigc-demo/internal/app/*`
- `tools/molii-aigc-demo/.env.example`
- `tools/molii-aigc-demo/.gitignore`
- `tools/molii-aigc-demo/README.md`

集成内部 API：

```text
GET    /healthz
GET    /api/bootstrap
GET    /api/environments
POST   /api/environments
PUT    /api/environments/{id}
DELETE /api/environments/{id}
POST   /api/environments/{id}/select
POST   /api/environments/{id}/test
POST   /api/preview
POST   /api/runs
GET    /api/runs
GET    /api/runs/{id}
POST   /api/runs/{id}/cancel
GET    /api/runs/{id}/media
```

使用 HttpOnly、SameSite=Strict 会话 Cookie；写接口校验 CSRF、Origin 与 Host；静态资源 `Cache-Control: no-store`。

## 验收

- 多环境和 Key 加密持久化到 SQLite，SQLite 中不存在明文 Key。
- UI/源码不调用任何浏览器持久化 API。
- 所有模型/操作的字段、默认值、枚举和条件约束与现有适配器一致。
- 异步任务可自动轮询、重启恢复、取消轮询和下载结果。
- 日志展示不包含 Authorization、Cookie、签名 query 或明文 Key。
- 预估价格来自目标 `/api/pricing`，实际价格来自 `/api/log/token`；不可用时明确标记待同步。
- `GOWORK=off go test ./...`、`go test -race ./...`、`go vet ./...` 通过。
- 使用本地 New API 完成环境连通、价格目录和无效 Token 请求验证；不发起付费生成。

## 实施状态

- [x] Layer 1A：SQLite、密钥加密、迁移、运行与交换记录
- [x] Layer 1B：模型目录、请求校验、HTTP 客户端、轮询与计费
- [x] Layer 1C：独立响应式测试界面
- [x] Layer 2：会话安全、API 集成、媒体代理与进程入口
- [x] 单元测试、Race Detector、Vet、前端静态检查
- [x] 本地进程联调与非付费 New API 连通性验证
- [x] 独立审查与最终修复
