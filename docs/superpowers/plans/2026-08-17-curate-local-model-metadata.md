# Curate Local Model Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用精确公开来源一次性补全本地 17 个模型的中英文简介、图标、日期、模态、能力、参数和 Token 上限，同时保证价格、渠道、状态及发布配置完全不变。

**Architecture:** 将公开资料先整理为可审计的 JSON 清单，再用独立校验脚本检查目标集合、枚举、Token 约束、来源与 LobeHub 图标键。写入前从 PostgreSQL 导出完整备份和受保护字段快照；随后由单个事务按精确模型 ID 覆盖指定字段，回读后同时核对目标字段和受保护字段。

**Tech Stack:** PostgreSQL 15、Docker Compose 本地数据库、Bun/TypeScript 校验脚本、New API Go 后端、TanStack/React 管理与定价页面。

## Global Constraints

- 只处理设计文档列出的 17 个精确模型 ID，不使用同系列或近似版本资料。
- ZenMux 精确模型页优先，models.arts 精确模型页仅作补充；来源未披露的字段保持空值或零值。
- 仅覆盖 `description`、`description_en`、`icon`、`release_date`、`knowledge_cutoff`、`input_modalities`、`output_modalities`、`capabilities`、`supported_parameters`、`context_length`、`max_output_tokens`。
- 不修改价格、渠道、分组、端点、状态、发布开关、标签和生成模型计费/规格字段。
- 不新增自动同步、定时任务、运行时外部依赖或生产构建。
- 不调用 antigravity 或 Claude，不推送远端。

---

### Task 1: 冻结目标集合与来源证据

**Files:**
- Create: `.ccg/tasks/curate-local-model-metadata/source-manifest.json`
- Create: `.ccg/tasks/curate-local-model-metadata/research-notes.md`

**Interfaces:**
- Consumes: PostgreSQL `models` 表当前 17 行；ZenMux/models.arts 精确模型详情页。
- Produces: 以模型 ID 为键的 17 项资料清单，供校验和事务写入使用。

- [ ] **Step 1: 重新查询数据库目标集合**

Run:

```bash
docker exec molii-new-api-dev-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "select model_name from models where deleted_at is null order by model_name"'
```

Expected: 精确返回设计文档中的 17 个模型 ID；若数量或集合不同，停止写入并更新盘点。

- [ ] **Step 2: 为每个 ID 查找精确来源**

对每个本地 ID 记录：精确页面 URL、来源模型 ID、抓取日期、原始英文摘要、页面明确披露的日期/Token/模态/能力/参数，以及未披露字段。命名空间可忽略，但尾部模型 ID 必须精确一致。

- [ ] **Step 3: 形成双语和规范化字段**

人工把英文摘要翻译为中文；删除来源平台价格、供应商数量和促销信息。数组仅使用设计文档允许枚举；Logo 仅使用当前 `getLobeIcon` 可解析的键。

- [ ] **Step 4: 保存结构化清单**

`source-manifest.json` 顶层包含 `captured_at`、`models`；每个模型包含全部 11 个目标字段和 `sources`、`notes`。未知标量使用空字符串或 `0`，未知数组使用 `[]`，不得省略目标字段。

- [ ] **Step 5: 提交研究清单**

```bash
git add .ccg/tasks/curate-local-model-metadata/source-manifest.json \
  .ccg/tasks/curate-local-model-metadata/research-notes.md
git commit -m "data: curate local model metadata sources"
```

### Task 2: 编写并运行离线契约校验

**Files:**
- Create: `.ccg/tasks/curate-local-model-metadata/validate-manifest.ts`
- Test: `.ccg/tasks/curate-local-model-metadata/validate-manifest.test.ts`

**Interfaces:**
- Consumes: `source-manifest.json`。
- Produces: `validateManifest(input): string[]`，空数组表示清单满足写入契约。

- [ ] **Step 1: 写失败测试**

测试必须覆盖：目标模型缺失/多余/重复、非法模态/能力/参数、负 Token、最大输出大于上下文、缺少中英文简介、缺少精确来源 URL，以及完整清单通过。

- [ ] **Step 2: 验证测试先失败**

Run:

```bash
bun test .ccg/tasks/curate-local-model-metadata/validate-manifest.test.ts
```

Expected: 因 `validateManifest` 尚未实现而 FAIL。

- [ ] **Step 3: 实现最小校验器**

导出固定的 17 模型集合和三组允许枚举；逐行返回带模型 ID 和字段名的错误。要求 `sources` 至少包含一个 URL，且来源声明的标准化模型 ID 与本地模型 ID 一致。

- [ ] **Step 4: 跑聚焦测试和真实清单校验**

Run:

```bash
bun test .ccg/tasks/curate-local-model-metadata/validate-manifest.test.ts
bun .ccg/tasks/curate-local-model-metadata/validate-manifest.ts \
  .ccg/tasks/curate-local-model-metadata/source-manifest.json
```

Expected: 测试 PASS；真实清单输出 `17 models validated` 并退出 0。

- [ ] **Step 5: 提交校验器**

