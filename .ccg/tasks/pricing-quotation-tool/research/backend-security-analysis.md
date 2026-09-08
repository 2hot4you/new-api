# 产品报价工具：后端权限与导出安全研究

研究基线：`feat/product-quotation-calculator`，HEAD `1ac88772e`。本报告只分析现有实现与建议，不修改产品代码。

## 结论摘要

1. 当前 `GET /api/pricing` **默认公开**。`HeaderNavModules.pricing.requireAuth=true` 时也只调用 `UserAuth()`，因此它不是超级管理员权限边界（`router/api-router.go:35`，`middleware/header_nav.go:104-122`）。
2. 不应把现有 `/api/pricing` 直接改成 `RootAuth()`：公开 `/pricing` 模型广场正在使用它，改动会破坏现有产品行为。
3. 首版报价草稿、预览和 HTML 下载全部可在浏览器完成，因此从功能角度 **不需要新增写接口、导出接口或数据库表**。超级管理员页面应放在 `/_authenticated` 下，并像 Task Plugins / System Settings 一样在路由 `beforeLoad` 做 `ROLE.SUPER_ADMIN` 精确判断；侧边栏同时设 `requiredRole`，但侧边栏隐藏本身不算鉴权。
4. 如果验收要求“报价功能调用的后端数据接口也必须是超级管理员专用”，建议增加一个极薄的只读别名，例如 `GET /api/quotation/pricing`，挂 `RootAuth()` 与 `DisableCache()`，内部复用 `controller.GetPricing`，不要复制定价组装逻辑。报价页面改调该别名；公开 `/api/pricing` 保持不变。
5. 导出安全的核心是：预览使用 React 文本节点；独立 HTML 使用静态模板加**逐字段、逐上下文转义**；禁止把标题、报价人、备注、vendor/model/task-plugin 元数据或 `footer_html` 原样拼进 HTML。文件内不含脚本、外链、表单或远程资源，并加入离线 CSP。
6. 报价不能只复用卡片上的格式化字符串。必须先建立带币种、单位、价格来源、分组倍率、折扣系数和获取时间的结构化快照，再让预览与导出消费同一快照，否则很容易重复乘倍率/折扣，或把 USD、直接 CNY、Token、按次、图片、视频和 task usage 混在一起。

## Files Found

