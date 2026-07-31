# Molii 独立前端认证 API

本文档面向独立开发的 Molii 浏览器前端，契约来自当前 New API 源码。
本文只覆盖第一阶段所需的密码注册、登录、刷新、退出、当前用户和登录设备管理。

## 1. 接入原则

- API 默认地址为 `http://localhost:3000`，接口统一位于 `/api`。
- 推荐开发环境由前端开发服务器代理 `/api`，生产环境由同一公开域名反向代理
  `/api` 到 New API。
- 如果前端和 API 使用同站点的不同 Origin，例如
  `https://app.molii.co` 和 `https://api.molii.co`，后端必须显式配置
  `DASHBOARD_CORS_ALLOWED_ORIGINS`。
- 不支持把前端放在与 API 不同的 Site。Refresh Cookie 固定为
  `SameSite=Strict`，不会为了跨站点托管而改为 `SameSite=None`。
- 浏览器请求统一使用 `credentials: 'include'`。Access Token 只保存在内存，
  不写入 `localStorage`、`sessionStorage`、IndexedDB 或 Cookie。

成功响应遵循：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

失败响应遵循：

```json
{
  "success": false,
  "code": "AUTH_UNAUTHORIZED",
  "message": "Unauthorized"
}
```

密码登录和注册的业务校验错误为兼容既有客户端仍返回 HTTP 200，必须同时判断
HTTP 状态与 `success`。会话、鉴权、限流和请求体错误使用对应的 4xx/5xx 状态。

## 2. 生命周期

- Access Token：HS256 JWT，有效期 15 分钟；通过
  `Authorization: Bearer <access_token>` 发送。
- 登录 Session：服务端保存在 PostgreSQL，最长 30 天。
- Refresh Token：随机不透明值，只存在于名为 `new_api_refresh` 的
  HttpOnly Cookie；数据库仅保存 HMAC 摘要。
- Refresh Cookie：host-only、`Path=/api/user/auth`、`HttpOnly`、
  `SameSite=Strict`；生产环境由 `SESSION_COOKIE_SECURE=true` 启用 `Secure`。
- 每次 refresh 都轮换 Refresh Token 并签发新的 Access Token。
- Redis 只缓存用户和 Session 鉴权快照并承担限流；数据库是最终权威。
- 密码、账号状态或安全因子变化会推进 `auth_version`，旧 Token 和旧 Session
  随之失效。

登录或刷新成功的 `data`：

```json
{
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "access_expires_at": 1730000000,
  "user": {
    "id": 1,
    "username": "alice",
    "display_name": "alice",
    "role": 1,
    "status": 1,
    "email": ""
  },
  "session": {
    "sid": "95fb43bd-98f9-4f60-9879-ff2521503418",
    "current": true,
    "login_method": "password",
    "ip": "127.0.0.1",
    "user_agent": "Mozilla/5.0 ...",
    "created_at": 1730000000,
    "last_active_at": 1730000000,
    "expires_at": 1732592000
  }
}
```

前端同时在内存保存 `access_token`、`access_expires_at` 和 `session.sid`。

## 3. 公共状态

### `GET /api/status`

无需鉴权。认证 UI 至少读取：

- `data.setup`
- `data.register_enabled`
- `data.password_login_enabled`
- `data.password_register_enabled`
- `data.email_verification`
- `data.turnstile_check` 和 `data.turnstile_site_key`
- OAuth 与 Passkey 的启用状态及公开客户端配置

该接口表示服务能力，不表示当前浏览器是否已登录。

## 4. 注册

### `POST /api/user/register`

请求头：

```http
Content-Type: application/json
```

请求体：

```json
{
  "username": "alice",
  "password": "a-strong-password",
  "email": "alice@example.com",
  "verification_code": "123456",
  "aff_code": "INVITE"
}
```

- `username`：必填，最多 20 个字符。
- `password`：必填，8–20 个字符。
- `email`、`verification_code`：仅在 `/api/status` 返回
  `email_verification=true` 时必填；关闭邮箱验证时，注册接口不会保存请求中的
  `email`。
- `aff_code`：可选，表示邀请人的邀请码。
- 开启 Turnstile 时，通过查询参数传递：
  `POST /api/user/register?turnstile=<token>`。

成功：

```json
{
  "success": true,
  "message": ""
}
```

注册不会自动登录，也不会设置 Refresh Cookie。成功后前端应调用登录接口。

## 5. 登录

### `POST /api/user/login`

请求体：

```json
{
  "username": "alice",
  "password": "a-strong-password"
}
```

`username` 也可以填写用户邮箱。开启 Turnstile 时同样使用
`?turnstile=<token>`。

未开启 2FA 时，成功响应为第 2 节的登录数据，同时响应写入 Refresh Cookie。

