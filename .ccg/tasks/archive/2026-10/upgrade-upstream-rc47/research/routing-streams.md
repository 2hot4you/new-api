# rc.47 routing, task polling, and stream research

Read-only analysis against local base `e32fdcda17c5c48d26a461f6e4fd6c2ce862aa11`, upstream rc.46 `7d8b91268dab69d7150f345c448b5e250163146a`, and rc.47 `e835b1bfc43b565d625537830dbfb19328925134`. Read root AGENTS.md, billing rules, Task Plugin API v1, billing-expression documentation, backend/guides specs, and task requirements. No tests, database connections, provider requests, refs/index changes, external executors, or implementation changes were performed. Only this research artifact was written.

## Files Found

| File | Responsibility / preservation concern |
| --- | --- |
| `middleware/task_plugin.go:340` | Current shared-endpoint ownership, canonical/alias lookup, generation pin, mode filtering, per-candidate decoding and origin intents. rc.47 adds ordinary/plugin preselection ahead of decoding. |
| `middleware/distributor.go:33` | Final channel selection and policy checks; rc.47 reuses an eligible preselected ordinary channel. |
| `middleware/distributor.go:193` | Candidate selected from exact pinned generation; first bound candidate wins for generic/New API channels, independent of prior retry. |
| `middleware/distributor.go:567` | Selected-provider rebind, stale identity clearing, per-candidate origin pin application and safe switch to origin channel. These are Molii changes that must survive. |
| `service/channel_select.go:221` | Request-specific plugin identities, native Seedance compatibility, token/origin pin precedence, affinity, auto groups and ordinary weighted selection. |
| `model/channel_constraint.go:10`, `dto/channel_constraints.go:37` | Filter order and candidate evaluation. rc.47 adds `FilterTaskPluginEndpoint` plus relay-only retry restriction. |
| `constant/channel.go:67` | Persisted Molii channel IDs; must not take upstream numeric assignments. |
| `pkg/jsplugin/routing.go:217` | Immutable request generation/candidate pins; rc.47 adds `ContextKeyRelayEndpoint` for pricing separation. |
| `pkg/jsplugin/registry.go:302`, `pkg/jsplugin/engine.go:107` | Fork source hash and nonblocking generation leases used to prove catalog source/runtime identity. |
| `relay/relay_adaptor.go:162` | Native StarAI/Grok/ByteDance adaptor precedence before plugin dispatch, including selected-channel fallback. |
| `relay/relay_task.go:370` | Final adaptor and mapped identity before validation; selected task pricing captured before usage hooks. |
| `relay/helper/catalog_pricing.go:20` | Fork atomic selected pricing, provider override precedence, currency/FX/tool/native rates and compiled-default provenance. |
| `relay/channel/task/jsplugin/adaptor.go:117` | Final selected candidate decode/model pin; recursive request billing bounds, outbound request preparation, header snapshot, task state and SSE submission contracts. |
| `service/task_polling.go:161` | Timeout sweep and existing durable CAS/job settlement/refund rather than upstream's historical direct mutations. |
| `common/init.go:213`, `plugins/tasks/kling/plugin.js:168` | Upstream 72-hour default and bounded metadata-first Kling outbound duration changes. |
| `relay/helper/{common,stream_scanner,stream_result}.go` | Stream writer errors, byte-based write deadline budget, protocol completion cancellation race. |
| `relay/channel/openai/relay_responses.go:77`, `service/responses_usage.go:39` | Terminal delivery and existing Molii billable-tool filtering/XSearch/code-interpreter accounting. |
| `router/task-plugin-protocol-router.go:29` | New RouteTaskPluginEndpoint entry; preserve video studio snapshot, auth, rate limit and signed proxy chains. |
| `relaykit/relayconvert/internal/{oai_chat,oai_responses}/` | Raw schema property order and usage-detail cloning; do not overwrite fork's richer segmented stream conversion. |
| `deploy/tests/test_catalog_postgres_gate.py` | Audited positive no-skip test inventory; upstream test imports are not automatically covered. |

## Dependencies

Shared host create route currently runs TokenAuth → rate limiting → PinTaskPluginEndpoint → PrepareTaskPluginEndpoint → Distribute → controller plugin/ordinary dispatch. rc.47 changes the first pin step to RouteTaskPluginEndpoint: use original body/model and the pinned routing generation to select an ordinary or plugin channel under the same group, priority, weight, affinity, and explicit pin rules. Ordinary winner bypasses plugin decoding/model rewriting, retains a relay-endpoint pricing marker, and restricts retries to ordinary candidates. Plugin winner proceeds through mode filtering and candidate decode before final distribution; decoder rejection can remove that candidate, and origin intent can repin the selected provider's channel.

