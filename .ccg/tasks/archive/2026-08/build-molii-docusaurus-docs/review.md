# 默认 MDX API 参考审阅记录

日期：2026-08-10

## 范围

- 公开 API 页面改为 Docusaurus classic 默认 Docs/MDX 渲染。
- OpenAPI 继续作为用户侧接口契约和 lint 输入，不再作为公开页面主题。
- API 导航按 Models、Images、Videos、Seedance、Assets、Errors 分组。
- 修正 `/v1/video/generations` 的 Seedance 多模态请求契约漂移。

## 结论

- Critical：0
- Important：0
- Minor：0

## 验证证据

- `bun test`：92 passed，0 failed。
- `web: bun run typecheck && bun test`：269 passed，0 failed。
- `go test ./... -count=1` 与 `go vet ./...`：通过。
- `bun run api:lint`：OpenAPI 3.1 文档有效。
- `bun run check:forbidden`：公开内容禁词检查通过。
- `bun run check:secrets`：文档密钥检查通过。
- 本地浏览器：`/api-reference` 使用默认 Docs 布局，sidebar=1、toc=1、OpenAPI Explorer=0；正文与标题使用 New API 同款 Lora Serif，代码块保持等宽字体。
- 本地服务：文档 `127.0.0.1:3100` 与后端 `127.0.0.1:3000` 均健康。
- 未运行静态构建、Docker 或部署。

## 审查方式

依据用户明确要求，未调用 antigravity 或 Claude，也未新增子代理；由当前会话执行 TDD、差异审阅和真实浏览器验证。
