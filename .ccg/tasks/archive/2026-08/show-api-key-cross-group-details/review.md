# Review

## Scope

审查 `/keys` 表格跨分组详情 Popover、查询状态、响应式溢出、国际化和回归测试。

## Findings and fixes

- Critical：无。
- Important：首次审查发现查询加载/失败会被误判为分组不可用或空配置。现已保留 `loading` / `error` / `ready` 状态并分别展示安全状态。
- Important：首次审查发现短屏设备缺少可用高度限制。现已使用 Base UI `--available-height` 限制 Popover，并让列表成为 `min-h-0` 的纵向滚动区。
- Minor：无剩余问题。

## Verdict

复审通过。审查代理未修改文件。

仓库规范指定的 antigravity 与 Claude 外部 wrapper 在当前机器不存在；两次并行调用均以 exit 127 结束，已改用隔离的只读 `ccg-review` 审查并完成修复复审。

## Verification

- Keys 组件测试：22/22 通过。
- TypeScript：通过。
- 变更文件 oxlint / oxfmt：通过。
- i18n 完整性：通过。
- Web 生产构建：通过。
- `git diff --check`：通过。