| 文件 | 作用与发现 |
| --- | --- |
| `router/api-router.go:35` | `/api/pricing` 使用 `HeaderNavModuleAuth("pricing")`，不是 Root 权限。 |
| `middleware/header_nav.go:17-36,104-122` | pricing 模块默认 `enabled=true, requireAuth=false`；启用 requireAuth 也只执行 `UserAuth()`。 |
| `middleware/auth.go:48-78,95-110` | `authHelper` 检查启用用户和最低角色；`RootAuth()` 的最低角色为 `common.RoleRootUser`。这是报价专用后端接口应复用的机制。 |
| `router/api-router.go:237-253` | Task Plugins 管理 API 整组使用 `RootAuth()`；这是“仅超级管理员”的直接后端模式。`task_plugin_options` 是 AdminAuth + 可授权 permission，不适合“永远仅 root”的报价要求。 |
| `web/src/routes/_authenticated/task-plugins/index.tsx:25-30` | Task Plugins 页面在 `beforeLoad` 精确检查 `ROLE.SUPER_ADMIN`，非 root 跳 `/403`。 |
| `web/src/routes/_authenticated/system-settings/route.tsx:25-35` | System Settings 的同类 root-only 页面模式。 |
| `web/src/hooks/use-sidebar-data.ts:155-171` | Task Plugins 和 System Settings 的现有侧边栏顺序；新报价项应插入二者之间，并设置 `requiredRole: ROLE.SUPER_ADMIN`。 |
| `controller/pricing.go:36-76` | 组装 pricing 响应。登录用户会按自身 user group 覆盖 group-to-group ratio，再过滤 usable groups；响应包含 models、vendors、group ratios、usable groups、endpoint、auto groups、pricing version。 |
| `model/pricing.go:21-65` | 后端 `Pricing` 完整字段，包括固定/Token 比率、缓存、图片、音频、动态表达式、task usage schema/example、视频和 Grok 直接价格、币种。 |
| `model/pricing.go:402-510` | 定价记录的真实构建链：`model_price` 优先确定按次；否则 model ratio/expr 为 Token/动态；随后补各种倍率、表达式、task usage、视频和 Grok 价格。 |
| `web/src/features/pricing/types.ts:23-112` | 前端定价、vendor、task usage、视频和 Grok 的接口类型。注意 `PricingData` 当前没有声明后端顶层的 `pricing_version`。 |
| `web/src/features/pricing/hooks/use-pricing-data.ts:26-78` | 已有消费模式：请求 `/api/pricing`，按 `vendor_id` 映射 vendor 名称，并把顶层 `group_ratio` 注入每个模型。provider 应沿用此 vendor 映射，不能使用 `owner_by`。 |
| `web/src/features/pricing/lib/price.ts:63-102,131-239` | 固定 Token/按次价格公式，以及充值率换算。缓存/图片/音频等维度的计算在这里。 |
| `web/src/features/pricing/lib/model-helpers.ts:45-90` | 可用分组和 group ratio 读取；`getConfiguredGroupRatio` 能保留合法的 0 倍率。 |
| `web/src/features/pricing/lib/dynamic-price.ts:172-196,482-628` | 动态模型识别、普通 tier/task usage tier 解析、动态价格条目和“无法结构化解析”的识别。 |
| `web/src/features/pricing/lib/task-expr.ts:25,197-255,300-326` | task usage expression 的 1M Token 缩放、矩阵解析和样例成本计算。 |
| `web/src/features/pricing/lib/video-pricing.ts:21-31` | `video_pricing` 是直接人民币/每 1M 或 1K Token 的展示路径。 |
| `web/src/features/pricing/lib/grok-pricing-table.ts:57-85` | Grok 按质量/分辨率输出价格，以及图片/视频输入价格的结构化展开。 |
| `controller/misc.go:75-101` | 公共 `/api/status` 已提供 `usd_exchange_rate` 和 `price`。后者是系统固定充值价格率；二者语义不能混用。 |
| `web/src/components/html-content.tsx:62-131,186-205` | 现有任意 HTML 展示使用 DOMPurify，并明确禁止 `base/embed/link/meta/object/script` 与 `srcdoc`。可借鉴其威胁模型，但不适合直接拿来清洗完整离线文档。 |
| `web/src/components/ui/markdown.tsx:165-172,737-786` | 现有 HTML 实体转义和“先 sanitize 再 dangerouslySetInnerHTML”的模式。`escapeHtml` 是文件内私有函数，报价功能需要自己的可测试工具。 |
| `web/src/features/models/components/dialogs/view-logs-dialog.tsx:157-165` | 现有浏览器文件下载模式：Blob → object URL → `<a download>` → revoke。 |
| `web/src/components/ai-elements/code-block.tsx:498-510` | 同一下载模式，带 `charset=utf-8`，适合报价 HTML 下载骨架。 |
| `middleware/disable-cache.go:5-10` | root-only 报价定价别名如新增，应使用此中间件避免用户相关 group ratio 被中间缓存。 |

## Dependencies

### 现有定价读取链

```text
GET /api/pricing
  -> HeaderNavModuleAuth("pricing")
     -> 默认 TryUserAuth；配置 requireAuth 时 UserAuth
  -> controller.GetPricing
     -> model.GetPricing / model.GetVendors
     -> ratio_setting.GetGroupRatioCopy
     -> 登录用户的 GetGroupGroupRatio 覆盖
     -> service.GetUserUsableGroups 过滤模型和倍率
  -> web/features/pricing/api.getPricing
  -> usePricingData
     -> vendor_id -> vendors[id]，得到 provider/vendor 名称
     -> 顶层 group_ratio 注入每个 model
  -> price / dynamic-price / video / Grok formatter
```

### 建议的报价链

