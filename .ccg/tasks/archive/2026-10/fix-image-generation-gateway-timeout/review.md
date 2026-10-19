# 图片生成网关超时排查

## 根因证据

2026-10-19 UTC 08:09:04 左右开始的4K请求：网关08:10:04报等待本地应用响应头超时，客户端504；应用08:10:38完成，耗时93.3857秒，HTTP200并记录用量。此前2K请求54.2987秒完成200。当前站点root.conf无proxy_read_timeout；应用未配置RELAY_TIMEOUT（代码默认0）。Nginx默认读取等待60s，与观测一致。

官方语义：https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_read_timeout

## 修复范围

新增目标站点的image-generation-timeout.conf，从现有root.conf完整复制全部代理配置，仅调整前缀为/v1/images/并增加proxy_read_timeout600s。完整有效配置已读取：原根location没有额外鉴权/限流/缓存/请求体限制；WAF在http层继承。更长/v1/images/proxy/保留75s和access_log off。所有图片API使用相同等待策略，未扩展到其他API或其他站点；不改应用/数据库/Redis，不触发生成或重试。

## 审查

独立Codex只读审查无Critical；指出401/404仅证明连通性，不能证明Nginx路由；增加独立路由标记及日志检查。提示同级location继承问题：已核对完整root.conf并逐项复制。建议精确路径，评估后采用图片命名空间统一600s策略，专用下载路径仍采用更长前缀，当前完整配置无冲突正则。

按项目要求并行尝试antigravity和Claude，均因codeagent-wrapper不存在返回127；未取得外部双模型审查。

## 验证

旧配置同版本隔离OpenResty测试：模拟65秒延迟，实际60.04秒返回504。后续结果在完成记录中补充。隔离服务仅监听127.0.0.1，不产生模型费用。


## 最终结果

- 旧配置65秒延迟模拟：60.04秒504；新配置：generations65.06秒200，edits65.05秒200。
- 隔离路由标记验证：generations/edits命中新600s配置；proxy命中原下载配置且无访问日志；/api/status命中原根配置。
- 2026-10-19 UTC08:17:13 nginx -t通过后graceful reload成功，nginx -T确认600s生效；根配置、下载配置、文档配置字节未变；所有容器ID未变。
- 公网健康接口200且success=true；生成/编辑无鉴权401；无效图片token404。未新增任何真实生图请求，未声称4K端到端复测通过。
- 服务器备份：/opt/claudeye/production/backups/image-timeout-20261019。回滚为删除新增image-generation-timeout.conf，nginx -t通过后reload。
- 变更仅网关配置与运维说明，无应用源码/契约/数据库变化；不运行无关应用全套测试。配置语法、真实OpenResty延迟回归、路由隔离、日志隔离、公网冒烟及diff检查通过。
