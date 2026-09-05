# 分析结论

## 当前语义

- 每次请求只携带一个 `model_id`，不会并行调用 Anthropic 和 OpenAI。
- `auto_groups` 是一个全局有序数组，但选路时会先用当前 `model_id` 过滤渠道；不支持该模型的分组会被直接跳过。
- 因此 AWS Bedrock 与 Azure Foundry 分别只支持 Claude、GPT 时，二者在有效路由上互不竞争；当前 UI 的全局箭头链造成了误导。

## 推荐设计

- 将“分组”改为“模型路由”或“接入点”。
- 选择弹层继续按模型厂商分区，已选结果也改成按厂商分区的卡片，而不是单一全局排序列表。
- 每个厂商卡片内展示该厂商的接入点；仅当两个以上接入点支持同一模型集合时允许排序并说明为故障转移优先级。
- 摘要按厂商展示，例如 `Anthropic：AWS Bedrock`、`OpenAI：Azure Foundry`，不再使用跨厂商箭头。
- 第一阶段复用当前 `auto_groups` 与后端路由，不做数据库迁移。只有未来需要让同一分组在不同厂商下拥有互相冲突的优先级时，才引入 `provider/model -> ordered groups` 的新策略结构。

## 工具状态

- 按项目规则尝试调用 antigravity 与 Claude 并行分析，但本机缺少 `~/.claude/bin/codeagent-wrapper`，因此本次结论基于代码审阅完成。
