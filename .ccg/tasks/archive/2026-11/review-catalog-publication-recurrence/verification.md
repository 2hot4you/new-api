# 复审诊断验证

审查源0d07bc769，HEADbb729e952仅归档文档，无后续应用差异。本次不执行新全套PG/MySQL/前端验收，也不把上一轮绿色结果当新结论。没有修实现。

证据目录：工作树`.superpowers/sdd/review-catalog-publication-recurrence/`（ignored，保留原始临时测试和失败/成功日志）。Go overlay映射到两个不存在的测试路径，不修改原文件、index、分支或应用提交；model TestMain仅建进程内SQLite。显式unset全部相关SQL/PG-only环境，未连接既有开发服务、生产、外部上游。

原始确认：status-lock.log/status-lock-race.log实际exit0；model-all-normal.log/model-all-race.log实际exit0，确认全部4项异常。首次observation-race.log仅因临时测试import错误build failed，修临时overlay为真实relaykit/dto后通过，不改业务源码。

最后映射移动后再次执行以下命令的普通与race变体：

```sh
env -u SQL_DSN -u LOG_SQL_DSN -u TEST_POSTGRES_DSN \
  -u TEST_CATALOG_SYNC_POSTGRES_DSN -u TEST_MYSQL_DSN \
  -u CATALOG_SYNC_POSTGRES_ONLY GOWORK=off \
  go test -race -overlay .superpowers/sdd/review-catalog-publication-recurrence/overlay.json \
  ./model ./controller \
  -run '^TestReview(RecurringTryWriterHasNoReaderAdmissionPriority|StaleExpectedRevisionInvalidatesHealthyPublication|UpstreamObservationTimestampReopensPending|StatusWriteRetainsOptionMapAndCatalogWriter)$' \
  -count=1 -timeout=45s -v
```

final-normal.log：两个包实际exit0，model3项1.566s、controller1项0.926s；final-race.log：两个包实际exit0，model3项3.658s、controller1项2.665s。全部4项诊断被执行且无skip。PASS表示诊断成功确认异常，不表示修复成功；race未报数据竞争不能否定逻辑/活性缺陷。复审本身完成与待修问题完成是两回事。

两位fresh审查者均已复读overlay源码和原始日志，独立支持4项Important及本报告的限制。应用差异`git diff 0d07bc769 HEAD -- . ':(exclude).ccg/**'`和当前unstaged非CCG diff均为空；仅新复审文档及ignored临时诊断有写入。用户原有删除/草稿未stage或修改。

边界：I1是实际handler+相同发布锁调用接缝，不是完整publication或真实TCP；I2是受控重叠读者准入证明，不是QPS阈值；I3/I4是SQLite真实存取/恢复，不是PG/MySQL全兼容或现场首触发复现。详见review.md。
