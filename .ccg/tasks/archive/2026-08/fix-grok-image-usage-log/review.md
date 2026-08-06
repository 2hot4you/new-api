# 审查与验证报告

## 结果

通过。无 Critical 或未解决的 Important 问题。

## 已修复审查问题

- `usage-logs-table.tsx` 最初复制了约 220 行通用筛选器逻辑。已改为参数化复用 `CommonLogsFilterBar`，Image API 保持自己的路由、分页和数据源，但不再维护第二套筛选代码。
- 增加普通用户日志格式化回归测试，确认 `admin_info` 被剥离时，`grok_image_billing` 及其中的数字 0 完整保留。
- 增加零价计费回归：两个 Grok 图片模型在合法价格快照 subtotal=0 时最终 quota=0，预扣可由结算退回，不会按内部 anchor ¥1 或最低 1 quota 收费。

## 自动化验证

- `go test ./...`：通过。
- Grok 图片 Web 新增测试：9/9 通过。
- `bun run typecheck`：通过。
- 变更范围定向 oxlint：通过。
- `bun run build`：通过。
- 24 个变更 Web 文件 oxfmt 检查：通过。
- Go `gofmt -l`：无输出。
- `git diff --check`：通过。
- 常见密钥/私钥/数据库凭据差异扫描：无匹配。
- 全仓 Web lint 仍有任务外既有问题；本任务全部变更文件定向 lint 已通过。

## 页面验证

- launchd 常驻服务 `com.molii.new-api` 已替换为新构建并重启，PID 44451，端口 3000 正常，`GET /api/status` 返回 200。
- `/usage-logs/drawing` 默认选中 Image API，能分页查询同一张通用日志表中的 Grok 图片记录。
- 历史 Grok 日志在列表中显示 Token `-`，详情不显示 Per-call、Model Price ¥1、Token Breakdown 或 1/0。
- 历史详情只显示模型 ID、最终结算和“历史记录缺少分项计费数据”。
- Midjourney 标签切换到 `?source=midjourney&page=1`，保留原任务 ID 筛选和原表格列；切回 Image API 正常。
- 浏览器控制台 error 日志为空。

## 兼容性结论

- 不新增数据库表或 migration；同一消费记录不会重复写入。
- 无 `log_category` 时通用日志查询行为不变，仍能看到 Grok 图片日志。
- 分类只精确匹配 `grok-imagine-image` 与 `grok-imagine-image-quality`。
- Seedance、Midjourney、Grok 视频及其他固定价模型未改变。
- 历史日志不推测缺失参数；只有新日志使用版本化 `grok_image_billing` 完整公式。