Final plugin execution is selected-channel → pinned generation's plugin binding → final decode and model mapping → `CaptureTaskPricing` → usage extraction/validation → frozen billing snapshot/reservation → submit → durable result → CAS plus billing job → once-only settlement/refund. Background polling reconstructs origin/upstream identities from saved task state. Native adaptors must remain native even when reached through the shared openai_video endpoint.

Catalog mutation uses `catalogReferenceTransaction` → real SQL transaction + `TryPinGeneration` → metadata/source identity checks → COMMIT/ROLLBACK → release generation pin → publish option maps/cache. Request pointer pins and catalog publication leases are different mechanisms; preserve both.

Responses SSE uses scanner → decode and accounting observation → checked event/data/delimiter writes and flush → protocol Done only after delivered completion → join all scanner/ping/handler goroutines → narrowly correct cancellation-at-terminal-flush classification → Finish usage → normal billing/logging. Do not return a new top-level relay error that causes already delivered output to be refunded.

## Patterns and conflict recommendations

### 1. Preserve persisted IDs and native routing (critical)

Molii IDs are New API 60, StarAI 61, Molii Grok AIGC 62, Task Plugin 63, ByteDance Seedance 64, VLLM 65, SGLang 66. Upstream documentation/tests refer to Task Plugin 61. All copied code must use symbolic constants. Do not replace constant tables, platform mapping, or frontend IDs with upstream values.

`service/channel_select.go:283` appends native StarAI and ByteDance channel types only for doubao openai_video Seedance 2.x names (including mini and 2.5), even when doubao is not the first shared candidate. `middleware/distributor.go:233` deliberately maps these selected native channels to the accepted doubao candidate while `relay/relay_adaptor.go:218` still chooses the native wire adaptor.

Upstream's new preselection gathers only `candidate.Plugin.Meta.ChannelTypes`. Copied verbatim, it classifies Molii native StarAI/ByteDance channels as ordinary because those types intentionally are not plugin-owned channel types. They then bypass pinned candidate setup, receive a relay-only filter, and may use the wrong pricing/handler path. The new initial endpoint classification and final identity filtering must share the same request-specific native compatibility rules. Do not globally add native IDs to doubao's manifest; compatibility is model/protocol-specific. Preserve `CaptureVideoStudioRequestSnapshot` in the video create chain, signed `VideoProxyAuth`, Grok native adaptors and ordinary image/task fallback.

### 2. Merge the new selection/filter changes, retain Molii final rebind/origin code

Bring in RouteTaskPluginEndpoint, generation-preserving lookup helper, new DTO filter kind/relay-only bit, model filter evaluation order, and ordinary preselection reuse together. Keep request bytes and spelling intact for ordinary winners. Do not invoke plugin decode or reject unsupported plugin modes before an ordinary channel can win. After a plugin wins, retain current repeatable candidate decoding, accepted-candidate list, final selected-candidate decode and origin-ID drift checks.

Keep `setupContextForSelectedChannel` and `applySelectedTaskPluginOriginIntent` (`middleware/distributor.go:702,726`): root base contains fixes absent from upstream that avoid applying an unselected provider's origin task. Keep token pin precedence and stricter retry mode. The preselected channel is reusable only while enabled, model unchanged, not superseded by a pin, and all final filters still match. Retain existing auto-group context/affinity side effects from the first selection. Ordinary retries must not switch into plugin execution mid-request; plugin retries must retain their selected surface and use per-attempt provider pricing.

### 3. Add separate shared-endpoint pricing without bypassing frozen selection (critical)

rc.47 correctly rejects an ordinary expression using `u()` anywhere in the AST, including unreachable branches or dynamic keys. A plain constant task expression is also ambiguous: task output is USD while ordinary token expressions are USD×1M. Ordinary `fixed()` or legacy per-call pricing has an explicit ordinary contract; provider task expressions belong in provider overrides.