```text
/_authenticated/quotation
  -> parent route: 必须已登录
  -> quotation beforeLoad: role === SUPER_ADMIN
  -> （严格后端边界时）GET /api/quotation/pricing
     -> RootAuth + DisableCache
     -> 复用 controller.GetPricing
  -> normalizePricingSnapshot（结构化价格、单位、币种、来源、因素）
  -> quotationSnapshot（深拷贝 + fetchedAt + pricingVersion）
     -> React preview
     -> pure buildQuotationHtml(snapshot)
        -> Blob(text/html;charset=utf-8) -> download
```

## Patterns

### 1. 权限模式

- 前端页面同时复用两个既有层次：父级 `/_authenticated` 保证登录；页面 `beforeLoad` 精确检查 `ROLE.SUPER_ADMIN`。参考 `web/src/routes/_authenticated/task-plugins/index.tsx:25-30`。
- 导航项用 `requiredRole: ROLE.SUPER_ADMIN` 只负责可见性。参考 `web/src/hooks/use-sidebar-data.ts:155-171`。不可只做这一层，否则手输 URL 仍可进入。
- 后端“永远仅 root”使用 `RootAuth()`；Task Plugins 的 `/api/plugin/task` 已采用此模式（`router/api-router.go:237-251`）。不要使用 `AdminAuth()` 单独保护，也不要套用可被授予普通管理员的 `TaskPluginBind` permission。
- `RootAuth` 同时支持项目现有 dashboard session/PAT 认证链，并返回规范的 401/403 错误（`middleware/auth.go:48-66,107-110`）。

### 2. 是否新增后端接口

**默认建议：不新增写/导出接口。** 首版没有报价历史，HTML 在客户端冻结并下载，服务器没有需要持久化或生成的状态。

对只读价格接口有两个可接受方案：

- 最小改动：继续读公开 `/api/pricing`。安全含义是“报价编辑 UI 仅 root 可用”，而不是“价格数据仅 root 可读”；价格数据本来就是公开模型广场数据。
- 严格验收方案（更推荐）：新增 `/api/quotation/pricing` 或同等命名的 root-only 别名，路由挂 `RootAuth()` + `DisableCache()`，handler 仍为 `controller.GetPricing`。它提供真正可测的后端权限边界，并保留 root 用户的 group-to-group override/usable group 语义。不要复制 `GetPricing`，也不要把现有 `/api/pricing` 改成 root-only。

除非将来增加共享、审批或历史，不要新增 `POST /quotation/export`、报价表或服务器临时文件。客户端导出更符合“离线、当前快照、浏览器草稿”，也消除了服务器端存储客户备注的额外风险。

### 3. 价格结构与计算来源

| 类型 | 接口判定/字段 | 基准与单位 | 报价注意点 |
| --- | --- | --- | --- |
| Token 固定价格 | `quota_type=0` 且非动态 | `base = model_ratio * 2 * groupRatio`，input 为 base；output 再乘 `completion_ratio`；cache/create-cache/image/audio 使用相应倍率（`price.ts:63-98`）。展示默认每 1M Token，1K 时除 1000。 | 必须保留 input/output/cache write/cache read/image/audio in/audio out 每个独立维度；不能相加成总价。 |
| 按次 | `quota_type=1`, `model_price` | `model_price * groupRatio`，单位 request（`price.ts:213-239`）。 | `0` 是合法价格，不可用 truthy 检查判断“未配置”。 |
| 动态/阶梯 Token | `billing_mode="tiered_expr"`, `billing_expr`，无 task usage schema | `p/c/cr/cc/cc1h/img/img_o/ai/ao` 等表达式系数，通常每 1M Token；`billing_currency=CNY` 时系数是直接人民币。 | 复用 `getDynamicPricingTiers`/`getDynamicPriceEntries`。保留 tier 条件和 request/time multiplier；不能只展示第一档。解析失败时展示条件/原表达式或“待确认”，禁止落为 0。 |
| Task usage 动态 | 上述动态字段 + `billing_usage_schema` | unit 可为 `second/count/token/credit`，可含 base request charge；Token usage 以 1,000,000 缩放（`task-expr.ts:25,316-319`）。 | 必须携带 schema 单位、enum 条件、所有 tiers，必要时带 examples；插件提供的字段名、description、enum、example label 都按不可信文本处理。 |
| StarAI video | `video_pricing` | 直接 CNY/Token，按分辨率、是否有视频输入分档；另有 FPS、extra frames 和 Token 公式。 | 不得当 USD 再换汇；保留两列、分辨率与公式。 |
| Molii Grok | `molii_grok_pricing` | 直接 CNY，输出按 image/second，输入可按 image/second，按质量/分辨率。 | 不得和 Token 单价合并；保留输入/输出单位与每个 tier。 |

