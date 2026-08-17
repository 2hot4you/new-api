# 本地模型元数据资料补全设计

## 目标

对本地 `/models/metadata` 当前配置的全部 17 个模型做一次性资料整理，以公开模型目录的精确模型页面为证据，覆盖现有错误、过期或缺失的模型广场元数据。此次工作只更新数据库内容，不引入新的同步服务、管理按钮或运行时外部依赖。

## 范围

目标模型为：

- `minimax-m3`
- `qwen3.5-flash`
- `qwen3.5-plus`
- `deepseek-v4-flash-202605`
- `deepseek-v4-pro-202606`
- `doubao-seedance-2-0-260128`
- `doubao-seedance-2-0-fast-260128`
- `glm-5.2`
- `grok-imagine-image`
- `grok-imagine-image-quality`
- `grok-imagine-video`
- `grok-imagine-video-1.5`
- `kimi-k3`
- `kimi-k2.5`
- `kimi-k2.6`
- `kimi-k2.7-code`
- `qwen3.8-max`

仅更新：`description`、`description_en`、`icon`、`release_date`、`knowledge_cutoff`、`input_modalities`、`output_modalities`、`capabilities`、`supported_parameters`、`context_length`、`max_output_tokens`。

明确不更新：模型 ID、厂商关联、价格、渠道、分组、端点、状态、发布开关、标签、生成模型分辨率和计费字段。

## 资料来源与证据规则

1. ZenMux 的精确模型详情页优先。
2. models.arts 的精确模型详情页作为补充。
3. 页面必须能与本地模型 ID 建立明确的一对一映射；命名空间前缀可以忽略，例如 `moonshotai/kimi-k3` 对应 `kimi-k3`。
4. 不使用同系列、后继版本、预览版本或 HighSpeed 版本替代精确模型。
5. 来源未披露发布日期、知识截止日期、最大输出 Token 等字段时，该字段保持空值或零值。
6. 每个模型在写入清单中记录来源 URL、抓取日期和未确认字段。

## 内容与枚举映射

英文简介忠实保留来源语义，去除平台价格、供应商数量和营销活动等非模型能力信息。中文简介由英文内容人工翻译，保持技术事实一致，不引入来源没有披露的判断。

LobeHub Logo 名从当前项目可用的图标键中选择，并按模型厂商统一：MiniMax、Qwen、DeepSeek、Doubao、Zhipu、Grok/XAI、MoonshotAI。写入前通过项目图标解析逻辑验证键名。

模态、能力和参数只写入后端允许的枚举：

- 模态：`text`、`image`、`audio`、`video`、`file`
- 能力：`function_calling`、`streaming`、`vision`、`json_mode`、`structured_output`、`reasoning`、`tools`、`system_prompt`、`web_search`、`code_interpreter`、`caching`、`embeddings`、`image_generation`、`image_editing`、`video_generation`、`video_editing`、`audio_generation`
- 参数：`stream`、`temperature`、`top_p`、`max_tokens`、`tools`、`tool_choice`、`reasoning_effort`、`response_format`

来源只描述功能但没有明确参数名时，不反推参数支持。

## 数据流与写入方式

1. 导出当前 17 行完整元数据作为带时间戳的本地备份。
2. 生成结构化 JSON 清单，每个模型包含目标字段、来源和核验备注。
3. 对清单执行离线校验：模型集合完全一致、无重复 ID、枚举合法、Token 数非负、最大输出不超过上下文。
4. 在单个 PostgreSQL 事务中按模型 ID 更新目标字段；任意一行缺失或模型集合变化时整体回滚。
5. 回读数据库并与清单逐字段比较。
6. 重启本地后端以刷新定价缓存，检查 `/models/metadata` 和 `/pricing`。

备份文件只保存在任务目录，不包含密钥或用户凭据。若页面结果异常，可使用备份在单个事务中恢复原字段。

## 错误处理

- 精确来源不存在：保留清单中的空字段并记录原因，不用近似模型补齐。
- 来源之间冲突：采用 ZenMux 精确详情页并记录冲突；若 ZenMux 没有精确页，采用 models.arts。
- 当前数据库模型集合发生变化：停止写入并重新盘点。
- 任意 SQL 更新数量不等于 1：回滚全部更新。
- 写入后发布完整性下降：报告具体缺失字段，不自动更改发布状态。

## 验收标准

- 17 个模型全部出现在资料清单和回读结果中。
- 所有写入字段都有精确来源或明确的“未披露”记录。
- 中文和英文简介均可读且语义一致。
- LobeHub Logo 键在当前前端可解析。
- 所有数组符合后端允许枚举。
- 价格、渠道、分组、状态和发布开关写入前后完全一致。
- 本地 `3000` 服务健康，模型元数据页面和模型广场可正常加载。
