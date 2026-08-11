# Playground Assistant-UI Base Style Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不替换现有 Playground 请求运行时的前提下，将 `/playground` 改造成 assistant-ui Base 风格，并让所有高级参数默认关闭且可按项启用。

**Architecture:** 保留现有状态 hooks、模型/分组选择器、消息组件、SSE 与非流式请求链。只调整页面 shell、空状态、composer 和参数面板；通过独立存储版本迁移一次性重置高级参数开关。

**Tech Stack:** React 19、TypeScript、TanStack Router、Base UI、AI Elements、StickToBottom、Tailwind CSS、Bun test。

## Global Constraints

- 不引入 `@assistant-ui/react` 或新聊天运行时。
- 不改变 `/pg/chat/completions`、SSE、非流式、编辑、重试、停止和消息存储协议。
- 不新增 `/app/playground`；正式页面仍为 `/playground`。
- 不提供假的 Share、附件、搜索、mentions 或 slash commands。
- 模型与分组必须复用现有 `ModelGroupSelector`。
- 六个高级参数默认关闭；迁移只重置启用状态，不删除参数值、模型、stream 或消息。
- 不运行生产构建，只运行测试、typecheck、lint/format 和本地开发服务。

---

### Task 1: 高级参数默认值、存储迁移与请求体契约

**Files:**
- Modify: `web/src/features/playground/constants.ts`
- Modify: `web/src/features/playground/lib/storage/storage-schema.ts`
- Modify: `web/src/features/playground/lib/storage/storage.ts`
- Modify: `web/src/features/playground/lib/state/playground-state-utils.ts`
- Create: `web/src/features/playground/lib/storage/storage-migration.test.ts`
- Create: `web/src/features/playground/lib/streaming/payload-builder.test.ts`

**Interfaces:**
- Produces: parameter-enabled schema version 2.
- Produces: `migrateParameterEnabledState` that resets exactly six flags once.
- Preserves: config values, selected model/group, stream and message storage.

- [ ] **Step 1: Write failing migration and payload tests**

Assert clean state is all false; version-1 true flags become false while stored values/messages remain; version-2 user choices remain; disabled fields are omitted; enabled `0` and negative values are preserved; enabled `seed=null` is omitted.

- [ ] **Step 2: Run RED tests**

```bash
cd web && bun test src/features/playground/lib/storage/storage-migration.test.ts src/features/playground/lib/streaming/payload-builder.test.ts
```

- [ ] **Step 3: Implement the one-time migration and defaults**

Set the six `DEFAULT_PARAMETER_ENABLED` fields to false. Version only the parameter-enabled envelope; when an old version is read, return all-false enabled flags and immediately persist the new envelope without modifying other storage keys.

- [ ] **Step 4: Run GREEN and existing stream tests**

```bash
cd web && bun test src/features/playground/lib/storage/storage-migration.test.ts src/features/playground/lib/streaming/payload-builder.test.ts src/features/playground/hooks/use-stream-request.test.ts
```

### Task 2: Base-style page shell and empty state

**Files:**
- Modify: `web/src/features/playground/index.tsx`
- Modify: `web/src/features/playground/components/chat/playground-chat.tsx`
- Modify: `web/src/features/playground/components/chat/playground-empty-state.tsx`
- Create: `web/src/features/playground/components/chat/playground-empty-state.test.tsx`
- Create: `web/src/features/playground/playground-layout.test.tsx`

**Interfaces:**
- Consumes: existing conversation props and `onSelectPrompt`.
- Produces: centered 44rem thread, independent scroll area, Base-style header and suggestions.
- Preserves: all message actions and latest-24-message behavior.

- [ ] **Step 1: Write failing empty-state and layout contract tests**

Assert suggestions invoke `onSelectPrompt`, the page exposes one new-conversation action, no Share control exists, thread is scrollable, composer region remains separate, and narrow layouts have no fixed overflowing width.

