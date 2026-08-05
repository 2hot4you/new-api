# Requirements

- 将 `molii-aigc-video-pricing` 当前上下排列的 Seedance 与 Grok Imagine 表单改为顶部标签切换。
- 交互和视觉复用 `model-pricing` 已有的 `Tabs`、标题栏 Portal 与内容区布局。
- 切换时只展示当前标签对应的输入表单。
- 保持所有价格字段、保存行为、后端配置键和路由不变。
- 增加覆盖默认标签和标签切换的前端测试。
- 不调用 antigravity 或 Claude，遵守用户对本项目的既有约束。
- 顶部标签固定为 `Seedance 2.0`、`Grok Imagine`，默认打开 `Seedance 2.0`。
- 任一时刻顶部只显示当前标签对应的一个“保存更改”按钮。
