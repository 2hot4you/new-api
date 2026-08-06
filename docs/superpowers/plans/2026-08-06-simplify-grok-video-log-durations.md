# Grok Video Billing Labels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 精简 Grok Video 日志详情中的时长与分辨率字段，只展示有效请求参数和最终计费依据。

**Architecture:** 保留 Grok Video V1 计费快照及解析器不变，仅在 `GrokVideoBillingCard` 中按值条件渲染请求字段，并将终态字段改为计费语义。组件测试直接渲染卡片，覆盖字段显示、隐藏及公式不变。

**Tech Stack:** React 19、TypeScript、happy-dom、Node test runner、i18next。

---

### Task 1: 调整 Grok Video 日志详情字段

**Files:**
- Modify: `web/src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx`
- Modify: `web/src/features/usage-logs/components/dialogs/grok-video-billing-card.tsx`
- Modify: `web/src/i18n/static-keys.ts`
- Modify: `web/src/i18n/locales/*.json`

- [ ] **Step 1: 写失败的组件测试**

修改视频编辑测试，断言文本包含 `Billing Duration`、`Billing Resolution`，且不包含 `Requested Duration`、`Requested Resolution`、`Estimated Duration`、`Estimated Resolution`、`Actual Duration`、`Actual Resolution`。修改图生视频测试，断言有效请求参数仍展示 `Requested Duration`、`Requested Resolution`。

- [ ] **Step 2: 运行测试确认正确失败**

Run from `web/`: `bun test src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx`

Expected: FAIL，因为组件仍展示三组旧字段，且尚无 `Billing Duration`、`Billing Resolution`。

- [ ] **Step 3: 实现最小展示修改**

在 `GrokVideoBillingCard` 中：

```tsx
{state.billing.requested_duration_seconds > 0 && (
  <BillingMetric
    label={t('Requested Duration')}
    value={formatDuration(state.billing.requested_duration_seconds)}
    mono
  />
)}
{state.billing.requested_resolution.trim() !== '' && (
  <BillingMetric
    label={t('Requested Resolution')}
    value={formatResolution(state.billing.requested_resolution)}
    mono
  />
)}
<BillingMetric
  label={t('Billing Duration')}
  value={formatDuration(state.billing.actual_duration_seconds)}
  mono
/>
<BillingMetric
  label={t('Billing Resolution')}
  value={formatResolution(state.billing.actual_resolution)}
  mono
/>
```

删除两个预估字段和两个旧的实际字段展示。新增静态翻译键；简体中文使用“计费时长”“计费分辨率”，其他语言通过现有 i18n 同步机制补齐键值。

- [ ] **Step 4: 运行组件测试确认通过**

Run from `web/`: `bun test src/features/usage-logs/components/__tests__/grok-video-log-display.test.tsx`

Expected: 4 tests PASS。

- [ ] **Step 5: 运行定向回归与静态检查**

Run from `web/`:

```bash
bun test src/features/usage-logs/lib/__tests__ src/features/usage-logs/components/__tests__
pnpm format:check
pnpm typecheck
pnpm build
```

Expected: 35 tests PASS，格式、类型和构建均退出 0。

- [ ] **Step 6: 重建本地二进制并做浏览器验收**

沿用现有 LaunchAgent 部署方式重建内嵌前端。确认 `/api/status` 返回 200，并在 Grok Video 日志详情中验证：无预估字段；空请求字段被隐藏；计费时长、计费分辨率和原计费公式正常显示。

- [ ] **Step 7: 审查、提交并归档 CCG 任务**

运行 `git diff --check`、变更文件定向 oxlint 和密钥扫描；记录审查结果，提交实现，移动任务到 `.ccg/tasks/archive/2026-08/` 并提交归档。
