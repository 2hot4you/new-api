# rc.46 升级评估：前端、依赖、品牌、CI 与部署

评估日期：2026-10-22。仅基于 Git 对象只读比较；没有 merge、业务修改、依赖安装、构建、测试、push 或部署。唯一写入为本报告。

## 基线与范围

- 自定义分支依据 `origin/develop=c2442919e29403123cf8e5bcd15eea4db4dcc3e6`；没有用工作树旧 HEAD `e05…` 替代它。
- 上游基线 rc.40=`0aec08fee811ec6136828fda790551b49e410301`；目标 rc.46=`7d8b91268dab69d7150f345c448b5e250163146a`。
- `git diff rc.40..rc.46 -- web Dockerfile .github`：275 个文件，20637 行增加、3132 行删除。与 `rc.40..c2442919` 的相同范围交集为 **67 个路径**。交集代表双方修改同一文件，不等于已确认文本冲突，更不等于语义兼容。
- 已读根目录及 `web/AGENTS.md`、`.ccg/spec/guides/index.md`、`.agents/rules/billing.md`。未调用被用户禁止的 antigravity/Claude 执行器，未连接任何数据库。

## 各版本实际新增（逐相邻 tag 的 Git log 核实）

| 版本 | 面向用户/管理员的实际前端变化 | 主要证据 |
| --- | --- | --- |
| rc.41 (`2035a82ae`) | 安全中心单一 dashboard access token 改为可命名、可编辑权限的 scoped access tokens；管理员用户管理增加 step-up verification；渠道刷新按钮与 Responses WebSocket 设置保存修复；插件市场缓存校验修复；任务定价 usage profile 收窄时保留可编辑价格；源价格表达式差异高亮 | `caca52f8d`, `4924361ae`, `6cbb1c7ed`, `d04c118c8`, `3509ae7c1`, `6c14c0762` |
| rc.42 (`6370b2942`) | 渠道高级参数覆盖支持 regex；用户列表分组筛选；非 root 管理员不显示系统设置；zhCN/zhTW 缓存语言码修复；搜索工具价格配置扩展。构建增加 tokenkit 独立模块与 moejs PGO | `04c597bf6`, `956019109`, `dfd3cd893`, `5fe8917be`, `d10f11fff`, `bb28f2cca`, `192802ee0` |
| rc.43 (`c6741c36a`) | 每渠道 TLS 跳过证书验证开关（默认 false）；插件上传展示声明访问域名；后端消息/错误/日志由前端翻译 | `c6741c36a`, `3f0224478`, `128089fa7` |
| rc.44 (`633b4c5b1`) | 普通用量日志导出、用户导出开关及时间范围设置；日期到秒/跟随界面语言；Playground 按账户隔离；页面代码块加载失败自动重载一次；模型卡显示 cache read 价；日志预览保留结算时记录的 tier；近期成功率样本不插入无数据空档；字体 Auto 恢复浏览器 sans-serif；依赖集中升级 | `633b4c5b1`, `93ea55f3c`, `96275d224`, `06dd76a9d`, `b892a6b2e`, `3827fd830`, `fc08b76f0`, `1ca79d57e`, `0f38a14a5` |
| rc.45 (`6cf0d90b5`) | 渠道优先级拖动/引导；AWS Mantle 与 Vertex API-key project_id 表单；API key 允许查询账户额度开关；模型输入/输出/按次价格分别排序；新增价格分组置顶；任务未配置为非零的计费项隐藏；首屏主题脚本；通知按类型开关；DeepSeek search/fetch 工具价格；控制台搜索遵循侧栏权限 | `5e9595190`, `5c31c1fa9`, `d03220473`, `a0bf6a8a4`, `7de10f2c6`, `c9f7b6208`, `b4be2a784`, `5a59d6915`, `b0a98ac49`, `6461f084f`, `6cf0d90b5`, `908f94d65` |
| rc.46 (`7d8b91268`) | 仪表盘 token 统计新增缓存使用；邮件设置新增 Resend provider；Vitest 5；Go 1.26.10 安全升级 | `603a5ceb5`, `32e587406`, `4d36f2c52`, `7d8b91268` |

注意：rc.41 发布说明的 Full Changelog 是 rc.39..41，不能把其中所有内容都当作 rc.40 后新增。本表只采用相邻 tag 的真实差异。

## 必须合并处理的构建与发布要求

