# 模型广场能力元数据

## 目标

- Molii 自有目录是模型广场的最终数据源。
- models.dev 仅作为初始化和更新参考，不在浏览器运行时请求。
- 只展示当前 Molii API 链路实际开放的能力。
- 文本模型卡片仅展示少量关键能力；完整能力放在详情页概览。
- Seedance 与 Grok 继续使用各自的图片、视频能力和价格结构。

## 当前范围

- `deepseek-v4-flash-202605`
- `deepseek-v4-pro-202606`
- `glm-5.2`
- `kimi-k3`
- `minimax-m3`
- `qwen3.5-flash`
- `qwen3.5-plus`

## 数据规则

- 每条目录数据记录参考来源和最近验证日期。
- 上下文、最大输出、发布日期等模型事实可以参考 models.dev。
- 输入输出模态按 Molii 当前 OpenAI 兼容 Chat Completions 链路收敛为实际可用范围。
- 不导入 models.dev 的价格，Molii 定价继续由现有计费配置提供。
- 不把未经验证的视觉、音频、视频输入能力标记为可用。
