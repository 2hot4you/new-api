# Grok 图片 WebUI 预览修复

## 已知证据

- Development `development-0030e7455c57` 的 `POST /v1/images/generations` 返回 HTTP 200。
- 响应包含可信 `https://imgen.x.ai/...` 临时图片 URL。
- WebUI 绘图记录目前无法预览该图片。

## 目标

- 确认图片 URL 在响应、计费日志、日志 DTO 与 WebUI 之间丢失的真实位置。
- 让当前用户在绘图记录中预览成功的 Grok 图片结果。
- 明确提示 xAI 链接为临时结果，建议及时下载保存。

## 安全边界

- 不向普通调用日志、服务端文本日志或错误信息写入完整临时 URL。
- 预览结果只能由任务所属用户访问；管理员行为保持现有权限语义。
- 不把 Grok 渠道密钥、Authorization 或 URL query 写入诊断日志。
- 不恢复 COS 转存，不发起服务端图片抓取，不改计费或客户端生成响应。
- 不发起真实付费请求。
