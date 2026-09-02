# 评估复核

## 已完成的核对

- 官方 GitHub 发布说明：rc.24、rc.25、rc.26、rc.27、rc.28、rc.29、rc.30。
- 本地 upstream 标签与提交区间、patch equivalence、文件重叠统计。
- `git merge-tree --write-tree HEAD v1.0.0-rc.30` 只读模拟合并。
- GORM 依赖、数据库迁移、quota schema、任务插件和 Molii 自定义 task adaptor 对比。

## 结论复核

- Critical：不得把 rc.30 直接合入或部署到生产。
- Critical：rc.26 前必须完成 64 位额度列迁移和克隆库演练。
- Critical：rc.27 不得删除 Molii StarAI/Molii Grok 原生链路，除非已有等价兼容实现。
- Critical：rc.28/rc.29 不得作为运行版本，必须包含 rc.30 的迁移修复。
- Warning：完整 rc.30 仍被上游标记为实验性插件系统、不建议生产使用。
- Info：生产线可先选择性回移已审计的稳定性和计费修复。

## 双模型审查状态

按项目规则并行调用 antigravity 与 Claude analyzer，但本机不存在
`~/.claude/bin/codeagent-wrapper`，两个调用均以状态 127 结束；因此没有伪造
外部模型结论。本报告使用官方发布说明、Git 提交与本地只读模拟结果复核。
