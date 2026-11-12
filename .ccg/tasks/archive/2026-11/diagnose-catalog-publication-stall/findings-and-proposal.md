# 定价目录发布停滞：证据与修复提案（已批准，保留初轮记录）

本文保留批准前的诊断、故障版本代码行号及原始方案，用户随后已批准仅本地实施。当前实现与测试状态以plan.md、review.md、verification.md及checkpoint.md为准，不将历史“未实施”段落当作当前状态。

## 结论与边界

已确认代码设计缺陷：正常周期选项重载复用了“未就绪目录恢复”，即便同版本且已就绪也提交全局 pending；随后两阶段发布依赖多处一次性 TryLock。短暂竞争可升级成持久拒绝新定价读取，失败后只等下一次固定周期再次尝试。渠道缓存刷新又会占用相同目录锁，缺少独立恢复调度和协调。

这是已证实的可致故障机制，不等于已经还原本次 12:56 的首个触发请求和持锁者。现有证据没有首次 ready→pending 日志、goroutine 栈、具体锁名或持锁时长，不能把周期碰撞认定为唯一现场根因。不能因 DB 无阻塞快照就断言过去没有数据库等待。

本轮没有连接任何线上环境，没有改应用代码、生产数据库、配置或服务。目标是用户新指定的腾讯容器，不能沿用历史 LA 容器。

## 证据清单

故障代码：d92203da2101ff90ebb45dcef83a4f9045232da5。当前审查工作树 HEAD 为 bec3e330e657c8f783a918ff1f703f7410c75a69；对用户列出的九个核心文件以及 reference transaction、candidate、实例资格和币种读取文件逐项 git diff，均与故障版本相同。

用户文件（未改动）：

- `db-inspection-second.json` SHA256 `57d296c2261d6f199fc0842ffa717bd305ef84d9aee0bd9f2fd9d4df4b584430`
- `runtime-evidence.txt` SHA256 `322a7939d830dde537ecaf12b3bff09b8896763a0ed50dfb09180c87624e9930`
- `db-after-restart.json` SHA256 `479c1a845a24fc5b7de5f0ef6feb30d9f926648abfd49a898e871c34c3e5f161`

根目录：`/Users/naf/Documents/Codex/Projects/price/evidence/catalog-stall-20261110/`。

| 事项 | 证据与结论 |
| --- | --- |
| 故障时状态 | revision=runtime_revision=235，pending，空 operation，baseline_generation=0；operations=[]。不能据此证明是哪次业务写入，也不能证明内存缓存一致。 |
| 重启后 | 仍235/235但ready；结合用户重启操作，说明启动恢复有成功路径，不证明活进程恢复具备活性。 |
| 重复故障窗口 | 所供220行日志覆盖13:27:19–13:43:34；17轮options busy，17轮channels周期，152条请求pending错误、17条reconciliation pending警告。12:56首次触发不在该文件内。 |
| 周期相位 | 前几轮每分钟:33，后移到:34；13–16、165–168、217–220行都呈现channel开始→options busy→reconciliation pending→channel结束。秒级相邻不是goroutine级因果证明。 |
| DB锁 | 快照sessions idle且blockers为空、locks=[]，只排除采样时显示出的DB阻塞；不能观测进程内mutex，也不能排除池内等连接。 |
| 表达式存在 | 用户提供的已确认事实；三个证据文件不含完整报价。请求在目录门禁被拒绝，不必然意味着表达式缺失。 |

## 执行时序与具体位置

