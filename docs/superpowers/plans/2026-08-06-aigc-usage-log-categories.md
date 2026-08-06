# Molii AIGC Usage Log Categories Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Molii 日志页重组为“生成记录 → 图像生成 / 视频生成 → 模型族”，并用稳定的后端字段真实筛选 Grok Image、Grok Video 和 Seedance。

**Architecture:** 保留 `/usage-logs/drawing`、`/usage-logs/task` 和现有后端端点。新增一个纯前端模型族注册表负责标签、默认值和平台映射；页面只负责路由交互，API 层负责把模型族转换为 `log_category` 或 `platform` 参数。

**Tech Stack:** React 19、TypeScript、TanStack Router、TanStack Query、Zod、Node test runner、i18next。

---

### Task 1: 模型族注册表与查询映射

**Files:**
- Create: `web/src/features/usage-logs/source-registry.ts`
- Test: `web/src/features/usage-logs/lib/__tests__/source-registry.test.ts`
- Modify: `web/src/features/usage-logs/types.ts`
- Modify: `web/src/features/usage-logs/api.ts`
- Modify: `web/src/features/usage-logs/lib/utils.ts`
- Test: `web/src/features/usage-logs/lib/__tests__/log-routing.test.ts`

- [ ] **Step 1: 写模型族注册表失败测试**

测试应断言：图像生成只有 `grok-image`；视频生成依次为 `grok-video`、`seedance`；非法或跨分区 source 回退到该分区默认值；视频 source 映射到平台 `62`、`61`。

- [ ] **Step 2: 运行失败测试**

Run from `web/`: `bun test src/features/usage-logs/lib/__tests__/source-registry.test.ts`

Expected: FAIL，原因是 `source-registry.ts` 尚不存在。

- [ ] **Step 3: 实现最小注册表与类型**

导出 `GenerationLogSection`、`UsageLogSource`、`GENERATION_LOG_SOURCES`、`resolveUsageLogSource` 和 `getVideoPlatformForSource`。注册值固定为：

```ts
{
  drawing: [{ id: 'grok-image', labelKey: 'Grok Image' }],
  task: [
    { id: 'grok-video', labelKey: 'Grok Video', platform: '62' },
    { id: 'seedance', labelKey: 'Seedance', platform: '61' },
  ],
}
```

- [ ] **Step 4: 运行注册表测试并确认通过**

Run from `web/`: `bun test src/features/usage-logs/lib/__tests__/source-registry.test.ts`

Expected: PASS。

- [ ] **Step 5: 写 API 路由失败测试**

修改 `log-routing.test.ts`：保留 Grok Image 的 `log_category=grok_image` 断言，删除 Midjourney 页面构造器断言，新增 Grok Video 和 Seedance 分别追加 `platform=62`、`platform=61` 的断言。

- [ ] **Step 6: 运行 API 路由测试并确认失败**

Run from `web/`: `bun test src/features/usage-logs/lib/__tests__/log-routing.test.ts`

Expected: FAIL，原因是视频模型族请求构造器尚不存在。

- [ ] **Step 7: 实现 API 参数映射**

为 `GetTaskLogsParams` 增加 `platform?: string`。在 `api.ts` 中增加纯函数 `buildVideoTaskLogRequest(params, isAdmin, source)`，复用 `/api/task` 和 `/api/task/self`，并从注册表取得平台值。`fetchLogsByCategory` 对 task 数据源调用新的 Grok/Seedance 请求函数；移除日志页面对 Midjourney 请求函数的依赖。

- [ ] **Step 8: 运行两个定向测试**

Run from `web/`: `bun test src/features/usage-logs/lib/__tests__/source-registry.test.ts src/features/usage-logs/lib/__tests__/log-routing.test.ts`

Expected: PASS。

### Task 2: 页面结构、路由和文案

