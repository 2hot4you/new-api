# rc.30 PostgreSQL / Redis 验证记录

## 外部模型审查

- 按 CCG 要求并行尝试 antigravity 与 Claude 分析。
- 两个调用均因本机不存在 `/Users/naf/.claude/bin/codeagent-wrapper` 而以退出码 127 失败；没有外部模型输出可供采纳。

## 本地 CI 检查点

- PostgreSQL：`postgres:15-alpine`，四个隔离测试数据库。
- Redis：`redis:7-alpine`，仅使用唯一命名空间测试键，不执行共享实例清空操作。
- `make test`：通过（全新 PostgreSQL/Redis 服务容器）。
- `go test -race ./common ./middleware ./service ./controller ./relay ./router -count=1`：通过。
- `.github/workflows/ci.yml`：Ruby YAML 解析通过。
- `gofmt -d`：无输出。
- `git diff --check`：通过。

## 审查结论

- CI 的数据库均由 job 内服务容器提供，不读取 development 或 production 凭据。
- 完整迁移测试使用专用数据库，验证首次迁移、重复迁移、表数量稳定、令牌与预填分组数据保留。
- Redis 集成测试验证读写、原子增量、TTL 保留和精确测试键删除，不执行 `FLUSHDB`/`FLUSHALL`。
- 本地首次完整复跑曾因前序单测遗留数据失败；重建四个明确命名的本地临时数据库后，CI 顺序完整通过。该现象不影响 GitHub Actions 的全新服务容器，但说明本地重复执行前必须使用新的隔离数据库。