1. `main.go:76,120` 启动独立渠道/选项同步循环，两者使用 common.SyncFrequency。`channel_cache.go:166-171` 与 `option.go:312-318` 都 sleep 后执行；没有共同调度器。
2. `option.go:244-247` 每轮 `loadOptionsFromDatabase` 都调用 `recoverCatalogSyncRuntime`。`catalog_sync_runtime.go:468-476` 只读出revision，未判断本地是否已加载相同内容。
3. `catalog_sync_runtime.go:419-440` 取得目录写屏障和reference事务；`:427-435` 对ready无条件写pending（revision不变），提交。捕获失败也刻意保留pending。
4. 释放目录锁、SQL事务和registry pin后，`:457` detached编译候选。这一点已有正确隔离，但此时全局门禁已关闭。
5. `:461` 再次单次Try取得目录写锁。失败立即返回，不撤销已提交pending，也没有短间隔有界恢复循环。
6. `:489-498` 复核完整输入；`:502` 本地ready=false；`:503-518` 依次发布普通选项、typed价格、渠道缓存、别名、pricing投影；每步仍可能出错。
7. `:520-538` 最终事务再次验证，并写runtime_revision/ready。确认失败不回滚已发布内存。未知COMMIT仅做精确后像查询（`:539-559`），不是重放业务写。
8. 只有`:563`恢复本地ready=true。`WithCatalogPricingReads`在`:594-610`要求本地与共享DB状态匹配，否则拒绝新选择。

### 从请求错误前缀进一步缩小范围

所供请求错误均有 `catalog pricing selection:` 前缀。该前缀在 `catalog_sync_runtime.go:679` 产生；`:594-595` 的本地未ready拒绝没有此前缀。因此，对这些采样请求所在进程，可以确认读取时本地ready=true且获得了catalog读锁，随后DB状态事务拒绝了请求。

这与“在pending已落库之后、进入`:502`之前失败”相容，尤其是第二次catalog TryLock失败；也与其他实例关闭共享门禁相容。它不支持直接把这些采样请求归结为该进程已进入`:502`后缓存TryLock失败。不能凭这些错误定位每条同步busy的具体阶段。

### 首次进入pending能否确定？

不能确定首个触发者。代码存在以下入口：

- 启动 `CompleteOptionMapInitialization` 与周期 `loadOptionsFromDatabase` 均调用恢复；ready同版本会重写pending。
- ordinary mutation 在内容digest改变时先revision++再pending（`:148-159`），不创建managed operation。
- managed apply持久化operation及pending（本次空表不支持当前处于此managed流程）。

235/235空operation与同版本reload直接匹配；真实ordinary变更后曾成功ack再遭reload也可能产生相同状态。没有首次事件记录，不能锁定用户请求、管理员写入或后台某轮。

## 锁与等待审计

| 返回同一 busy 的位置 | 实际锁/模式 | 可能竞争者 |
| --- | --- | --- |
| `catalog_sync.go:23` | catalogBarrier.TryLock，prepare或publish | 请求短读、跨SQL的channel刷新读、普通元数据/目录写 |
| `catalog_sync_runtime.go:587` | catalogBarrier.TryRLock（请求端） | 写者已持锁或排队；名字writer busy并不证明持锁者身份 |
| `channel_cache.go:135` | channelSyncLock.TryLock | 渠道路由/缓存读写 |
| `task_model_alias.go:84` | taskAliasRebuildMu.TryLock | TTL/普通渠道刷新重建 |
| `pricing_refresh.go:33` | updatePricingLock.TryLock | GetPricing、InvalidatePricingCache |
| `pricing_refresh.go:42` | modelSupportEndpointsLock.TryLock | endpoint读取 |
| `pricing.go:209` | channelSyncLock.TryRLock | Advanced Custom缓存读与渠道缓存写 |
| `pricing.go:686` | modelEnableGroupsLock.TryLock | 模型分组/额度类型读取 |

现有TryLock均即时失败；30秒外层context不等于等待30秒。普通 `WithCatalogWriteBarrier` 使用不可取消的blocking Lock。

渠道普通刷新 `channel_cache.go:52` 从SQL读取到缓存安装持续持catalog R；释放后`:56` reconciliation尝试catalog W，遇pending退出。失败的reconciliation本身不会完成恢复。此循环可与两次Try W反复撞窗。

已审路径中没有找到确定的递归目录锁或必然ABBA环：

- pricing持锁时可取channel读锁，代码规定 updatePricingLock→channelSyncLock。
- 渠道刷新在channel锁释放后才InvalidatePricingCache，避免反序。
- RefreshPricing的guarded内部不再递归取catalog锁。
- 渠道刷新读锁释放后再进行writer reconciliation。