provider 映射应严格使用 `model.vendor_id -> response.vendors[id].name`，沿用 `usePricingData`（`web/src/features/pricing/hooks/use-pricing-data.ts:46-62`）。`model_name` 是对外 Model ID。`owner_by` 在当前 pricing 构建路径没有赋值，不能作为 provider。

顶层 `pricing_version` 已由后端返回（`controller/pricing.go:67-76`），但 `PricingData` 类型尚未声明；报价快照若要记录版本，应补类型并显式读取。接口没有服务器生成的 fetched-at，前端应在**成功响应/成功 refetch**时记录时间（最好使用 React Query 的 `dataUpdatedAt`），失败重试不能更新获取时间。

### 4. 汇率、分组倍率与折扣的安全计算约束

- `/api/status` 的 `price` 是“1 系统 USD 的固定充值价格率”，`usd_exchange_rate` 是当前显示换汇率；现有 `applyRechargeRate` 用 `priceRate / usdExchangeRate` 转换（`price.ts:105-138`）。新计算器中的“系统固定汇率”应明确对应哪个字段，按需求示例通常是 `status.price`，不能把二者同名混用。
- USD 来源的报价若以系统固定汇率展示，可用：`报价分组倍率 = 实际汇率 * 折扣系数 / 系统固定汇率`，然后只乘一次。等价结果是 `USD 基准价 * 实际汇率 * 折扣系数`。
- 直接 CNY 来源（`billing_currency=CNY`、`video_pricing`、`molii_grok_pricing`）不应再次做 USD 汇率转换；通常只应用折扣。必须在快照中记录 `sourceCurrency`/`conversionMode`，不要让统一 formatter 盲目套汇率。
- `/api/pricing` 返回的 `group_ratio` 可能已经按当前 root 用户组执行了 group-to-group override（`controller/pricing.go:45-54`）。快照至少应分别保存 `sourceGroup`、`sourceGroupRatio`、`globalDiscount`、`providerDiscount`、`effectiveDiscount`，provider 覆盖全局后只应用一次。
- 如果计算器生成的 group ratio 本身已经包含折扣系数，就不能在最终价格上再乘 provider/global 折扣。建议让计算函数返回 factor breakdown，而不是只返回一个数。
- 所有输入需 `Number.isFinite`、边界检查并拒绝空串/NaN/Infinity/负值；“8 折”规范化为 0.8。不要用 `value || default`，否则合法的 0 会被替换。
- 小额价格保持数值到最终格式化再舍入；内部不要通过格式化字符串反解析。

### 5. 动态价格表达的具体风险

- `getDynamicPricingSummary` 能标记 `isSpecialExpression`，这是“待确认”路径，应复用（`dynamic-price.ts:570-628`）。
- 普通动态条目和 task usage 条目当前都过滤 `value <= 0`（`dynamic-price.ts:509-515,544-548`）。因此直接把 summary 当报价完整维度会隐藏“明确配置为 0”的维度。报价归一化层需区分“字段不存在”和“字段存在且为 0”；若现有 parser 无法可靠区分，显示条件/原表达式或待确认，不能擅自把缺失字段显示成免费。
- `hasDynamicRequestRules` 只说明存在 request rule，基础 summary 不会替你应用所有条件 multiplier（`dynamic-price.ts:495-500`）。导出应保留条件说明或标成条件价；没有明确请求参数时不能导出一个伪造的唯一单价。
- Task plugin alias 可能继承 declared model 的表达式，同时 usage schema 来自插件（`model/pricing.go:468-503`）。因此不要按 model ID 白名单判断动态类型；以接口字段为准。

