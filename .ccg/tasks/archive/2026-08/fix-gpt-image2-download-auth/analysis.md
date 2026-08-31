# 根因分析

## 证据

- GPT Image 2 预览查询通过项目 Axios 客户端发起，请求拦截器会从认证状态注入 `Authorization: Bearer ...`。
- 下载按钮当前渲染为普通 `<a href>`，浏览器导航不会执行 Axios 拦截器，也不会附加保存在前端状态中的 Bearer token。
- 下载后端路由使用 `middleware.UserAuth()`，只接受 Authorization Bearer 凭证，因此该导航稳定返回 401。
- 预览查询能成功说明登录状态、资源归属和 COS 临时对象本身均正常，失败边界位于浏览器下载请求的鉴权方式。

## 修复原则

- 不降低后端鉴权，不公开 COS 对象，不把 token 放入 URL。
- 使用现有 Axios API 客户端请求附件 Blob，使请求继续获得 Bearer 注入和 401 token 刷新能力。
- 请求成功后创建短生命周期 object URL，并通过临时下载锚点触发浏览器保存。

## 双模型分析状态

- 已并行尝试 antigravity 与 Claude debugger。
- 两路均因本机缺少 `/Users/naf/.claude/bin/codeagent-wrapper` 未能启动。
