# 本地验证记录

## 身份、范围与当前状态

- 用户批准仅本地复现、修复、测试及审查；不推送、不合并、不部署、不连接生产，不重启既有服务。
- 故障应用源d922；开发基线bec3e330e657c8f783a918ff1f703f7410c75a69的相关应用文件与d922相同。分支fix/catalog-publication-liveness。
- 当前最终冻结应用/测试diff SHA256：`5b5866da49a555f47197ea720b39a3d2ca69a0bf4f18c56cf3b9dd203445fd1a`，原件`final-persisted-fx-fixture-source.diff`；包含round4公开刷新入口、完整旧夹具初始化和持久FX夹具修正。
- 完整PG normal/race均已实际exit0；本地实现提交`0d07bc76964937b6c9ca6ef58b0b7ca76ed59a5a`。未推送、未合并、未部署。
- 日志和各worker报告统一位于工作树`.superpowers/sdd/plan-diagnose-catalog-publication-stall/`；表中日志名均相对此目录。旧失败日志保留，不覆盖成绿。

## 证据与确定性复现

用户原始三个文件未修改，SHA256见findings-and-proposal.md。220行日志含17轮同步、17次options busy、152条请求pending及17条reconciliation警告。没有历史首次转换/持锁栈，不能认定12:56首次触发者或某把历史锁已被查明。

初轮无测试DSN的SQLite managed选择失败在健康单实例资格，未进入竞争目标，不能计为故障RED。批准后使用独占loopback PG17.11及真实初始化得到以下行为RED：

- 无变化reload写pending、占用pricing锁后请求不可用；读锁竞争/取消/超时、非法同版本漂移、预安装竞争半投影；见repro-report.md及repro-baseline-full-red.log。
- 独立OS进程same-revision FX发布后，旧实例没有epoch fence；见process-fx-epoch-red.log。修复后noop/fx/interleave三子case race通过9.262s。
- 模型请求错误被误归为缺价、native renderer丢category、local503污染渠道健康；真实后端、renderer、前端RED详见errors-report.md，修复后的normal/race、负例和UI对照均通过。
- 审查发现的mixed passkey+FX、不可取消选项mutex、已有读者下未知capture失效丢失、reconcile提交后不短唤醒、缓存误标后续epoch、ordinary-noop混price9/FX7、格式JSON重发布、legacyFX同值race、changed-policy在同一捕获内变化，均真实RED后修复。命令、原始日志和结果见runtime-report.md/repro-report.md。
- 公共Recover(nil)原本price/frozenFX已新而legacy/rawFX仍旧；双child明确RED后中央批次修复，focused normal4.418s/race19.969s通过。

## 已验证的用户可见结果

| 验证 | 实际结果/边界 |
| --- | --- |
| 十轮真实后台同步+并发读取 | normal1094/1094；race1148/1148，0busy、0pending、0race；完整state不变，价格/CNY/FX一致、路由可用、goroutine及锁退出。不是日志颜色判定。 |
| 真实本地LLM计费链 | 本地mock usage1000输入/100输出；旧CNY/FX7请求结算1500，改价/FX14后的三请求各2250；wallet/token最终991750、used8250、4条消费记录及upstream request ID一致。跨3轮刷新，不调用付费上游。 |
| 独立进程 | 共享PG但不同Go锁/缓存；noop不关共享ready；外部FX已ACK后旧实例被fence；在校验/捕获间插入另一进程commit，已准入请求完整旧FX7、新请求拒绝，恢复后FX14。 |
| 三数据库 | SQLite3.50.4/MySQL8.0.46/PG17.11增量指纹迁移与普通货币/失败恢复路径通过5.596s；MySQL5.7.44增量迁移另通过1.358s。不得称全部三库legacy managed套件通过。 |
| 最终目标普通/race | Changed-policy+十轮流量+全部Reload普通22.679s/race27.365s；原三条审查回归补充race12.691s。 |
| 公开渠道刷新 | 真实priority/mapping+既有Ability/metadata、两缓存模式：normal8.077s/race9.919s。未变零DBwrite、失败留门禁/不装新路由/无敏感日志，业务写不重放。原controller alias不改fixture normal1.765s/race7.296s。 |
| 启动/中途恢复 | 9种实际InitResources进程模式+2项Init startup完整normal34.711s/race117.306s；sharedhelper改最终ACK真实失败，保持quote7已提交、operation pending及bootstrap不ACK、后启动恢复/权限/坏cache断言。 |
| 严格历史quote安全 | RC47全部19child+parent20events普通15.585s/race20.226s；只有合法unchanged成功夹具seed后增加真实ACK，unsafe-unit负例/报价/断言不变。 |
| 路由全矩阵 | 原31种选择×两缓存模式62个叶子及multipart目标：normal17.542s/race129.407s，零失败/跳过；只补真实完整初始化与plugin源镜像，原选择断言不变。 |
| 历史orphan/FX夹具 | 最后旧测试原先只写内存FX7.25而DB为空；权威恢复正确使用DB默认7.3，故原断言失败。改为真实持久FX7.25并增加frozen/typed一致且无metadata断言，单目标normal1.376s/race2.594s，完整13项neutral normal12.669s/race18.862s。 |
| 前端全套 | JSON结果3174/3174测试通过，0failed/pending；663 reported suites通过。frontend-test-results.json。 |
| 类型/构建 | `bun x tsgo -b`及`bun run build` exit0。原typecheck脚本使用installed Bun1.3.14不支持的`bun check`，明确使用项目已安装编译器，不改工具链。 |
| Lint/i18n | 本次TSX lint/format及7locale一致性通过；全站lint142条错误与不可变基线排序诊断逐项相同，不冒称全仓lint绿，不顺手修无关文件。 |
| 门禁自身 | Python catalog verifier7项、postgres CI verifier4项通过；新测试加入mandatory，原inventory不删除，无required skip。 |
| 独立代码审查 | 两路native审查最终APPROVED，无剩余Critical/Important；缺外部wrapper，未虚报antigravity/Claude。review.md记录完整闭环。 |

