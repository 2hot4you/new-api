# 后端实施报告

## TDD 过程

- Task 1 RED：图片 allowlist 尚不存在，图片响应仍调用 Redis/COS 并返回 502。
- Task 1 GREEN：三种 Grok 图片模型经集中 URL 校验后直接返回上游结果，不调用持久化。
- Task 2 RED：视频终态仍调用 COS persistence，伪造 host 可通过旧校验，标准 metadata 仍返回本地代理 URL，content 仍读取旧 COS。
- Task 2 GREEN：视频 URL 写入 private data，标准查询直返，content 受保护 307，计费和安全 polling 数据保持不变。
- 补充 RED/GREEN：非 Grok StoredResult 保持无需 channel lookup；Grok platform 与 channel type 双向不一致均 fail-closed。

## 实施内容

- 图片 allowlist：`imgen.x.ai`、`files-cdn.x.ai`。
- 视频 allowlist：`vidgen.x.ai`、`files-cdn.x.ai`。
- 仅 HTTPS、无 userinfo、仅无端口或显式 443、大小写不敏感、单个末尾点规范化、精确 hostname 比较。
- Grok 图片不再调用 Redis/COS persistence；保留多图、MIME、revised prompt、实际输出计费和 `grok_image_billing`。
- Grok 视频轮询不再调用 COS；合法结果只写入 `TaskPrivateData.ResultURL`，公开轮询数据不含 URL。
- 标准视频查询 `metadata.url` 直返已复验的 xAI URL；生成记录继续返回同源签名 content URL。
- content 接口验证归属、成功状态、platform 62、channel type 62 和 URL allowlist 后，以空 body 返回 307，并设置 `private, no-store`、`no-referrer`、`nosniff`。
- Grok 忽略历史 StoredResult；非 Grok 既有 StoredResult 行为保持不变。

## 验证结果

- `go test ./service ./relay/channel/moliigrok ./relay/channel/task/moliigrok ./relay ./controller -count=1`：通过。
- `go test ./... -count=1`：通过。
- `go vet ./...`：通过。
- `go build ./...`：通过。
- 不同已认证用户访问他人 Grok 视频任务的安全回归测试及 `-race`：通过；返回 404、无 `Location`、无 StoredResult/远端 fetch、无 URL/query 泄露。
- 所有负责 Go 文件 `gofmt -l`：无输出。
- `git diff --check`：通过。
- 最终验证使用被 Git 忽略的本地空 `web/dist/index.html` 满足 Go embed；未执行前端生产构建，未将该占位文件纳入提交。

## 自审

- 未发起真实外部请求。
- 未把完整 URL、path、query、Token 或密钥写入日志或客户端错误。
- 307 在创建远程 HTTP client/request 前返回，不执行 GET、HEAD 或 DNS 探测。
- 未修改 Seedance、其他渠道或全局对象存储实现。
- 独立最终复审结论为 SHIP，无 Critical、Important 或 Minor。