Do not copy upstream price.go wholesale. Molii's `ModelPriceHelper` reads `selectedModelPricing`, and `modelPriceHelperTiered` takes the selected record and freezes source currency, FX, expression version and amounts. `CaptureTaskPricing` already captures the executing plugin override first and supports mapped fallback without allowing unrelated provider metadata to veto a selected quote. Existing catalog/default-prerequisite/orphan behavior must remain.

Upstream's new ambiguity guard reads live `GetPluginBillingExpr`/`GetBillingMode` for every endpoint candidate during price evaluation. Adapt the guard so override-presence/inheritance facts are captured within the existing catalog pricing read boundary, alongside the selected identity, then evaluated from that immutable selection. Do not mix a frozen expression with later provider overrides or resolve unselected candidate currencies. Only detect whether a candidate inherits the same selected expression identity; do not turn unused malformed pricing into a new selected-currency blocker. Keep configured-map provenance for builtin fallback: an administrator expression numerically identical to a builtin is still configured and uses its configured currency.

Keep `pkg/jsplugin/engine.go` sourceHash and `Registry.TryPinGeneration`/GenerationPin fields and lock lifetime. Upstream lacks these fork features. Extending routing.go is additive; it must not replace registry.go or remove catalog pin proof/tests.

### 4. Retain recursive billing bounds; add Kling's outbound-specific fix

Upstream rc.47 removes `Meta.AcceptsAutoDuration`, `validateResolvedUsageRequest/Value`, numeric-string request parsing, and their regression cases. That conflicts directly with the task's billing.md requirement to bound nested duration/count values before any quota work. Preserve these Molii/base guards and duration-auto=-1 capability behavior. Usage facts stay finite/nonnegative; auto duration is request-only and reserves the model maximum. Keep profile validation after final upstream rewrite.

Apply Kling's metadata-first `outboundDuration`: `metadata.duration` (native body) overrides top-level duration; absent/null values fall back; otherwise `validateKlingDuration` requires positive integer ≤3600. Set outbound `body.duration = String(outboundDuration(req))` after metadata merge and use the same helper for estimated credit usage. This closes the actual sent-duration vs reserved-duration mismatch.

Upstream new Kling invalid-case assertions expect its build hook text "at most 3600". With retained host recursive validation, the same bad input may correctly fail earlier with the established host message. Adapt regression assertions around 400, no submission/reservation, and sent/billed agreement; do not delete host validation to make that exact string pass.

### 5. 72-hour default is an initialization change, not settlement rewrite

Change only `GetEnvOrDefault("TASK_TIMEOUT_MINUTES", 1440)` to 4320 in common/init.go and associated descriptions. Explicit override values, including 0/negative disabling the sweep, remain effective. `service/task_polling.go:167` already checks `<=0` and derives its cutoff from the configured value. Do not hardcode 72h inside the sweep or change the protocol observer's separate 600s timeout.

Retain the fork's `RunTaskPollingOnceWithError`, error propagation, authoritative reload on CAS loss, terminal billing jobs, rollout cutoff, native StarAI sanitization/Grok URL facts and once-only durable refund/consume logging. Upstream rc.46..47 modifies only a polling comment in this domain; a large diff vs local base reflects fork divergence and is not a reason to replace the implementation.

Task Plugin API v1 currently says 24h in its polling contract, so update that description to 72h/default4320 while preserving the explicit environment override. No plugin API/schema version change is required for the timeout default.

### 6. Native multipart and sanitized submit headers

Adopt upstream native submit/dynamic JSON-or-multipart body acceptance only when the route has no Models restriction. Scoped native routes still require canonical top-level JSON model; query routes still require no body. Preserve multipart part/file/filename/UTF-8/duplicate-content-type checks, opaque request_file refs, repeated file indexing, file byte limits, cleanup and the root body limit. Upstream removes the separate 1MiB text-field ceiling; the root reusable body limit must remain enforced before io.ReadAll(part), including multipart paths.

In submitContext, preserve owned header snapshots for later no-client hook calls. Copy client headers using canonical names/first value, excluding Authorization, Proxy-Authorization, Cookie, X-Api-Key, X-Goog-Api-Key and Mj-Api-Secret. Keep current channel Request Header Override precedence and explicit `{client_header:...}` behavior; raw credential headers must not enter ordinary driver hook requestHeaders accidentally. Native route decoder contexts do not gain requestHeaders from this change; this is the existing driver submit context extension. Do not expose file bytes or paths to JavaScript.

### 7. Slow SSE and delivered terminal completion