## 完整最终门禁（完成）

明确unset SQL_DSN/LOG_SQL_DSN，只给本次loopback测试DSN与独占Redis DB15/14：

```sh
GOWORK=off make test-postgres
GOWORK=off make test-postgres-race
GOWORK=off go vet ./...
GOWORK=off go build ./...
```

- 普通：postgres-ci-normal-persisted-fx-final.log，会话62709实际exit0；53组/599个top-level选择、2006 tests/subtests，零跳过。旧complete-fixtures普通会话18286失败于上述FX夹具，不计通过。
- Race：postgres-ci-race-complete-fixtures.log，会话95285实际exit0；相同53组、2006 tests/subtests，零跳过，无race报告。
- 两条完整命令各还独立执行43个数据库无关包2733项、relaykit9包583项、tokenkit1包7项，均零skip；即各5329个tests/subtests通过（组间可有交叠，不将其当唯一测试函数数）。
- Race启动时diff为e5bf，后续唯一改动是上述独立测试函数内的FX seed/断言；修改发生在该命令编译model-runtime-neutral组之前，前面的组不执行此函数。受影响组另独立normal/race完整重验，完整race实际通过，不隐瞒源时点。
- Vet/build：go-vet-complete-fixtures.log / go-build-complete-fixtures.log，独立会话9331 / 17829，各exit0；之后仅上述测试函数改变。最终FX夹具vet go-vet-persisted-fx-final.log（50632）也实际exit0。
- Python门禁最新按`PYTHONPATH=deploy/tests python3 -m unittest test_catalog_postgres_gate test_postgres_ci`复验11项exit0，日志python-gates-persisted-fx-final.log。首次错误包调用因旧测试import路径失败，未改源码，随后使用正确项目路径。
- 这些门禁包含43个数据库独立包全套、独立relaykit/tokenkit全套及明确审计的PG mandatory inventory。数据库相关包不是无差别`go test ./...`，不得描述成旧make test全仓三库全绿。

## 失败历史与约束

旧full normal/race尝试暴露真实取消传播缺陷、旧W-only/FX-revision断言、未ACK货币或合法报价夹具、已消失的root-vendors重建注入点。对应生产缺陷真实修复；夹具只改完整初始化或真实最终ACK失败注入，保留已提交业务变更、新请求拒绝、旧请求冻结、恢复新价、no-media/no-debit/消费记录约束。所有目标重验通过才进入最终完整门禁，没有skip或削弱安全校验。

公开入口回归由完整门禁真实捕获并经两路独立确认：普通渠道变更后，需要入口bounded恢复而非仅等下一周期，不能仅改fixture掩盖。round4只增加入口7行，双路复审通过。随后middleware路由旧fixture因IsMaster=false只建4表缺catalog状态，安全abort造成旧路由；该单一fixture补完整schema/bootstrap、复制真实factory源、将原runtime测试plugin精确source/hash/API/version镜像DB，未造价格或修改31种路由选择×两缓存模式的任何断言。normal整目标+multipart17.542s、race129.407s通过；两路实际diff审查均批准。只读audit确认其余15个mandatory Init caller已有完整PG前提。

四个测试容器均本次标签创建，未碰用户5432/6379服务。验收后再次核对精确ID及两项本次标签，仅停止并删除本次4个临时服务，docker stop/rm均exit0；非业务临时数据不保留。原始测试日志/失败证据保留于ignored SDD目录。未做任何生产请求、生产扣费/SQL/配置/重启或发布。因此这是本地修复验收，不能声称已在生产根治；部署与至少10轮生产安全观察仍需另行授权，也不承诺混旧新二进制或分布式managed发布。

## 后续部署风险（未授权执行）

RuntimeFingerprint是兼容增量列而非仅日志改动，迁移已验证SQLite/MySQL8/PG和MySQL5.7。它只在完整运行态最终ACK时建立，不在迁移中制造ready。部署必须核对真实腾讯目标/实例数，安排同版本切换、备份与正常启动恢复；不承诺混旧新二进制共享库滚动。回滚到部署前另行核实的固定镜像，不删除新列或回滚业务价格，不直接写ready；旧代码可能重新暴露原活性缺陷。详见历史提案部署/回滚章节和本次decisions.md。
