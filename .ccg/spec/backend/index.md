# 后端约定

## 公开图片代理

- 对外图片链接使用带有效期的认证加密令牌；只签名或 Base64 编码原始地址不能隐藏来源。
- 公共代理取图必须固定到验证过的 DNS 地址，拒绝内网地址，并验证每次重定向；全局关闭 SSRF 检查、允许内网或跳过 TLS 验证的设置不能降低该公开入口的保护。
- 不将请求方的鉴权信息、上游响应头或上游错误正文传给图片访问者。只返回受支持的栅格图片类型，并限制读取时间及大小。
- 修改流式图片响应时，测试“已交付完整图片后发生错误”的结算路径。若返回顶层错误会触发整次退款，必须保留已交付内容的正常结算，同时向客户端发送脱敏的流错误。
- 改写图片链接时同时检查 JSON、真实 SSE 和 JSON 转 SSE 三条路径，以及原始模型名和映射模型名。
- 缓冲图片响应必须限制进程级并发，槽位覆盖整个响应写入；设置固定写超时，避免慢速客户端长期占用全部下载槽位。

## 同步生成网关超时

- 同步图像生成可能超过一分钟；核对网关等候响应头超时与应用处理时间，客户端等待参数不能延长网关超时。按图片路径设置独立读取等待，并保留更长下载路径的超时与日志规则；使用不计费的延迟响应测试验证。

## Shared ordinary and task pricing

- Task expressions evaluate usage facts in currency per request; ordinary token variables are not populated there. Reject unavailable token variables and fixed ordinary pricing from the original AST before reservation, even for unchanged stored quotes and unreachable branches. Legitimate task token pricing uses a declared `u("tokens")` fact and an explicit million-token conversion.
- Keep unit safety separate from schema compatibility: an unchanged sole-provider usage expression may retain the documented narrowed-schema exception, but cannot bypass unit safety. Dormant ratio-mode expressions and ordinary quotes shadowed by valid source/mapped task overrides must remain editable.
- Cross-model alias validation must use the complete candidate override map, including additions, removals and resets, before per-row validation. Never let batch order decide validity, retain removed old overrides, or consult a later live map during prospective catalog validation.