Adopt upstream StreamWriteBudget: 30s slack plus accumulated drain time at 8KiB/s, set before each writer Write/WriteString, carry remaining budget across subsequent writes, reset when drained. Install the wrapper once via SetEventStreamHeaders and retain Unwrap. Preserve write serialization for data and ping, request keepalive handoff, bounded cleanup and scanner max buffer. Do not leave fixed deadline calls resetting a long image's earned budget.

Removing exported ExtendWriteDeadline requires companion edits outside relay/helper: controller/plugin_protocol.go has heartbeat and event call sites and controller/relay.go has the stream-error call. rc.47 removes these calls. A partial port fails compile or reintroduces fixed deadline truncation. Preserve Molii's stronger sanitization and stream-error handling when merging those controller lines.

Port checked `CustomEvent.Render` calls and FlushWriter's unwrap-to-FlushError traversal. Gin Render/flusher wrappers otherwise discard errors. In OaiResponsesStreamHandler, preserve current Molii accountingResponse filtering of failed/cancelled tool items and explicit XSearch/code-interpreter counting, observe accounting before returning on a write failure, and call sr.Done only after successful complete frame+flush and a completed protocol outcome. Both response.completed and response.done aliases qualify; failed/incomplete/missing response terminal payloads do not.

Move StreamResult ownership outside handler goroutine, track successful Done separately, suppress EOF/read errors caused by deliberate cleanup cancellation, and correct client_gone→done only after all goroutines join when outcome=completed, end error=context.Canceled and no recorded errors. Bare EOF, prior decode errors, scanner errors, write/flush errors and disconnect before flush remain failures. Keep RequireTerminal, SGLang integer timestamps, vendor tool usage, image deduplication and interrupted-stream token estimates.

### 8. relaykit conversion changes are narrow

rc.47 preserves schema property order by carrying raw JSON members in tools/response-format conversions (`kitutil.ObjectMember`, raw function parameters, raw schema child members) and deep-clones usage detail structs when converting Responses/Chat. Adopt these with standalone module independence and existing common/kitutil codec boundaries. Preserve fork segmented reasoning/text/tool stream support, sequence numbers, custom tool restoration and hosted XSearch/code-interpreter behavior; upstream's smaller stream converter is not a replacement. Cache-write12h DTO/normalization integration should be coordinated with the billing owner so JSON, SSE and compaction carry the same split.

## Deterministic regression inventory

Existing tests worth retaining/including in the explicit no-skip selection (these names were inspected, not executed):

