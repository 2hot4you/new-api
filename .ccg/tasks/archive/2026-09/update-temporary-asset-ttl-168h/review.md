# Review

## Outcome

- Seedance 临时素材的默认生命周期由 24 小时调整为 168 小时（7 天）。
- Redis 主键 TTL、响应 `expires_at`、用户索引 TTL 和 COS 清理索引共用同一生命周期。
- `STARAI_ASSET_TTL_HOURS` 正数覆盖能力保留；已有素材不延长原过期时间。
- `/temporary-assets` 页面、七种语言、Docusaurus 指南/API/OpenAPI 示例统一说明默认 168 小时并以 `expires_at` 为准。
- `/v1/files`、GPT Image 2、Grok 与生成结果的 24 小时策略未修改。

## TDD evidence

- RED：服务测试观察到默认回退仍为 24 小时；文档契约未找到 `168 小时`。
- GREEN：默认 TTL、生产初始化默认、72 小时环境覆盖、COS 清理时间和文档契约均通过。

## Verification

- `go test ./... -count=1`：通过。
- `go vet ./common ./constant ./service`：通过。
- `bun run i18n:check && bun run typecheck && bun run build`：通过。
- Docusaurus 两组契约测试：35/35 通过。
- Docusaurus 生产构建：通过。
- 两路独立审查的 Critical 均为 0；提出的文案、平台文档、初始化隔离和 COS 覆盖问题均已修复。

## External model limitation

CCG 要求的 antigravity 与 Claude wrapper 在本机不存在（`~/.claude/bin/codeagent-wrapper`），两次并行调用均退出 127；使用失败测试、全量验证和两路独立审查代理完成交叉验证。
