# ByteDance Seedance 代理渠道审查记录

## 审查范围

- 新增原生 `ByteDance Seedance` 渠道及共享 Seedance 协议核心。
- 授权模型发现、临时素材、异步轮询、持久化结算、视频内容代理和后台配置界面。
- 代理商运维文档、Docusaurus 公共说明及 PostgreSQL/Redis 端到端回归。

## 分任务审查结果

- Task 1：通过。修复渠道编号变化导致的 MoliiGrok 回归断言。
- Task 2：通过。移除签名结果地址、拒绝未知状态和私有/畸形任务 ID，并修复统一路由测试。
- Task 3：通过。强制配置 Bearer Key、补齐自指 URL 与端口规范化、修复撤权统计。
- Task 4：通过。生成前执行本地用户素材校验，脱敏失败详情，强制上游过期与 168 小时上限；直连 StarAI 兼容行为保持不变。
- Task 5：通过。处理额度溢出、免费倍率来源证明和畸形用量的人工复核，重启恢复与幂等结算通过。
- Task 6：通过。鉴权 GET/HEAD/Range 内容代理、提交 Key 快照、416 与文件名脱敏通过。
- Task 7：通过。修复渠道类型切换时旧 Key 和 Ark Base URL 被错误继承的问题。
- Task 8：通过（两个非阻塞改进项待全分支审查裁定）。PostgreSQL/Redis 全链路、竞态测试、前端构建和 Docusaurus 构建通过。

## 外部双模型审查可用性

约定工具 `/Users/naf/.claude/bin/codeagent-wrapper` 不存在，检查返回：

```text
ls: /Users/naf/.claude/bin/codeagent-wrapper: No such file or directory
```

因此未执行、也未声称执行 antigravity 与 Claude 外部双模型审查。替代措施为逐任务独立实现/审查子代理、全分支高能力审查、针对性安全测试及完整受影响测试。

## 已验证的关键不变量

- 代理实例只保存/展示本地任务 ID 和允许根级管理员查看的 Molii 公共任务 ID，不存储或返回 StarAI 私有任务 ID、Key、原始响应或签名结果 URL。
- 素材按本地用户绑定校验后才访问上游，代理层之间原样透传 `asset://asset-...`。
- 成功任务只使用可信实际 Token 与本地提交时价格快照结算；事实缺失、畸形或不可表示时进入 `review_required`。
- 视频内容使用提交时选中的渠道 Key 鉴权代理；终端 Authorization 不会转发。
- 原有直连 StarAI 的 COS、计费、时间和历史素材行为保持独立。

## 已知非功能基线

- 仓库全量前端 lint 仍有既有错误；本功能改动文件的定向 lint、类型检查、测试和生产构建通过。
- Docusaurus 可选链接爬虫会报告现有应用根路径；文档测试、构建、敏感词和密钥检查通过。
- 外部双模型审查工具缺失，需在工具恢复后补做才可声称完成该审查。

## 待全分支审查裁定的非阻塞项

- Base URL 带 query/fragment 当前仍可通过连接校验。
- 编辑表单的“从上游获取模型”沿用已保存渠道值，不使用未保存 Base URL/Key。
- 端到端测试对原始 asset URI 使用子串断言，可加强为 JSON 精确等值。
- 运维指南中删除操作的描述应与“仅删除本地绑定/本地 COS”语义进一步拆分。
