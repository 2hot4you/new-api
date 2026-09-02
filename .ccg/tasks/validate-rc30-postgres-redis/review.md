# rc.30 PostgreSQL / Redis 验证记录

## 外部模型审查

- 按 CCG 要求并行尝试 antigravity 与 Claude 分析。
- 两个调用均因本机不存在 `/Users/naf/.claude/bin/codeagent-wrapper` 而以退出码 127 失败；没有外部模型输出可供采纳。
- 完成实现后再次并行尝试双模型审查，结果仍均为退出码 127；因此以完整自动化测试、真实 development 部署和人工差异审查替代，未伪造外部审查结论。

## 本地 CI 检查点

- PostgreSQL：`postgres:15-alpine`，四个隔离测试数据库。
- Redis：`redis:7-alpine`，仅使用唯一命名空间测试键，不执行共享实例清空操作。
- `make test`：通过（全新 PostgreSQL/Redis 服务容器）。
- `go test -race ./common ./middleware ./service ./controller ./relay ./router -count=1`：通过。
- `.github/workflows/ci.yml`：Ruby YAML 解析通过。
- `gofmt -d`：无输出。
- `git diff --check`：通过。
- `bun run typecheck`：通过。
- `bun run test`：144 个文件、792 项测试全部通过。
- `bun run build`：通过。
- 部署契约测试：44 项断言全部通过。

## development 备份与候选部署

- 受保护工作流：[GitHub Actions 33630723092](https://github.com/2hot4you/new-api/actions/runs/33630723092)。
- 候选源码：`afc084f33f3a39e79b249660be1d9df0db090c2c`；镜像 digest：`sha256:e958bc3e50739f2537d29f2a14c4736c2a79d7dba6bc7bc9455414d42590202f`。
- 备份：`/opt/molii/development/backups/new-api-before-afc084f33f3a39e79b249660be1d9df0db090c2c.dump`。
- 备份 SHA-256：`67b4c934993b06808d2357d5d7a466b53b35b39de87baa93e1b27188a65f059e`。
- 备份非空、权限为 `0600`，且 `pg_restore --list` 校验通过；旧镜像引用保存于 `/opt/molii/development/.rc30-previous-image`。
- 候选镜像首次部署健康，随后主动重启 development 容器，第二次启动仍健康。
- 公网检查：`/`、`/dashboard/overview`、`/keys`、`/pricing`、`/rankings`、`/docs/quick-start`、`/temporary-assets`、`/usage-logs/common`、`/usage-logs/drawing` 均返回 HTTP 200；状态、设置、公告和 uptime 接口返回成功。
- 未持有可复用的登录会话或测试 API Key，因此没有执行需要鉴权或可能产生费用的真实媒体请求；相关接口的未授权请求保持 401，没有出现 500。

## 审查结论

- CI 的数据库均由 job 内服务容器提供，不读取 development 或 production 凭据。
- 完整迁移测试使用专用数据库，验证首次迁移、重复迁移、表数量稳定、令牌与预填分组数据保留。
- Redis 集成测试验证读写、原子增量、TTL 保留和精确测试键删除，不执行 `FLUSHDB`/`FLUSHALL`。
- 本地首次完整复跑曾因前序单测遗留数据失败；重建四个明确命名的本地临时数据库后，CI 顺序完整通过。该现象不影响 GitHub Actions 的全新服务容器，但说明本地重复执行前必须使用新的隔离数据库。
- 手工审查发现 `source_ref` 若不限制可能被 production 手动运行采用；已增加门禁，使候选源码、development 备份和重复启动参数仅能在 `develop` 工作流运行中使用，并补充契约测试。
