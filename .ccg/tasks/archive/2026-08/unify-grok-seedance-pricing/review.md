# 审查结果

## 结论

- Critical：0
- Warning：0
- Info：1（现有 Molii Grok 渠道需要在模型列表中启用或重新获取 `grok-imagine-image-2.0`，公开模型广场才会展示该模型；本次不自动改写已保存的渠道配置。）

## 契约复核

- 官方依据：`https://docs.x.ai/developers/models/grok-imagine-image-2.0`。
- 图片输入：¥0.01 / 张。
- Low：1K ¥0.04 / 张，2K ¥0.06 / 张。
- Medium：1K ¥0.06 / 张，2K ¥0.08 / 张。
- 缺省 `quality` 按官方基础价格对应的 `medium` 档处理。
- 新模型使用独立模型族，不与旧 Grok 图片模型互相映射。
- `quality` 已贯穿请求转换、预估计费、计费快照、使用日志、公开目录、模型广场、Demo 和文档。
- Seedance、Grok Imagine 与通用模型定价已统一到模型定价页；旧 Imagine 定价地址保留兼容但从侧栏隐藏。

## 验证证据

- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `web/bun run typecheck`：通过。
- 前端 10 个定向测试文件：47 项通过，0 失败。
- `web/bun run format:check`：通过。
- `web/bun run i18n:check`：通过。
- 变更前端文件 scoped `oxlint`：通过。
- `docs-site/bun test scripts/grok-content-contract.test.ts`：8 项通过，0 失败。
- `docs-site/bun run api:lint`：OpenAPI 校验通过。
- `tools/molii-aigc-demo/go test ./... -count=1`：通过。
- `git diff --check`：通过。
- `GET http://127.0.0.1:3000/api/status`：HTTP 200，`success=true`。

## 说明

用户明确要求不调用 antigravity 或 Claude，因此本任务未执行 CCG 外部双模型审查；改为本地源码复核、自动化测试与运行时页面验证。
