# 根因

开发环境的 `HeaderNavModules` 当前将模型广场配置为 `requireAuth: true`，因此匿名访问 `/api/pricing` 会返回 `401 AUTH_UNAUTHORIZED`。

首页 `DefaultHome` 无条件调用 `usePricingData()`，即使根路由 `/` 本身是公开页面、用户会话已经失效，也仍会请求受保护的 `/api/pricing`。该 401 进入 `web/src/lib/http-client.ts` 的全局响应拦截器，拦截器尝试刷新会话；刷新结果为匿名后调用 `window.location.replace('/sign-in')`。

因此行为链路为：

1. 会话失效后访问或刷新首页；
2. 根级认证 bootstrap 确认用户已匿名；
3. 首页仍请求受保护的定价接口；
4. 全局 401 处理将公开首页送往 `/sign-in`；
5. 用户再次回首页时重复上述过程，看起来像登录页持续弹出。

这不是数据库迁移、登录路由守卫或服务端会话上限导致的循环。

# 建议修复

首页应根据 `pricing.requireAuth` 与当前认证状态决定是否启用 `usePricingData`。匿名且模型广场要求登录时，不请求 `/api/pricing`，相关首页模型区域显示无数据占位。全局 401 重定向还应增加一次性去重作为防御，但不能用它代替修复首页错误的数据依赖。

# 外部模型状态

按项目 CCG 要求并行尝试了 antigravity 与 Claude 分析；本机缺少 `~/.claude/bin/codeagent-wrapper`，两者均以 127 停止，未获得外部模型结果。
