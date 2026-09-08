# Review — 产品报价及计算器

## 结论

已完成实现、测试、代码审查和实际页面验收。功能仅复用现有 `/api/pricing` 与前端价格规则，没有新增或修改实际计费配置，也没有部署生产环境。

## 实现核对

- 超级管理员导航入口位于 Task Plugins 与系统设置之间；路由使用精确 `ROLE.SUPER_ADMIN` 守卫。
- 复用 pricing 数据、vendor/model 映射、用户分组倍率、币种和全部已支持计费维度。
- 支持搜索、模型/Provider/全选，原始价或明确用户分组价格基准，全局折扣和 Provider 覆盖折扣、备注。
- 折扣使用“折”语义并只应用一次；独立汇率计算器不会写入系统计费配置。
- 动态表达式仅在能够无损展开时生成价格行；不能可靠计算时显示待确认并阻止导出。
- 草稿使用版本化浏览器本地存储；手动刷新价格保留输入。
- HTML 导出使用同一快照与展示规则，内嵌 CSS、无脚本和外部资源，带 CSP、A4 打印样式及完整转义。
- 视频价格单位规范为每百万 Token，并根据单位保留 USD/CNY 币种；导出明确显示实际生效折扣。

## 审查记录

- 内部领域、导出、UI/路由审查均完成。
- UI 审查发现 `zhCN`/`zhTW` 直接传给 `Intl.DateTimeFormat` 会导致运行时崩溃；已改用项目 `toIntlLocale()` 并补回归测试。
- 最终增量审查发现美元视频单位的潜在币种误标；已补币种识别和 USD/CNY 回归测试。
- 最终复审结论：无 Critical 或 Warning。
- CCG 要求的 antigravity/Claude 外部 wrapper 在当前环境不可执行（status 127）；未虚构双模型结果，失败记录保存在 `research/`。

## 验证结果

- Product quotation focused tests: 89/89 passed.
- Frontend full suite: 893/893 曾完整通过；最终小范围币种修复后再次运行得到 892/893，唯一失败为未改动的 rankings 测试 5 秒超时，随后该文件独立复跑 3/3 通过。
- TypeScript typecheck: passed.
- i18n locale parity: passed.
- Changed-file oxlint: passed.
- Changed-file oxfmt check: passed.
- Production build (`bun run build:check`): passed.
- `git diff --check`: passed.
- 变更范围密钥扫描：无匹配；临时视觉验收路由已移除。
- 全仓库 lint/format 检查仍会命中 develop 基线中与本任务无关的既有问题；本任务涉及文件均通过定向检查。

## 浏览器与导出验收

- 使用开发服务器和真实 dev `/api/pricing` 数据检查了桌面布局及 390×844 小屏布局。
- 验证了 OpenAI USD 与 ByteDance CNY 同一报价单、Provider 7.5 折覆盖全局 8.5 折、长备注换行和长模型 ID。
- 实际下载并通过本地静态服务器打开独立 HTML，确认无需登录或站点资源即可渲染；单位、折扣、备注与实时预览一致。
- 页面刷新后无新增运行错误或缺失翻译日志。

## 已知限制

- 草稿仅保存在当前浏览器，不提供数据库报价历史。
- 不可无损解释的动态计价表达式不会伪造价格，需先完善 pricing 数据后才能导出包含该项的报价单。