- Native IDs/adaptors: `TestPersistedChannelMoliiTaskPlatformsSelectNativeAdaptors`, `TestPersistedChannelMoliiGrokSelectsOrdinaryAdaptor`, `TestPersistedChannelTaskPluginStaysOnTaskPlatform`.
- Native shared selection: `TestPinnedTaskPluginChannelTypesIncludesNativeSeedanceChannelsForUnifiedSeedance2`, `TestPinnedTaskPluginChannelTypesKeepsNativeSeedanceWhenDoubaoIsNotFirstCandidate`, `TestPinnedTaskPluginChannelTypesDoesNotIncludeStarAIForLegacySeedance`, `TestUnifiedStarAIEndpointRebindsToAcceptedDoubaoCandidate`, `TestSetupContextForSelectedNativeChannelClearsStalePluginIdentity`.
- Shared provider/origin: `TestSharedEndpointRebindsToSelectedLegacyProvider`, `TestSharedEndpointRebindsToSelectedType61Plugin`, `TestSharedEndpointRebindsToBoundNewAPIExtension`, `TestDistributeSwitchesSelectedCandidateToItsValidatedOriginChannel`, `TestPrepareTaskPluginEndpointEnforcesSelectedCandidateOriginIntent`, `TestPrepareTaskPluginEndpointIgnoresUnselectedCandidateOriginIntent`, `TestTaskAdaptorRejectsFinalCandidateOriginIntentDrift`.
- Immutable generation/source: `TestPrepareTaskPluginEndpointPinsGenerationBeforeParseAndDistribution`, `TestAdjacentNodeGenerationsKeepPinnedPluginExecutable`, `TestRegistryPinSourceIdentityAndPublication`, `TestRegistryPinReportsDesiredAndRetainedEffectiveIdentity`, `TestRegistryPinIncludesFactoryAndMasterSwitches`, `TestRegistryPinDoesNotInventUnprovableProgramIdentity`, `TestRegistryPinOwnsRouteMetadata`.
- Pricing: `TestSharedTaskBillingExpressionSelectionAndFrozenSettlement`, `TestCatalogTaskExpressionPrecedence`, `TestCatalogTaskPendingRejectsBeforeUsageHook`, `TestCatalogTaskPluginHookPublicationOverlap`, `TestCatalogRequestPendingAndFrozenSelection`, `TestCatalogRequestToolPricesRemainSelected`, `TestCatalogTaskNativePrepareAndDurableCompletion`, `TestRelayTaskSubmitRetryRepricesAndReservesBeforeUpstream`, `TestRelayTaskSubmitTieredUsesFrozenBillingModelCurrency`, `TestModelPriceHelperTieredUsesFrozenBillingModelCurrency`.
- Multipliers/model identity: `TestTaskAdaptorBoundsNativeUsageBeforeQuotaCalculation`, `TestTaskAdaptorUsageProfilesFollowExecutionModel`, `TestTaskAdaptorReDecodesFinalCandidateAndRejectsModelDrift`, `TestTaskAdaptorFinalDecoderOmissionClearsCandidateSnapshots`, `TestTaskAdaptorBuildSubmitReceivesMappedUpstreamModel`, `TestTaskAdaptorFetchTaskExposesModelIdentities`.
- Native/multipart/headers: extend/rename `TestPrepareTaskPluginNativeRouteRejectsMultipartBeforeDecoder` to upstream `TestPrepareTaskPluginNativeRoutePassesMultipartToDecoder`; preserve scoped route rejection in `TestPrepareTaskPluginRouteModelScope`. Keep `TestBuildTaskPluginRouteRequestRejectsUnsafeBodies`, `TestPrepareTaskPluginEndpointExposesIndexedRefsForRepeatedImageFiles`, `TestTaskAdaptorDoesNotEmitInjectedMultipartDispositionHeaders`, `TestTaskAdaptorHeaderOverrides`, `TestTaskQueryRedirects`.
- Polling/durable completion: `TestSweepTimedOutTasksHonorsRefundRolloutBoundary`, `TestUpdateSunoTasksStalePollsRefundExactlyOnce`, `TestUpdateBatchTasksEnqueuesDurableTieredSettlement`, `TestPollingPassesExecutingChannelTypeToAdaptor`, `TestUpdateVideoSingleTaskKeepsRawStarAIURLAndPersistsOnlySafeResponse`, `TestMoliiGrokFailureRefundsExactlyOnceAcrossStalePolls`, `TestMoliiGrokSuccessDoesNotDoubleSettleAcrossStalePolls`, `TestMoliiGrokFinalUsageSuccessLogsExactlyOnceAcrossStalePolls`.
- Streaming: existing scanner keepalive/cancellation/error tests; `TestOaiResponsesStreamHandlerDeduplicatesCompletedImageOutput`, `TestOaiResponsesStreamHandlerCountsImageOutputOnIncomplete`, `TestOaiResponsesStreamHandlerDoesNotCountPartialImageEvent`, `TestImageExpressionUsesCompletedCountAndProtectsAbortedStreams`, `TestOpenaiImageStreamHandlerClientDisconnectKeepsRequestedCount`, `TestOpenaiImageStreamHandlerWrapsNonStandardDataShapes`, `TestResponsesInterruptedStreamHealth`.
- Conversion: `TestGoldenStreamConversionMatrix`, `TestConvertResponsePreservesBillingUsageAcrossChatResponsesBridge`, `TestResponseUsageMatrixChatAndResponsesDetails`, `TestClaudeTargetStatefulStreamTerminalTail`, `TestConvertRequestRecordsResponsesCustomToolsForEachTarget`.

New upstream tests to import/adapt: `TestRouteTaskPluginEndpointUsesSelectedChannel` (ordinary/plugin priorities, pins, aliases, group/model/enable filtering, multipart, unsupported plugin modes, decoder bypass, retries, origin pin, shared accepted candidate; both memory-cache modes); `TestSharedEndpointBillingKeepsOrdinaryAndPluginPricesSeparate` (u AST branches/dynamic keys, constants, mapped override, all-provider inheritance, fixed/per-call, frozen settlement); `TestSubmitContextExposesClientHeadersWithoutCredentials`; Kling's expanded `TestKlingResponsesProtocol`; `TestStreamWriteBudget`; `TestSetEventStreamHeadersBoundsStreamWritesByBudget`; `TestOaiResponsesStreamHandlerCompletionLifecycle`; `TestOaiResponsesHandlersKeepCacheWriteTTLSplit`.

