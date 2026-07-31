# StarAI Seedance 2.0 实施计划

## 分析结论

- StarAI 是纯异步任务渠道，不新增普通聊天 APIType，也不映射到 OpenAI Chat Adaptor。
- 复用现有任务预扣、轮询 CAS、成功结算、失败/超时退款链路；只从 nested usage 提供 token，缺失 usage 时保留管理员配置的 ModelRatio 预扣结果。
- StarAI POST 必须至多提交一次；通过可选任务提交重试策略阻止控制器对 429/5xx 的通用重试，同时保留渠道错误处理。
- 轮询必须先解析原始签名 URL供受控下载，再将替换真实上游任务 ID、脱敏签名/credential/token/key 的安全副本保存到 Task.Data 和日志。
- 未签名的精确 Ark 私有 TOS 主机只在内容下载时返回 502，不改变已成功任务或触发退款。
- 管理端最小注册包括静态枚举、顺序、Doubao 图标、默认 Base URL、两个模型、聊天测试黑名单，以及 `/api/models` 的 task-only 模型映射供“填充相关模型”使用。
- CCG 外部双模型包装器在本机缺失，分析阶段以主代理源码审计和三个只读子代理交叉检查替代。

## Layer 1：并行实现

### A. 异步适配器与渠道注册

- `constant/channel.go`
- `common/endpoint_type.go`
- `relay/channel/adapter.go`
- `relay/relay_adaptor.go`
- `relay/channel/task/starai/*`
- `controller/relay.go`
- `controller/channel-test.go`
- `controller/model.go`
- 对应后端测试

实现请求转换、指针标量、metadata duration 二次边界校验、模型映射、四级任务 ID 解析、状态/usage/错误解析、PathEscape、禁止提交重试和 task-only 模型注册。

### B. 视频结果安全

- `service/task_polling.go`
- 新增 `service` 内结果安全 helper 与测试
- `controller/video_proxy.go`
- 对应 controller 测试

实现 Task.Data/日志脱敏、真实上游 ID 隔离、未签名私有 TOS 识别与 502、签名 URL 继续 SSRF/代理路径，禁止 Bearer/TOS 自签名。

### C. 余额与管理端最小注册

- `controller/channel-billing.go`
- 对应余额测试
- `web/src/features/channels/constants.ts`
- `web/src/features/channels/lib/channel-utils.ts`
- `web/src/features/channels/lib/channel-type-config.ts`
- `web/src/features/channels/api.ts`
- `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- `web/src/features/channels/lib/__tests__/starai-channel.test.ts`
- 必要 i18n 静态键与 locale

实现余额尾斜杠/Bearer/代理/有限非负校验/CNY 转 USD，以及 StarAI 下拉、顺序、图标、配置和精确“填充相关模型”。

## Layer 2：集成与修复

1. 合并并行交付，运行 gofmt 和定向测试。
2. 补齐跨模块测试：常量索引、禁止重试、公开 ID、duration 400、日志/错误不泄密。
3. 运行 `go test ./relay/channel/task/starai/...`。
4. 运行 `go test ./relay/... ./controller/... ./service/...` 与 `go test ./...`。
5. 前端运行 StarAI 测试、`bun run typecheck`、`bun run lint`、`bun run build`。
6. 运行 `git diff --check`，确认没有无关创作页改动。
7. 双模型审查若包装器仍不可用则记录限制，使用本地审查代理和主代理复核；Critical 修复后重测。

## 验收边界

- 不访问真实 StarAI 收费创建接口；所有协议验证使用 httptest。
- 不修改已有渠道编号，不创建数据库迁移。
- 不暴露 Key、Authorization、真实上游任务 ID或签名参数。
- 不修改图片、视频、音乐创作页。
- 不提交、不推送、不创建 PR；仅按 CCG 规则在完成后归档任务记录。
