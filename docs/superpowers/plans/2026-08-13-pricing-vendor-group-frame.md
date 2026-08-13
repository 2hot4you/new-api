# 模型广场厂商分组外框 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 使用低对比度圆角外框包裹每个厂商的标题、简介与模型卡片网格。

**Architecture:** 继续由 `ModelCardGrid` 负责厂商分组展示，不新增组件或数据层。只调整分组 `section` 与 Logo 容器的设计令牌，并通过现有 DOM 契约测试保护结构和响应式网格。

**Tech Stack:** React 19、Tailwind CSS、Bun test、happy-dom

## Global Constraints

- 仅修改模型广场厂商分组组件与对应测试。
- 使用现有主题设计令牌，不新增硬编码颜色。
- 保留现有模型卡片列数、筛选、分页与数据来源。
- 不运行生产构建。

---

### Task 1: 厂商分组整组外框

**Files:**
- Modify: `web/src/features/pricing/components/model-card-grid.tsx`
- Test: `web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx`

**Interfaces:**
- Consumes: `groupModelsByVendor(pagedModels)` 返回的厂商模型分组。
- Produces: 每个带 `data-model-vendor-section` 的 `section` 同时包裹厂商标题与 `data-model-vendor-group` 模型网格。

- [ ] **Step 1: 写入失败测试**

在现有厂商分组测试中断言：

```tsx
const sections = [
  ...container.querySelectorAll('[data-model-vendor-section]'),
]
assert.equal(sections.length, 2)
for (const section of sections) {
  assert.match(section.className, /rounded-2xl/)
  assert.match(section.className, /border/)
  assert.match(section.className, /bg-muted\/20/)
  assert.equal(section.querySelectorAll('[data-model-vendor-heading]').length, 1)
  assert.equal(section.querySelectorAll('[data-model-vendor-group]').length, 1)
}
```

并断言 Logo 容器不再包含 `shadow-sm`。

- [ ] **Step 2: 运行测试确认 RED**

Run:

```bash
cd web && bun test src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
```

Expected: FAIL，因为分组尚未提供外框标记和外框样式。

- [ ] **Step 3: 实施最小样式修改**

将厂商 `section` 调整为：

```tsx
<section
  key={groupKey}
  className='bg-muted/20 space-y-3 rounded-2xl border p-3 sm:space-y-4 sm:p-4'
  data-model-vendor-section='true'
>
```

将 Logo 容器的 `shadow-sm` 移除，其余标题和模型网格结构保持不变。

- [ ] **Step 4: 运行定向验证确认 GREEN**

Run:

```bash
cd web
bun test src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
bun run typecheck
bunx oxlint -c .oxlintrc.json src/features/pricing/components/model-card-grid.tsx src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
```

Expected: 测试、类型检查及定向 lint 全部通过。

- [ ] **Step 5: 本地页面验证并提交**

确认 `http://127.0.0.1:3000/pricing` 中每个厂商标题、简介和模型网格均位于同一外框内，然后提交：

```bash
git add web/src/features/pricing/components/model-card-grid.tsx web/src/features/pricing/components/__tests__/model-card-grid-vendor-groups.test.tsx
git commit -m "style: frame pricing vendor groups"
```
