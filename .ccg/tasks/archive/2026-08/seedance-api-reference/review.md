# 文档审查

## 交付结果

- 新文档：`docs/Molii Seedance API 接口文档.md`。
- 删除范围过宽的 `docs/molii-user-api.md`。
- 品牌统一为 Molii，Base URL 统一为 `https://aigc.claudeye.com`。
- 只覆盖两个 Seedance 2.0 模型，不包含平台操作和其他模型接口。

## 契约来源

- 完整阅读 `../StarAI API 接口文档.md`（754 行）作为结构和媒体规格参考。
- 路由来自 `router/video-router.go`。
- 请求字段、默认值、组合限制、状态映射和模型限制来自 `relay/channel/task/starai/adaptor.go` 与 `constants.go`。
- 任务响应来自 `relay/relay_task.go`、`relaykit/dto/openai_video.go` 和 `model/task.go`。
- 临时素材来自 `controller/starai_asset.go` 与 `service/starai_asset.go`。
- 下载、签名 URL 和异常结果处理来自 `controller/video_proxy.go` 与 `service/video_playback_url.go`。

## 验证

- 文档 1350 行，含 53 个代码块。
- 25 个 JSON 示例全部可解析。
- 13 个 Bash 示例全部通过 `bash -n`。
- Python 完整示例通过语法编译。
- Node.js ES Module 完整示例通过 `node --check`。
- Markdown 围栏成对，`git diff --check` 通过。
- `go test ./relay/channel/task/starai ./controller ./router` 通过。
- 线上 HTTPS 状态接口返回 200；未授权模型请求返回 401。
- 按用户要求未调用 antigravity 或 Claude。