开启 2FA 时：

```json
{
  "success": true,
  "message": "需要二次验证",
  "data": {
    "require_2fa": true,
    "flow_token": "<opaque-token>",
    "expires_at": 1730000300
  }
}
```

此时还没有登录 Session。继续调用：

### `POST /api/user/login/2fa`

```json
{
  "flow_token": "<opaque-token>",
  "code": "123456"
}
```

`code` 可以是 TOTP 或备用码；`flow_token` 有效期 5 分钟。成功后返回标准登录
数据并写入 Refresh Cookie。

## 6. 冷启动与刷新

### `POST /api/user/auth/refresh`

无请求体。浏览器自动发送 Refresh Cookie。

如果内存已有 Session，应发送：

```http
X-Auth-Session: <session.sid>
```

冷启动尚无内存 Session 时省略该请求头。成功响应为新的
`access_token`、`access_expires_at`、`user` 和 `session`，同时轮换 Cookie。

建议的启动状态机：

1. 应用启动时状态为 `checking`，不要先显示“已退出”。
2. 调用 refresh，且不携带不存在的 SID。
3. 成功后进入 `authenticated`，把 Access Token 与 SID 保存到内存。
4. `401 AUTH_UNAUTHORIZED` 或 `401 AUTH_SESSION_REVOKED` 表示服务端确认匿名，
   进入 `anonymous`。
5. 网络错误或 `5xx` 保持可重试状态，不要误判为退出。
6. `409 AUTH_SESSION_MISMATCH` 时清空旧内存身份，不带 SID 重试一次。

多个标签页应串行化 refresh。可用 Web Locks 协调，并用
BroadcastChannel 只广播 SID 和登录/退出事件；不要广播 Token。

## 7. 当前用户

### `GET /api/user/self`

请求头：

```http
Authorization: Bearer <access_token>
```

成功时 `data` 是安全的用户 DTO，包含身份、角色、状态、分组、额度、外部账号
绑定和前端权限等字段；不包含密码、Refresh Token、管理 PAT、管理员备注或
`auth_version`。

Access Token 过期返回 `401 AUTH_TOKEN_EXPIRED`。前端应先 refresh，再用新
Access Token 重试原请求一次。不要对 refresh 请求本身做递归刷新拦截。

## 8. 退出

### `POST /api/user/auth/logout`

无请求体。推荐同时发送：

```http
Authorization: Bearer <access_token>
X-Auth-Session: <session.sid>
```

有 Bearer 时成功数据：

```json
{
  "success": true,
  "message": "",
  "data": {
    "revoked_sid": "95fb43bd-98f9-4f60-9879-ff2521503418",
    "cookie_cleared": true
  }
}
```

仅有 Cookie 或已经没有凭据时，退出仍是幂等的 HTTP 200。请求完成后，无论
服务器返回成功还是已确认凭据无效，前端都应清空本标签页内存中的 Access Token
和 SID。

## 9. 登录设备管理

以下接口必须使用浏览器登录 Session 的 Bearer Access Token；管理 PAT 不能代替。

### `GET /api/user/sessions`

返回当前鉴权版本下仍有效的 Session，当前 Session 排在前面，最多 100 条。

### `DELETE /api/user/sessions/:sid`

撤销指定 Session。

```json
{
  "success": true,
  "message": "",
  "data": {
    "revoked_sid": "<sid>",
    "current": false
  }
}
```

撤销当前 Session 时，服务端会在 Cookie 确实属于该 SID 时清除 Cookie；前端也要
清空内存身份。

### `POST /api/user/sessions/revoke-others`

保留当前 Session 并撤销同一用户的其他 Session：

```json
{
  "success": true,
  "message": "",
  "data": {
    "revoked_count": 3
  }
}
```

## 10. 错误处理

前端只用 `code` 做流程判断，`message` 仅用于展示并允许后端本地化。