## HTML 与文件导出安全建议

### 输出架构

1. 在点击导出时深拷贝当前已验证的 `quotationSnapshot`；之后刷新价格或修改草稿不得改变这份对象。
2. 预览直接由 React 组件渲染结构化快照，所有用户/接口文本放 JSX 文本节点，不用 `dangerouslySetInnerHTML`。
3. `buildQuotationHtml(snapshot)` 只接受已验证快照；文档骨架、CSS 和标签名全部为源码常量。所有动态字符串经单一 `escapeHtmlText`/`escapeHtmlAttribute` 工具处理。
4. 备注换行用 CSS `white-space: pre-wrap`，或先转义后把换行换成 `<br>`；绝不能反过来。
5. 用 `new Blob([html], { type: 'text/html;charset=utf-8' })`、object URL 和 `<a download>` 下载，完成后在 `finally`/下一事件循环 revoke URL。现有文本下载模式见 `view-logs-dialog.tsx:157-165` 与 `code-block.tsx:498-510`。

### 独立 HTML 的最小安全基线

- 包含 `<!doctype html>`、`<meta charset="utf-8">`、`<meta name="referrer" content="no-referrer">`。
- 添加 meta CSP，例如：`default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:; base-uri 'none'; form-action 'none'`。
- 不生成 `<script>`、事件属性、`iframe/object/embed/base/form`，不把快照 JSON 塞进 script 标签。
- CSS 内不插入用户输入。标题、报价人、日期、备注、vendor/model 名、动态 tier/field/enum/description/example、品牌名都当不可信文本。
- “无需联网或站点资源”意味着不能引用站点 CSS、Google Fonts、远程 logo 或 vendor icon。品牌可以使用安全转义后的 `system_name`；logo 只允许经 MIME/大小校验后的 `data:image/png|jpeg|webp|svg+xml;base64,...` 快照。不要运行时 fetch 任意 logo URL，也不要直接复用公共 status 的 `footer_html`。
- A4 CSS 使用静态 `@page { size: A4; margin: ... }` 与 `break-inside: avoid`；避免固定高度导致长备注/多维度表被截断。

### 文件名净化

项目目前没有通用的 HTML 文件名净化工具。建议 feature 内建立独立纯函数并测试：

- 标题先 trim/规范化，移除控制字符及 `/\\:*?"<>|`，折叠空白，去掉结尾点/空格；限制长度；空结果回退 `quotation`。
- 日期只接受规范化 `YYYY-MM-DD`，最终固定拼成 `<safe-title>-<date>.html`。
- 不把文件名放进 HTML 属性或服务端 `Content-Disposition`。如未来改成服务端导出，应参考 `controller/log.go:147-163` 使用 `mime.FormatMediaType`，不要手拼 header。

### 浏览器草稿

- localStorage key 带 feature 与 schema version；读取时 `try/catch`，做结构校验、长度/数量上限和安全默认值，拒绝 prototype keys/异常大 payload。
- 草稿可能含客户名、报价人和商务备注；这是同源脚本可读数据。页面应说明“仅保存在本浏览器”，提供清除草稿，且不得把 access token、完整导出 HTML 或远程资源内容存进去。

## Risks

### Critical

- **导出型持久 XSS**：若备注/标题/vendor/model/task-plugin metadata 原样拼进 HTML，收件人打开文件即可能执行攻击代码。DOMPurify 在当前应用预览中的保护不会自动延续到手写导出字符串。

### High

- **权限边界误判**：`/api/pricing` 不是 root-only；只隐藏侧边栏也不是鉴权。必须有页面 route guard；若需求按后端强制验收，则使用 root-only 别名。
- **破坏公开定价**：直接把现有 `/api/pricing` 改成 `RootAuth` 会让模型广场不可用。
- **倍率/折扣重复**：用户组 override、系统 group ratio、计算器生成倍率、global/provider 折扣和充值率混用时容易重复乘。
- **币种重复转换**：直接 CNY 动态/视频/Grok 价格若按 USD 再乘汇率会系统性报错。
- **动态价伪造**：只取第一 tier，忽略 request/time rules，或把解析失败/字段缺失当 0，会输出错误商务报价。