**Files:**
- Modify: `web/src/features/usage-logs/index.tsx`
- Modify: `web/src/features/usage-logs/components/usage-logs-table.tsx`
- Modify: `web/src/features/usage-logs/section-registry.tsx`
- Modify: `web/src/routes/_authenticated/usage-logs/$section.tsx`
- Modify: `web/src/hooks/use-sidebar-data.ts`
- Modify: `web/src/features/profile/components/sidebar-modules-card.tsx`
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh.json`
- Modify: other locale JSON files through the existing i18n sync command
- Test: `web/src/features/usage-logs/lib/__tests__/source-registry.test.ts`

- [ ] **Step 1: 扩充失败测试覆盖用户可见元数据**

在注册表测试中断言页面标题键为 `Generation Records`，一级标签键为 `Image Generation` 和 `Video Generation`，并断言所有模型族标签不包含 `Midjourney` 或 `Image API`。

- [ ] **Step 2: 运行测试并确认失败**

Run from `web/`: `bun test src/features/usage-logs/lib/__tests__/source-registry.test.ts`

Expected: FAIL，原因是注册表尚未包含页面和分区元数据。

- [ ] **Step 3: 实现页面和 URL 交互**

`UsageLogsContent` 使用注册表渲染二级标签；`drawing` 固定表格数据源为 `image`，`task` 固定为 `task`。切换一级分类或模型族时写入有效 `source` 并把 `page` 设为 1。`UsageLogsTable` 显式接收已解析的 source，并将其放入 React Query key 和 `fetchLogsByCategory` 配置。

- [ ] **Step 4: 更新路由搜索校验**

将 source 枚举调整为 `grok-image`、`grok-video`、`seedance`；旧 `image` 或 `midjourney` 值由 Zod 回退为未设置，再由当前分区默认值归一化。移除旧 Midjourney 特有的 type 清理分支。

- [ ] **Step 5: 更新所有用户可见文案**

把侧栏和页面标题改成 `Generation Records`，一级标签改成 `Image Generation`、`Video Generation`；中文翻译分别为“生成记录”“图像生成”“视频生成”。个人侧栏模块设置同步使用新文案，但保留模块键 `midjourney`、`task` 以兼容已有权限配置。

- [ ] **Step 6: 同步 i18n 并运行定向测试**

Run: `pnpm --dir web i18n:sync`

Then run from `web/`: `bun test src/features/usage-logs/lib/__tests__/source-registry.test.ts src/features/usage-logs/lib/__tests__/log-routing.test.ts`

Expected: i18n 文件同步完成，测试 PASS。

### Task 3: 静态质量与浏览器验收

**Files:**
- Modify only if validation exposes a scoped defect in the files listed above.

- [ ] **Step 1: 运行前端格式、类型和构建检查**

Run: `pnpm --dir web format:check && pnpm --dir web lint && pnpm --dir web typecheck && pnpm --dir web build`

Expected: 全部退出码为 0。

- [ ] **Step 2: 运行 usage-logs 全部定向测试**

Run from `web/`: `bun test src/features/usage-logs/lib/__tests__ src/features/usage-logs/components/__tests__`

Expected: 全部 PASS。

- [ ] **Step 3: 重建并重启本地后端**

使用项目现有 LaunchAgent 构建/部署方式更新内嵌前端，确认 `http://127.0.0.1:3000/api/status` 返回 200，且进程保持运行。

- [ ] **Step 4: 浏览器桌面验收**

目标流程：`/usage-logs/task` → 页面标题“生成记录” → 一级“视频生成” → 切换 `Grok Video`/`Seedance` → 两类请求和列表分别使用平台 62/61。检查页面非空、无框架错误覆盖层、控制台无相关错误，并保留截图证据。

- [ ] **Step 5: 浏览器图像与移动验收**

目标流程：`/usage-logs/drawing` → 一级“图像生成” → 二级仅 `Grok Image` → 不出现 `Image API` 或 `Midjourney`。在移动视口确认标签可见、不溢出，并保留截图证据。

- [ ] **Step 6: 检查范围并提交实现**

Run: `git diff --check && git status --short && git diff --stat`

Expected: 只有本计划所列文件和 CCG 任务文件发生预期变化，无密钥和临时截图进入仓库。