New focused gaps: native Seedance selected by new preselection in both cache modes (including doubao not first candidate); configured timeout4320 vs explicit1440/0 without sleep; channel-priority and zero-weight deterministic rules (avoid statistical weighted-distribution tests); same identity/currency/provider captured across concurrent catalog publication while separate-price guard evaluates; XSearch/code-interpreter terminal success and failed-item filtering through checked delivery. Prefer extending the files above.

## PostgreSQL-only fixture hazards

1. Do not run broad root tests, make test, or arbitrary package tests to “discover” safety. `service/task_billing_test.go:37`, `relay/helper/price_test.go:40`, and model/relay TestMain paths may connect/migrate SQLite by default before a pure selected test runs. Use audited make test-postgres / make test-postgres-race with `CATALOG_SYNC_POSTGRES_ONLY=1`, validated numeric-loopback owned PostgreSQL fixtures, `GOWORK=off`, required inventory and zero-skip checks. No tests were run for this research.
2. Upstream `TestRouteTaskPluginEndpointUsesSelectedChannel` loops sqlite/mysql/postgres, skips unset DSNs, and calls model.InitDB before installing a table-prefix NamingStrategy. Importing verbatim can migrate non-owned tables/database before the apparent isolation. Rewrite to an owned disposable PG fixture with strict URL validation before first connection, simple protocol, teardown pool-close before drop; cover memory_cache=false/true as required children. Never point it at a user's existing database.
3. `setupTaskPluginRouteDB` (`middleware/task_plugin_test.go:1934`) and `setupOriginTaskDB` (`middleware/task_plugin_origin_task_test.go:27`) unconditionally open SQLite. Thus many candidate/origin tests listed above are not PG-safe yet, even though the requested test body looks like pure routing. Existing audited middleware selection includes only specifically adapted groups. Convert shared fixtures or add guarded PG fixture branches before adding those names.
4. `relay/channel/task/jsplugin/batch_poll_validation_test.go:23` explicitly opens a SQLite poll.db and is outside the safe pure-adaptor list; preserve the regression but convert fixture before selection. Router plugin/options/video-studio/header-nav fixtures likewise contain explicit SQLite and must not be accidentally swept in by broad regexes.
5. Root service/relay/helper PG fixtures must bootstrap the real catalog, not merely register runtime plugins. New shared-price tests need valid persisted plugin source/hash/API version/version/enabled state and catalog recovery so guarded reads are realistic; do not bypass pending/readiness checks to satisfy old fixture setup.
6. Current gate has exact rc46 adaptor, task-billing, helper, relay and middleware lists. Merely importing new rc47 tests or running existing make targets does not execute them. Add every new required name, update renamed native-multipart name, assert cache-mode/PG matrix children, and preserve negative skip/empty-selection verifier tests. Existing pure/adaptor tests that are already listed remain listed; do not silently remove multiplier safety regressions along with upstream's validation removal.
7. Gin mode and configuration/registry/database pointers are process-global. Preserve service TestMain's one-time gin.TestMode initialization and do not add concurrent test-body SetMode/config writes. Retain deterministic channel barriers in upstream completion-lifecycle tests rather than sleep-based schedules.

## Risks / recommended integration boundary

Highest-risk conflicts are shared preselection misclassifying native Seedance, live new pricing guards bypassing Molii immutable selection, deleting recursive multiplier protection, replacing durable polling with upstream direct settlement, or overwriting custom Responses tool accounting during terminal delivery fixes. These require explicit merge resolution and focused PG/deterministic regression coverage. The upstream timeout/Kling/header/schema-order/write-budget changes are otherwise bounded additive changes. Keep separate ownership coordination for catalog pricing fields, cache12h DTOs and controller deadline-call removals; those cross this research domain.

Root coordination addendum: billing research identified extreme signed-integer overflow in `relaykit/relayconvert/internal/shared/claude/cache.go` `NormalizeCacheCreationSplit`, newly reached by service expression normalization. The routing/relaykit implementation owner must bound that helper and add deterministic integer-boundary coverage while preserving valid partial/unsplit cache semantics; coordinate the DTO field contract with the billing owner. This finding was supplied by root and was not independently implemented or tested in this read-only research task.
