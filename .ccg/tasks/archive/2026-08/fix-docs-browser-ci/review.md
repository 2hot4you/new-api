# 文档浏览器 CI 超时审查

## 结论

- Critical：0
- Warning：0
- 通过。

## 根因

GitHub Runner 上最后一个测试在测试套件末尾再次访问已经覆盖过的 `/api-reference`，并使用 `networkidle` 等待。该页面独立运行约 3 秒即可通过，但在完整开发服务器/HMR 浏览器套件末尾重复导航时达到 30 秒上限。原测试还把多个页面放在同一超时预算中，放大了波动。

## 修复

- 示例与帮助页改为独立测试，各自保留 30 秒上限和原完整断言。
- API orientation 断言合并到开头已经稳定加载 `/api-reference` 的测试中。
- 未增加笼统超时，未删除或放宽任何页面内容断言。

## 验证

- 单独 API orientation 基线：1 passed，约 3.3 秒。
- 修复后完整浏览器文件：12 passed，0 failed，74 assertions，约 33.5 秒。
- `git diff --check`：通过。
- CCG antigravity/Claude wrapper 均不存在（退出 127）；已完成本地根因与范围审查。