连续的 `channels synced` 说明这些轮次仍能完成相关catalog、pricing、alias锁路径，不像这些锁始终被同一死锁永久占住；不能排除未覆盖路径或短暂长等待。

`catalog_sync_references.go:72-76` 的pool.Conn及事务受15秒context约束，但等待期间外层可能仍持catalog W；普通锁等待没有该取消保证。现有材料没有pool wait计数或栈，池耗尽/历史DB瞬时阻塞仍未证实。不能把所有busy理解成PostgreSQL行锁。

## 部分发布及跨实例

部分发布是代码明确允许的失败状态：typed maps先交换，再重建可失败的派生缓存，再最终ack。`:502`之后受保护的新计费选择会fail closed，这是正确安全性，不能移除。独立pricing展示读取未必都受同一外层屏障保护，因此还需消除半更新投影，不能只处理DB状态。

catalogBarrier和catalogRuntime均为进程内。共享DB的一个进程即使只做同版本reload，也会把其他进程的新定价选择关门；DB ready/runtime_revision不是所有进程的ack。

`catalog_sync_startup.go:14-28` 明确将敏感managed同步限制为显式健康单实例，注释说明它不是分布式运行态发布；ordinary刷新并非因此自动具备多实例原子性。

另有读一致性窗口：状态事务在 `catalog_sync_runtime.go:677` 结束后才`:681`执行capture；`relay/helper/catalog_pricing.go:110`又从root DB读币种。其他实例可在两者之间提交新币种，旧内存价与新币种可能混用。这是读码发现的独立风险，不是本次已证实发生的扣费错误。后续方案必须将币种/汇率与价格绑定同一候选快照，不能简单“revision相等就跳过”。

## 渠道测试为什么误报没配价格

`controller/channel-test.go:431-436` 将所有ModelPriceHelper错误统归 `model_price_error`/400。前端 `channel-test-dialog.tsx:1236-1244` 与`:259-263`对这个code一律替换摘要为未配置价格，并显示去设置按钮。

因此目录暂不可用会伪装成缺报价，前端原始details保留也不能纠正摘要。应该在错误产生/传播处保留typed原因，而不是在前端匹配英文字符串。

## 修复选项（推荐A，待用户确认）

**A：在现有协议内做最小闭环修复。** no-op判定、本地统一发布协调、有界恢复、完整候选/一致性读、typed错误及日志一起完成。变更面不止一处if，但不引入新的分布式协议。

**B：完整分布式发布/多实例ack协议。** 独立epoch/lease、节点加载确认及隔离策略，成本和迁移风险更大。当前证据并未确认腾讯环境多实例，不能以本批修复顺带宣称支持。

不建议只加TryLock重试或只调整周期：仍会无意义地关闭全局门禁，且不能解决部分发布。

### A的安全边界与实现方向

