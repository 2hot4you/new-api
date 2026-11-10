# 根因与官方latest对照 — 2026-11-10

## 结论

本次同revision、空operation的catalog pending/busy故障链来自定制分支新增的目录发布/恢复协议。官方当前latest没有这套状态机，不能说“官方也有同一bug”或“官方已修复这套协议”；但官方仍保留GetStatus跨网络响应持OptionMap读锁的通用风险，不能宣称官方没有任何并发卡顿问题。

## 定制分支的设计原因和来源

- `d9dccd930e0d147228f5677bc82e8cf77d6223a6`，2026-10-09，新增持久化目录协调状态/计划。
- `dcb26873ea387933952a99ff983a1e5c476c550d`，同日，新增runtime发布/恢复，已具备ready→pending→释放W→构建→重新TryW的两阶段结构。
- `908583c517c93eda9c73247c5633400bb356010f`，2026-10-10，将普通权威选项刷新接入恢复协议。
- 故障源d922的`model/option.go:244-247`周期调用recover；`model/catalog_sync_runtime.go:427-434`同版本ready也无条件持久pending，`:461`第二次TryW失败立即退出；渠道/选项两个同频率loop相互竞争。于是本来短暂的刷新竞争可能留下持久门禁，后续新定价选择拒绝；channel-test又把目录不可用统归缺价400。

设计意图是防止未完整发布的价格被使用，安全门禁本身不能删除。缺陷是把无变化刷新也纳入关停，且关闭后的恢复推进不足、发布锁和缓存锁协调不完整。没有证据据此确定历史首个持锁者、第一次ready→pending由哪轮任务触发，或认定历史云数据库一定无瞬时等待。

当前本地0d07的复审4项并非都能倒推为原事故原因：新fingerprint中的观察时间和stale-revision invalid latch属于本地修复新增路径；OptionMap和TryW活性问题是剩余组合风险。本地修复尚不宜推广。

## 官方源与镜像身份

实际公开API/网页核对：

- [官方latest Release](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.49)：`v1.0.0-rc.49`，2026-11-08T05:06:38Z发布。
- tag精确SHA：`48a6c87f9e7757fdc58e5be9bcb4f8281ad0dfd0`。
- 本次官方main：`bd6bb53019fb7e21804b31ab7365c9b2097f4b6c`；比release多2commit，完整tree同样无catalog_sync/catalogmanifest。
- Docker Hub公开tag API：`calciumion/new-api:latest`与`:v1.0.0-rc.49`清单digest实际相同：`sha256:92dbb9b34b2693bf5d4721ab55f83de78b0724e5b3aa780cd485dcbd7c2818fe`；amd64/arm64子镜像digest也各自相同。只核registry元数据，没有拉取/执行镜像，不代表已核线上实际容器。

## 官方实际代码

- 下载并解包固定tag完整源码；完整tree `truncated=false`。全Go源码搜索未找到`catalog_sync_states`、`committed_pending_publish`、`RecoverCatalogSyncRuntime`、`WithCatalogPricingReads`及本分支两类catalog错误。
- [model/option.go:201-231](https://github.com/QuantumNous/new-api/blob/v1.0.0-rc.49/model/option.go#L201)：周期刷新直接读options并逐项更新，没有先pending、完整重建和最终ACK的全局目录协议。
- `relay/helper/price.go:65-120`读取ratio/billing getters，`relay/request_billing.go:59-67`把真实取价错误转model_price_error；不能把同名错误码与定制门禁误报混为一谈。
- [controller/misc.go:45-46,171-175](https://github.com/QuantumNous/new-api/blob/v1.0.0-rc.49/controller/misc.go#L45)仍在OptionMap R期间c.JSON；`model/option.go:380-381`更新等待该写锁；`main.go:205-208`无WriteTimeout。这是相同底层锁作用域隐患，但官方没有外层catalog W/持久pending门禁，不能推导出完全相同的全局拒绝故障链。

## 验证边界、交付与证据

- 两路独立native分别做本地git溯源与官方完整源码核验，均确认上述区别；指定外部wrapper仍缺失，未虚报antigravity/Claude。
- 未运行官方软件/测试、未复现官方真实慢TCP响应、未审计官方全部并发路径；结论是固定源码机制对照，不是全功能安全认证。
- 不建议凭“官方没有此状态机”直接用官方镜像覆盖现有定制数据库；自定义价格/币种/插件与数据兼容仍须另行评估。未获得或执行升级授权。
- 证据保留工作树ignored `.superpowers/sdd/compare-catalog-stall-official-latest/`：固定release完整源码/tar、release-meta.json/release-tree.json、Docker两tag原始JSON。
- tar SHA256：`84eeed2085fbc90266f74898b601b4303707180d8dde7944d5e61342c3a3b30d`；官方option.go `07c3b32cdbf9c6bbe7804f12808f12f0c65812f10344df039725cf5640aac831`；misc.go `6f459416027b915f10a120e8e03519637a432e6a5cc40ab1d2c229d281085e99`。
- 仅公开官方代码/registry查询及本地文档写入；没有连接业务环境、改应用实现/生产状态、重启、推送、合并或部署。用户无关修改保留。
