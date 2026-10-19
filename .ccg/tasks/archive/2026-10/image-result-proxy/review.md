# 图片结果代理验证记录

## 本地交付

实现提交：3d691c409（已对齐生产 main 提交 ffe1296f5）。分支：codex/image-result-proxy。工作区：/Users/naf/Documents/Codex/Projects/molii.co/.worktrees/image-result-proxy。

- 生成响应将图像 URL 改写为本站一小时有效的加密能力链接；生成及编辑共享 JSON、SSE 和 JSON 转 SSE 路径均覆盖。
- 公开取图接口不要求 API Key，直接返回受限制的栅格图片字节，不存储、不跳转。
- 固定 DNS 解析结果，强制拒绝内网目标、验证 TLS 和重定向，限制大小与时间。全局宽松网络设置不能绕过保护。
- 开启代理的生成请求不透传上游响应头和错误详情。流式已交付完整图片后失败会保留正常结算，避免整次退款。
- 现有 Worker Proxy 设置不变；它不负责改写输出 URL。当前图片访问采用独立的安全直连取图。

## 审查

独立 Codex 审查发现两项 Warning：流式已交付后错误触发退款、JSON/SSE 报错泄露上游地址。均已修复并添加回归测试；复审无剩余 Critical/Warning。复审指出传输错误分支仅静态验证，随后已补充真实本地连接失败测试并通过。

项目要求的 codeagent-wrapper 和 antigravity 后端在当前环境不存在。直接尝试 Claude CLI：分析多分钟无输出后终止；审查 90 秒超时。未取得外部双模型审查，不能宣称已通过。该限制已向用户说明。

## 验证

- `go test ./...`：通过，54 个有测试的根模块包通过（生产基线移植后）。
- `go vet ./relay ./relay/channel/openai ./service ./controller ./router`：通过。
- `go build ./...`：通过。
- `go test -race ./service ./relay/channel/openai ./controller ./router ./relay -run ImageResultProxy -count=1`：通过。
- 新增 HTTP/传输失败脱敏测试：通过。
- `git diff --check`：通过。
- 对外 Markdown：品牌/真实域名扫描、JSON 示例解析、Bash 语法检查通过。
- 旧基线初次测试使用主工作区前端产物。移植到生产基线后使用生产实际品牌参数重新执行 bun install --frozen-lockfile 和 bun run build，成功；未修改前端源码。

## 部署状态

**已完成单站生产发布及一次真实计费出图验证。** 已获得连接方式并核实目标容器。远端最新 main e05a25706 有无关的 catalog PostgreSQL 测试失败；以当前生产 main 提交 ffe1296f5 为基础，只移植代理变更。普通 main 推送会发布四个站点，因此使用独立的、保留原健康检查与回滚的离线发布入口，仅更新目标站点。原镜像、运行配置与部署配置已备份。

公开文档：docs/image-api.md。部署说明：deploy/image-result-proxy.md。生产启用前按部署说明验证配置、同域图片下载和原有能力，再提供正式对外文档。

## 规范回馈

已新增 .ccg/spec/backend/index.md，记录公开媒体代理、DNS/TLS 和流式结算约束。

## 生产基线复审与安全约束

- 保留对象式 data、UpdateImageCount、预览日志、输入缓存明细 Clone 与输出 token 明细；新增对应回归。
- 应用访问日志隐藏图片令牌；服务器新增专用图片路径关闭访问日志和落盘缓冲。Nginx错误日志/WAF日志仍属运维保密范围，未声称所有层均无日志。
- 768 MiB容器采用进程级最多两条下载，从取图到响应写完均占用槽位，超限503+Retry-After。
- 参考 OWASP ASVS 5.0.0 https://owasp.org/projects/asvs 、Authentication Cheat Sheet https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html 、Session Management Cheat Sheet https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html 。本次适用的是随机、认证加密、用途隔离、有效期、服务端验证、TLS、通用错误和令牌日志保护；图片能力链接在有效期内允许重复下载，不是一次性登录令牌。未进行或声称完整ASVS认证。

最终独立复审：并发两槽位和固定60秒写超时均已实现；测试验证成功/失败写入后清除写超时及释放槽位，无剩余Critical/Warning代码问题。一次全套测试与测试驱动的red阶段重叠，捕获了尚未实现写超时的失败；实现后重新执行最终全套测试。

## 最终上线验收

- 最终源码提交863dd0788；Linux amd64二进制SHA256 e6721beb658fe7d565bfc1d06801378a526d88b9bbee33895bb23ad325c4142b。
- 生产镜像ID sha256:01dc56e4e5a1c2dadb83a4fcc2dbea6874f1181f2f7331e7dcbcd27886082530；使用原生产镜像为基础，仅替换应用二进制。完整源码包已保存在服务器release目录。
- 只重建model-claudeye-production；其他应用、PostgreSQL、Redis容器ID保持不变。内部/公网健康检查通过，公开版本production-model-claudeye-image-proxy-20261019。
- 一次curl生成：gpt-image-2.5，1024x1024，medium，n=1，response_format=url，stream=false；HTTP200，26.1秒。返回同域/v1/images/proxy/链接，无Base64；usage输入26、输出1056、总计1082。未自动重试。
- 不带鉴权读取图片HTTP200，image/png，2073626字节，SHA256 25660e830f73281bbda971425e780dd8e18ad7fb3e3845a1c4bdf6f2aaa97f75；no-store/no-referrer，无Location/Set-Cookie，无重定向。篡改令牌404，错误脱敏。
- 最终go test ./...通过54包；并发/写超时新增测试、race、vet通过。对外Markdown品牌/域名扫描、JSON与Bash语法校验通过。
- 未合并main：当前远程main包含尚未验证的无关升级，后续常规发布前须合入本次功能，避免覆盖已上线功能。
- 回滚备份：服务器/opt/claudeye/production/backups/image-proxy-20261019，保留原镜像和运行配置；离线发布脚本保留原deploy.sh，未覆盖它。
