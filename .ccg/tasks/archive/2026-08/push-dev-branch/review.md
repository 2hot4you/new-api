# 推送核对

- 用户所称 `dev` 环境在仓库中对应正式分支 `develop`。
- `.github/workflows/deploy.yml` 监听 `develop`，并部署到 development / `dev.molii.co`。
- 远端不存在 `dev` ref；`origin/develop` 存在。
- 推送前 `origin/develop...HEAD` 为 `0 5`，HEAD 是远端分支的严格快进。
- 工作区除本任务记录外无未提交产品代码。
- 本次只推送到 `origin/develop`，不创建 PR，不更新远端 `main`。
