# 审查结果

## Critical

- 无。

## Warning

- 无。

## Info

- 创建态移除了 `default` / 首分组兜底，同时保留管理员显式配置的全局自动分组默认值。
- 模型列表改用既有 `/api/user/models?group=<group>` 接口；多分组结果按选择顺序合并并去重，不依赖 Provider 元数据推断。
- 仅在用户主动改变分组且新分组查询成功后清理失效模型；编辑态首次加载不会静默修改旧限制。
- 新增必填标签与中、英、法、日、俄、越、繁中翻译。
- 外部 antigravity 与 Claude 审查均因本机缺少 `~/.claude/bin/codeagent-wrapper` 无法启动；已执行本地逐项差异审查。

## 验证

- 相关 Vitest：43 项通过。
- TypeScript 类型检查通过。
- 本次文件定向 oxlint 通过。
- 本次文件格式检查通过。
- 国际化键完整性检查通过。
- Rsbuild 生产构建通过。
- `git diff --check` 通过。
