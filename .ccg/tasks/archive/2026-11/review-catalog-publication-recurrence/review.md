# 复审结论：仍有复发路径，暂不宜推广

审查应用提交`0d07bc76964937b6c9ca6ef58b0b7ca76ed59a5a`，基线`bec3e330e657c8f783a918ff1f703f7410c75a69`。两位fresh独立native审查分别审计锁/恢复活性与快照/一致性，root交叉核验并补4个临时overlay诊断。外部antigravity/Claude wrapper缺失，不虚报指定模型认证。

原始“完全无变化的周期reload无条件写pending”的路径已修，但不能据此判定不会再卡住。以下4项Important未修，上一轮“无剩余阻断项”的交付判断现被本次证据推翻。没有证明历史12:56的首次触发就是这些路径，也没有执行生产复现。

## I1 / P1 — 发布仍可能无限等待OptionMap锁，30秒预算不是硬截止

代码：`controller/misc.go:45-46,171-175`、`router/api-router.go:32`、`model/catalog_sync_candidate.go:272`、`model/catalog_sync_runtime.go:735-759`、`main.go:224-227`。

可达序列：

1. 活跃`/api/status`处理器获取OptionMap读锁，使用defer直到`c.JSON`实际写完响应才释放。
2. 较大状态响应遇慢读/停止读取的下游，响应写可能阻塞；本服务http.Server未配置WriteTimeout。不是每个小响应都会阻塞，线上代理缓冲/超时未在本轮核查。
3. 真实发布已获得catalog W和5个派生缓存锁、已将本地ready置false，随后candidate或option loader尝试普通OptionMap.Lock。
4. 普通Mutex/RWMutex.Lock不支持context取消；即使恢复预算到期，goroutine仍在锁等待中，W不能释放、scheduler不能进入下一轮恢复。

影响：全局新计价持续busy，直到外部响应解除阻塞。`GetStatus`持锁方式是既存问题，此次恢复的“有界锁竞争”覆盖不完整，不能宣称由本补丁新引入该handler。

同类边界：`model/option.go:339-347`无变化raw FX更新仍会在catalog R中普通Lock OptionMap；会阻塞当前coordinator，虽不直接持catalog W，也可能延迟随后恢复。GetMidjourney有相似代码，但路由已注释，不作为可达证据。

最小修复方向：状态处理器锁内只复制需要的不可变值/配置快照，释放锁后编码/网络写；OptionMap安装锁也应纳入首个交换前的可取消保留集合/统一锁顺序，不能仅靠给流式服务设置粗暴全局写超时。新增真实慢ResponseWriter和取消退出/无半安装回归。

## I2 / P1 — TryLock退避没有writer准入优先，重叠读流量可让恢复长期无进展

代码：`model/catalog_sync.go:71-81`、`model/catalog_sync_runtime.go:573-582,663,855-880`、`model/catalog_sync_refresh.go:21-60,68-96`。

可达序列：捕获真实变化先提交pending，但本地ready仍为true；后续新请求仍TryRLock，持R查询共享状态后才拒绝。恢复只TryLock+随机退避，从不排队，也没有writer-intent/admission gate。若每次尝试时都有至少一个重叠读请求，所有读者各自完成也不意味着存在无R窗口；Try写者无法利用Go RWMutex中等待writer阻止新reader的公平性。30秒结束后，1秒后的恢复仍可能重复同样模式。

影响：有效数据、可用数据库也可能一直pending，需出现流量空隙才获得发布机会。不是每次高QPS都必然发生，不是确定性ABBA；是可达调度下没有恢复进展保证。不是允许“旧价放行”的理由。

最小修复方向：针对真实待发布/恢复建立可取消writer准入预约，阻止持续新的读锁进入，让已有短读退出后完成短发布；不能持W退避，也不能把业务SQL closure重放。补真实变化+持续新请求下的进展/预算/旧请求结算回归，不能只测no-op流量。

## I3 / P2 — 上游“只检测、不改模型”的观察时间仍会重新写pending

代码：`model/catalog_sync_runtime.go:346-357`、`controller/channel_upstream_update.go:581-589,607-613,639-643`。

正常上游模型检查会写UpstreamModelUpdateLastCheckTime，抓取失败也写；没有新增/删除模型时仍持久化OtherSettings。指纹只清除了顶层quota/balance/latency/test/polling计数，随后序列化完整Channel，包含OtherSettings，因此纯观察时间也改变routing/fingerprint。

下一轮恢复会把ready置为同revision、空operation的pending并重建；观察更新若与发布并发，也能使freshness recheck/ACK失效。这个形态与旧故障相同，但单次更新不证明永久卡住；默认300秒检测限频不能被说成天然无限饥饿。它给I1/I2提供了无需真实改价的正常触发入口。

最小修复方向：routing identity使用明确配置投影，排除可证明仅观察的检测时间/待确认列表，保留真实路由、映射、provider行为字段。补实际持久化无变化检查和发布中间观察更新回归，不放宽真实依赖校验。

## I4 / P2 — 过期预读revision误触发失效，健康ready的新版本也会拒绝计价

代码：`model/catalog_sync_runtime.go:681-687,550-555,565-569,862-863`。