1. **Go 工具链与 Docker 构建同时升级。** rc.46 `go.mod` 要求 `go 1.26.10`；自定义 Docker 当前仍为 `golang:1.26.5-alpine`。上游 Docker 目标镜像为 `golang:1.26.10-alpine@sha256:9ce231b28998b32e116927c61aa5c406183061d4f3f2d61b292c88e7f6bdf7ca`。只改模块文件而漏改 builder 会使容器构建不满足版本。上游提交将此升级归因为 CVE-2026-111943；这里未独立验证 CVE 公告。
2. **tokenkit 是新的本地 Go 模块。** Docker 在 `go mod download` 前需要 `ADD tokenkit/go.mod ./tokenkit/go.mod`，与现有 relaykit 相同。CI、deploy verification 的缓存应加入 `tokenkit/go.sum`，并增加 `GOWORK=off` 的独立 vet/build/test。现有自定义 CI/deploy/makefile 只显式覆盖 root 与 relaykit。不能因 root 编译通过就推断 tokenkit 独立模块通过。
3. **PGO 加入实际采用的构建路径。** 上游 Docker/release/electron 增加 `-pgo="$(go list -m -f '{{.Dir}}' github.com/Calcium-Ion/moejs)/default.pgo"`。将它与自定义 `${APP_VERSION}` ldflags 合并，保留正确模块路径 `github.com/QuantumNous/new-api/common.Version`；不要恢复上游通过 VERSION 文件取值而丢掉自定义构建参数。
4. **PostgreSQL 专用验收必须保留。** 自定义 `.github/workflows/ci.yml`、`deploy.yml` 已建立 PostgreSQL 15 + Redis、隔离 DB、`make test-postgres` / `make test-postgres-race`。上游 ci.yml 仍执行广义 `make test`；不能整份采用上游文件覆盖本分支，也不能因上游新增 tokenkit 就恢复 SQLite/MySQL 测试。需要检查 PostgreSQL runner 的独立模块列表，并纳入 tokenkit 的非数据库测试。
5. **部署行为要保留且推送有外部影响。** 自定义 `.github/workflows/deploy.yml` 在 push develop/main 触发，`deploy/resolve-targets.py` 把 develop 映射 development，main 映射四个生产站点。升级分支推到 develop 不是“仅提交源码”；会触发部署。评估阶段没有授权触发这些流程。后续实施应先在独立升级分支完成 review 与验收。
6. **Docker 自定义能力不能被替换。** 保留所有 `VITE_SITE_*` build args、APP_VERSION/VCS_REF/OCI labels、`pkg/billingexpr/calendars/cn.v1.json` 在前端构建前的复制、`catalog-sync` 二进制构建和复制。它们都不在上游 Docker 原版内。

额外现存发布边界：自定义应用发布实际使用 `deploy.yml`（GHCR digest、每站点品牌、备份/二次启动验证、回滚和结果记录）；遗留 `docker-build.yml` / `docker-image-branch.yml` 仍指向 `calciumion/new-api` 且没有传新 Dockerfile 的 APP_VERSION/品牌参数。它们不是本次上游新增，但不能将这些遗留流程误当成此次多站点升级的发布入口。

## 依赖合并清单

`web/package.json` 与 `web/bun.lock` 都有双方修改，应合并 manifest 后由 Bun 重生成 lock，最后验证 `bun install --frozen-lockfile`；不能任选某一侧 lock。

| 依赖 | rc.40 → rc.46 | 与自定义分支的注意点 |
| --- | --- | --- |
| Vitest | `^4.1.10` → `^5.0.4` | 自定义 `vitest.config.ts` 有较多测试别名、SSR inline、超时配置与 happy-dom 依赖；是测试主版本升级，必须运行实际全套前端测试确认，不应只改数字 |
| DOMPurify | pinned/override `3.4.13` → `3.4.16` | 两处同步，保留自定义其他安全 overrides |
| KaTeX | `^0.17.0` → `^0.18.2`，新增 override `0.18.2` | 验证 Markdown/公式渲染 |
| ai | `^7.0.14` → `^7.0.136` | 验证 Playground streaming/reasoning/账户隔离 |
| stream-markdown-parser | `^1.0.9` → `^1.2.18` | 同上，覆盖流式渲染 |
| @lobehub/icons | `^5.10.1` → `~5.22.0` | 保留 custom provider/group icons；上游也更新 model-provider 规则 |
| oxfmt / router-plugin / yace | `^0.57.0→^0.72.0` / `^1.168.19→^1.168.42` / `^1.1.0→^1.1.1` | 格式器变更需限制改动范围，不应顺便格式化全部自定义文件 |