- [ ] **Step 2: Run RED tests**

```bash
cd web && bun test src/features/playground/components/chat/playground-empty-state.test.tsx src/features/playground/playground-layout.test.tsx
```

- [ ] **Step 3: Recompose the shell without changing runtime hooks**

Build a compact header, centered empty greeting, suggestion chips and a max-width thread. Reuse the existing clear-conversation callback for “新建对话”; do not add a new message store or request state.

- [ ] **Step 4: Run GREEN and message regressions**

```bash
cd web && bun test src/features/playground/components/chat/playground-empty-state.test.tsx src/features/playground/playground-layout.test.tsx src/features/playground/hooks/use-stream-request.test.ts
```

### Task 3: Base-style composer, model selector and parameter panel

**Files:**
- Modify: `web/src/features/playground/components/input/playground-input.tsx`
- Modify: `web/src/features/playground/components/input/playground-input-controls.tsx`
- Modify: `web/src/features/playground/components/input/playground-input-tools.tsx`
- Modify: `web/src/features/playground/components/input/playground-parameter-panel.tsx`
- Create: `web/src/features/playground/components/input/playground-parameter-panel.test.tsx`
- Create: `web/src/features/playground/components/input/playground-input-controls.test.tsx`

**Interfaces:**
- Consumes: existing `ModelGroupSelector`, submit/stop callbacks, config and parameter-enabled state.
- Produces: one composer containing model/group, parameter, stream and submit controls.
- Preserves: desktop Popover, mobile Sheet/Drawer and current disabled rules.

- [ ] **Step 1: Write failing interaction tests**

Assert all six switches start off, disabled values cannot be edited, enabling one field only changes that field, stream switch calls `onConfigChange('stream', value)`, generation disables model/parameter changes, and send/stop states remain mutually exclusive.

- [ ] **Step 2: Run RED tests**

```bash
cd web && bun test src/features/playground/components/input/playground-parameter-panel.test.tsx src/features/playground/components/input/playground-input-controls.test.tsx
```

- [ ] **Step 3: Restyle the composer and reorganize settings**

Keep the existing submit event and textarea behavior. Place the shared model selector, parameter trigger and send/stop button inside the composer footer. Add stream as a regular switch and remove nonfunctional attachment/search controls from the primary UI.

- [ ] **Step 4: Run GREEN and payload regressions**

```bash
cd web && bun test src/features/playground/components/input/playground-parameter-panel.test.tsx src/features/playground/components/input/playground-input-controls.test.tsx src/features/playground/lib/streaming/payload-builder.test.ts
```

### Task 4: i18n, accessibility and final local verification

**Files:**
- Modify if needed: `web/src/i18n/static-keys.ts`
- Modify if needed: `web/src/i18n/locales/en.json`
- Modify if needed: `web/src/i18n/locales/zh.json`
- Modify if needed: `web/src/i18n/locales/zh-TW.json`
- Modify if needed: other existing locale JSON files for newly introduced keys only
- Modify: `docs-site/docs/platform/model-square-and-playground.mdx`

**Interfaces:**
- Consumes: Tasks 1–3 final UI behavior.
- Produces: accessible labels, complete translations and accurate user documentation.

- [ ] **Step 1: Add a failing static contract test for visible controls**

Extend the closest Playground layout test to require accessible names for new conversation, model selector, parameter settings, stream and send/stop, and to reject untranslated literal keys.

- [ ] **Step 2: Add only required translation keys and documentation updates**

Reuse existing strings wherever possible. Update the user page to explain model selection, per-parameter opt-in and stream without mentioning internal storage or admin configuration.

- [ ] **Step 3: Run the complete local frontend checks**

```bash
cd web && bun test src/features/playground && bun run typecheck
```

- [ ] **Step 4: Run repository hygiene checks**

```bash
git diff --check
```

Expected: Playground tests and typecheck pass, the development route remains `/playground`, and no production build is executed.
