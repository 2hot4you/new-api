# CI/CD review

## Critical

无。

## Warning

1. 当前 `develop` 的完整前端测试基线为 138 通过、69 失败、61 个加载错误，主要错误为 Bun 对 `node:test` 嵌套执行的限制。本变更在原始 `develop` 与隔离分支复现了相同结果，未修改这些无关测试。发布工作流执行前端 typecheck/build；完整 `bun test` 继续由已有 PR CI 负责。
2. 首次部署没有可用的旧镜像，健康检查失败时只能保留失败容器供排查；第二次及以后部署支持回滚。
3. 真实 SSH、GHCR、1Panel、PostgreSQL、Redis 和 Telegram 集成必须在用户配置六项 GitHub Secrets 与两套服务器 `.env.runtime` 后才能验证。

## Info

- 部署脚本 36 项契约断言覆盖环境映射、必需秘密、成功部署、失败回滚、Compose 隔离和工作流安全边界。
- actionlint v1.7.12、ShellCheck v0.11.0、Bash 语法与 Compose 渲染通过。
- root Go 与独立 relaykit 测试通过，前端类型检查/生产构建通过。
- 本机 ARM 环境已实际完成 `linux/amd64` Docker 镜像构建。
- 1Panel OpenResty 的宿主机网络前置检查已写入上线手册。
- 仓库要求的 antigravity + Claude 双模型分析/审查未执行：配置指定的 `~/.claude/bin/codeagent-wrapper` 在当前机器不存在，文件系统中也未发现替代调用器。
