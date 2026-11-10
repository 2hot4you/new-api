# Catalog Recovery Four Gaps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. 用户CCG明确要求按文件归属并行，覆盖技能默认单implementer；核心共享model文件仍只有一个owner。不得spawn子代理、stage或commit。

**Goal:** 本地消除4项复发路径，证明真实改价/恢复在受控竞争下有界推进，同时保留完整报价和ACK安全保护。

**Architecture:** 保留现有目录事务与RWMutex协议。恢复真正需要W时使用短生命周期writer-intent，读者准入前后核验并保留blocking读语义；完整安装预留OptionMap与全部缓存锁，使用显式locked内部函数避免递归。仅路由配置投影去除3个已知观察字段；预读目标过期与实际捕获/提交失败区别处理。

**Tech Stack:** Go1.26/Gin/GORM，真实SQLite/MySQL8/PG17，本地Redis与mock LLM，无新依赖/数据库迁移。

**Spec:** 同目录requirements.md及已批准上轮review-catalog-publication-recurrence/recommendation.md。用户已明确执行，本计划细化边界不扩权，不再重复停等批准。

## Global Constraints

- 不推送、不合并、不部署，不连接任何生产/用户既有数据库或Aigc；不修改线上配置/数据、不重启既有服务、不付费生成。
- 仅本次标签numeric-loopback临时数据库；完整证据保留到本任务ignored工作区，不能覆盖旧任务证据。
- 真实改价或unknown ACK失败继续拒绝新选择，不force-ready、不重放业务SQL、不放宽价格/插件/币种/单实例保护。
- 不新增全局HTTP写超时、不改LLM/图片/视频超时或数值定价；不改认证验证/信任规则或插件协议。
- 先正式RED后实现；保留原测试和mandatory inventory，不通过skip、吞错或无限重试取得绿灯。
- 用户dirty删除、重复名草稿和其他工作树不stage/覆盖；workers不commit，由root精确本地提交。

## Review Focus

- 取消发生在writer-intent/OptionMap获取前、中、后：退出无锁/预约泄漏，后续正常读写仍可用。
- loader的passkey/policy/config helper不能重入已保留OptionMap；任何失败不得在首个价格交换后才争锁。
- 无变化周期和纯观察字段更新在pricing锁被占时仍可读，不写pending；真实未知/provider/Models变化不可被剔除。
- 旧目标revision只返回stale，不清除既存invalid，不掩盖joined rollback/unknown commit；真正捕获失败仍拒绝。
- 持续请求期间真实发布有进展，在途旧选择正确结算、新选择完整新版本或503；不是仅测试无变化流量。

## Task 1 — 状态响应脱离OptionMap锁

**Files / ownership:** status worker只能改`controller/misc.go`、`controller/misc_oidc_test.go`。
**Interfaces:** 保持`GetStatus(*gin.Context)`、全部JSON字段/类型、状态码和已有header语义。网络写前释放OptionMap，不提供新对外API。

- [x] 在原status测试文件追加受控慢ResponseWriter回归，断言实际GetStatus写期间OptionMap可写、响应旧快照保持完整；观察当前代码RED并留日志。
- [x] 完整envelope在R内common.Marshal为自有bytes，释放R后再沿用Gin JSON渲染（如common.RawMessage）；禁止c.Data无意增加Content-Length，禁止浅复制留下live map。
- [x] 编码错误也先释放R再处理；保持原OIDC/status字段回归，运行目标normal/race，写task-1-report.md。

## Task 2 — 核心恢复4项缺口

**Files / ownership:** core worker可改`model/catalog_sync.go`、`model/catalog_sync_runtime.go`、`model/catalog_sync_refresh.go`、`model/catalog_sync_diagnostics.go`、`model/catalog_sync_candidate.go`、`model/option.go`、`model/passkey_option.go`、`model/request_policy.go`；测试扩展现有`model/catalog_publication_liveness_test.go`及必要`model/catalog_refresh_projection_test.go`。后来明确批准补`model/catalog_sync_snapshot.go`的ExportManagedCatalog读准入漏点；最终兼容修复wave临时移交一个root门禁文件，仅同步本次新增matrix测试，之后已交还。不得修改Task1或其他root文件，禁止并发重叠写入。
**Interfaces:** 原公开函数签名/错误sentinel保留。内部新增`withCatalogRecoveryWriter(ctx context.Context, write func() error) error`负责intent+可取消W获取+一次callback；locked OptionMap内部入口只供明确已持锁者使用，普通wrapper保留。

