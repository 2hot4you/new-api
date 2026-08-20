# Requirements

- `/keys` 创建和编辑 API Key 时，分组按管理员配置顺序展示，不再依赖 Go map 的 A–Z 序列化顺序。
- `/group-pricing` 现有分组表增加 LobeHub icon 键与推荐指数字段。
- 推荐指数为整数 1–5；空值或 0 不展示推荐徽标。
- 分组表最左侧拖拽柄始终可用，拖动只修改展示顺序，并通过现有保存入口持久化。
- `/keys` 分组下拉和分组表格以精致的小尺寸展示图标；下拉同时显示推荐徽标。
- 保留现有 GroupRatio、TopupGroupRatio、UserUsableGroups、AutoGroups、计费与路由语义。
- 展示顺序不得改变 AutoGroups 的实际渠道尝试顺序。
- 没有元数据的旧分组保持可用，并稳定追加到已配置分组之后。
- 所有新增文案同步现有七种语言。

# Analysis

- 当前 A–Z 来自 Go map JSON 序列化，不是可靠 API 顺序。
- 采用独立 `group_ratio_setting.group_metadata` 有序配置，避免改变旧 map 值类型。
- antigravity 与 Claude 外部分析均已按 CCG 要求并行尝试，但本机缺少 `~/.claude/bin/codeagent-wrapper`，均以 exit 127 退出。
- 已由两个只读代码审计代理分别完成后端和前端路径分析。
