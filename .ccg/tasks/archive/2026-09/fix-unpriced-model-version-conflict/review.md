# 审查结论

## 根因

`GetModelPricingSnapshot(nil)` 原先只从价格配置和内置表达式中收集模型名，遗漏仅存在精确模型元数据、尚未配置价格的模型。前端对遗漏模型使用 `empty_version`（按 USD 计算），而服务端对 CNY 元数据模型按 CNY 计算版本，首次保存因此被误判为并发冲突。

## 修复

- 全量快照额外读取精确匹配的模型元数据名称，并与已有价格模型去重、排序。
- 前缀等非精确规则不进入可编辑定价快照。
- 版本计算和服务端乐观锁比较逻辑保持不变，真实并发冲突仍会被拒绝。

## 验证

- TDD RED：新增测试在修复前因全量快照不存在 `glm-5.3` 而失败。
- TDD GREEN：`go test ./model -run 'TestModelPricingSnapshot(AllIncludesUnconfiguredExactMetadata|AndSaveIncludeBillingCurrency)' -count=1` 通过。
- 相关包：`go test ./model ./controller` 通过。
- 全仓后端：`go test ./...` 通过。
- 静态检查：`go vet ./...` 通过。
- 补丁检查：`git diff --check` 通过。

## 风险复核

- 数据库表不存在的遗留测试/环境仍由表存在性判断保护。
- 查询只读取 `name_rule = exact` 的模型名；不改变价格数据、货币或扣费逻辑。
- 未调用 antigravity 或 Claude 外部执行器，遵守用户明确限制；本次由主代理完成代码审查。