- [x] 先在真实PG fixture将4项临时坏行为诊断转为正确行为断言：OptionMap竞争取消后完整旧投影不变且锁释放；真实pending+连续新短读时writer进展；只观察更新时间不pending/fingerprint变化；普通R+1 ACK后旧R仍可读但返回stale。每项留当前源RED。
- [x] writer-intent用counter而非bool，仅包真实最终W获取/发布，不能跨自身R捕获或detached构建；所有live读入口准入前及取R后检查，blocking读仍等待，context读可取消。普通blocking W公平性不变。无变化不进入intent/W。
- [x] OptionMap加入全部安装预约，任何失败释放所有锁后再退避；loader及candidate的已持锁路径不再次加锁。适配policy snapshot提取与passkey域批次的locked内部方法，仅锁契约变更，业务校验/认证语义不变；optionsOnly路径同样可取消，不漏rawFX。
- [x] 路由Channel副本对OtherSettings仅去除`upstream_model_update_last_check_time`、`upstream_model_update_last_detected_models`、`upstream_model_update_last_removed_models`。空/null/观察-only统一空表示；保留unknown raw JSON并无损canonical/重复键检查，非法/非object拒绝，不修改原始input.channels或DB。
- [x] 恢复内部同一权威捕获取当前revision，或精确分类旧expected参数；显式Publish旧目标保持stale，不新增invalid、不清理已有invalid；capture/cleanup/unknowncommit错误仍失效。
- [x] 回归包含3观察字段及首次空→观察、preinstall/ACK期间观察更新；provider/ignored/unknown高精度JSON/Models真实变化仍重建，坏JSON不ACK。取消/新读准入/旧冻结价格/实际DB后像断言不能变成log-only。
- [x] 跑目标normal/race和原reload/startup/public-refresh/liveness/contracts，写task-2-report.md，说明额外文件/架构风险先向root报告，不能自扩协议。

## Task 3 — 集成及真实数据库验收

**Files / ownership:** root扩展`controller/catalog_publication_billing_test.go`、`deploy/tests/test_catalog_postgres_gate.py`、`deploy/tests/test_postgres_ci.py`及task文档。不得碰运行中的worker文件。
**Interfaces:** 只用原公开恢复/选价/普通改价API和真实本地mock，无生产hook。

- [x] 固定usage的真实本地LLM请求，在流量和后台同步中受控改价/FX及故障恢复，核预扣、冻结选择、新价格、结算余额和consume log；慢status已由Task1/核心锁回归分别验证，必要时集成同一公开流水线。
- [x] 把新增回归加入mandatory，原53组/599 top-level保留。schema无变化；受影响同步/事务路径跑真实SQLite、MySQL8、PG17；不谎称旧全仓三库套件通过。
- [x] 两worker源冻结且定向/审查无阻断后，完整`make test-postgres`与`make test-postgres-race`，Go vet/build、Python gate、前端全套/类型/构建与lint基线对比。原Bun脚本兼容/lint债务单列，不顺手改工具链。
- [x] SelfUse/plugin热更附注只做范围核验并明示，不将4项完成外推为分布式/跨二进制/整个插件发布问题全部解决。

## Task 4 — 独立审查与本地交付

- [x] 每任务spec/quality审查，最终两路独立全差异审查；实际外部wrapper缺失以native替代并明示。
- [x] 所有本次Critical/Important有真实修复/回归/定向复审闭环，之后最终固定源完整门禁；未完成不归档或冒称完成。
- [x] 精确本地代码提交、更新verification/review/checkpoint、沉淀spec并归档本任务本地审计提交，绝不push。只清理本次验证标签服务，保留用户worktree及原始日志。
- [x] 最终报告4项、真实SHA/测试/审查/剩余边界；用户已选择保留本地分支，不合并或部署。