自定义覆盖还包括 `@hono/node-server=2.0.12`, `brace-expansion=2.1.3`, `fast-uri=4.1.2`, `hono=4.12.34`, `postcss=8.5.25`，以及 `i18n:check` 和 happy-dom。上游 manifest 的旧值不能覆盖这些现有加固。

## 品牌与定价的高风险重叠

### 品牌

- `web/index.html`：上游新增首屏主题脚本，读取 `newapi:theme:v1:mode`；自定义已有 siteBrand 标题、描述、favicon/apple-touch-icon 模板。应追加脚本并保留模板，而不是采用任一整份文件。
- `web/src/lib/theme-customization.ts`、provider、`styles/theme.css` / `styles/index.css`：上游将 `ResolvedThemeFont` 扩展为 `system`，Auto 降级 browser sans。自定义通过 `SITE_BRAND.defaultFont` 默认 anthropic/md/sm，并有 Claudeye 品牌颜色。需区分“用户 Auto”与“站点指定 sans/serif”，验证新会话、保存偏好、light/dark、切换站点构建。
- `web/build/site-brand.ts` 定义 molii/ixiaozu/claudeye；Claudeye 动态 favicon 使用 `/api/branding/claudeye/favicon.svg`。这些是自定义保留项。生产有两个 Claudeye 站点但共享 brand profile，故应验证四个生产目标而非只验证三类 profile。

### 定价目录与元数据

- `pricing/components/model-card.tsx`、`model-details.tsx`、`model-card-grid.tsx`、`constants.ts`、`lib/filters.ts`、`lib/dynamic-price.ts`、`lib/price.ts`、`lib/billing-expression/display.ts` 均为双方修改路径。自定义并非仅换皮：有模型目录、多模态/能力/上下文筛选、推荐排序、USD/CNY 计价、Grok/视频/文本专门价格展示、时段/节假日表达式。
- **需要专门适配的语义问题：** rc.45 `filters.ts` 新增输入/输出/按次价排序，假定抽出的数值是 USD。自定义 dynamic-price 中 `.value` 仍保留表达式原币值，仅 `.formatted` 调用 `sourceAmountToUSD`；直接采用上游新排序会让 CNY/USD 模型用不同币种的原数值比较。`getCardExamplePrice` 上游新增 `.total` 时也必须统一货币/单位再参与排序。此为依据两侧代码的兼容性推断，尚未合并/运行复现。
- 自定义还保留 `SORT_OPTIONS.RECOMMENDED`、`QUOTA_TYPES.DYNAMIC`、目录筛选函数；上游新的 sort 常量不能把这些功能删掉。需明确兼容旧 `price-low/price-high` 搜索参数/保存状态。
- rc.44 `model-card-grid.tsx` 新增 `recent_model` 参数与 `recent_success_series`：上游按分页请求当前页模型；自定义已改为目录 grid/无限加载。应按当前已显示模型集合适配请求，保留自定义布局并防止重复无限请求，不能照搬回分页。
- `display.ts` rc.41 对 usage schema 收窄后的 dropped boolean/enum/count、shadowed branches 做兼容；rc.45 task-matrix-display 隐藏全零项但保留全免费模型。它们应与自定义表达式扩展共同回归，而非覆盖定价表达式。保存旧报价、历史账单必须保留当时快照。
- rc.45 `model-card.tsx` 仅在 `dynamicSummary.tierCount > 1` 时展示任务 tier 标签。自定义已有“默认价/基础时段价/按输入长度”等表达语义；需要验证单 tier、时间规则、多 tier 各自文案，不要简单删除所有自定义 tier 描述。
- `system-settings/models/model-ratio-visual-editor.tsx` / `upstream-price-cells.tsx`：上游增加选中文本保护及源价格表达式差异高亮；自定义已有 currency override / pricing snapshot / visual editor，需组合交互。
- `features/models/types.ts` / `lib/model-form.ts` 自定义模型元数据是本地维护，含 metadata_source、capabilities/modalities/context、marketplace ordering/visibility；`sync_official` 明确为只读后端字段、无同步控制。不能因升级上游 models-table.tsx / delete-dialog 恢复官方同步入口或覆盖本地元数据。