### Medium

- **0 值被吞**：已有动态 summary 会过滤 0；多处旧展示模式使用 truthy fallback 的风险也存在。必须以字段存在性和有限数检查为准。
- **版本与时间不完整**：当前前端类型缺顶层 `pricing_version`，hook 也不暴露 `dataUpdatedAt`；导出可能无法证明价格快照来源。
- **缓存串用户**：`GetPricing` 会因登录用户组返回不同 group ratio/usable models，同一 URL 不应被共享缓存。新增 root-only 别名应 `private/no-store`；若继续直接用公开路由，至少确认网关不会跨 Authorization 缓存。
- **本地草稿泄露/膨胀**：同源 XSS 可读，畸形或巨大 localStorage 内容可导致页面卡死。
- **远程品牌资源泄露**：离线 HTML 引用 logo/icon URL 会在打开文件时联网、暴露收件人 IP，并违反“独立离线”要求。

## 测试建议

### 权限与路由

- 前端路由：未登录跳 sign-in；普通用户/管理员跳 `/403`；`ROLE.SUPER_ADMIN` 可渲染。
- 侧边栏：非 root 不出现报价项；root 出现且顺序严格为 Task Plugins → 产品报价及计算器 → System Settings。
- 如新增报价 pricing 别名：无认证 401、普通用户 403、管理员 403、root 200；验证 dashboard session 与允许的 PAT 认证路径；响应带 private/no-store。
- 回归：匿名访问现有 `/api/pricing` 仍成功；公开 `/pricing` 页面仍可加载。

### Pricing 合约

- controller/前端 fixture 覆盖：`quota_type=0/1`、合法 0、极小非零、cache read/write、image、audio in/out、video matrix、Grok image/video、不同单位与直接 CNY。
- 验证 provider 必须由 `vendor_id` 映射；缺 vendor 时显示明确 fallback，不能错误分组。
- 验证 root 用户 group-to-group override 与 usable group 过滤，且快照记录实际使用的 group/ratio。
- 动态表达式：普通多 tier、input-length、time/request rule、task usage second/count/token/credit、base charge、alias expression、解析失败；失败时输出“待确认/条件价”，不输出 0。
- 0 值：明确配置 0 应显示“0/免费”而非 `—`；字段缺失仍显示不适用，二者必须可区分。
- 汇率示例：实际 6.8、5 折、固定 7 得 0.4857x；provider override 优先于 global；每个 factor 只应用一次；直接 CNY 不重复换汇。

### 导出与 XSS

- 对标题、报价人、备注、vendor、model、tier label、usage field/description/example 分别注入：`</style><script>...`、`<img onerror=...>`、引号、`&`、Unicode、换行。解析导出文档后应保持原始 `textContent`，且不存在 `script`、事件属性、`iframe/object/embed/base/form` 或外链资源。
- 验证 CSP/charset/referrer meta 存在，所有 CSS 内联，图片只为受控 data URI；断网打开仍完整。
- 文件名覆盖斜杠、反斜杠、控制字符、Windows 非法字符、全空标题、超长标题和中文。
- Blob MIME 严格为 `text/html;charset=utf-8`，object URL 在下载后释放。
- 快照冻结：开始导出后再修改备注/折扣或成功刷新 `/api/pricing`，已生成 HTML 内容不变；下一次导出才使用新值。
- 阻止条件：无模型、pricing 加载失败、动态价无法安全表达且未标“待确认”、日期/汇率/折扣无效时不得创建 Blob，并给出具体原因。

### 打印与视觉

- 使用浏览器打印预览检查 A4、长 provider 分组、跨页表头、长备注、中文/英文混排、超长 model ID、小数精度和空字段；确认任何一行价格的币种/单位不会在分页时与数值分离。
