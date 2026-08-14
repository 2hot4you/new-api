# Review

## 根因

- 3000 与 3001 返回的 favicon 声明和文件哈希一致。
- 浏览器会按 origin（含端口）分别保存站点状态与 favicon 缓存。
- 启动时会先应用各 origin 缓存的 Logo；当最新 `/api/status` 返回空 Logo 时，旧逻辑不会重置 favicon，因此两个端口可能继续显示不同的历史图标。

## 修复

- 新增系统 favicon 归一入口：空值或空白值恢复为 Molii 默认 favicon，自定义 Logo 继续保留。
- 缓存状态读取和网络状态刷新都调用归一入口，确保网络刷新后清除过期的端口级图标。
- 新增回归测试覆盖“旧图标 + 空系统 Logo”的场景。

## 审查结论

- 变更仅涉及 favicon 初始化与对应测试，不影响站点 Logo、主题或后端接口。
- 未发现 Critical 或 Warning。
- 按用户约束未调用 antigravity 或 Claude。

## 验证

- favicon 单元测试：通过。
- 前端类型检查：通过。
- 定向 lint 与格式检查：通过。
- 实际访问 3000 与 3001：两者最终均使用 `/molii-favicon-32.png?v=3`。
- 未执行生产构建。
