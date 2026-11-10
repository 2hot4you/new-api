# Retained-option no-op writer fix

> **For agentic workers:** 使用executing-plans按序inline实现；用户要求双路独立复审。已有两路根因分析可复用，不重做已完成旧修复或部署。

**Goal:** 真实启动遗留键不会让无变化同步误申请写锁，保留有效变化保护并只重新验收dev。
**Architecture:** 精确运行态排除谓词由no-op预检和publisher共用；不改变DB行、目录状态机、writer admission、真实变更发布或认证策略。现有liveness测试文件集中增加回归，mandatory gate显式选入。
**Tech Stack:** Go/GORM真实三库、现有PG normal/race gates、GitHub Actions development。
**Spec:** 本目录requirements.md；用户已批准上层review中的最小本地修复和dev-only顺序，无新产品决策。

## Global Constraints

- main与全部生产本轮不动；Aigc不联系，不动任何线上配置/DB/服务。
- 只修改model/option.go、model/catalog_publication_liveness_test.go、deploy/tests/test_catalog_postgres_gate.py及精确任务/spec记录。
- 不放宽missing effective key、规范化、真实policy/price/FX漂移的writer/失败关闭规则；不删遗留状态，不新增依赖/生产钩子。
- 保留用户dirty；测试仅任务自己的临时本地库，不用用户5432/6379服务或线上DSN。不输出秘密/请求正文。
- 普通/并发测试、独立双审、fresh refs通过后才普通push develop；不force/重复调度/推进main。

## Review Focus

- 仅一个遗留键也须no-op，多轮加载不能重新暴露它或改变DB期限/theme。
- 有效的缺失/改变设置不能因排除规则被误跳过；真实RetryTimes更改仍要求W并正确发布。
- 并发回归必须协调真实optionsOnly loader、证明R尚持有时同步完成和新pricing读可用，不靠睡眠或mock返回。
- 三库分支进入PG选中matrix且缺失/skip仍失败；不缩减原门禁/断言。
- 修复不扩展auth信任/期限、定价读取准入或pending放行；新源dev必须全量重新验收，不推广旧5fb。

## Task 1 — 最小回归和修复

**Files:** model/option.go；model/catalog_publication_liveness_test.go；deploy/tests/test_catalog_postgres_gate.py。
**Interfaces:** 消费真实EnsureLegacyAccessTokenRetireAt、normalizeRetiredThemeOption、loadOptionsFromDatabaseGuarded、recoverCatalogSyncRuntime、WithCatalogPricingRead；产出精确两键排除的no-op语义及mandatory回归。

- [x] 新增TestCatalogPublicationRetiredOptionsKeepNoopReadOnly三库表：none/deadline/theme/both；真实启动函数种行，常规load稳定，再optionsOnly重复3次；literal原值/DB状态/fingerprint/不暴露断言；有效changed/uncached设置正控制。
- [x] 新增TestCatalogPublicationRetiredOptionsKeepReadersAvailable：真实PG/R与loader rendezvous；无变化必须在R释放前完成，不申请W且新定价读成功，失败时取消并释放资源不污染后续。
- [x] 用仅本地owned三库运行上述测试，确认旧源以need-writer/并发读受阻原因RED，非编译或fixture错误；记录输出。
- [x] 在option.go增加复用精确两键排除谓词，原publisher和预检使用它；不改别处状态机或日志分类。
- [x] 同一三库测试GREEN；加race与原changed-policy、真实price/FX drift、writer公平/取消等相邻回归。
- [x] 新测试显式进入原mandatory PG liveness/matrix组，补required children，运行gate自身校验/原Python契约，原库存保持。
- [ ] gofmt/diff-check，精确暂存自己的改动并本地commit；验证打包源包含相同受验源码。

## Task 2 — 完整本地验收与双审

**Interfaces:** 消费Task1精确源与测试，产出真实正常/race和双路报告。

- [ ] 在同源干净打包产物运行现有make test-postgres及make test-postgres-race，保留完整正常/独立套件结果；原工作树忽略的重复dist文件不删除或纳入源。
- [ ] 两个独立native reviewer审查base5fb..补修源及测试证据；Critical/Important修复需RED/GREEN并按用户要求重新双审；无阻断才发布。
- [ ] 审计无秘密/无无关diff、三库版本/命令、剩余边界；任务记录真实SHA。

## Task 3 — 仅dev重新部署及验收

**Interfaces:** 消费已通过的精确新SHA；产出新run/attempt/jobs/live version，不授权main。

- [ ] fresh refs develop=已审5fb、main=d922且无正常待执行同源任务；正常push新source到develop一次。
- [ ] 跟进实际新source完整Verify（frontend/PG normal/race/contracts）与Release development，核事件/source/attempt/备份/重复启动及真实版本；正常运行不重触发。
- [ ] dev只读核至少4个正常periodic起点，前三完整周期不再因遗留键申请W；真实镜像/状态与用户实际计费记录足够才完整验收，不主动付费。
- [ ] 用户当前main/prod hold不因dev成功自动解除。只完成本地审计/归档自己任务，不为归档push main；上层任务保留最新hold。
