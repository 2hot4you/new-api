# 模型性能页 500 修复审查

## 根因

视频性能接口在没有任务数据时返回 `groups: null` 与 `series: null`。前端性能组件将二者视为数组并读取 `.length`，触发运行时异常；HTTP 接口本身返回 200，页面错误边界将异常呈现为 500。

## 修复

- `GetStarAIVideoPerformance` 初始化空分组和时间序列为非 nil slice，JSON 契约固定为 `[]`。
- 测试覆盖无任务数据时两个集合均非 nil 且为空。

## 验证

- 修复前新增测试失败，准确复现 nil slice。
- `go test ./model -run TestGetStarAIVideoPerformance -count=1` 通过。
- `go test ./...` 通过。
- `git diff --check` 通过。
- 部署后接口返回 `groups: []`、`series: []`。
- 浏览器回归“模型广场 → Seedance 2.0 → 详情 → 性能”正常显示空数据状态，不再进入错误页。
- launchd 常驻服务健康，`/api/status` 返回 200。
