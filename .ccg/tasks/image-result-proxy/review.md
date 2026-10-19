# 图片结果代理验证记录

## 本地交付

实现提交：707a3b658。分支：codex/image-result-proxy。工作区：/Users/naf/Documents/Codex/Projects/molii.co/.worktrees/image-result-proxy。

- 生成响应将图像 URL 改写为本站一小时有效的加密能力链接；生成及编辑共享 JSON、SSE 和 JSON 转 SSE 路径均覆盖。
- 公开取图接口不要求 API Key，直接返回受限制的栅格图片字节，不存储、不跳转。
- 固定 DNS 解析结果，强制拒绝内网目标、验证 TLS 和重定向，限制大小与时间。全局宽松网络设置不能绕过保护。
- 开启代理的生成请求不透传上游响应头和错误详情。流式已交付完整图片后失败会保留正常结算，避免整次退款。
- 现有 Worker Proxy 设置不变；它不负责改写输出 URL。当前图片访问采用独立的安全直连取图。

## 审查

独立 Codex 审查发现两项 Warning：流式已交付后错误触发退款、JSON/SSE 报错泄露上游地址。均已修复并添加回归测试；复审无剩余 Critical/Warning。复审指出传输错误分支仅静态验证，随后已补充真实本地连接失败测试并通过。

项目要求的 codeagent-wrapper 和 antigravity 后端在当前环境不存在。直接尝试 Claude CLI：分析多分钟无输出后终止；审查 90 秒超时。未取得外部双模型审查，不能宣称已通过。该限制已向用户说明。

## 验证

- `go test ./...`：通过，42 个有测试的根模块包通过。
- `go vet ./relay ./relay/channel/openai ./service ./controller ./router`：通过。
- `go build ./...`：通过。
- `go test -race ./service ./relay/channel/openai ./controller ./router ./relay -run ImageResultProxy -count=1`：通过。
- 新增 HTTP/传输失败脱敏测试：通过。
- `git diff --check`：通过。
- 对外 Markdown：品牌/真实域名扫描、JSON 示例解析、Bash 语法检查通过。
- 初次全量测试因隔离工作区缺少忽略的 web/dist 构建文件而失败；复制主工作区现有前端产物后通过。未修改前端源码。

## 部署状态

**未部署到生产，未执行真实计费出图验证。** 用户确认目标为仓库 main 分支，部署配置位于服务器。本地没有目标 SSH 别名，尚需服务器连接信息。尚未合并 main 或推送。生产部署和回归完成前任务继续保持 in_progress，不归档、不将对外文档标为已生效。

公开文档：docs/image-api.md。部署说明：deploy/image-result-proxy.md。生产启用前按部署说明验证配置、同域图片下载和原有能力，再提供正式对外文档。

## 规范回馈

已新增 .ccg/spec/backend/index.md，记录公开媒体代理、DNS/TLS 和流式结算约束。
