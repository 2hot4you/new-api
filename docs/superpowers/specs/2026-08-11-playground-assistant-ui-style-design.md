# Playground assistant-ui Base 风格改造设计

**状态：** 已确认  
**日期：** 2026-08-11  
**范围：** `/playground` 用户页面的布局、参数交互和兼容迁移

## 1. 目标与边界

把现有 `/playground` 改造成 assistant-ui Base demo 的默认聊天布局，同时保留 New API 当前的模型、分组、流式/非流式、编辑、重试、停止、消息记录和请求协议。

采用“视觉复刻、运行时不迁移”方案：继续使用项目已有 React、Base UI、AI Elements、StickToBottom、SSE 和请求 hooks，不引入 `@assistant-ui/react`，不替换现有后端 `/pg/chat/completions`。

本轮不新增 `/app/playground` 路由，不新增 Share、真实附件、联网搜索、mentions 或 slash commands。现有没有后端实现的占位按钮不在新主界面中伪装成可用能力。

## 2. 页面结构

### 顶部栏

- 左侧显示页面标题和“新建对话”按钮；新建对话复用现有清空会话逻辑，有消息时保留确认。
- 不显示无实现的 Share。
- 页面使用现有 New API 前端同款字体、颜色变量、圆角、暗色模式和响应式断点。

### 对话区

- 桌面端内容最大宽度约 44rem，居中显示；移动端使用可用宽度。
- 消息区独立滚动，底部编辑器保持可见；继续保留滚到底部按钮。
- 消息组件、源码查看、复制、编辑、重试、删除、错误状态和最近 24 条渲染策略不变。

### 空状态

- 居中显示简洁问候与说明。
- 现有 starter prompts 改为 assistant-ui Base 风格建议卡片/胶囊按钮，点击后仍调用现有 `onSelectPrompt`。

### 输入编辑器

- 输入框和底部工具栏合并为一个 Base 风格 composer。
- 模型/分组继续复用共享 `ModelGroupSelector`，不创建第二套模型状态。
- 工具栏包含模型切换、参数设置、发送/停止；生成中禁用模型和参数变更。
- 移动端继续使用现有 Drawer/Sheet，桌面端使用 Popover。

## 3. 参数交互

保留六个高级参数：

```text
temperature
top_p
max_tokens
frequency_penalty
presence_penalty
seed
```

每项均以“开关 + 数值控件”展示：

- 默认全部关闭。
- 关闭时数值控件不可编辑，请求体不包含该字段。
- 开启后才允许输入或滑动，并继续沿用现有范围、步长和规范化逻辑。
- `0` 和合法负数不能因真假值判断而丢失。
- `stream` 作为普通设置开关，默认保持开启；它不属于高级参数启用映射。

### 一次性兼容迁移

为参数启用状态增加独立 schema 版本。旧用户首次打开新版 Playground 时：

- 仅把上述六个高级参数的启用状态重置为关闭。
- 保留模型、分组、六项参数已有数值、stream 设置、消息记录和其他用户数据。
- 写入新版本标记后不再重复重置；用户后续手动开启的状态正常持久化。

## 4. 数据流

页面继续使用现有状态和请求链：

```text
Playground UI
  -> usePlaygroundState
  -> payload-builder（只加入已启用参数）
  -> useChatHandler
  -> stream: useStreamRequest / non-stream: api.ts
  -> POST /pg/chat/completions
```

视觉组件不得直接发请求，也不得复制模型、消息或流式状态。模型选择、发送、停止和重试仍由现有 hooks 负责。

## 5. 错误与可访问性

- 继续使用现有错误归一和消息错误操作，不新增另一套 toast 状态。
- 模型未选择、请求进行中或输入为空时保持原有禁用规则。
- 所有开关、弹层、建议按钮和发送/停止按钮提供可访问名称和键盘操作。
- 桌面和移动端不得出现横向溢出；参数弹层不得遮挡输入区。
- 暗色模式、中文、繁体中文和已有语言继续使用现有 i18n；只有新增文案才增加翻译键。

## 6. 测试与验收

按 RED → GREEN → REFACTOR 实施，至少覆盖：

- 清洁状态下六项高级参数全部关闭。
- 旧存储迁移只重置启用状态，保留模型、数值、stream 和消息。
- 关闭参数不会进入请求体；单独开启后只发送对应字段。
- `0`、负数和 `seed=null` 的请求构建行为正确。
- stream 开关仍控制流式/非流式链路。
- 模型/分组选择器、发送、停止和生成中禁用状态不回归。
- starter prompt 点击、清空会话、滚动与窄屏布局正常。
- 使用 `bun test`、前端 typecheck、受影响文件 lint/format 和 `git diff --check`；不运行生产构建。

本地开发环境继续由现有 3000 端口统一入口提供，不新增独立 Playground 服务。