1. **拆分刷新意图。** 普通非目录设置reload、目录变化检测、pending恢复、首次本地初始化分别处理。记录已发布完整指纹：DB/incarnation、revision/digest、价格options、FX、插件generation及有效依赖、币种/元数据、影响路由/投影的渠道和abilities版本。无变化必须经一致性读取及最终复核证明，不能仅比较235==235。现有authoritative reload允许同版本外部DB漂移，必须识别；变化或未知仍fail closed。
2. **无变化刷新不写pending。** 已ready且完整指纹一致，不重编译/重发目录，只更新经过验证的非目录选项。非目录无关项错误不应人为把健康目录变成全局pending；相关插件开关、FX等不得当无关项。启动本地未ready不能误走快速通道。
3. **先构建完整候选，但区分变更来源。** 价格、币种/FX、别名兼容性、渠道/abilities和pricing/endpoint/group投影尽量从同一捕获输入离锁构建。正常受控改价在业务提交前构建候选、业务提交时建立pending；已经pending的恢复始终保留门禁。若检测到同revision的外部DB内容漂移，因新内容早已落库，必须先可靠建立/保留共享pending，再进入可能失败的解析编译；不能编译失败就退出且让其他节点继续相信旧ready。若读取本身失败而未能证实漂移，至少本地拒绝新选择并报警，不凭未知状态伪造DB成功。只有完整证明no-op才不动共享ready。当前stage只覆盖一部分，后续还在写屏障内查DB、改全局缓存，要改为可失败准备与不可失败安装分离。
4. **统一本地协调。** 周期刷新、恢复、渠道刷新/元数据reconciliation进入同一coordinator（合并重复唤醒）；pending恢复优先，避免独立循环相互抢锁。普通管理写与最终commit遵循同一锁顺序。快照捕获/构建不持长catalog W；渠道轮询索引等易变状态必须保留，不能用静态快照覆盖。渠道/abilities普通更新不一定推进catalog revision，因此必须建立覆盖全部相关写入、与最终校验/安装串行的可靠路由版本token，或在最终边界完整重捕获比较。该保护和“旧快照延迟安装不能覆盖新发布”的确定性回归建立前，禁止移除现有channel capture-through-install目录读屏障；本地coordinator本身不足以替代它。
5. **短安装与双重校验。** 最终短写屏障内重查revision、digest、incarnation、registry/dependencies及可靠路由版本；陈旧则丢弃重建。真实变化按上项的来源规则进入/保留pending。首个全局交换前必须确认完整安装对象及必要锁均已准备妥当，或使用真正单指针bundle并迁移全部相关读者；不能先换typed maps再逐个Try派生锁。不可失败的安装发布同一版本bundle，所有计价/钱上下文读取绑定该bundle；展示/路由派生视图也不能看到半安装。最终DB ack仍要确认，未知提交查精确后像，不重放业务写。ack不明时禁止新选择，即使内存交换完成。
6. **有界、可取消恢复。** 建议总deadline沿用30秒，指数退避起始20ms、上限500ms、带随机抖动；参数是提案，不是已测性能保证。竞争失败释放所有已取得锁/事务再等待；一次只进行一个恢复。只可重试幂等重建/复核和明确可恢复的锁竞争，不能重放SQL业务闭包、提交未知的修改或虚构ready。deadline用尽保持真实变更门禁并报警，由独立恢复worker后续重试，不等待同相位的60秒循环；worker外层也需有上限退避、合并唤醒、防重试风暴。
7. **多进程边界。** 本地加载未就绪不应仅因本地refresh而更改共享ready；pending真实变化仍由共享协议拒绝旧价。请求使用单一已发布snapshot中的币种/FX/价格，并以DB版本证明是否可选；状态校验到capture窗口必须通过同一DB快照或不可变epoch输入封住。双进程集成测试必须证明一个节点refresh不关另一节点、旧节点不混价；保留managed单实例限制。若要求真正多实例热发布可用性及节点ack，转B另行审批。
8. **错误分类。** missing quote与catalog unavailable分开typed code。relay的busy/pending建议503 +稳定暂不可用code；渠道测试既有外层JSON/HTTP兼容格式单独保留，返回结构化子code。前端仅missing quote显示去配价；busy/pending显示“定价服务暂不可用，请稍后重试”，详情含安全request ID。不自动重试用户生成请求，不自动关闭渠道。
9. **诊断。** 统一emit source(trigger)、instance、phase(capture/stage/acquire/recheck/install/ack)、lock、target/local/db revision、pending age、attempt、wait/hold/phase耗时、结果和安全错误类别。跟踪锁的逻辑operation owner而非虚构goroutine ID。记录DB pool Stats的wait增量及context timeout类别。跨重启pending age需要可信持久进入时间/审计事件；旧记录无起点时标unknown，不把first observed冒充实际起点。不得输出options原值、密钥、渠道key、请求正文。重复busy限频但保留计数和最终恢复结果。

## 实施前必须新增的测试及验收

以下是待实施门禁，不是已通过结果：