| HTTP | Code | 建议处理 |
| --- | --- | --- |
| 200 | `AUTH_INVALID_REQUEST` | 标记输入无效 |
| 200 | `AUTH_INVALID_CREDENTIALS` | 显示用户名或密码错误 |
| 200 | `AUTH_PASSWORD_LOGIN_DISABLED` | 隐藏或禁用密码登录 |
| 200 | `AUTH_REGISTRATION_DISABLED` | 隐藏注册入口 |
| 200 | `AUTH_PASSWORD_REGISTRATION_DISABLED` | 禁用密码注册 |
| 200 | `AUTH_EMAIL_VERIFICATION_REQUIRED` | 要求邮箱验证码 |
| 200 | `AUTH_VERIFICATION_CODE_INVALID` | 提示验证码错误 |
| 200 | `AUTH_EMAIL_ALREADY_TAKEN` | 提示邮箱已使用 |
| 200 | `AUTH_USERNAME_TAKEN` | 提示用户名已存在 |
| 200/401 | `AUTH_USER_DISABLED` | 清空身份并提示账号不可用 |
| 401 | `AUTH_TOKEN_EXPIRED` | refresh 后重试一次 |
| 401 | `AUTH_UNAUTHORIZED` | 清空身份并进入匿名状态 |
| 401 | `AUTH_SESSION_REVOKED` | 清空身份并进入匿名状态 |
| 403 | `AUTH_ORIGIN_FORBIDDEN` | 检查生产 Origin 配置，不自动重试 |
| 409 | `AUTH_SESSION_MISMATCH` | 清空旧 SID，不带 SID refresh 一次 |
| 409 | `AUTH_REFRESH_RACE` | 等待并重试 refresh |
| 409 | `AUTH_SESSION_LIMIT` | 提示先撤销旧设备 |
| 429 | `AUTH_SESSION_ISSUANCE_LIMIT` | 按策略稍后重试 |
| 429 | `RATE_LIMITED` | 读取 `Retry-After` 后重试 |
| 413 | `REQUEST_BODY_TOO_LARGE` | 不重试，修正请求 |
| 400 | `INVALID_REQUEST_BODY` | 不重试，修正请求 |
| 200 | `TURNSTILE_REQUIRED` | 获取 Turnstile Token 后重试 |
| 200 | `TURNSTILE_INVALID` | 刷新 Turnstile 后重试 |
| 200 | `TURNSTILE_UNAVAILABLE` | 稍后重试 |
| 200/500 | `AUTH_INTERNAL_ERROR` | 显示通用错误并允许重试 |
| 500 | `INTERNAL_ERROR` | 显示通用错误并允许重试 |

具体密码登录/注册错误码以当前响应为准；2FA、OAuth 和 Passkey 的部分历史业务
错误仍使用 `success=false` 与本地化 `message`，第一阶段前端不应依赖其文本分支。

## 11. 本地代理

推荐让前端始终请求相对路径 `/api`：

```ts
// vite.config.ts
export default {
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
    },
  },
}
```

```ts
await fetch('/api/user/auth/refresh', {
  method: 'POST',
  credentials: 'include',
})
```

代理模式不需要设置 `DASHBOARD_CORS_ALLOWED_ORIGINS`。本地 `.env` 保持：

```env
SESSION_COOKIE_SECURE=false
# SESSION_COOKIE_TRUSTED_URL 不设置
# DASHBOARD_CORS_ALLOWED_ORIGINS 不设置
```

如果必须让 `http://localhost:5173` 直接请求 `http://localhost:3000`：

```env
SESSION_COOKIE_SECURE=false
DASHBOARD_CORS_ALLOWED_ORIGINS=http://localhost:5173
```

该模式只用于可信本地网络。

## 12. 生产部署

### 推荐：同一公开 Origin

向用户只暴露 `https://app.molii.co`，由反向代理把 `/api` 转发到本机 New API。
这种拓扑不需要 CORS：

```env
SESSION_SECRET=<openssl-rand-hex-32-output>
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://app.molii.co
# DASHBOARD_CORS_ALLOWED_ORIGINS 不设置
TRUSTED_PROXIES=127.0.0.1
SQL_DSN=postgresql://<user>:<password>@127.0.0.1:5432/<database>?sslmode=require
REDIS_CONN_STRING=redis://<user>:<password>@127.0.0.1:6379/0
```

### 可选：同站点兄弟域名

前端为 `https://app.molii.co`，API 为 `https://api.molii.co`：

```env
SESSION_SECRET=<openssl-rand-hex-32-output>
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://app.molii.co
DASHBOARD_CORS_ALLOWED_ORIGINS=https://app.molii.co
TRUSTED_PROXIES=127.0.0.1
SQL_DSN=postgresql://<user>:<password>@127.0.0.1:5432/<database>?sslmode=require
REDIS_CONN_STRING=redis://<user>:<password>@127.0.0.1:6379/0
```

两个配置含义不同：

- `SESSION_COOKIE_TRUSTED_URL` 是 refresh/logout 的 CSRF Origin 白名单。
- `DASHBOARD_CORS_ALLOWED_ORIGINS` 是浏览器读取 `/api` 响应的 CORS 白名单。

两者都要求精确 Origin，不支持通配符、路径、查询参数或域名后缀。TLS 在反向代理
终止时，仍需显式配置公开 HTTPS Origin。生产环境必须使用稳定且所有节点一致的
`SESSION_SECRET`，并将 `TRUSTED_PROXIES` 收窄到实际反向代理地址。
