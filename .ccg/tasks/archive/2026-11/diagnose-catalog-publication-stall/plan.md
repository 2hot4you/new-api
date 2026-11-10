# Catalog publication liveness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. CCG user instructions require disjoint-file parallel implementation; they override the skill's single-implementer preference. No worker may spawn or modify another worker's owned files.

**Goal:** 修复无变化同步开启pending及瞬时锁竞争导致发布长期不可用，并保留真实改价的一致性门禁。

**Architecture:** 基于现有catalog reference事务/运行态协议，区分完整指纹相同的周期刷新、本地初始化、真实变更/漂移及pending恢复。候选准备与缓存安装分离，先取得全部必要锁并复核再安装，恢复重试不重放业务修改，币种与FX绑定已发布输入。保留单实例敏感操作限制。

**Tech Stack:** Go/GORM、SQLite/MySQL/PostgreSQL、React/TypeScript/Vitest。

**Spec:** 同任务目录`findings-and-proposal.md`（用户2026-11-10已批准推荐A，仅本地实现，不推送或改线上）。

## Global Constraints

- 不推送、不部署、不连接线上，不重启现有本地或生产服务，不读取/输出生产凭据。
- 保留所有既有dirty/untracked文件；仅精确stage本次自有路径。本地审计提交允许，不合并/推广。
- 所有测试数据库是本次独占、数字loopback地址、临时数据库；不用现有5432用户开发数据库。后续清理只操作已验证本次标签的容器。
- 真正改价提交后失败及ACK不确定继续fail closed，不直接改ready，不重放业务SQL闭包，不削弱单实例/价格/币种/插件验证。
- 目录、币种/FX、别名/插件、路由及pricing派生视图必须保持完整版本；不得删除旧渠道快照防覆盖读屏障，除非建立等效可靠版本证明。
- 跨实例仅验证刷新不关其他实例和旧epoch不混价；不引入或冒称分布式managed发布协议。
- 新测试先观察RED再实现；核心PG normal/race、相关前端/typecheck/lint/build及两路独立审查通过才可完成。

## Review Focus

- 同revision外部DB漂移或非法数据不能被no-op优化放过：增加price/currency/FX/plugin/dependency漂移回归。
- 本地空缓存或ACK不确定不能凭共享ready放行：初始化/最终ACK精确后像矩阵。
- 延迟渠道旧快照不能覆盖新发布：保留现有保护并做可控并发回归。
- 首个全局交换后不得再因某个派生TryLock失败留半投影：逐锁故障注入并核对所有可见缓存。
- 流量、后台刷新、恢复并发必须有进展且退出时无锁/连接泄漏：受控调度及有界取消回归，不靠长sleep随机压力。

## Task 1 — 隔离夹具与确定性RED复现

**Ownership:** 测试worker仅新建 `model/catalog_publication_liveness_test.go`；root负责本地数据库服务、证据记录与验收inventory。不得修改旧测试降低门禁。

- [x] 使用本次Docker标签创建独立PG17/Redis（镜像已存在，不接用户开发数据库），获取随机loopback端口并记录。
- [x] 使用catalogReloadTestDB真实初始化写 no-op reload 测试：已ready时占住pricing锁或观察pending转换，reload不应进入pending/重发目录；观察旧代码明确FAIL。
- [x] 写目录读锁被渠道刷新持有时恢复重试、释放后成功、deadline取消不泄漏、同版本非法漂移保持门禁、首个内存交换前争锁失败不部分更新测试；在旧代码运行并保存真实结果。
- [x] RED通过后通知runtime worker实施，不写生产代码。

## Task 2 — 发布协议、协调与一致性修复

**Ownership:** runtime worker负责 `model/catalog_sync_runtime.go`, `model/catalog_sync.go`, `model/option.go`, `model/channel_cache.go`, `model/pricing_refresh.go`, `model/pricing.go`, `model/task_model_alias.go`, `model/model_billing_money.go`, `relay/helper/catalog_pricing.go`, `main.go`；可新增 `model/catalog_sync_refresh.go` / `model/catalog_sync_diagnostics.go` / `model/catalog_pricing_projection.go` 及自己的 `model/catalog_refresh_*_test.go`。不改Task1、controller/types/frontend及root的验收inventory。