恢复在锁外先SELECT revision R；在capture之前，正常普通保存可能已经完整commit/install/ACK R+1。之后恢复拿着R参数进入，直接返回PlanStale，尚未开始完整capture；通用defer却将catalogRuntimeInvalid置位。

此时数据库和运行态原本均是完整ready R+1，新请求仍被拒绝。下一次恢复不能走no-op，需要重新争抢W并完整安装；与I1/I2组合可放大中断。必须区别“旧参数”与“权威数据捕获失败/未知”，不能简单删掉必要的失效保护。

最小修复方向：从同一权威事务读取并使用当前revision，或分类处理已知过期预读；只有完整证明当前local/durable epoch一致才保留读取。补恢复预读R→普通R+1 ACK→恢复capture的可控交错回归。

## 诊断证据及准确边界

仅通过Go overlay增加临时测试文件，未改仓库应用/已有测试，未连接生产或用户数据库。model诊断用现有真实SQLite内存fixture；所有外部DSN及PG-only模式显式unset。没有创建容器、监听公网或调用付费上游。

| 测试 | 实际证明 | 结果 |
| --- | --- | --- |
| TestReviewStatusWriteRetainsOptionMapAndCatalogWriter | 实际GetStatus进入受控阻塞ResponseWriter后保持OptionMap R；public TryWithCatalogWriteBarrier回调在与candidate相同的OptionMap.Lock边界，context取消后仍保持W，释放响应才退出。不是整条真实Recover/真实TCP阻塞端到端测试。 | normal0.879s / race2.764s，exit0 |
| TestReviewRecurringTryWriterHasNoReaderAdmissionPriority | 使用真实retryCatalogRefresh/TryWithCatalogWriteBarrier；每次尝试后让旧reader完成、新reader接替，5个reader完成后writer仍deadline；无单个永久占锁读者。是确定性准入机制证明，不是线上QPS压测。 | normal/race均确认400ms测试预算耗尽 |
| TestReviewStaleExpectedRevisionInvalidatesHealthyPublication | 真实SQLite初始化+普通改价完整ACK；旧expected R调用后共享state未变且ready，本地invalid置位、实际WithCatalogPricingRead拒绝；正常Recover后恢复。 | normal/race exit0 |
| TestReviewUpstreamObservationTimestampReopensPending | 真实SQLite仅更新OtherSettings.last_check_time 100→101；实际Recover写pending一次，revision/digest不变但fingerprint改变。未调用外部上游检查；持久化操作与该生产函数一致。 | normal/race exit0 |

诊断测试的PASS表示成功确认缺陷，不表示缺陷已修。其首次LastCheck测试构建因误用root dto包失败；仅修正临时overlay为实际relaykit/dto后重验，没有修改应用来让诊断通过。最终三项model合并normal/race及status单项normal/race均成功确认。

所有overlay源码、映射和原始日志已从临时目录移至工作树ignored目录`.superpowers/sdd/review-catalog-publication-recurrence/`保留，可独立核验；不进入应用提交。两位审查者随后各自读取测试源码与日志，均支持4项Important（I1/I2为P1、I3/I4为P2）及上述证据边界，没有将锁接缝测试夸大为TCP/生产端到端复现。

此前完整PG普通/race和前端通过是事实，但未覆盖上述组合：十轮流量测试是在预发布完成后只做no-op，单goroutine每10ms顺序读取；读锁deadline测试先人工释放再恢复；取消测试仅覆盖policy/passkey；5个缓存锁测试不含OptionMap。因此不能用其绿色结论反驳新发现。

## 其他既存一致性缺口（只读发现，未动态复现）

- 普通管理保存SelfUseModeEnabled绕过catalog barrier，而倍率缺省逻辑读取普通bool；新测试只覆盖周期loader的changed-policy。位置`model/option.go:492-495,523-528,692-693`、`setting/ratio_setting/model_ratio.go:399-405`。它不证明pending复发，需另补真实管理保存/冻结输入一致性回归。
- 插件热更新独立替换registry generation，请求pin新generation，定价读取只比较durable/local fingerprint，未比请求generation与已ACK generation。位置`controller/task_plugin.go:844`、`middleware/task_plugin.go:352,428,507`、`model/catalog_sync_runtime.go:874-878`。现有schema/表达式保护仍有效，不能据此断言少扣费；但不能笼统承诺插件与价格始终同代。

## 未判定/不扩大保证

- 不根据本次代码机制认定历史首次触发/持锁者；没有现场goroutine栈。
- 非法价格、真实依赖不一致、数据库不可用或unknown commit后拒绝读取是必要安全性，不一概算活性缺陷。
- 不承诺任意直接SQL修改即时发现/自动修复、分布式managed发布或混旧新二进制共库。编译默认行为未全纳入共享fingerprint仍是已声明部署边界。
- 没有证实新的确定性锁顺序反转、死锁或本次新增直接错币种/汇率扣费；不能由暂未找到推出绝无此类问题。

## 处理建议

暂不推广0d07bc769，先修I1–I4后再做针对性RED/GREEN、双路复审和完整限定验收；策略/插件缺口需明确闭环或单独裁定边界。本轮用户只请求复审，未修改实现、推送、部署、重启或写生产状态。
