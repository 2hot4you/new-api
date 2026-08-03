# 验收

- 交付 `docs/examples/seedance-complete-curl.sh`。
- 创建图片、视频、音频三类临时素材并逐个轮询状态。
- 使用三个 `asset://` 引用创建标准版 Seedance 多模态任务。
- 请求包含音频生成、分辨率、比例、时长、水印和联网搜索字段。
- 创建 POST 明确只发送一次，不自动重试。
- 轮询 `/v1/videos/{task_id}`，并展示通用查询视图。
- 演示 API Key 下载和短期签名 URL 下载。
- 支持任务完成后的临时素材清理。
- 脚本不包含真实 API Key 或素材 URL。
- `bash -n` 和 `git diff --check` 通过。