**Interfaces:** 保留RecoverCatalogSyncRuntime/PublishCatalogSyncRevision/WithCatalogPricingReads现有签名和sentinel errors.Is；可增加 `ResolvePublishedCatalogMoneyContext(modelName string) (billingmoney.Context, bool, error)` 在目录read callback内替代root DB货币读取。具体新私有结构由worker依据现有投影选择最小设计，报告确定接口。

- [x] 阅读全部原文件/spec，等Task1 RED证据后才写应用实现。
- [x] 缓存完整发布指纹；只有DB状态ready且本地完整相同输入才no-op；本地初始化不无谓关闭共享ready；检测外部漂移先建立门禁再编译；本地读取未知fail closed。
- [x] 将所有派生可失败构建放在首个全局交换前；先取得必要锁、完整fresh校验，再统一安装。维持原有渠道capture-through-install保护及多key轮询状态。
- [x] 以发布快照冻结币种/FX，并封住跨进程DB状态验证与capture组合窗口。
- [x] 同实例周期任务统一协调，pending恢复优先；可恢复竞争使用30s总界限、20ms起始/500ms上限带抖动退避。退避不持catalog锁/SQL事务/registry pin；不自动重放真实业务提交或未知commit。
- [x] 增加脱敏诊断phase/lock/source/revisions/pending age（未知起点标unknown）/attempt/结果和耗时，保留errors.Is语义；日志不能泄露配置原值、密钥或正文。
- [x] 运行新RED回归和相关旧runtime/reload/startup PG normal/race；报告旧测试预期如需调整，交root审核，不自行弱化契约。

## Task 3 — 定价错误分类与前端

**Ownership:** 错误worker负责 `types/new_api_error.go`（或实际对应错误码文件），新增 `relay/helper/catalog_error.go` 及对应测试，`controller/channel-test.go`, `controller/relay.go`（仅价格错误分类/重试健康相关位置），新增controller目标测试，前端 `web/src/features/channels/components/dialogs/channel-test-dialog.tsx`、其模块 `__tests__/` 和 `web/src/i18n/locales/*.json`。不改Task2的relay/helper/catalog_pricing.go及model文件。

**Interfaces:** 公共临时故障code `catalog_pricing_unavailable`，由errors.Is识别model.ErrCatalogWriterBusy/ErrCatalogPublicationPending，relay503；渠道test保留原外层格式并携带新code。真正缺价仍model_price_error。不要根据英文错误子串分类。

- [x] 先新增后端wrapped-sentinel/API返回及真实前端错误展示测试，记录旧代码RED。
- [x] 仅目录busy/pending映射暂不可用；不触发上游自动重试、渠道禁用或计费，不显示去配价按钮；保留安全详情及既有真正缺价行为。
- [x] i18n覆盖现有全部locale；运行目标测试、typecheck、涉及文件lint及自审。

## Task 4 — 集成回归与完整本地验收（root）

**Ownership:** root负责验收inventory `deploy/tests/test_catalog_postgres_gate.py` / `test_postgres_ci.py`、必要新跨进程与结算测试（与worker先确认路径）、文档/task/spec回馈。不得覆盖worker进行中的文件。

- [x] 双独立进程共享本次PG：no-op刷新不修改共享ready、旧epoch拒绝/快照不混币种；保留单实例managed限制。
- [x] 固定usage的本地mock upstream核对价格选择/预扣/结算/余额/消费日志，不产生付费外部请求。
- [x] 更新新增测试进mandatory inventory，不删除原回归；运行完整 `make test-postgres` / `make test-postgres-race`（含独立包及relaykit/tokenkit全套）、相关三库路径和Python gate测试。
- [x] 前端完整test/typecheck/lint/build。记录所有失败、skip及覆盖边界；修复本次引入的问题，不为绿灯弱化门禁。

## Task 5 — 独立审查、修复闭环与本地交付

- [x] 针对runtime与error两个责任域做独立spec/quality审查，再做全量跨域双路审查；外部双模型wrapper缺失如仍存在，明确以native独立审查替代，不虚报。
- [x] Critical/Important修复后定向重审、重新跑受影响验收；记录review verdict和真实日志路径。
- [x] 仅全部满足后精确本地提交并归档任务；未完成不归档、不宣称修复完成。
- [x] 最终报告本地SHA、RED/GREEN、完整测试/审查结果、限制和部署建议；明确未推送、线上未改。
