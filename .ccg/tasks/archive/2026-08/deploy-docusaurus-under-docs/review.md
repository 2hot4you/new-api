# Docusaurus `/docs/` CI/CD 审查

## 结论

- Critical：0
- Warning：0
- Info：2
- 结论：通过，可在服务器完成一次性目录授权后推送触发部署。

## 审查范围

- Docusaurus 在 `DOCS_BASE_URL=/docs/` 下的构建与站内链接。
- 独立 GitHub Actions 文档流水线及与应用流水线的触发隔离。
- SSH 上传、SHA-256 校验、归档路径/链接校验、并发锁、静态目录发布与失败回滚。
- 正式/开发环境的固定域名与固定服务器路径映射。
- 敏感信息边界、最小 GitHub Actions 权限和现有产品/部署范围。

## 结果

### Critical / Warning

未发现。

### Info

1. 服务器需预先创建并授权以下目录给 `molii-deploy`，部署脚本不会自行扩大系统目录权限：
   - `/opt/1panel/www/sites/molii.co/index/docs`
   - `/opt/1panel/www/sites/dev.molii.co/index/docs`
2. 当前环境缺少 CCG 指定的 `~/.claude/bin/codeagent-wrapper`；antigravity 与 Claude 两个审查调用均以 127 退出。已执行本地逐项安全审查并保留命令结果，但无法生成外部模型意见。

## 安全与正确性核对

- GitHub Actions 仅授予 `contents: read`，Actions 均固定到 commit SHA。
- 分支仅映射 `main -> production`、`develop -> development`，域名和服务器目录在服务端再次固定校验。
- 远端归档路径必须匹配发布环境与 release ID；SHA-256 不匹配直接失败。
- tar 包拒绝绝对路径、`..` 路径、符号链接及特殊文件，解压时禁用 owner/permission 继承。
- 发布前保存完整旧快照；发布或健康检查失败时使用 `rsync --delete` 恢复完整旧树。
- `flock` 防止同环境文档并发发布，Actions concurrency 不取消已开始的部署。
- SSH 使用已配置 known_hosts，不启用宽松主机验证；未新增业务数据库、Redis、API Key 或应用 Secret。
- 应用部署忽略纯 `docs-site/**` 变更；文档拥有独立流水线，不修改生产应用配置。

## 验证证据

- 非浏览器 Bun 测试：101 passed，0 failed，1699 assertions。
- 浏览器文档测试：11 passed，0 failed，74 assertions。
- `check:forbidden`、`check:secrets`、`api:lint`、`catalog:check`：通过。
- Development `/docs/` production build：通过。
- 内部链接爬取：111 links，全部 HTTP 200。
- 部署脚本测试：14/14，通过（含校验、陈旧文件清理、健康检查失败回滚）。
- `bash -n`：三个 shell 脚本通过。
- Ruby YAML 解析：两个 workflow 通过。
- `git diff --check`：通过。

