# Adaptive Seedance duration review

## Cause and change

The shared task validator exempted only ChannelTypeStarAI from its generic positive-duration rule for the -1 automatic-duration sentinel. ChannelTypeByteDanceSeedance, introduced for reseller instances, was omitted. Its own protocol validator already accepts -1 for all four supported models and its serializer preserves it.

Added the reseller channel to the same narrowly scoped exception. Effective duration still undergoes model-specific validation before submission. No pricing or final settlement formula was changed.

## Verification

- Regression tests failed before the change with the exact reported seconds must be between 1 and 3600 error for duration=-1 and seconds="-1".
- Complete package tests passed for relay/common, relay/channel/task/bytedanceseedance, relay/channel/task/starai, and relay/channel/task/seedanceprotocol.
- The same four package suites passed with -race.
- Covers raw JSON content requests, all four models, both supported duration field forms, preservation of -1 in the outgoing JSON, rejection on unrelated channels, and rejection of -2 and 3601.
- An independent read-only reviewer found no actionable issue, including effective-duration validation and billing consequences.
- git diff --check passed.

## Broader test limitations

Attempted go test ./.... Root package setup failed because main.go embeds web/dist, which is absent in this isolated worktree. Controller tests also exposed catalog/bootstrap/PostgreSQL fixture failures and streaming failures. The broader run was explicitly interrupted after these failures rather than claiming a full pass; targeted suites and their race runs completed normally.

TestImmediateTaskSettlementDatabase was separately reproduced on the unchanged baseline worktree at the same base commit: it fails with catalog publication pending recovery before duration validation, independently of this patch.

The following top-level controller test failures were observed in the interrupted broader run (not all were separately baseline-tested):

- `TestUpdateOptionAliasBillingExprUsesPluginSchema`
- `TestPreConsumePolicyDatabaseMatrix`
- `TestPreConsumeMultiplierRejectsInvalidRuntimeAndOverflow`
- `TestCatalogImageRetrySelectsNewMappedPricing`
- `TestCatalogImagePendingRejectsBeforeMedia`
- `TestCatalogImageZeroAndAnchorStaySelected`
- `TestCatalogImageQualityCountAndQuotaBounds`
- `TestCatalogImageMediaPublicationOverlap`
- `TestCatalogSubmitPersistedSelection`
- `TestCatalogSubmitNativeGrokSnapshot`
- `TestCatalogSecurityManagementCredentials`
- `TestCatalogSecurityManagementLiveState`
- `TestCatalogSecurityStrictOrigin`
- `TestCatalogSecuritySourceGuard`
- `TestCatalogSecurityAuthoritativeReadBoundary`
- `TestCatalogSyncConfiguredSourcePGWire`
- `TestCatalogSyncHeartbeatCreationAcrossSecond`
- `TestCatalogSyncReceiptDoesNotMutateCommittedOperation`
- `TestCatalogSyncHistoryRejectsChangedProvenance`
- `TestCatalogSyncSourceReadOnlyCoherentSnapshot`
- `TestCatalogSyncSourceRejectsPrelockReplacement`
- `TestCatalogSyncSourceDDLStabilityAndCancellation`
- `TestCatalogSyncReceiptCurrentAuthorizationAndBinding`
- `TestCatalogSyncReceiptPhysicalAuthorizationStorage`
- `TestCatalogSyncReceiptAuthoritativeLocksAndCancellation`
- `TestCatalogSyncSourceRestrictedRole`
- `TestCatalogSyncReadCommitFailureReturnsNoClaim`
- `TestCatalogSyncSourceInputAndDetachedValidation`
- `TestCatalogSyncSourceRejectsUnsafePhysicalView`
- `TestCatalogSyncSourceConcurrentInheritance`
- `TestCatalogSyncReceiptConcurrentInheritance`
- `TestCatalogSyncSourceConcurrentSchemaSwap`
- `TestCatalogSyncReceiptConcurrentSchemaSwap`
- `TestCatalogSyncReadSchemaSwapABA`
- `TestCatalogSyncSourceConcurrentEmptySchemaSwap`
- `TestCatalogSyncReadInheritanceDetachedAfterCommit`
- `TestCatalogSyncReadRejectsHistoricalInheritanceHint`
- `TestCatalogSyncTargetHistoricalHintPolicyUnchanged`
- `TestCatalogSyncTargetNamespaceOrdinaryRollback`
- `TestCatalogSyncTargetNamespaceRelationReplacement`
- `TestCatalogSyncTargetNamespaceManagedRollback`
- `TestCatalogSyncTargetNamespaceShadowAuthorization`
- `TestCatalogSyncTargetNamespaceProofFailures`
- `TestCatalogSyncTargetNamespaceLostCommitAck`
- `TestListModelsIncludesTieredBillingModel`
- `TestModelManagementDatabaseMatrix`
- `TestSharedModelPluginPricingDatabaseMatrix`
- `TestCreateModelMetaPersistsNormalizedCatalogFields`
- `TestUpdateModelMetaRejectsInvalidCatalogFieldsWithoutChangingRow`
- `TestUpdateModelMetaRejectsChangedModelName`
- `TestCreateModelMetaAllowsCompleteMarketplacePublication`
- `TestCreateModelMetaAllowsIncompleteDraft`
- `TestUpdateModelMetaRejectsIncompleteNewPublicationWithoutChangingDraft`
- `TestUpdateModelMetaAllowsCompleteNewPublication`
- `TestUpdateModelMetaWithdrawsIncompletePublishedModel`
- `TestGetModelMetaDynamicBlockersDoNotChangePublicationIntent`
- `TestModelMarketplaceRuleCustomObjectEndpointUsesRuntimeEndpointSet`
- `TestModelMarketplaceRuleMatchedRuntimeEndpointsUseUnion`
- `TestModelMarketplaceRuleStaleEndpointArrayDoesNotCountAsRuntimeEndpoint`
- `TestKlingNativeRouteSubmitPollSettleAndQuery`
- `TestPrepareImageRequestBillingAllowsGrokImageIdentity`
- `TestPrepareGrokImageUnknownFileIDFailsBeforeSnapshotOrPreconsume`
- `TestPrepareImageRequestBillingPreservesOrdinaryImageMapping`
- `TestOrdinaryImageAttemptPreparationTracksSelectedChannelAndRetry`
- `TestMoliiGrokFailureCallsSelectedUpstreamOnceWithEligibleFallback`
- `TestRelayMoliiGrokImagePricingErrorDoesNotPanic`
- `TestImmediateTaskSettlementDatabase`
- `TestResponsesInterruptedStreamHealth`
- `TestResponsesWebSocketReusesConnectionAndSettlesEachRequest`

## External review availability

The required CCG antigravity/Claude dual-model wrapper ~/.claude/bin/codeagent-wrapper is absent. The available independent subagent review is recorded above as a fallback, not represented as that dual external review.

## Delivery

Prepared and verified locally on fix/seedance-adaptive-duration. No remote push, branch integration, production deployment, paid generation request, or server modification was performed.
