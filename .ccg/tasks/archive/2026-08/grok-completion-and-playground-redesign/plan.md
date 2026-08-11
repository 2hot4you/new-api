# Implementation Plan

1. 加固 Grok 视频媒体探测、冻结计费快照并补生成别名。
2. 抽取 COS 通用存储，持久化图片和异步视频结果，建立 24 小时清理队列。
3. 增加视频延长与 1–7 张参考图模式及对应计费。
4. 增加用户级 Files API、所有权校验、媒体元数据、COS 内容代理与 Grok `file_id` 解析。
5. 将 Playground 改为 Base 风格并保持高级参数显式启用。
6. 同步公开 OpenAPI、默认 MDX API 页面、模型广场示例与回归测试。
7. 运行 Go、Web、Docs、迁移、race、vet、secret 和禁词检查，最后重启本地服务。

详细分步计划见 `docs/superpowers/plans/2026-08-11-grok-imagine-completion-cos.md`。
