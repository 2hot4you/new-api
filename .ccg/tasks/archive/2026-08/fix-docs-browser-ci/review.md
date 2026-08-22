# 文档浏览器 CI 超时审查

## 结论

- Critical：0
- Warning：0
- 通过。

## 根因

GitHub Runner 上的页面内容断言均通过。失败由三个测试基础设施问题叠加造成：开发服务器按页冷编译、Playwright `networkidle` 作为静态内容就绪条件不稳定，以及每项测试都启动/关闭一个完整 Chromium，最后一次进程回收卡住直至 30 秒上限。超时后 Bun 清理 Docusaurus 子进程，后一项才表现为 `ERR_CONNECTION_REFUSED`。

## 修复

- 示例与帮助页改为独立测试，各自保留 30 秒上限和原完整断言。
- API orientation 断言合并到开头已经稳定加载 `/api-reference` 的测试中。
- 浏览器测试先执行 production build，再使用 Docusaurus 静态 `serve`，验证对象与实际部署产物一致。
- 导航等待改为 `DOMContentLoaded`，页面就绪由现有的标题、链接、布局和交互断言判定，不再依赖不稳定的全局网络空闲状态。
- 整份文件只启动一个 Chromium；每项只创建/关闭页面，套件结束时统一清理浏览器与静态服务器，并为清理钩子设置独立 15 秒上限。
- 应用 workflow 忽略纯文档 CCG 归档和设计计划；包含真实应用代码的混合提交仍会触发应用部署。
- 未增加笼统超时，未删除或放宽任何页面内容断言。

## 验证

- 单独 API orientation 基线：1 passed，约 3.3 秒。
- 最终共享浏览器版本连续两次通过：12 passed，0 failed，74 assertions；约 17.5 秒和 15.0 秒。
- 浏览器生命周期契约：5 passed，0 failed，47 assertions。
- 完整非浏览器文档套件：102 passed，0 failed，1704 assertions。
- `git diff --check`：通过。
- CCG antigravity/Claude wrapper 均不存在（退出 127）；已完成本地根因与范围审查。
