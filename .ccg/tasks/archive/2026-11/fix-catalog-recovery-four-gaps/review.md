# Independent review — four local recovery gaps

Reviewed application/test/gate diff against base `0feb59bcc`. Initial diff SHA-256 `c4863d00ac425e5b2200a20ca0f6a243d9ab9d3a41b4845ed814acb68ab20814`; after one final integration fix wave, frozen final diff SHA-256 `28e637c59c1db9a4229e030c7a081d0d9931ed3409d35d9e63e001f2fc3451aa`.

| Independent reviewer | Spec | Quality | Actionable findings |
| --- | --- | --- | --- |
| Native A, architecture/lock/ACK | PASS | Approved | Critical0 / Important0 / Minor0 |
| Native B, admission/identity/regressions | PASS | Approved | Critical0 / Important0 / Minor0 |

The requested external antigravity/Claude wrapper does not exist locally. Two independent native reviewers were used transparently; this is not external-model certification. Neither reviewer authored this task's code. Status task also had a separate nonauthor spec/quality PASS before integration.

Both full reviewers read the actual initial diff, plan, reports and relevant call sites, including the deterministic changed-policy test correction. Checked complete OptionMap/cache reservation and nonrecursive locked helpers, counter unwind/all live reader admission, narrow raw JSON fingerprint projection, exact obsolete-target identity after transaction cleanup, real HTTP billing/settlement assertions, and no removal of original mandatory tests. Independent Python verifier runs passed (A23 tests; B7+5 tests). Full gate subsequently exposed the compatibility issue below; initial approval was not treated as full acceptance.

Coverage retained: original53groups/599 selection entries, now54groups/611; zero removed package/test pairs or required child selections. There are610 unique pairs because one existing pair is intentionally selected twice.

The old350ms test failed before its intended rendezvous under real SQL latency. Authorized test-only correction waits for actual loader/writer admission, then cancels while the real reader is held; cancellation/fallback assertions and separate deadline test remain. Raw failed logs preserved, not counted as passing. Final normal target56top-level115.032s; broad targeted race56top-level133.372s; postcompile joined-stale and deterministic fixture additions separately rerun under race. Full acceptance outcomes are recorded separately in verification.md, not inferred from review approval.

Raw independent reports: `.superpowers/sdd/plan-fix-catalog-recovery-four-gaps/final-review-a.md`, `final-review-b.md`, `task-1-report.md`, `task-2-report.md`. The first A report reserves an in-progress normal run; root subsequently observed its actual successful terminal result.

## One final integration fix wave

Full normal gate caught new early JSON validation changing the unchanged advanced-channel error classification. Proactive unchanged alias-removal regression then proved a prefix-only correction insufficient: it also changed the established ordinary SQL commit/pending/recovery stage. Both failures and the unsuccessful prefix-only attempt are preserved, not counted as green.

The final three-file wave retains exact raw identity only when new strict normalization fails and AdvancedCustom's existing typed parser also rejects it, then uses the unconditional lifecycle projection to reject before any exchange/ACK, including disabled/cache-off paths. Ambiguity accepted by the typed parser still fails strict normalization. Well-formed typed-invalid JSON receives no new exemption. Original two historical test files and all assertions are unchanged. Existing routing safety test now runs on actual three-engine ordinary fixtures and adds enabled/disabled malformed/typed-invalid/ambiguous/noACK/readclosed/no-write/non-disclosure cases; its same required name moved into the PGmatrix group, preserving54/611.

Exact fix-wave diff SHA-256 `7bf956021a1a26afdba6a4d39f600cb33832d45ef014782ae4e42ff4e13bfe18`; reports `scratch/fix-wave-review-a.md` and `scratch/fix-wave-review-b.md` in raw evidence. Both independent reviewers verdict: classification ADDRESSED, postcommit semantics ADDRESSED, no new Critical/Important/Minor breakage, Approved. They checked no-op/prospective/committed paths, unconditional validation before cache gating and disabled filtering, actual final covering logs and actual PG verifier11 pass events/zero skips/nonPG. Full mandatory normal/race subsequently both completed after this amended freeze; see verification.md, not an inference from review approval.

## Explicit remaining boundaries

验收环境另经独立read-only audit：3265个非任务记录tracked源码/配置/资源路径逐项相同，1384个Go/module条目实际重哈希相同；Makefile/门禁脚本未改谓词。重放正常完整54组命令及实际验证器得到611selection/2039通过，完整独立包43/2733、relaykit9/583、tokenkit1/7，零失败/测试跳过；116+6个证据文件与原runner目录逐字节相同。干净前端产物确由原配置重新构建，不谎称与原忽略目录含重复文件的整体逐字节相同。详见raw `scratch/acceptance-artifact-audit.md`；该audit自身只核normal，后续完整race执行与root重放结果单列verification.md，均已通过。

Preexisting ordinary SelfUse-setting saves and plugin-generation hot updates are outside the approved four issues, unchanged and not certified consistent by this task. No new worsening found; this is not a guarantee for all runtime option APIs, mixed binaries or distributed publication. Ordinary OptionMap wrappers retain existing blocking semantics; authoritative recovery/install paths are cancellable. Joined-query regression is not claimed as native rollback-fault injection. The local billing integration does not claim traversing authentication middleware; separate mandatory authorization regressions are included in the full gate. No paid upstream, production runtime or rollout validation was performed.
