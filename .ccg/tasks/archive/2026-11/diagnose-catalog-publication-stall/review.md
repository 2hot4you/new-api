# 独立交叉分析与提案复审

本机缺少 `/Users/naf/.claude/bin/codeagent-wrapper`。已告知用户，未声称调用了antigravity或Claude；替代为两个独立只读分析代理。

1. `catalog_runtime_rootcause`：runtime/option/reference transaction、一致性和跨实例。
2. `catalog_channel_locks`：channel/pricing/alias锁时序、周期任务、渠道测试前后端映射。

共同结论：同版本周期reload写pending和分离两次TryLock是确定的代码机制；同周期恢复饥饿与现场日志高度相容；历史首次触发者、具体busy锁/持锁时长无法据现有材料证明；未找到所审路径的确定性递归锁或ABBA。两个代理随后独立只读复审findings-and-proposal.md。

复审无Critical，三项Warning已修改提案：

- 同revision外部DB漂移属于已提交变化，须在可失败编译前建立/保留共享门禁；不能先编译失败后仍共享ready。已区分正常预构建、既有pending、漂移、未知读取和本地初始化。
- 渠道普通写不一定推进catalog revision；可靠路由token或完整重捕获和旧快照延迟安装回归建立前，不得移除现有capture-through-install读屏障。
- 首个全局交换前确认所有安装前提，或真单指针bundle且迁移全部读者；禁止先typed swap再逐个可失败cache TryLock。

Info已纳入：ACK实际提交成功/失败×后像查询成功/失败矩阵，核对完整epoch以及managed operation不可变绑定；所有billing identities的币种/FX来自同一快照。

结论仅为方案评审，不是实现审查或测试全绿。应用代码和线上环境未修改，方案等待用户确认，修复任务未完成。

## 用户批准后的本地实现审查（进行中）

上述段落为初轮提案时点。用户随后明确批准本地复现/修复/测试/审查，不批准推送或线上操作。

- Runtime责任域独立审查先发现4项Important：mixed passkey+FX绕过发布、选项mutex等待不可取消、未知capture失效在已有读者下丢失、reconcile提交后仍等完整周期。全部真实RED后修复，scoped复审判4项ADDRESSED。
- 错误/UI责任域发现native task renderer sanitizer丢暂不可用code，及health回归未启用实际disable路径。真实RED后修正，native/missing/throwing renderer及负例normal/race通过；scoped复审APPROVED，无新Critical/Important。
- 最终全量交叉审查：独立gpt-6-astra及gpt-6.1-sol审阅冻结快照SHA256 `02185ac720d811974048d509854032083c7fdf3e4d2aaa08531ca9bc2c28284d`。无Critical，但两路均暂不批准：reconcile缓存可能把后续B误记为已处理；ordinary SQL-noop在外部同版本价格/FX漂移后可能混用price9/FX7。原owner同一修复波次补真实RED后修复；目标normal初验通过，待最终scoped复审。
- Minor：格式等价JSON保存后，指纹仍按旧raw值，会让后续无变化刷新重新安装。已补持pricing锁的后续reload RED并改为复用无损CanonicalJSON、保留非法/重复key校验和存储原值；不能删减原RawNoop完整state断言。
- 补充十轮真实scheduler流量验收显示no-op非目录options的写屏障仍造成24/1176（普通）及37/1210（race）次busy。已移除该不必要W窗口并以读屏障保护，初次traffic-race通过；还在验证不允许读屏障内同值plain重写typedFX的竞态。没有吞错或自动重试用户读请求。
- 历史测试适配仅更换已不再存在的root-cache-query故障注入时机，或批准的读屏障/FX epoch预期；保留真正提交后发布失败、冻结旧请求、完整ACK、实际结算与失败关闭约束。所有失败日志保留，不以skip或删除用例换绿。

当前结论：仍未完成最终稳定源全量验收与复审，不能报告修复完成。完整原始审查/RED/GREEN及边界记录在本任务SDD报告目录。

### Round3 最终代码复审结论

两路独立审查均批准源码快照 SHA256 `69754df30bfb53a6743cc59658b4d4a8fd2874ebf26091171d733bd1f3344240`：原两个Important、raw-noop Minor、中央FX批次、changed-policy新Important均ADDRESSED。Read-guard路径全批预检后在所有typed setter、规范化持久化及snapshot更新前退出；真实changed设置先释放R，再取得W重查完整输入并加载，保留无变化十轮零busy和变化时排他。不重放业务SQL。无剩余Critical/Important，未虚报外部antigravity/Claude。

最终normal/race目标22.679s/27.365s，原三条review1补充race12.691s；审查方仅核对代码/报告，不冒称执行完整测试。完整PG门禁仍为独立待完成验收。随后合法stored-quote测试只允许在直接SQL seed后真实恢复，再做snapshot/no-op save；unsafe-unit负例和所有断言不改变，最终实际diff另核对。

### Round4 公开刷新入口与最终夹具复审

完整门禁暴露正常channel priority/mapping变更后公开Init入口未恢复ACK的真实回归，原owner与独立reviewA分别查证所有生产调用位于W外。fresh架构worker新增8项真实case并观察RED，仅在public Init入口增加7行bounded恢复+安全失败早退，内部构建/安装器不递归调用Init。两路scoped均APPROVED，无新阻断；代价为旧void API可能同步等待原30s预算，不能直接返回刷新错误。

后续startup helper及middleware routing完整初始化两项实际diff均经两路独立APPROVED：故障注入迁移到真实finalACK不削弱commit/pending/recovery；非master routing fixture真实迁移/源镜像，不造价格、不强制ready、不改变选择/decoder/retry断言。最终源码快照SHA256 `e5bf372a0057c3d5ac48cc8f687b677e2947c44d6378f833b4e29094590d1b55`，代码无剩余Critical/Important。完整门禁仍由root验证，不将审查批准当作测试通过。

### 最后持久FX夹具复核

最终普通全套识别TestCatalogRuntimeResetAndHistoricalOrphan只写内存FX7.25、DB没有该值；中央权威发布正确恢复DB默认7.3，旧断言因此失败。唯一后续源码差异为该测试函数内持久seed USDExchangeRate=7.25，并增加冻结/typed一致及无伪造metadata断言；旧orphan0、移除、SiteName及默认不落库断言不变。两路最终审查均核对实际diff并APPROVED，未削弱门禁，无生产代码再改。最终应用/测试diff SHA256为`5b5866da49a555f47197ea720b39a3d2ca69a0bf4f18c56cf3b9dd203445fd1a`。单目标及13项neutral普通/race通过，完整门禁仍待root确认退出结果。

## 最终结论

Root现已核对完整normal62709/race95285各exit0、各53组选定PG回归2006项零skip、各独立包2733+583+7项通过，最终vet、build及前端/门禁结果见verification.md。实现本地提交0d07bc76964937b6c9ca6ef58b0b7ca76ed59a5a与最终被审快照一致。双路APPROVED，无未解决Critical/Important；审查批准与实际测试退出均有独立证据。既有全站lint债务/工具脚本兼容问题明示，不伪称全仓全部命令全绿。不推送或操作生产。
