# 需求

- `/pricing/gpt-image-2` 的“支持的参数”必须展示该模型真实支持的完整参数，而不是通用旧版图片参数集合。
- 参数包括 `model`、`prompt`、`n`、`size`、`quality`、`background`、`output_format`、`output_compression`、`moderation`、`user`、`stream`、`partial_images`、`images` / `image[]` 与 `mask`。
- 展示必填项、默认值、允许范围及条件约束。
- 其他图片模型继续使用现有参数配置，不受此次修复影响。
