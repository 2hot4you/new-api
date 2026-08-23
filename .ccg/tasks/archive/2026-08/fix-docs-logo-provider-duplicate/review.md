# Review

## Scope

- Docusaurus Classic 官方 Header/Footer 结构保持不变。
- Header 使用 `web/public/molii-wordmark.png` 的精确副本，不再叠加文字标题。
- `Provider 与模型` 的手动侧栏分类拥有官方 `generated-index` 链接 `/providers`。
- 生成器不再创建 `docs/providers/index.mdx`，避免同名子页面。

## Findings

- Critical: 0
- Warning: 0
- Info: 生成目录 `_category_.json` 不能控制手动声明的侧栏父分类；入口链接必须配置在 `sidebars.ts` 的分类对象上。

## Verification

- Focused RED：生成目录仍携带错误入口元数据、手动侧栏分类缺少 `link`，3 个契约断言失败。
- Focused GREEN：21 tests / 397 assertions passed。
- Browser GREEN：13 tests / 69 assertions passed，覆盖 `/docs/providers`、Logo、固定浅色与移动布局。
- Full docs gate：125 tests / 1818 assertions passed；禁词、Secretlint、生产构建与 29 条内部链接检查通过。
- OpenAPI lint、catalog deterministic check、`git diff --check` 通过。
- 本地视觉复核：Wordmark 清晰、侧栏仅一个 `Provider 与模型` 父分类、官方 generated-index 展示 10 个 Provider。

## External Review Availability

按 CCG 要求并行尝试 antigravity 与 Claude reviewer，但环境中缺少 `/Users/naf/.claude/bin/codeagent-wrapper`，两者均无法启动。已使用完整 diff、契约测试、生产浏览器测试与视觉复核完成替代审查。

## Security / Compatibility

- 未修改 API、鉴权、计费、部署配置或密钥处理。
- `/docs/` 子路径、Provider/模型详情页、搜索和固定浅色行为保持不变。
- 未发现值得追加到项目 Spec 的新通用约定。
