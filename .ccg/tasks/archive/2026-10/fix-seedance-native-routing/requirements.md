用户要求修复 aigc 的 Seedance duration:-1 报错；不检查上游服务。保持渠道身份、时长边界、计费保护；不重试付费请求，不修改生产数据。用户提供路径 /v1/video/generations、模型 doubao-seedance-2-0-260128、渠道标签 Molii Volcengine Imagine API。

当前证据纠正：直接JS decoder可复现同文案，但此路径不注册openai_video绑定，不能认定为此次根因。独立分析确认必须先核对真实选中渠道和版本。

最终根因证据：只读生产日志匹配两个 request ID，实际入口分别是 /v1/videos 与 /api/video-studio/estimate；均在 doubao endpoint prepare_rejected 阶段失败。修复共享预检，不改上游、不改变渠道选择或负计费量校验。
