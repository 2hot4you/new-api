# Review: Molii ZenMux 风格首页

## Scope

- 默认首页使用公开 `/api/pricing` 数据生成模型数量、厂商数量、能力类别、厂商跑马灯、搜索建议和最新模型。
- 自定义 URL、HTML、Markdown 首页分支保持不变。
- 未修改 Playground、后端接口、付费请求链、主题切换或部署配置。

## Correctness

- 搜索支持模型 ID、厂商、描述、能力和模态，普通提交跳转 `/pricing?search=...`，建议项直达模型详情。
- 厂商列表只包含当前启用模型使用的厂商，并按名称去重。
- 最新模型仅使用合法发布日期，按日期倒序取前三名。
- 中文 Hero 在桌面保持两行；移动端均衡换行，不产生单字句号孤行。
- 所有循环动效在 `prefers-reduced-motion` 下停用并保留可见内容。

## Verification

- `bun test`: 322 pass, 0 fail。
- `bun run typecheck`: pass。
- `bun run format:check`: pass。
- Scoped `oxlint` on every changed TypeScript/TSX file: pass。
- `git diff --check`: pass。
- Local browser: desktop and 390px mobile verified; no horizontal overflow; `grok` suggestions list four enabled models; search navigates to `/pricing?search=grok`; no console errors。
- Simplified Chinese and Traditional Chinese visible-key scans: 0 missing keys。

## Review Findings

- Critical: 0
- Warning: 0
- Info: Existing footer attribution and navigation behavior remain intentionally unchanged.

External antigravity/Claude review was not invoked because the user explicitly prohibited those models. No subagent was started for this implementation.
