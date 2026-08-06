# Grok 视频编辑任务类型与预览实施计划

> 设计依据：`docs/superpowers/specs/2026-08-06-fix-grok-video-edit-task-preview-design.md`

## 目标

让视频生成日志正确显示 Grok 视频编辑任务，并为成功任务复用现有 Seedance 视频预览弹框。保持后端签名代理、数据库结构和接口契约不变。

## Task 1：以测试固化任务识别规则

**文件：**

- 新增：`web/src/features/usage-logs/lib/__tests__/task-video-preview.test.ts`
- 新增：`web/src/features/usage-logs/lib/task-video-preview.ts`
- 修改：`web/src/features/usage-logs/constants.ts`

**步骤：**

1. 先新增失败测试，覆盖 `video_edit` 动作映射、平台 `62` 映射、Grok 成功任务预览资格、缺少结果地址、非成功状态和平台 `61` 回归。
2. 运行目标测试，确认因缺失映射/判断能力而失败。
3. 在常量中新增 `VIDEO_EDIT`、`MOLII_GROK` 及对应显示映射。
4. 新增纯函数，统一判断平台 `61`/`62` 的成功视频任务以及是否存在可用预览地址。
5. 再次运行目标测试并确认通过。

**验证命令：**

```bash
cd web
node --import tsx --test src/features/usage-logs/lib/__tests__/task-video-preview.test.ts
```

## Task 2：接入任务列表的预览呈现

**文件：**

- 修改：`web/src/features/usage-logs/components/columns/task-logs-columns.tsx`

**步骤：**

1. 将 `TaskProgressCell` 的平台硬编码判断替换为 Task 1 的纯函数。
2. 成功视频任务统一显示“已生成”；仅有非空 `result_url` 时显示“预览”按钮和 `VideoPreviewDialog`。
3. 保留失败、排队和进行中任务的现有进度显示。
4. 运行 usage-logs 测试、类型检查和目标文件 lint。

**验证命令：**

```bash
cd web
node --import tsx --test src/features/usage-logs/**/__tests__/*.test.ts src/features/usage-logs/**/__tests__/*.test.tsx
pnpm typecheck
pnpm exec oxlint -c .oxlintrc.json src/features/usage-logs/constants.ts src/features/usage-logs/lib/task-video-preview.ts src/features/usage-logs/components/columns/task-logs-columns.tsx src/features/usage-logs/lib/__tests__/task-video-preview.test.ts
```

## Task 3：构建、服务重启和真实任务验收

**文件：**

- 更新：`.ccg/tasks/fix-grok-video-edit-task-preview/task.json`
- 新增：`.ccg/tasks/fix-grok-video-edit-task-preview/review.md`

**步骤：**

1. 执行前端格式检查、完整构建及 Go 全量测试。
2. 重建本机 Go 二进制并通过 LaunchAgent 重启常驻服务。
3. 验证 `/api/status` 返回 200。
4. 在浏览器打开 `视频生成 → Grok Video`，检查任务 `task_rR90qPDjBcnNcJP7cOGvcu2NhADWecJw` 显示 `Grok · 视频编辑`。
5. 点击“预览”，确认弹框使用 `/v1/videos/{task_id}/content` 签名代理地址加载，不泄露上游地址。
6. 检查控制台错误、Git diff 和敏感信息，完成审查记录。
7. 提交功能变更，归档 CCG 任务并提交归档；不推送远端。

**验证命令：**

```bash
cd web
pnpm format:check
pnpm build
cd ..
go test ./...
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3000/api/status
git diff --check
git status --short
```
