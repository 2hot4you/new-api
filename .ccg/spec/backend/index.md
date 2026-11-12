# 后端约定

## 公开图片代理

- 对外图片链接使用带有效期的认证加密令牌；只签名或 Base64 编码原始地址不能隐藏来源。
- 公共代理取图必须固定到验证过的 DNS 地址，拒绝内网地址，并验证每次重定向；全局关闭 SSRF 检查、允许内网或跳过 TLS 验证的设置不能降低该公开入口的保护。
- 不将请求方的鉴权信息、上游响应头或上游错误正文传给图片访问者。只返回受支持的栅格图片类型，并限制读取时间及大小。
- 修改流式图片响应时，测试“已交付完整图片后发生错误”的结算路径。若返回顶层错误会触发整次退款，必须保留已交付内容的正常结算，同时向客户端发送脱敏的流错误。
- 改写图片链接时同时检查 JSON、真实 SSE 和 JSON 转 SSE 三条路径，以及原始模型名和映射模型名。
- 缓冲图片响应必须限制进程级并发，槽位覆盖整个响应写入；设置固定写超时，避免慢速客户端长期占用全部下载槽位。

## 同步生成网关超时

- 同步图像生成可能超过一分钟；核对网关等候响应头超时与应用处理时间，客户端等待参数不能延长网关超时。按图片路径设置独立读取等待，并保留更长下载路径的超时与日志规则；使用不计费的延迟响应测试验证。

## Shared ordinary and task pricing

- Task expressions evaluate usage facts in currency per request; ordinary token variables are not populated there. Reject unavailable token variables and fixed ordinary pricing from the original AST before reservation, even for unchanged stored quotes and unreachable branches. Legitimate task token pricing uses a declared `u("tokens")` fact and an explicit million-token conversion.
- Keep unit safety separate from schema compatibility: an unchanged sole-provider usage expression may retain the documented narrowed-schema exception, but cannot bypass unit safety at explicit saves or selected-request execution. Dormant ratio-mode expressions and ordinary quotes shadowed by valid source/mapped task overrides must remain editable.
- Cross-model alias validation must use the complete candidate override map, including additions, removals and resets, before per-row validation. Never let batch order decide validity, retain removed old overrides, or consult a later live map during prospective catalog validation.
- Committed catalog reconstruction is storage compatibility, not permission to execute a task. It may retain syntax-valid stored quote identities with unchanged mode/override evidence while task compatibility stays false. Compiled defaults are not stored provenance; direct price saves, changed/adopted quotes, mode activation and override removal remain strict. Never bypass the pre-funding execution guard.
- Upgrade fixtures must seed historical quotes before option bootstrap and real startup completion, then test repeat startup, pending recovery, authoritative reload and unrelated writes. Recovering valid defaults before inserting historical data tests saving only; it cannot prove upgrade startup compatibility.

## Server address and Passkey trust

- When replacing unusable implicit bootstrap RP/origin defaults, filter only retention of the old trust; still materialize the newly requested origin as explicit configuration. Otherwise a later ServerAddress change can silently expand trust. Keep explicit historical validation and credential-removal confirmation intact, and test preview rollback, stored origins, repeat save and rejection of a second foreign-origin change through both controller and mixed catalog-save paths.

## Catalog publication and recovery

- An unchanged refresh requires a complete acknowledged input proof, not revision equality. Bind semantic price options, currencies/FX, plugin dependencies and routing; keep exact raw storage separately, canonicalize without decimal precision loss, and reject duplicate identities. Exclude operational counters from routing identity, not real routing/configuration changes.
- Prepare all fallible projections and reserve every install lock before the first global exchange. A genuine installation publishes typed/raw FX with prices in the same batch; request money must use the immutable published input. A failed or uncertain ACK remains closed until authoritative recovery proves the complete postimage; never force ready or replay an ordinary business SQL closure.
- A failed authoritative capture must invalidate local selection even while existing readers prevent taking W. A DB-scoped invalidation latch is not a readiness proof and may be cleared only by a fully acknowledged reconstruction.
- Keep unchanged periodic loading read-only: full pure batch preflight must exit before every typed setter, normalization write and snapshot publication. Changed settings such as SelfUseMode affect fallback pricing; release R and reacquire W with a fresh proof before publishing them. Same-value setter suppression alone is not synchronization.
- Coordinate periodic recovery before channel reconciliation, use cancellable jittered contention retries without held locks/transactions, and wake short recovery after committed publication failure without repeating business writes. Public channel refresh must also recover a committed routing change before unrelated cache/alias installation; otherwise an unchanged metadata reconciliation can strand pending after priority/mapping edits.
- Cache a successful reconciliation's operation-bound acknowledged postimage while its writer is still owned. Sampling the current epoch after releasing W can mislabel a later, unreconciled publication as complete.
- Catalog busy/pending is local `catalog_pricing_unavailable`, not missing price: reject before funding, do not retry a generation or penalize upstream health, and preserve the category through native error renderers only with local/status/code/sentinel proof. Keep upstream lookalikes sanitized and missing-price UI/configuration guidance distinct.
- Public cache/recovery fixtures need the actual catalog schema and persisted identities matching registered plugin sources; non-master InitDB does not migrate them. Inject postcommit publication failures at the real final ACK, prove business commit and rollback of acknowledgement, and retain frozen-request/recovery assertions. Valid unchanged-save fixtures must acknowledge their seeded quotes; unsafe explicit-save validation cases must retain their original strict boundary.
- Never hold OptionMap R through response header/body I/O: serialize the complete envelope into owned bytes before unlocking, and preserve renderer/error/header compatibility. Recovery must reserve OptionMap with all other install locks before any global exchange, use explicit locked helper variants, and unwind reservations on cancellation; a context cannot cancel a blocking mutex acquisition.
- TryLock retry alone does not enqueue a writer. Cancellable publication needs counter-based writer admission across every live reader entrypoint, with checks before and after R acquisition. Intent covers actual W acquisition/ownership only, never the writer's own R capture or detached staging; cancellation of one waiter cannot erase another's intent.
- Routing fingerprints may exclude only named observational fields, not a whole JSON column or a lossy typed DTO projection. Preserve unknown raw members and exact decimal values; reject ambiguous/invalid input before publication acknowledgement. Preserve existing advanced-channel validation stage/ordinary postcommit semantics: typed-parser-invalid settings may retain exact raw identity only until unconditional lifecycle projection rejects them before any exchange/ACK, including disabled channels and cache-off paths. Normalize detached identity copies, never stored/channel configuration.
- An obsolete expected-revision result is not an authoritative capture failure. Exempt only the attempt-local known precondition result after transaction cleanup; joined rollback/restoration failures still invalidate, and obsolete requests never clear existing invalidation. Deterministic concurrency tests rendezvous at real loader/writer phases; short fixed wall-clock budgets must not replace the intended scheduling proof.
