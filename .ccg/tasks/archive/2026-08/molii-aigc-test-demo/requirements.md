# Molii AIGC 测试 Demo 需求

## 范围

- 独立于 New API 内置前端，不修改或嵌入 `web/`。
- 支持保存多个 New API 环境：名称、Base URL、API Key，并自由切换。
- 环境和请求历史持久化到 SQLite；浏览器端不得使用 localStorage/sessionStorage/IndexedDB 保存配置或历史。
- API Key 必须加密后落库，列表与日志不得返回或记录明文 Key。
- 覆盖 Molii Volcengine Imagine API 的两个 Seedance 模型。
- 覆盖 Molii Grok Imagine API 的图片与视频模型，包括 1.5 Preview。
- 所有受支持参数使用文本框、数字框、开关、下拉框或动态数组完整呈现。
- 实时生成请求 JSON 和 curl 预览。
- 保存完整请求日志、响应日志、HTTP 状态、耗时与错误。
- 异步任务自动轮询，可查看进度、最终媒体和失败原因。
- 展示提交时计费预估、完成后实际计费以及差额。

## 非目标

- 不修改 New API 的认证、渠道或计费实现。
- 不把 Demo 打进 New API 主程序或现有 Docker Compose。
- 不保存用户上传的媒体正文，只保存必要的 URL、file_id 与脱敏日志。
