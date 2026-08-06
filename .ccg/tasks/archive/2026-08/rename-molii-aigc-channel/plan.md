# 实施计划

完整的 TDD 实施步骤、文件归属与验证命令见：

`docs/superpowers/plans/2026-08-07-volcengine-rebrand-grok-management.md`

并行文件所有权：

- 后端子智能体：Go 管理接口、余额、模型查询、TCP 可达性、环境变量及部署示例。
- 前端子智能体：渠道/计费/素材/生成记录文案、类型 62 动作与 i18n/tests、Docker 描述。
- 主代理：设计与任务文档、合并检查、全量测试、本地构建与浏览器验证。

实现不得写入真实系统访问令牌，不修改历史 docs/.ccg 记录，不改变渠道类型编号、数据库记录或 `molii-aigc` owner。
