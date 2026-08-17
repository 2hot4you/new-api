# 模型元数据资料核验记录

核验日期：2026-08-17

## 结论

- ZenMux 提供 9 个与本地 ID 一对一匹配并直接采用的模型详情页：MiniMax M3、Qwen3.5 Flash、Qwen3.5 Plus、GLM-5.2、Kimi K2.5、Kimi K2.6、Kimi K2.7 Code、Kimi K3、Qwen3.8-Max。两个 DeepSeek 页面经过核验，但因本地日期后缀不一致而未采用。
- `deepseek-v4-flash-202605` 与 `deepseek-v4-pro-202606` 不能和 ZenMux 当前不带日期后缀、已指向更新版本的页面建立精确映射，因此保留本地版本化契约数据，只补全双语简介和 LobeHub 图标。
- ZenMux 没有两个 Seedance 2.0 日期版和四个 Grok Imagine 精确页面；models.arts 当前无法建立 HTTPS 连接。上述模型只采用当前仓库已测试适配器的精确能力，不套用同系列页面。
- ZenMux 精确详情页未披露任何目标模型的知识截止日期，因此清单统一使用空字符串，不保留来源不明的旧值。
- LobeHub 当前包明确导出 `Minimax`、`Qwen`、`DeepSeek`、`Doubao`、`Zhipu`、`Grok`、`Moonshot`，清单使用这些精确键。

## 数据处理原则

- 简介为来源事实的人工改写与中译，不复制平台促销、价格、供应商数量或排行信息。
- `supported_parameters` 只保留当前后端允许枚举且在精确模型提供方协议中明确出现的参数。
- 生成类模型的 `context_length` 与 `max_output_tokens` 为 `0`，避免把图片/视频输入限制误当 Token 窗口。
- 本地适配器来源采用 `repo://` 标识，便于和外部 URL 区分。

## 未采用的近似来源

- `https://zenmux.ai/deepseek/deepseek-v4-flash`：当前页面为更新的 0731 版本，不等于本地 `deepseek-v4-flash-202605`。
- `https://zenmux.ai/deepseek/deepseek-v4-pro`：当前页面为更新的 0813 版本，不等于本地 `deepseek-v4-pro-202606`。
- `https://zenmux.ai/bytedance/doubao-seedance-2.5`：属于 Seedance 2.5，不等于本地 Seedance 2.0 日期版。

## 启动持久性核验

- 首次写入后重启发现，旧的兼容回填会把 `qwen3.5-plus` 已清空的知识截止日期恢复为 `2025-04`，并对空的来源追踪字段补写旧种子值。
- 已通过回归测试收紧兼容回填：不再为管理员明确留空的可选来源追踪字段造值，并移除 `qwen3.5-plus` 未核实的知识截止日期种子。
- 重新执行事务并再次重启后，17 个目标模型与清单逐字段一致，非目标字段与备份逐字段一致。
