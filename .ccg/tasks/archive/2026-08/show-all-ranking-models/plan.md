# 排行榜完整模型展示实施计划

**目标：** 热门模型历史和汇总 Tooltip 完整展示每个模型 ID，并为每行使用 metadata 图标。

**架构：** 后端取消模型历史的 top-10 聚合，并在历史模型元数据中附带 `model_icon`。前端使用历史元数据建立图标映射，保留所有 Tooltip 行，通过 VChart 的内容高度限制提供内部滚动。

**技术栈：** Go、React 19、TypeScript、VChart、Bun test。

**需求：** `.ccg/tasks/show-all-ranking-models/requirements.md`

## 全局约束

- 供应商市场份额的 `Others` 聚合保持不变。
- 不修改计费、模型 metadata、GPT Image 2 或部署配置。
- 先写失败测试，再实现最小变更。

### 任务 1：保护后端完整模型历史契约

**文件：**
- 修改：`service/rankings_test.go`
- 修改：`service/rankings.go`

- [ ] 添加超过 10 个模型的回归测试，断言每个模型和每个桶点均独立存在、没有 `Others`，并透传 `model_icon`。
- [ ] 运行定向 Go 测试并确认旧实现失败。
- [ ] 移除模型历史 top-10 聚合，生成完整模型元数据与点。
- [ ] 运行 `go test ./service -count=1`。

### 任务 2：保护前端完整 Tooltip 契约

**文件：**
- 修改：`web/src/features/rankings/components/__tests__/ranking-icons.test.tsx`
- 修改：`web/src/features/rankings/components/models-section.tsx`
- 修改：`web/src/features/rankings/types.ts`

- [ ] 扩展 VChart 测试替身，断言 12 个模型生成“总计 + 12 行”，没有聚合项，且历史模型图标可用于 Tooltip。
- [ ] 运行定向 Bun 测试并确认旧实现失败。
- [ ] 删除 Tooltip 行数上限与溢出合并，按当前桶 Token 降序返回全部行。
- [ ] 设置 Tooltip 最大内容高度，允许内部纵向滚动。
- [ ] 使用 `models_history.models[].model_icon` 构建完整图标映射。
- [ ] 运行排行榜测试、类型检查、scoped lint、i18n 和生产构建。

### 任务 3：审查、归档与发布

- [ ] 检查 diff、敏感信息和范围。
- [ ] 尝试 antigravity 与 Claude 双模型审查并记录结果。
- [ ] 修复 Critical 后重新验证。
- [ ] 提交实现，将 CCG 任务归档并提交归档。
- [ ] fetch 检查 `origin/develop` 未前进后 push。
- [ ] 等待 Development CI/CD，验证 Actions、状态接口和版本响应头。