### 日志、渠道、侧栏的组合风险

- `usage-logs/components/columns/common-logs-columns.tsx`：rc.44 明确保留 saved matched_tier，不能猜测当前/首个 tier 价格；自定义已有 `source_currency` 历史货币、Grok 图像/视频账单、无快照提示。应优先采用历史快照语义，再组合上游中性 fallback。新导出只说明“common log page”，不要假设已覆盖自定义聚合任务/视频日志。
- `system-settings/maintenance/log-settings-section.tsx` 上游拆成 `history-log-cleanup.tsx` + `server-log-files.tsx` 并改 props 为 defaultValues。自定义原文件已有区分日志/生成记录的清理计数（`log-cleanup-result.ts`）；拆分时要把行为移到新文件，不能留下未再调用的 helper 就算保留成功。
- `channels` 表单/API/type 与自定义 Grok/image、对象存储、模型配置共用；必须保留自定义字段并完整导入 `tls_insecure_skip_verify`、`aws_api_type`、`vertex_project_id` 的 default/read/write 逻辑，以及 priority 两个后端接口 `/api/channel/priority/order`、`/api/channel/priority/reorder`。单独复制 UI 不足以提供新功能。
- `keys` 表单/type 与自定义 group metadata 交集：新增 `allow_user_quota_query`，上游新 key 默认 true、读取缺字段默认 false；验证保存编辑已有 key 不意外开启权限，并保持名称/图标/小数价格等自定义展示。
- `use-sidebar-data.ts` / command menu 改为按实际可见导航搜索并仅 super-admin 显示系统设置；要保留自定义视频生成、临时素材、报价、账单日历导航，并确认权限规则在新增入口仍生效。安全中心 token 与用户 step-up 改动应随服务端契约一起验收；本报告不宣称完成认证安全审查。

## 后续实施的验收重点（本轮未执行）

1. 合并后执行 frozen Bun install、typecheck、前端测试、i18n:check、受影响 lint/format 检查和生产构建；覆盖全部三类品牌配置及四个生产目标使用的真实构建参数。
2. 定价验证至少包含 USD/CNY 同价模型排序、cache read、固定/动态/按次/任务价格、全零模型、usage schema 收窄、Grok/视频专用展示、历史 source_currency/matched_tier 快照、推荐排序与模型目录筛选。
3. 新旧 API key 保存、scoped token 与 step-up、渠道优先级/AWS/Vertex/TLS、普通日志导出/范围限制、Resend 保持 SMTP 兼容、通知开关与缓存统计，均需与升级后服务端联调。
4. Go 1.26.10 下 root/relaykit/tokenkit 各自独立 vet/build；数据库只用用户授权的隔离 PostgreSQL 验收与 race suites，扩充 tokenkit 非 DB 测试覆盖。真实容器构建确认 branding/calendar/catalog-sync 与版本号。
5. 开发环境上线、备份恢复与重复启动、生产审批/推送属于后续授权范围；本次评估没有执行，也不把旧 CI 报告视作 rc.46 集成成功证据。

## 交叉审阅 assessment.md / backend-contract-research.md

- **Critical：无。** 评估明确采用最新 develop，区分 92 个实际冲突路径（29 web）与本报告 67 个同路径重叠；建议按六版审查、一次集成 rc.46、开发验收后再按授权发布，没有提出六次部署，也没有把旧 CI cached-plan 故障当成当前阻塞。
- **Warning：无已确认的前端/部署事实错误。** assessment 已纳入混合币种排序、单 tier 展示、默认厂商排序、tokenkit、Go1.26.10、品牌构建及 PostgreSQL-only 门槛。Playground 一次清除旧浏览器共享数据的表述有 `06dd76a9d` 实现直接支持（旧共享键无法确定归属并被删除）。
- **Info：建议消除一句基线歧义。** backend-contract-research 的 “Root go.mod jumps from Go1.25.1 to1.26.10” 指上游 rc.40→46；本地最新 develop 实际是 `go 1.26.5`，本地升级应写 `1.26.5→1.26.10`。assessment 已正确区分，故不影响整体结论。
- 审阅只核对报告事实与授权边界，没有再次运行测试、合并或部署，也没有执行外部模型或启动子代理。
