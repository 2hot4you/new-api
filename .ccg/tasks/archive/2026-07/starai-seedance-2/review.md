# StarAI Seedance 2.0 审查结果

## 结论

未发现未解决的 Critical 或 Warning。实现保持 StarAI 为 task-only 渠道，不新增聊天 APIType，不访问真实收费接口，也未修改图片、视频、音乐创作页面。

## 审查中发现并已修复

- `ChannelTypeDummy` 因 Go 常量表达式继承而与 StarAI 同为 61：显式改为 62，并增加回归测试。
- 公共 `TaskSubmitReq.Duration` 无法区分未传与显式 0：在 StarAI 适配器内读取原始 JSON 字段存在性，保留 `duration: 0`，不改变其他渠道。
- 通用 prompt 校验会拒绝合法的媒体无文本模式：新增可空 prompt 的公共解析入口，由 StarAI 校验必须至少有文本或图片/视频，音频不可单独提交。
- metadata content 会覆盖顶层 images：改为合并，且 metadata model 无法覆盖映射后的上游模型。
- 非 2xx 上游原始响应可能进入 TaskError：增加可选错误清洗接口，StarAI 只返回脱敏消息。
- 安全 Task.Data 中的签名已脱敏，查询接口若直接复用会返回失效 URL：StarAI 查询统一返回公开 `/v1/videos/{public_task_id}/content` 代理地址。
- StarAI 失败日志不得序列化完整 Task：只记录 public task ID，避免 PrivateData 中签名 URL 或凭据进入日志。
- signed TOS 流程缺少端到端覆盖：增加本地 HTTP 代理测试，验证进入现有代理、没有转发 StarAI Bearer；unsigned TOS 502 不改变成功状态和 quota。

## 验证

- `go test ./relay/channel/task/starai/...`：通过。
- `go test ./relay/... ./controller/... ./service/...`：通过。
- `go test ./...`：通过。
- StarAI 前端测试：3/3 通过。
- 本次触及前端文件定向 oxlint：通过。
- `bun run build`：通过。
- `bun run lint`：未通过，失败均来自本任务未修改的既有前端文件；未扩大范围修复。
- `git diff --check`：通过。

## 审查限制

用户明确要求不调用 antigravity 或 Claude；本机也不存在 CCG 配置引用的 `~/.claude/bin/codeagent-wrapper`。因此未执行外部双模型审查，改用实现代理交叉检查、主代理逐项审计和完整测试闭环。