| 场景 | 必须断言的可见结果 |
| --- | --- |
| 同版本no-op多轮 | revision、digest不变；无pending转换；定价读取/请求持续可用；非目录项正常刷新 |
| 同版本DB漂移 | 价格、币种、FX、元数据、插件或路由变化均不误判no-op；非法变化拒绝，不使用陈旧组合 |
| 渠道/恢复并发 | 用channels/barriers确定性控制capture和安装窗口；两次catalog竞争及各内部锁竞争均标明原因，释放竞争后在deadline内恢复 |
| 持续读流量及多轮同步 | 固定输入、可控scheduler/clock，多次相位重合；无忙循环/饥饿；不依赖sleep碰运气 |
| 首次启动及重复恢复 | DB ready但本地空缓存时先完整加载；本地ready前不接计价；重复恢复幂等 |
| 真实改价 | 旧请求冻结旧selection可正常结算；新请求要么完整新价格/币种/FX，要么暂不可用；绝不旧价放行 |
| 分阶段故障 | capture/stage/安装前、各派生缓存、ack SQL分别注入；ACK COMMIT实际成功/失败×后像查询成功/失败逐项覆盖，只有完整epoch后像匹配（managed还须operation ID/状态/不可变绑定匹配）才可ready；未知保持门禁、不重放业务写 |
| 连接池/DB/registry竞争 | 受控池耗尽及锁等待可取消、资源释放，诊断正确，外层writer不无限占用 |
| 双独立进程共享PG | 一个进程无变化刷新不改变全局ready；真实pending阻止旧价；状态检查与货币读取间插入并发提交不能混价；不宣称managed多实例支持 |
| 渠道测试UI/API | 缺价与busy/pending的code、文案、去设置按钮不同；既有返回格式兼容 |
| 计费端到端 | 本地确定性mock upstream返回固定usage；按已知tiered_expr、币种/汇率核对预扣、最终用户/token余额、退款及consume log，避免真实付费上游 |

运行门禁：一次性本地PG的 `make test-postgres` / `make test-postgres-race`，相关三数据库路径、前端目标测试/类型检查/构建及双独立复审。缺fixture和required skip不能计为通过。功能改动不触及表达式语义和历史报价兼容约束。

### 本轮实际本地验证（仅旧代码）

运行了现有5个测试选择，明确unset生产/测试DSN，TestMain使用内存SQLite：

- `TestCatalogRuntimeReadyRecoveryFailureBlocksWrites` PASS，单独 GOWORK=off 再验。
- CacheContention、AdvancedCacheContention、GroupIndexContention、LostAcknowledgement的SQLite分支在 `runtimePendingOperation` 创建敏感managed plan时因 `catalog sync requires an explicitly configured healthy single instance` 失败，未进入竞争目标；PG/MySQL缺DSN跳过。故整条命令FAIL，不能称为锁机制已复现、完整验收通过或需要弱化资格门禁。
- 本轮未新建测试数据库服务、未创建复现代码；批准方案后优先用正确隔离PG夹具做RED回归。

## 部署与回滚（提案，未授权执行）

1. 先正确隔离PG复现和故障注入，再实现；develop完整CI与真实开发环境验收通过才考虑main。新任务不能沿用历史发布授权自动推送。
2. 发布前只读核对腾讯目标身份、当前镜像SHA、实例数和共享DB关系；不能把本次腾讯容器当作历史LA容器。明确目标、备份/恢复验证、维护窗口和停止条件后再请求部署批准；排除aigc和无关环境。
3. 建议最小方案不做破坏性schema变更；持久诊断时间字段如必需只能增量兼容迁移，须先三库验证。先开发/隔离环境灰度，生产是否可多实例滚动须取决于实例语义，不预设。
4. 生产验收需另行批准安全测试账号及限定扣费测试；无授权则仅隔离mock完成账务验收，不能声称生产实际扣费已验。观察至少10轮完整同步，覆盖无变更和一次受控真实变更；核对每轮请求、计价、账务及恢复指标，不只ready。
5. 回滚固定到发布前核实并保留的镜像digest，不猜当前部署仍为d922；停后续推广，不回滚用户业务价格或直接改ready。DB真实变更已提交但发布失败时用正常恢复重建；回滚旧代码可能重新引入同版本pending缺陷，不能把重启成功当根治。提交不确定先只读查精确状态，不盲重复部署/迁移。

下一步：请用户确认推荐A及其单实例/跨实例安全边界；随后写正式设计与逐步TDD实施计划，本轮不实施、不归档未完成修复任务。
