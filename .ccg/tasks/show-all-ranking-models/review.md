# 审查结果

## 双模型审查

- antigravity：调用失败，`/Users/naf/.claude/bin/codeagent-wrapper` 不存在，退出码 127。
- Claude：调用失败，`/Users/naf/.claude/bin/codeagent-wrapper` 不存在，退出码 127。

## 本地交叉审查

### Critical

- 无。

### Warning

- 完整模型历史会随活跃模型数增加响应体和图表序列数量；这是用户明确要求的完整展示语义，Tooltip 已通过 `maxContentHeight` 限制可视高度并在内容区滚动。

### Info

- 模型历史新增可选 `model_icon` 字段，属于向后兼容的 API 增强。
- Tooltip 图标映射改为来自完整的 `models_history.models`，不再受排行榜主体 20 行限制。
- 图表头部总 Token 改为完整历史模型总量，和图表及 Tooltip 总计保持一致。
- `buildVendorShareHistory`、`rankingVendorLimit` 和供应商 `Others` 聚合未修改。
- 未修改计费、模型 metadata、GPT Image 2 或部署配置。

## 验证

- TDD 红灯：后端缺少 `ModelIcon` 字段；前端历史外模型无图标且 Tooltip 无滚动高度限制。
- `go test ./service -count=1`：通过。
- `go test ./controller -count=1`：通过。
- `go test ./... -count=1`：通过（首次与前端构建并行时因 `web/dist/index.html` 尚未生成失败；构建完成后重新运行通过）。
- `go vet ./...`：通过（构建完成后重新运行）。
- 排行榜前端测试：9 通过，0 失败。
- `bun run typecheck`：通过。
- scoped oxlint：通过。
- `bun run i18n:check`：通过。
- `bun run build`：通过。
- `git diff --check`：通过。
