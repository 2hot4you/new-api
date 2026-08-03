# Review

## 结果

- 渠道类型 61 的前后端显示名称统一为 `Molii AIGC`。
- 渠道管理、任务日志、素材上传、计费设置、系统任务、Docker 元数据及对外错误消息不再展示上游品牌。
- `/v1/models` 中两个 Seedance 模型的 `owned_by` 改为 `molii-aigc`。
- 对上游返回的错误消息和任务结果增加品牌替换，避免原始上游文案透传给用户。
- 七种语言资源同步完成。

## 兼容性

- 保留渠道类型值 61、`ChannelTypeStarAI`、适配器目录、数据库/Redis key、计费设置键和系统任务持久化类型。
- 保留既有 `starai_api_error`、`starai_task_failed`、`starai_asset_upstream_error` 机器错误码，避免破坏已发布 API 契约；这些标识不作为界面品牌展示。
- 未修改模型 ID、路由、上游请求格式、鉴权和计费逻辑。

## 验证

- `go test ./...` 通过；最后修改后再次运行 `go test ./controller ./relay/channel/task/starai ./service` 通过。
- `npm run typecheck`、`npm run build` 通过。
- 渠道配置 Bun 单测 3/3 通过。
- 本次修改文件的定向 Oxc lint 和格式检查通过（两个原本已不符合全仓格式基线的计费文件仅修改字符串，未扩大格式差异）。
- 生产代码字符串扫描未发现用户可见的精确 `StarAI` 品牌文案。
- 全仓前端格式检查仍有 3 个与本任务无关的既有文件失败；全仓 lint 也存在大量既有问题，本任务未扩展处理范围。
