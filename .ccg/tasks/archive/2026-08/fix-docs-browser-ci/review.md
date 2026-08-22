# 文档浏览器 CI 超时审查

## 结论

- Critical：0
- Warning：0
- 通过。

## 根因

GitHub Runner 上的页面内容断言均通过；失败发生在页面导航的 `networkidle` 等待。文档页的 HTML 已可用，但字体、脚本或浏览器后台连接无法保证连续满足 Playwright 的网络空闲条件，导致某个后置页面达到 30 秒测试上限。超时后 Bun 会清理 Docusaurus 子进程，因此下一项表现为 `ERR_CONNECTION_REFUSED`。开发服务器的按页冷编译进一步放大了这种不稳定性。

## 修复

- 示例与帮助页改为独立测试，各自保留 30 秒上限和原完整断言。
- API orientation 断言合并到开头已经稳定加载 `/api-reference` 的测试中。
- 浏览器测试先执行 production build，再使用 Docusaurus 静态 `serve`，验证对象与实际部署产物一致。
- 导航等待改为 `DOMContentLoaded`，页面就绪由现有的标题、链接、布局和交互断言判定，不再依赖不稳定的全局网络空闲状态。
- 未增加笼统超时，未删除或放宽任何页面内容断言。

## 验证

- 单独 API orientation 基线：1 passed，约 3.3 秒。
- 最终完整浏览器文件连续两次通过：12 passed，0 failed，74 assertions；约 32.0 秒和 29.4 秒。
- 浏览器生命周期契约：5 passed，0 failed，45 assertions。
- `git diff --check`：通过。
- CCG antigravity/Claude wrapper 均不存在（退出 127）；已完成本地根因与范围审查。
