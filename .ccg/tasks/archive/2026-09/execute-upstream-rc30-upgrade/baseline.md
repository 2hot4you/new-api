# 升级分支基线

## Git

- 分支：`upgrade/rc30-dev`
- 起点：`develop@8d1f93c5fe1aadf61cb84c0966007f2375825ae3`
- 上游目标：`v1.0.0-rc.30@27ff6a8767e728f879d52770c273d4f73214a430`
- 真实共同祖先：`v1.0.0-rc.23@0ab02020603d22e5613bc4cf46bfab06f8567769`

## 工具链

- Go：`go1.26.5 darwin/arm64`
- Bun：`1.3.14`
- Node.js：`v22.22.3`

## 基线验证

- `make test`：通过，包括根 Go module 和独立 relaykit module。
- `cd web && bun run typecheck`：通过。
- `cd web && bun test`：当前 `develop` 基线失败，结果为 142 pass、94 fail、86 errors。
  主要错误来自测试文件使用 `node:test` 时被 Bun 识别为嵌套 `describe()`，以及若干
  既有 DOM 测试失败。升级阶段不得增加失败；相关模块另跑定向测试。
- 直接 `go test ./...` 会包含 Makefile 明确排除的根 embed package，不作为项目正式
  基线；正式后端入口为 `make test`。

## 安全边界

- 不连接开发或生产数据库。
- 不读取或记录密钥值。
- 不修改 `develop`、其他 worktree 或未跟踪用户文件。
- rc.28/rc.29 不作为可运行构建。
