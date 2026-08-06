# Grok 图片使用日志与计费详情实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:test-driven-development. 所有生产代码必须在对应失败测试之后编写。

**Goal:** 让 Grok 图片消费日志同时出现在通用日志与绘图 Image API 视图，并以真实价格快照展示完整计费公式。

**Architecture:** 后端在请求生命周期保存版本化 `grok_image_billing` 快照，并通过 `/api/log?log_category=grok_image` 对现有日志表进行精确模型筛选。前端把绘图页拆成 Image API 与 Midjourney 两个独立数据源，Grok 图片使用专用解析器和计费卡，历史数据安全降级。

**Tech Stack:** Go、Gin、GORM、React 19、TypeScript、TanStack Query/Table/Router、Bun test、i18next。

---

### Task 1: 后端计费快照与日志分类契约

**Files:**
- Modify: `relay/common/relay_info.go`
- Modify: `relay/channel/moliigrok/adaptor.go`
- Modify: `relay/channel/moliigrok/adaptor_test.go`
- Modify: `service/text_quota.go`
- Create/Modify: `service/grok_image_billing_test.go`
- Modify: `controller/log.go`
- Modify: `model/log.go`
- Create/Modify: `model/log_grok_image_test.go`

- [ ] 写失败测试：估算阶段保存标准化 generation/edit 快照；响应阶段以实际输出数更新小计。
- [ ] 运行 `go test ./relay/channel/moliigrok -run 'Grok|ImageBilling|ActualCount' -count=1`，确认因快照字段缺失而失败。
- [ ] 在 `RelayInfo` 增加 `GrokImageBillingSnapshot`，由 adaptor 在估算和响应阶段填充。
- [ ] 写失败测试：日志 `other.grok_image_billing` 包含版本化字段，content 使用实际数量和公式，普通日志不受影响。
- [ ] 运行 `go test ./service -run 'GrokImage' -count=1`，确认失败。
- [ ] 在 `PostTextConsumeQuota` 合并快照，并生成实际公式；最终费用使用结算后的 quota 对应金额。
- [ ] 写失败测试：`log_category=grok_image` 只匹配两个明确模型，分页 total 正确且无参数时行为不变。
- [ ] 运行 `go test ./model ./controller -run 'GrokImage|LogCategory' -count=1`，确认失败。
- [ ] 为 admin/self 查询增加可选分类参数和共享 GORM `IN` 筛选，兼容所有数据库方言。
- [ ] 运行上述定向测试和 `gofmt`，确保全部通过。

### Task 2: 前端绘图数据源切换与 Grok 专用计费展示

**Files:**
- Modify: `web/src/routes/_authenticated/usage-logs/$section.tsx`
- Modify: `web/src/features/usage-logs/index.tsx`
- Modify: `web/src/features/usage-logs/types.ts`
- Modify: `web/src/features/usage-logs/api.ts`
- Modify: `web/src/features/usage-logs/lib/utils.ts`
- Modify: `web/src/features/usage-logs/lib/columns.ts`
- Create: `web/src/features/usage-logs/lib/grok-image-billing.ts`
- Create: `web/src/features/usage-logs/lib/__tests__/grok-image-billing.test.ts`
- Modify: `web/src/features/usage-logs/components/usage-logs-table.tsx`
- Modify: `web/src/features/usage-logs/components/usage-logs-mobile-card.tsx`
- Modify: `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`
- Create: `web/src/features/usage-logs/components/dialogs/grok-image-billing-card.tsx`
- Modify: `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`
- Create: `web/src/features/usage-logs/components/__tests__/grok-image-log-display.test.tsx`
- Modify: `web/src/locales/*/translation.json` via `bun run i18n:sync`

- [ ] 写失败测试：精确识别两个 Grok 图片模型，解析 v1 快照，生成 generation/edit 公式，缺失字段安全降级。
- [ ] 运行 `bun test src/features/usage-logs/lib/__tests__/grok-image-billing.test.ts`，确认缺少模块而失败。
- [ ] 实现纯解析与展示模型函数，不用默认值伪造历史数据。
- [ ] 写失败测试：Image API 数据源请求 `/api/log` 并携带 `log_category=grok_image`；Midjourney 仍请求 `/api/mj`。
- [ ] 运行对应 Bun 测试，确认路由行为失败。
- [ ] 增加内部 `image` 日志类别；绘图页顶部用 `source=image|midjourney` 切换，默认 image，两个来源独立分页与筛选。
- [ ] 写失败组件测试：新日志展示模型、分辨率、比例、数量、公式和最终费用，且不出现 Per-call、Model Price、Token Breakdown、1/0；历史日志显示降级提示。
- [ ] 运行组件测试，确认旧通用组件暴露锚点和 token。
- [ ] 增加专用计费卡；详情、桌面列和移动卡使用同一 Grok 判别器隐藏锚点和哨兵 token。
- [ ] 运行 `bun run i18n:sync`，补齐新增文案。
- [ ] 运行定向 Bun 测试、`bun run typecheck`、`bun run lint` 与 `bun run build`。

### Task 3: 集成验证与回归审查

**Files:**
- Modify: `.ccg/tasks/fix-grok-image-usage-log/review.md`

- [ ] 运行 `gofmt` 检查和后端定向测试，再运行 `go test ./...`。
- [ ] 运行 Web 定向测试、typecheck、lint、format check 和 build。
- [ ] 重启 launchd 常驻后端并确认 `127.0.0.1:3000` 健康。
- [ ] 在本地页面验证绘图页两个标签、Grok 详情、历史降级和 Midjourney 回归。
- [ ] 审查 `git diff --check`、密钥扫描和变更范围；将结果写入 `review.md`。
- [ ] 如没有可沉淀的新通用约定，跳过 spec evolution；归档任务并提交归档记录。

