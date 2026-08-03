# 拉取与更改记录审阅

## 拉取结果

- 分支：`feat/molii-auth`
- 拉取方式：`git pull --ff-only origin feat/molii-auth`
- 更新范围：`c1e4fb3d..33f656f0`
- 结果：fast-forward 成功，无用户本地代码修改被覆盖
- 远端新增记录：10 个上游提交、1 个上游同步 merge 与 1 个 Molii 功能提交

## Molii 提交

`33f656f0 feat: complete StarAI video and asset workflow`

- 133 个文件，新增 10,308 行，删除 714 行
- 28 个测试文件，新增约 2,001 行测试
- 完成 StarAI/Seedance 异步视频、临时素材、COS 直传、视频签名代理、计费矩阵、性能指标和管理界面
- 两个 Seedance 2.0 模型已进入 StarAI 渠道列表、全局模型倍率/价格目录、默认描述和前端渠道目录

## 验证

- `go test ./...`：通过
- Go 格式与 `git diff --check`：通过
- `bun run typecheck`：通过
- `bun run build`：通过
- `bun run lint`：失败；全仓存在大量既有 lint 债务，本次触碰文件中也有错误
- `bun run format:check`：失败；命中本次提交中的 5 个文件

## 审阅发现

### Warning：临时素材没有绑定创建渠道

素材创建固定使用第一个启用的 StarAI 渠道，但 Redis binding 不保存 channel ID；视频提交时又使用实际分发到的渠道凭据验证 upstream asset。多 StarAI 渠道或不同组路由时，素材可能在另一渠道下不可见。建议将 channel ID 写入 binding，并确保使用素材时固定/校验同一渠道。

### Warning：环境变量缺少示例

新增了 `DISABLE_ALL_RATE_LIMIT`、`STARAI_RESULT_RETENTION_HOURS`、`STARAI_ASSET_TTL_HOURS`，但没有同步 `.env` 示例或开发文档。

### Warning：DELETE 未删除上游素材

`DELETE /v1/assets/:id` 和 dashboard/admin 删除接口仅删除 Redis 映射和可选 COS 对象，没有调用 StarAI 上游的 DELETE。若上游要求主动删除，会留下素材直到上游自行过期。

### Warning：主题定制被整体禁用

主题定制 provider 改为只使用默认值，所有 setter/resetter 均为空操作，同时默认隐藏主题开关和配置抽屉。这不是 StarAI 工作流的必要改动，应确认是否为产品决策。

### Info：前端质量门禁未全绿

格式检查失败文件包括主题 provider、渠道 drawer/form、StarAI 计费 registry/section；本次新增的日志详情布局还触发 nested ternary lint。构建与类型检查正常，但正式交付前建议修复这些门禁。

## 运行状态

审阅时 TCP 3000 无监听进程；本次只执行拉取与只读审阅，没有重启后端。
