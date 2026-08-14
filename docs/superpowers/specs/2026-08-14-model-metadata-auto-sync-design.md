# 模型元数据自动同步设计

## 目标

让新增且已启用的模型 ID 自动进入数据库模型目录，并由同一份持久化资料同时驱动 `/models/metadata` 和 `/pricing`。管理员不需要为每个新 ID 修改前端代码，但 Molii 的价格仍由本地计费配置控制。

## 数据流

1. 渠道能力刷新时，现有本地对账先为新模型创建最小 `Model` 记录。
2. 后台同步器收到非阻塞通知，从 models.dev 获取公开目录。
3. 同步器只处理当前已启用模型，并按模型所属厂商选择可信的官方目录条目。
4. 外部资料转换为 Molii 已支持的字段后，在数据库事务中锁定模型行并只补空字段。
5. 更新完成后失效定价缓存；`/api/pricing` 下次读取同一条 Model/Vendor 记录。

## 权威顺序

1. 管理员已保存的非空字段。
2. Molii 内置的 Seedance、Grok 和精选模型资料。
3. models.dev 自动补充资料。
4. 无可信资料时保持最小模型记录，不编造内容。

`sync_official=0` 表示该模型完全退出自动补充。软删除记录不会恢复，禁用状态不会被修改。

## 外部字段映射

- `description` → 模型简介
- `limit.context` → 上下文窗口
- `limit.output` → 最大输出 Token
- `knowledge` → 知识截止时间
- `release_date` → 发布日期
- `modalities.input/output` → 输入/输出模态；`pdf` 规范化为 `file`
- `reasoning` → `reasoning`
- `tool_call` → `tools`
- `structured_output` → `structured_output`
- 图片输入同时补充 `vision`

外部 `cost`、提供商价格、缓存价格和币种全部忽略。

## 厂商选择

同一模型 ID 可能被多个转售提供商收录，不能按最低价或字典序随意选择。同步器根据 Molii 的厂商推断选择固定候选：DeepSeek、OpenAI、Anthropic、Google、xAI，以及阿里巴巴、MiniMax、Moonshot、智谱的中国区/官方目录。没有可信匹配时不自动补充。

## 调度与容错

- 实际进程启动后台 worker；普通单元测试调用渠道缓存不会发起真实网络请求。
- 渠道刷新只发送容量为一的通知，不等待网络。
- worker 启动后立即处理首次通知，并按配置周期兜底刷新。
- HTTP 使用独立客户端、请求超时、响应体上限和成功状态校验。
- 单实例内合并重复通知；多实例并发依靠“事务锁行 + 仅补空字段”保持幂等。
- 失败只记录安全日志，保留已有数据并等待下一周期。

## 配置

- `MODEL_METADATA_AUTO_SYNC_ENABLED=true`
- `MODEL_METADATA_SYNC_URL=https://models.dev/api.json`
- `MODEL_METADATA_SYNC_INTERVAL_HOURS=6`
- `MODEL_METADATA_SYNC_TIMEOUT_SECONDS=10`
- `MODEL_METADATA_SYNC_MAX_MB=32`

这些都是公开运行参数，不包含密钥。

## 非目标

- 不自动导入 models.dev 价格。
- 不把外部目录作为请求时依赖。
- 不修改前端模型卡和详情布局。
- 不恢复已删除的 Preview 模型。