```bash
git add .ccg/tasks/curate-local-model-metadata/validate-manifest.ts \
  .ccg/tasks/curate-local-model-metadata/validate-manifest.test.ts
git commit -m "test: validate curated model metadata"
```

### Task 3: 备份并原子覆盖 PostgreSQL 数据

**Files:**
- Create: `.ccg/tasks/curate-local-model-metadata/backups/model-metadata-before-20260817.json`
- Create: `.ccg/tasks/curate-local-model-metadata/apply-model-metadata.ts`
- Test: `.ccg/tasks/curate-local-model-metadata/apply-model-metadata.test.ts`

**Interfaces:**
- Consumes: 已通过校验的 `source-manifest.json`；容器内 PostgreSQL。
- Produces: 一条可重复审计的事务写入，及写入前完整备份和受保护字段摘要。

- [ ] **Step 1: 导出目标行完整备份**

使用 `jsonb_agg(to_jsonb(m) order by model_name)` 导出当前 17 行；备份不得包含密钥或用户凭据。另计算价格、渠道、分组、端点、状态、发布字段的有序 JSON 摘要，供写后对比。

- [ ] **Step 2: 写事务生成器失败测试**

测试必须断言：SQL 包含 `BEGIN`/`COMMIT`、更新白名单仅含 11 个目标字段、每个 ID 使用精确等值条件、事务前后目标集合均为 17、每次更新通过行数校验失败即抛错、字符串和 JSON 数组正确转义。

- [ ] **Step 3: 验证测试先失败**

Run:

```bash
bun test .ccg/tasks/curate-local-model-metadata/apply-model-metadata.test.ts
```

Expected: 因 SQL 生成函数尚未实现而 FAIL。

- [ ] **Step 4: 实现并预览 SQL**

`apply-model-metadata.ts` 默认只输出 SQL；仅在显式 `--execute` 时通过 `docker exec ... psql -v ON_ERROR_STOP=1` 执行。SQL 使用临时值表一次性更新，并以 PL/pgSQL 检查更新数量和集合签名；任意检查失败使整个事务回滚。

- [ ] **Step 5: 跑测试、生成 SQL 并执行一次**

Run:

```bash
bun test .ccg/tasks/curate-local-model-metadata/apply-model-metadata.test.ts
bun .ccg/tasks/curate-local-model-metadata/apply-model-metadata.ts --preview
bun .ccg/tasks/curate-local-model-metadata/apply-model-metadata.ts --execute
```

Expected: 测试 PASS；执行报告精确更新 17 行并成功 COMMIT。

- [ ] **Step 6: 回读并比较**

逐字段比较数据库与清单，受保护字段摘要必须与写入前完全一致。任何差异均使用备份在单事务中恢复并报告。

- [ ] **Step 7: 提交可复现写入资产**

```bash
git add .ccg/tasks/curate-local-model-metadata/apply-model-metadata.ts \
  .ccg/tasks/curate-local-model-metadata/apply-model-metadata.test.ts \
  .ccg/tasks/curate-local-model-metadata/backups/model-metadata-before-20260817.json
git commit -m "data: apply curated local model metadata"
```

### Task 4: 重启本地服务并验收用户界面

**Files:**
- Modify: `.ccg/tasks/curate-local-model-metadata/task.json`
- Create: `.ccg/tasks/curate-local-model-metadata/review.md`

**Interfaces:**
- Consumes: 已写入的 PostgreSQL 数据和本地 LaunchAgent。
- Produces: 3000 端口健康服务及 API/UI 验收记录。

- [ ] **Step 1: 重启唯一后端实例**

Run:

```bash
launchctl kickstart -k "gui/$(id -u)/com.molii.new-api"
```

Expected: 最终仅 `127.0.0.1:3000` 对用户提供 New API，健康检查返回 2xx。

- [ ] **Step 2: 验证公开与管理接口**

查询 `/api/pricing`，确认 17 个模型仍按写前发布状态出现；使用已有登录会话检查 `/models/metadata`，确认所有目标字段可显示/编辑且中文环境显示中文简介。

- [ ] **Step 3: 验证模型广场**

打开 `/pricing`，逐厂商检查 Logo、中文简介、模态、能力、上下文和最大输出 Token；价格、渠道和发布状态不得变化。

- [ ] **Step 4: 运行最终校验**

Run:

```bash
bun test .ccg/tasks/curate-local-model-metadata/*.test.ts
git diff --check
git status --short
```

Expected: 全部 PASS，无格式错误，差异只包含本任务资产。

- [ ] **Step 5: 记录审查并归档任务**

在 `review.md` 记录来源覆盖、未披露字段、数据库/页面验证和回退备份路径；把 `task.json` 标为 `completed`，然后移动到 `.ccg/tasks/archive/2026-08/curate-local-model-metadata`。

- [ ] **Step 6: 提交归档**

```bash
git add docs/superpowers/plans/2026-08-17-curate-local-model-metadata.md \
  .ccg/tasks/archive/2026-08/curate-local-model-metadata
git commit -m "chore: archive ccg task curate-local-model-metadata"
```
