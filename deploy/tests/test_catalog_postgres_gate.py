"""Mandatory PostgreSQL catalog gate and tests for its result verifier.

Selections are an explicit audited inventory, not broad package test execution.
SQLite/MySQL-specific cases are intentionally outside this PostgreSQL gate.
"""
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from urllib.parse import urlsplit, parse_qs

ROOT = Path(__file__).resolve().parents[2]
GROUPS = [
  {
    "name": "router-rc46-access-token-route-inventory",
    "pkg": "./router",
    "tests": [
      "TestAccessTokenRouteRulesCoverEveryDashboardRoute",
      "TestAccessTokenCatalogCoversEveryPermission",
      "TestAccessTokenVerificationScopesAreAllListed"
    ],
    "matrix": False
  },
  {
    "name": "model-rc46-filtered-logs-and-export",
    "pkg": "./model",
    "tests": [
      "TestGetAllLogsGrokImageCategoryIsExactAndPaginated",
      "TestGetUserLogsGrokImageCategoryAndNoCategoryCompatibility",
      "TestGetUserLogsGPTImage2CategoryIsExact",
      "TestStreamImageLogsPreservesCategoryAndUserIsolation"
    ],
    "matrix": False
  },
  {
    "name": "task-jsplugin-rc46-pure-adaptor",
    "pkg": "./relay/channel/task/jsplugin",
    "tests": [
      "TestAlibabaSubmitDeltaDoesNotMutateControlState",
      "TestOAuth2JWTAuthCachesAndRefreshes",
      "TestOAuth2JWTContextDoesNotExposeServiceAccountKey",
      "TestPluginJSONValuesPreserveCodecNormalizationAndIsolation",
      "TestRequestDescriptorDecodingMatchesCodec",
      "TestSubmitContextExposesOriginTasks",
      "TestSubmitContextNormalizesRequestBodyOnEveryCall",
      "TestSubmitContextOmitsOriginTasksWhenEmpty",
      "TestSubmitContextOriginTasksNilDataOnInvalidJSON",
      "TestTaskAdaptorAcceptsNormalizedLegacyTokenCounters",
      "TestTaskAdaptorAllowsExplicitCredentiallessCDNRequest",
      "TestTaskAdaptorBatchBridge",
      "TestTaskAdaptorBatchQueryReceivesTaskObjects",
      "TestTaskAdaptorBoundsNativeUsageBeforeQuotaCalculation",
      "TestTaskAdaptorBuildContentRequestHookAndMissingFallback",
      "TestTaskAdaptorBuildSubmitReceivesMappedUpstreamModel",
      "TestTaskAdaptorBuildsMultipartFromOpaqueFileReference",
      "TestTaskAdaptorChainsDoubaoThroughNewAPIUpstream",
      "TestTaskAdaptorChainsSunoBatchFetchThroughNewAPIUpstream",
      "TestTaskAdaptorDoesNotEmitInjectedMultipartDispositionHeaders",
      "TestTaskAdaptorFetchTaskExposesModelIdentities",
      "TestTaskAdaptorFinalDecoderOmissionClearsCandidateSnapshots",
      "TestTaskAdaptorHeaderOverrides",
      "TestTaskAdaptorHookFetchReachesRequestHostsOnly",
      "TestTaskAdaptorInlinesJSONFilePlaceholders",
      "TestTaskAdaptorJSONFilePlaceholderErrors",
      "TestTaskAdaptorJSONOrderCapability",
      "TestTaskAdaptorMapsJSContract",
      "TestTaskAdaptorMasksChannelAddressInBuildFailure",
      "TestTaskAdaptorParseSubmitResponsePersistsState",
      "TestTaskAdaptorPreservesOpenAIVideoFailureSlotsAndOwnsLifecycle",
      "TestTaskAdaptorPreservesSoraVideoResponseFields",
      "TestTaskAdaptorQueryContextOmitsRequestBody",
      "TestTaskAdaptorReDecodesFinalCandidateAndRejectsModelDrift",
      "TestTaskAdaptorRejectsDeprecatedClientResponse",
      "TestTaskAdaptorRejectsFinalCandidateNonSubmitKind",
      "TestTaskAdaptorRejectsFinalCandidateOriginIntentDrift",
      "TestTaskAdaptorRejectsInvalidArtifactProjection",
      "TestTaskAdaptorRejectsNonObjectOpenAIVideoRendererOutput",
      "TestTaskAdaptorRejectsPostDistributionEndpointModelDrift",
      "TestTaskAdaptorRejectsRendererFromFinalProtocolDecoder",
      "TestTaskAdaptorRejectsRequestHostOverride",
      "TestTaskAdaptorSeparatesExpressionFactsFromLegacyBillingRatios",
      "TestTaskAdaptorSingleOversizedStateIsInvalidPoll",
      "TestTaskAdaptorUsageProfilesFollowExecutionModel",
      "TestTaskQueryRedirects",
      "TestTaskSubmitDeltaStreamContract",
      "TestTaskSubmitHooksReceiveIndependentRequestSnapshots",
      "TestTaskSubmitStreamCancellationClosesReader",
      "TestTaskSubmitStreamContract",
      "TestTaskSubmitStreamIdleTimeout",
      "TestTaskUsageValidationLeavesReturnedHostRequestUnchanged",
      "TestUpstreamContextsUseGatewayBearerOnNewAPIChannels"
    ],
    "matrix": False
  },
  {
    "name": "service-rc46-task-billing-and-refund",
    "pkg": "./service",
    "tests": [
      "TestApplyTaskBillingJobDoesNotReapplySucceededJob",
      "TestBuildTerminalTaskBillingJobUsesSnapshotEvaluationTime",
      "TestEvaluateTaskCompletionUsageUsesSnapshotEvaluationTime",
      "TestLogFailedStarAITaskRecordsSanitizedReasonAndElapsedTime",
      "TestLogSuccessfulStarAITaskCreatesSingleFinalConsumption",
      "TestLogTaskConsumptionIncludesTieredSnapshotUsageFacts",
      "TestLogTaskConsumptionMarksSynchronousProtocolsAndDiscardedArtifacts",
      "TestLogTaskConsumptionWithoutSnapshotKeepsRatioMode",
      "TestPollingChannelCacheBillingEnqueuesRefund",
      "TestPollingNullUpstreamBillingEnqueuesRefund",
      "TestRecalculate_ActualQuotaZero",
      "TestRecalculate_NegativeDelta",
      "TestRecalculate_PositiveDelta",
      "TestRecalculate_RejectsNegativeActualQuota",
      "TestRecalculate_Subscription_NegativeDelta",
      "TestRecalculate_ZeroDelta",
      "TestRefundTaskQuota_FundingFailureKeepsAccountingAndPendingMarker",
      "TestRefundTaskQuota_NoToken",
      "TestRefundTaskQuota_StarAIRefundsWithoutLog",
      "TestRefundTaskQuota_Subscription",
      "TestRefundTaskQuota_Wallet",
      "TestRefundTaskQuota_ZeroQuota",
      "TestServiceBillingPostgresDSNGuard",
      "TestSettle_NonPerCallBilling_AppliesAdaptorAdjustment",
      "TestSettle_PerCallBilling_AllowsOptInAdaptorAdjust",
      "TestSettle_PerCallBilling_SkipsAdaptorAdjust",
      "TestSettle_PerCallBilling_SkipsTotalTokens",
      "TestSettle_TieredEvaluationFailureKeepsPreConsumedCharge",
      "TestSettle_TieredFailureReturnsFalseForCallerRefund",
      "TestSettle_TieredSnapshotWriteBackUsesSettledFactsAndMatchedTier",
      "TestSettle_TieredSuccessStillRecomputes",
      "TestSettle_TieredUsageFactsMergeCompletionOverSubmission",
      "TestSettle_TokenRecalcFallsBackToCompletionTokens",
      "TestSweepTimedOutTasksHonorsRefundRolloutBoundary",
      "TestTaskBillingOtherCarriesPersistedSynchronousProtocolContract",
      "TestTaskBillingOtherFiltersHistoricalOtherRatios",
      "TestTaskBillingOtherIncludesTieredSnapshotAndKeepsUsageFactsNested",
      "TestTaskBillingOtherOmitsEmptyUsageFacts",
      "TestTaskBillingOtherSeparatesPluginAndRootDiagnostics",
      "TestTaskBillingReconciliationLogFailureDoesNotReplayFunds",
      "TestTaskBillingReconciliationRefund",
      "TestTaskBillingReconciliationSettleAndTargetZero",
      "TestUpdateBatchTasksEnqueuesDurableTieredSettlement",
      "TestUpdateSunoTasksStalePollsRefundExactlyOnce"
    ],
    "matrix": False
  },
  {
    "name": "model-rc46-schema-migration-idempotency",
    "pkg": "./model",
    "tests": ["TestPostgresFullSchemaMigrationIsIdempotent", "TestCatalogPricingNarrowedSchemaRetention"],
    "matrix": False
  },
  {
    "name": "controller-rc46-access-token-and-admin-proof",
    "pkg": "./controller",
    "tests": [
      "TestWriteAuthErrorI18nPreservesLegacyStatusAndAddsCode",
      "TestPasswordLoginInvalidRequestHasStableCode",
      "TestPasswordRegistrationInvalidRequestHasStableCode",
      "TestPasswordAuthFeatureFlagsHaveStableCodes",
      "TestWriteAuthInternalErrorDoesNotExposeCause",
      "TestAccessTokenLifecycleAndLateRequests",
      "TestAccessTokenCreationChecksGrantBeforeConsumingProof",
      "TestAccessTokenUpdateRenamesFreelyAndChangesGrantOnlyWithProof",
      "TestAccessTokenScopeDictionaryIgnoresTheViewersGrants",
      "TestScopedAccessTokenSecondaryChannelPermissions",
      "TestScopedAccessTokenArtifactOwnership",
      "TestAdminUserRiskOperationsRequireProofBeforeMutation",
      "TestAdminUserRoutineEditsDoNotRequireProof",
      "TestCreateUserRejectsNonStandardRole",
      "TestAdminUserProofIsBoundToTargetAndActionAndConsumedOnce",
      "TestAdminUserBindingProofDistinguishesBuiltInAndCustomBindings",
      "TestAdminUserVerificationPolicy",
      "TestSecurityEnrollmentProofConcurrentConsumption",
      "TestSecurityEnrollmentProofRequiresLiveRecordAndExactDeadline"
    ],
    "matrix": False
  },
  {
    "name": "middleware-rc46-scoped-token-authorization",
    "pkg": "./middleware",
    "tests": [
      "TestUserAuthAllowsOpaqueDottedPAT",
      "TestUserAuthNeverFallsBackForRecognizedInvalidInternalJWT",
      "TestTryUserAuthCredentialClassification",
      "TestUserAuthAppliesAccessTokenRouteRules",
      "TestAccessTokenIdentityAndSingleLookup",
      "TestRequirePermissionIntersectsScopedAccessTokenGrants",
      "TestSessionCookieOriginGuard",
      "TestSessionCookieOriginGuardDevelopmentCompatibility",
      "TestSessionCookieOriginGuardDoesNotTrustForwardedProtoFromClient"
    ],
    "matrix": False
  },
  {
    "name": "perf-metrics-rc46-query-boundaries",
    "pkg": "./pkg/perf_metrics",
    "tests": [
      "TestBuildQueryResultIncludesRequestCounts",
      "TestClassifyRelayOutcome",
      "TestStreamOutcomeClassification",
      "TestClientCancellationDuringUpstreamRead",
      "TestPerformanceWindowIncludesCurrentHour",
      "TestHourlySuccessSeriesWeightsSmallerBuckets",
      "TestRecordTaskResultSamplesTerminalTasks",
      "TestQuerySummaryAllBuildsWeightedGroupSummaries",
      "TestQuerySummaryAllExposesExactModelSampleCounts",
      "TestQuerySummaryAllRangeUsesExactCalendarBounds",
      "TestQuerySummaryAllAllowsAtMostOneYear",
      "TestQueryAllowsAtMostOneYear",
      "TestRecentHistoryRejectsOversizedModelLists"
    ],
    "matrix": False
  },
  {
    "name": "middleware-seedance-smart-duration",
    "pkg": "./middleware",
    "tests": [
      "TestSeedanceSmartDurationSharedEndpoint",
      "TestSeedanceSharedDurationPrecheckBounds"
    ],
    "matrix": False
  },
  {
    "name": "controller-ci-legacy-matrix",
    "pkg": "./controller",
    "tests": [
      "TestPreConsumePolicyDatabaseMatrix",
      "TestModelManagementDatabaseMatrix",
      "TestSharedModelPluginPricingDatabaseMatrix",
      "TestModelPricingConversionDatabaseMatrix",
      "TestVendorManagementDatabaseMatrix",
      "TestModelDeletionDatabaseMatrix"
    ],
    "matrix": True
  },
  {
    "name": "controller-ci-legacy-regressions",
    "pkg": "./controller",
    "tests": [
      "TestUpdateOptionAliasBillingExprUsesPluginSchema",
      "TestGetUserModelsExpandsAutoGroupsInConfiguredOrder",
      "TestListModelsIncludesTieredBillingModel",
      "TestCreateModelMetaPersistsNormalizedCatalogFields",
      "TestUpdateModelMetaRejectsInvalidCatalogFieldsWithoutChangingRow",
      "TestUpdateModelMetaRejectsChangedModelName",
      "TestCreateModelMetaAllowsCompleteMarketplacePublication",
      "TestCreateModelMetaAllowsIncompleteDraft",
      "TestUpdateModelMetaRejectsIncompleteNewPublicationWithoutChangingDraft",
      "TestUpdateModelMetaAllowsCompleteNewPublication",
      "TestUpdateModelMetaWithdrawsIncompletePublishedModel",
      "TestGetModelMetaDynamicBlockersDoNotChangePublicationIntent",
      "TestModelMarketplaceRuleCustomObjectEndpointUsesRuntimeEndpointSet",
      "TestModelMarketplaceRuleMatchedRuntimeEndpointsUseUnion",
      "TestModelMarketplaceRuleStaleEndpointArrayDoesNotCountAsRuntimeEndpoint",
      "TestKlingNativeRouteSubmitPollSettleAndQuery",
      "TestPrepareImageRequestBillingAllowsGrokImageIdentity",
      "TestPrepareGrokImageUnknownFileIDFailsBeforeSnapshotOrPreconsume",
      "TestPrepareImageRequestBillingPreservesOrdinaryImageMapping",
      "TestOrdinaryImageAttemptPreparationTracksSelectedChannelAndRetry",
      "TestMoliiGrokFailureCallsSelectedUpstreamOnceWithEligibleFallback",
      "TestRelayMoliiGrokImagePricingErrorDoesNotPanic",
      "TestImmediateTaskSettlementDatabase",
      "TestResponsesInterruptedStreamHealth",
      "TestResponsesWebSocketReusesConnectionAndSettlesEachRequest",
      "TestUpdateVendorMetaRefreshesPricingVendorIntroduction",
      "TestUpdateOptionRejectsInvalidTaskBillingExpressions",
      "TestUpdateOptionRejectsUsageExpressionWithoutTaskPlugin",
      "TestPreConsumeMultiplierRejectsInvalidRuntimeAndOverflow",
      "TestCreateModelMetaRejectsInvalidCatalogFields",
      "TestCreateModelMetaRejectsIncompleteMarketplacePublicationWithAllMissingFields",
      "TestModelMarketplaceBlockersAreStableAndComplete",
      "TestGetUserModelsFiltersByRequestedGroup",
      "TestListModelsUsesAdvancedCustomEndpointTypesFromPricingCache",
      "TestListModelsTokenLimitIncludesTieredBillingModel",
      "TestListModelsTokenLimitUsesResolvedCustomAutoGroups",
      "TestSetupLoginDoesNotTouchPasswordWhenPasswordFieldOmitted",
      "TestUpdateOptionSavesSeedanceMatrixAndGeneratedExpressionsTogether",
      "TestUpdateOptionRejectsIncompleteSeedancePriceMatrix",
      "TestMetadataSyncLocaleAndEndpointValidation"
    ],
    "matrix": False
  },
  {
    "name": "relay-ci-legacy-regressions",
    "pkg": "./relay",
    "tests": [
      "TestTaskRetryRecomputesInferredBillingAndPreservesOverride",
      "TestRelayTaskSubmitRetryRepricesAndReservesBeforeUpstream",
      "TestRealtimeFetchAppliesChannelHeaderOverrides",
      "TestRealtimeFetchRefundsTaskItFindsFailed",
      "TestTaskSelfUseDefaultDoesNotMaskMappedExpression",
      "TestRelayTaskSubmitTieredUsesFrozenBillingModelCurrency",
      "TestRelayTaskSubmitAliasBillingIdentityAndExprFallback",
      "TestRelayTaskSubmitPerCallBillingIdentity",
      "TestEstimateTaskSubmitReusesBillingWithoutPreconsumingOrCallingUpstream",
      "TestSharedTaskBillingExpressionSelectionAndFrozenSettlement",
      "TestRelayTaskSubmitAcceptsAnySuccessfulUpstreamStatus",
      "TestApplyChannelPinPreservesOriginTasksAndRetryMode",
      "TestRelayTaskSubmitMapsBeforeValidateWhenOriginSet",
      "TestRelayTaskSubmitDeclaredNameWithoutMappingIsUnchanged",
      "TestRelayTaskSubmitDoesNotApplyMappingTwice",
      "TestRelayTaskSubmitEmptyOriginKeepsLateMapping",
      "TestCatalogPricingPostgresDSNGuard"
    ],
    "matrix": False
  },
  {
    "name": "helper-ci-legacy-regressions",
    "pkg": "./relay/helper",
    "tests": [
      "TestMoliiGrokFixedPriceAnchorsAreAvailable",
      "TestPerCallPricingUsesBillingIdentity",
      "TestModelPriceHelperTieredUsesFrozenBillingModelCurrency",
      "TestModelPriceHelperLegacyRatioIgnoresBillingCurrencyMetadata",
      "TestModelPriceHelperTieredUsesPreloadedRequestInput",
      "TestFixedPricePreConsumeAndRealtimeRejection",
      "TestModelPriceHelperTieredInputPreConsumeMultiplier",
      "TestModelPriceHelperTieredRejectsPreConsumeOverflow",
      "TestModelPriceHelperRequestBillingRatiosOnlyApplyToFixedPrice",
      "TestModelPriceHelperUsesSuffixedOriginLikeMain",
      "TestModelPriceHelperHonorsCustomClaudeThinkingAlias",
      "TestModelPriceHelperCanonicalBillingLadder",
      "TestModelPriceHelperMigratesLegacyGeminiWildcardToCanonical",
      "TestModelPriceHelperModifierNameFallsBackToBase",
      "TestModelPriceHelperExemptAtNameBillsVerbatim",
      "TestModelPriceHelperPreservesGpt51CodexMaxIdentity",
      "TestModelPriceHelperNativeGeminiNoThinkingDoesNotAliasBillingModel",
      "TestInputPreConsumeMultiplierLegacyAndRequestPrices",
      "TestMoliiGrokModelsUseDirectCostAnchors",
      "TestHelperPostgresDSNGuard"
    ],
    "matrix": False
  },
  {
    "name": "common-json-contract",
    "pkg": "./common",
    "tests": ["TestValidateJsonNoDuplicateKeys", "TestHostJSONCodecConformance", "TestDecodeJsonWithValidation"],
    "matrix": False
  },
  {
    "name": "controller-pure-regressions",
    "pkg": "./controller",
    "tests": ["TestSecurityEnrollmentOperationContext", "TestSecurityEnrollmentPublicErrorsDiscardWrappedDetails", "TestMoliiImagineSyncRequestsNeverRetryOrSwitchChannel", "TestOrdinarySyncChannelRetainsRetryBehavior", "TestPrepareImageRequestBillingRejectsMappingBeforeEstimateOrPreconsume", "TestControllerPostgresDSNGuard"],
    "matrix": False
  },
  {
    "name": "model-store-pg",
    "pkg": "./model",
    "tests": [
      "TestCatalogSyncRestoreScope",
      "TestCatalogSyncRestoreGuards",
      "TestCatalogSyncRestorePreimages",
      "TestCatalogSyncRestoreMissingAndOwnership",
      "TestCatalogSyncRestoreBaselineIncarnations",
      "TestCatalogSyncRestoreAuthorizationAndRollback",
      "TestCatalogSyncRestoreLatestAndFallback",
      "TestCatalogSyncRestoreDeletedObject",
      "TestCatalogSyncRestoreDeletedModelDependency",
      "TestCatalogSyncRestorePriceModelRecreated",
      "TestCatalogSyncRestoreLostAck",
      "TestCatalogSyncHistoryPagination",
      "TestCatalogSyncBusinessPreview",
      "TestCatalogSyncBusinessApply",
      "TestCatalogSyncBusinessWaitedRoleDemotion",
      "TestCatalogSyncBusinessRollbackAndAuth",
      "TestCatalogSyncBusinessPricesAndDrift",
      "TestCatalogSyncBusinessTaskReferences",
      "TestCatalogSyncBusinessMidjourneyPriceRemoval",
      "TestCatalogSyncBusinessMatchRuleExpansion",
      "TestCatalogSyncBusinessRemovals",
      "TestCatalogSyncBusinessLostAckAndConcurrent",
      "TestCatalogSyncBusinessSpecialFrozenAndFallback",
      "TestCatalogSyncBusinessPluginDriftAndAdoption",
      "TestCatalogSyncBusinessAuthLayout",
      "TestCatalogSyncBusinessAuthTemporaryShadow",
      "TestCatalogSyncBusinessSeedanceFrozen",
      "TestCatalogSyncBusinessResolutionAndIncarnation",
      "TestCatalogSyncValidationAttestation",
      "TestCatalogSyncValidationMigration",
      "TestCatalogSyncValidationDependencies",
      "TestCatalogSyncStore",
      "TestCatalogSyncStoreBaselineIncarnation",
      "TestCatalogSyncStoreOptionWritersAndNoop",
      "TestCatalogSyncStoreReleasedUpgrade",
      "TestCatalogSyncSourceConcurrentMutation",
      "TestCatalogSyncStoreMigrations",
      "TestCatalogSyncChannelMigrations",
      "TestCatalogSyncReferenceFences",
      "TestCatalogSyncReferenceRootRollback",
      "TestCatalogSyncReferenceCommitLease",
      "TestCatalogSyncReferenceCancellationRollbackLease",
      "TestCatalogSyncReferenceSchemaAndFailure",
      "TestCatalogSyncReferenceWaitedSnapshot",
      "TestCatalogSyncReferenceRangesAndSession",
      "TestCatalogSyncReferenceTrustedNaming",
      "TestCatalogSyncReferenceSessionReset",
      "TestCatalogSyncReferenceDeadlockRollback",
      "TestCatalogSyncReferenceRealCommitFailure",
      "TestCatalogSyncReferenceRegistryBusy",
      "TestCatalogSyncReferenceUnsupportedTableLayouts"
    ],
    "matrix": True
  },
  {
    "name": "model-store-direct",
    "pkg": "./model",
    "tests": [
      "TestCatalogSyncGrokFixedPricingConflict",
      "TestCatalogSyncValidationPreservation",
      "TestCatalogSyncValidationRuntimeStates",
      "TestCatalogSyncSensitiveWriter",
      "TestCatalogSyncRefreshExcludesWriter",
      "TestCatalogSyncReferencePostgresPrivileges",
      "TestCatalogSyncReferencePostgresSearchPath",
      "TestCatalogSyncReferencePostgresSearchPathShadow",
      "TestCatalogSyncReferencePostgresUnsupportedNamespaces"
    ],
    "matrix": False
  },
  {
    "name": "model-runtime-pg",
    "pkg": "./model",
    "tests": [
      "TestModelPricingRemovalDatabaseMatrix",
      "TestCatalogRuntimeOrdinaryWriters",
      "TestCatalogRuntimeModelPricingSave",
      "TestCatalogRuntimeModelPricingOptions",
      "TestCatalogRuntimeModelPricingFreshPreparation",
      "TestCatalogRuntimeModelPricingSeedance",
      "TestCatalogRuntimeModelPricingFailureRecovery",
      "TestCatalogRuntimeModelPricingNativeAcknowledgements",
      "TestCatalogRuntimeOrdinaryOptions",
      "TestCatalogRuntimeOrdinaryRawNoop",
      "TestCatalogRuntimeOrdinaryHistoricalRetention",
      "TestCatalogRuntimeOrdinaryFailureRecovery",
      "TestCatalogRuntimeOrdinaryFreshRecheck",
      "TestCatalogRuntimeOrdinaryNativeAcknowledgements",
      "TestCatalogRuntimeOrdinaryMetadataCallbacks",
      "TestCatalogRuntimeOrdinaryCurrencyBoundary",
      "TestCatalogRuntimeOrdinaryUnknownMutation",
      "TestCatalogRuntimeManagedFailures",
      "TestCatalogRuntimeManagedFreshRecheck",
      "TestCatalogRuntimeManagedResetAndStrictAdoption",
      "TestCatalogRuntimeManagedUnknownMutation",
      "TestCatalogRuntimePublication",
      "TestCatalogRuntimeFailureRecovery",
      "TestCatalogRuntimeDrift",
      "TestCatalogRuntimeScopeAndAcquisition",
      "TestCatalogRuntimeLostAcknowledgement",
      "TestCatalogRuntimeAdvancedCacheContention",
      "TestCatalogRuntimeRebuildSQLContext",
      "TestCatalogRuntimeAdvancedSettingsReadOnly",
      "TestCatalogRuntimeGroupIndexContention"
    ],
    "matrix": True
  },
  {
    "name": "model-runtime-neutral",
    "pkg": "./model",
    "tests": [
      "TestCatalogRuntimeOrdinaryAliasDeletion",
      "TestCatalogRuntimeOrdinaryPreparationBudget",
      "TestCatalogRuntimeOrdinaryMissingPrerequisites",
      "TestCatalogRuntimeOrdinaryForeignRoot",
      "TestCatalogRuntimeResetAndHistoricalOrphan",
      "TestCatalogRuntimeReadyRecoveryFailureBlocksWrites",
      "TestCatalogRuntimeCacheContention",
      "TestCatalogRuntimeQueryHelperSemantics",
      "TestCatalogRuntimeRejectsInvalidCurrency",
      "TestCatalogRuntimeCompatibilityAndCacheRemoval",
      "TestCatalogRuntimeRecheckAfterRebuild",
      "TestCatalogRuntimeRejectsCorruptOperation",
      "TestCatalogRuntimeUnknownReadyState"
    ],
    "matrix": False
  },
  {
    "name": "controller-catalog_sync_test",
    "pkg": "./controller",
    "tests": [
      "TestCatalogSecurityManagementCredentials",
      "TestCatalogSecurityManagementLiveState",
      "TestCatalogSecurityStrictOrigin",
      "TestCatalogSecuritySourceGuard",
      "TestCatalogSecuritySourceSingleton",
      "TestCatalogSecurityAuthoritativeReadBoundary",
      "TestCatalogSyncConfiguration",
      "TestCatalogSyncReaderConfigAndAuthentication",
      "TestCatalogSyncReaderBoundedConcurrentRateLimit",
      "TestCatalogSyncReaderLimitsAndWindow",
      "TestCatalogSyncDisabledAndTargetConfigurations",
      "TestCatalogSyncConfiguredSourcePGWire",
      "TestCatalogSyncReceiptDoesNotMutateCommittedOperation",
      "TestCatalogSyncHistoryRejectsChangedProvenance",
      "TestCatalogSyncSourceReadOnlyCoherentSnapshot",
      "TestCatalogSyncSourceRejectsPrelockReplacement",
      "TestCatalogSyncSourceDDLStabilityAndCancellation",
      "TestCatalogSyncReceiptCurrentAuthorizationAndBinding",
      "TestCatalogSyncReceiptPhysicalAuthorizationStorage",
      "TestCatalogSyncReceiptAuthoritativeLocksAndCancellation",
      "TestCatalogSyncSourceRestrictedRole",
      "TestCatalogSyncReadCommitFailureReturnsNoClaim",
      "TestCatalogSyncSourceInputAndDetachedValidation",
      "TestCatalogSyncSourceRejectsUnsafePhysicalView",
      "TestCatalogSyncSourceConcurrentInheritance",
      "TestCatalogSyncReceiptConcurrentInheritance",
      "TestCatalogSyncSourceConcurrentSchemaSwap",
      "TestCatalogSyncReceiptConcurrentSchemaSwap",
      "TestCatalogSyncReadSchemaSwapABA",
      "TestCatalogSyncSourceConcurrentEmptySchemaSwap",
      "TestCatalogSyncReadInheritanceDetachedAfterCommit",
      "TestCatalogSyncReadRejectsHistoricalInheritanceHint",
      "TestCatalogSyncTargetHistoricalHintPolicyUnchanged",
      "TestCatalogSyncTargetNamespaceOrdinaryRollback",
      "TestCatalogSyncTargetNamespaceRelationReplacement",
      "TestCatalogSyncTargetNamespaceManagedRollback",
      "TestCatalogSyncTargetNamespaceShadowAuthorization",
      "TestCatalogSyncTargetNamespaceProofFailures",
      "TestCatalogSyncTargetNamespaceLostCommitAck",
      "TestCatalogSyncHeartbeatCreationAcrossSecond"
    ],
    "matrix": False
  },
  {
    "name": "controller-catalog_sync_http_test",
    "pkg": "./controller",
    "tests": [
      "TestCatalogSyncHTTPBusinessAndReceiptQuery",
      "TestCatalogSyncHTTPGrokPricingConflict",
      "TestCatalogSyncHTTPPricingConflictProjection",
      "TestCatalogSyncHTTPStrictBodies",
      "TestCatalogSyncHTTPUnavailableSourceCannotDelete",
      "TestCatalogSyncHTTPNetworkOutageCannotDelete",
      "TestCatalogSyncHTTPProofOrderingAndRestore",
      "TestCatalogSyncHTTPConfirmationUnitProjection",
      "TestCatalogSyncHTTPPendingReplayAndUnknownCommit",
      "TestCatalogSyncHTTPConcurrentProofAndExactBinding",
      "TestCatalogSyncHTTPAuthOriginAndAudit",
      "TestCatalogSyncHTTPSourceAndMissingOrigin",
      "TestCatalogSyncHTTPAuthoritativeSessions",
      "TestCatalogSyncHTTPDigestRepresentation",
      "TestCatalogSyncHTTPBlockResolveExpiryAndProofDeadline",
      "TestCatalogSyncHTTPStatusRoles"
    ],
    "matrix": False
  },
  {
    "name": "controller-catalog_image_pricing_test",
    "pkg": "./controller",
    "tests": [
      "TestCatalogImageRetrySelectsNewMappedPricing",
      "TestCatalogImagePendingRejectsBeforeMedia",
      "TestCatalogImageZeroAndAnchorStaySelected",
      "TestCatalogImageQualityCountAndQuotaBounds",
      "TestCatalogImageMediaPublicationOverlap"
    ],
    "matrix": False
  },
  {
    "name": "controller-security_enrollment_test",
    "pkg": "./controller",
    "tests": [
      "TestCatalogSecurityContext",
      "TestCatalogSecurityProofBindingAndConsumption",
      "TestCatalogSecurityFactorPolicy",
      "TestCatalogSecurityProofLifetimeAndLivePolicy",
      "TestCatalogSecurityVerificationHTTP",
      "TestCatalogSecurityDedicatedPasskey",
      "TestCatalogSecurityDedicatedOAuth"
    ],
    "matrix": False
  },
  {
    "name": "controller-task_plugin_test",
    "pkg": "./controller",
    "tests": [
      "TestInitialTaskPluginReadinessPostgres",
      "TestInitialTaskPluginSQLCancellationPostgres"
    ],
    "matrix": False
  },
  {
    "name": "controller-catalog_submit_integration_test",
    "pkg": "./controller",
    "tests": [
      "TestCatalogSubmitPersistedSelection",
      "TestCatalogSubmitNativeGrokSnapshot"
    ],
    "matrix": False
  },
  {
    "name": "relay-catalog_pricing_capture_test",
    "pkg": "./relay",
    "tests": [
      "TestCatalogRequestExpressionMoneyOverlap",
      "TestCatalogRequestToolSettlementRemainsSelected",
      "TestCatalogRequestToolPricesRemainSelected",
      "TestCatalogRequestPendingAndFrozenSelection",
      "TestCatalogRequestToolPrecedenceAndTieredSurcharge",
      "TestCatalogRequestAudioSettlementRemainsSelected",
      "TestCatalogRequestRealtimeRatesAndSaturation",
      "TestCatalogRequestCanonicalAndProviderInputs",
      "TestCatalogBuiltinFallbackCurrencyBoundary"
    ],
    "matrix": False
  },
  {
    "name": "relay-catalog_task_pricing_test",
    "pkg": "./relay",
    "tests": [
      "TestCatalogTaskExpressionPrecedence",
      "TestCatalogTaskPendingRejectsBeforeUsageHook",
      "TestCatalogTaskRetryClearsEstimateInputs",
      "TestCatalogTaskNativePrepareAndDurableCompletion",
      "TestCatalogTaskSelectedZeroMissingAndBounds",
      "TestCatalogTaskPluginHookPublicationOverlap",
      "TestCatalogTaskDirectConsumersRemainSelected"
    ],
    "matrix": False
  },
  {
    "name": "main_channel_cache_test",
    "pkg": ".",
    "tests": [
      "TestCatalogInitResourcesDefaultDisabled",
      "TestCatalogInitResourcesPending",
      "TestCatalogInitResourcesPlugin",
      "TestCatalogInitResourcesPartialPlugin",
      "TestCatalogInitResourcesRebuildFailure",
      "TestCatalogInitResourcesPermissionMissing",
      "TestCatalogInitResourcesUpgradeRepeat",
      "TestCatalogInitResourcesNonMaster",
      "TestCatalogInitResourcesCorruptSource"
    ],
    "matrix": False
  },
  {
    "name": "catalog_postgres_upgrade_test",
    "pkg": ".",
    "tests": [
      "TestCatalogPostgresReleasedBootTwice"
    ],
    "matrix": False
  },
  {
    "name": "model-catalog_sync_startup_test",
    "pkg": "./model",
    "tests": [
      "TestCatalogStartupEnrollmentRequired",
      "TestCatalogStartupEligibilityFacts",
      "TestCatalogStartupApplyRechecksHeartbeat",
      "TestCatalogStartupHeartbeatLock",
      "TestCatalogStartupHeartbeatTimestamps",
      "TestCatalogStartupPendingEligibility",
      "TestCatalogStartupFinalAckEligibility",
      "TestCatalogStartupInstanceStorage",
      "TestCatalogStartupMissingAbilityCache"
    ],
    "matrix": False
  },
  {
    "name": "model-catalog_sync_alias_removal_test",
    "pkg": "./model",
    "tests": [
      "TestCatalogAliasRemovalMultipleChannels",
      "TestCatalogAliasRemovalFreshProof",
      "TestCatalogAliasRemovalPostProofAndGuards",
      "TestCatalogAliasRemovalPendingRecovery",
      "TestCatalogAliasRemovalPublicationBoundary",
      "TestCatalogAliasRemovalNativeAcknowledgements",
      "TestCatalogAliasRemovalSQLRollback",
      "TestCatalogAliasRemovalFoldedSurvivor",
      "TestCatalogAliasRemovalAliasCacheContention",
      "TestCatalogAliasRemovalMemoryRecovery",
      "TestCatalogAliasRemovalMemoryProjection",
      "TestCatalogAliasRemovalMemoryFailure",
      "TestCatalogAliasRemovalOrdinaryRefreshOverlap",
      "TestCatalogAliasRemovalMemorySQLCancellation",
      "TestCatalogAliasAggregationDetached"
    ],
    "matrix": False
  },
  {
    "name": "model-catalog_sync_candidate_test",
    "pkg": "./model",
    "tests": [
      "TestCatalogCandidateDefaultsIsolation",
      "TestCatalogCandidateCompleteReplacement",
      "TestCatalogCandidateMalformedDoesNotMutate",
      "TestCatalogCandidateWholeScopeReset",
      "TestCatalogCandidateOrphanIsNotManagedCompleteness",
      "TestCatalogCandidateProspectiveRetention",
      "TestCatalogCandidateExplicitDependencies",
      "TestCatalogCandidateTypedOwnershipAndConcurrentReads"
    ],
    "matrix": False
  },
  {
    "name": "model-catalog_sync_postgres_gate_test",
    "pkg": "./model",
    "tests": [
      "TestCatalogPostgresHistoryLoadAndRollback",
      "TestCatalogPostgresDSNGuard"
    ],
    "matrix": False
  },
  {
    "name": "model-catalog_sync_pricing_identities_test",
    "pkg": "./model",
    "tests": [
      "TestCatalogPricingIdentitiesPending"
    ],
    "matrix": False
  },
  {
    "name": "model-catalog_sync_reload_test",
    "pkg": "./model",
    "tests": [
      "TestCatalogMixedPasskeySave",
      "TestCatalogReloadConcurrentOrdinary",
      "TestCatalogReloadDeletedPrice",
      "TestCatalogReloadSQLCancellation",
      "TestCatalogReloadErrorsAndFreshOptions",
      "TestCatalogReloadRejectsObsoleteOptionsAndFX",
      "TestCatalogMixedPasskeyNativeAcknowledgements",
      "TestCatalogMixedPasskeyConfirmationAndRollback",
      "TestCatalogMixedPasskeyInvalidAndPending"
    ],
    "matrix": False
  },
  {
    "name": "pkg-catalogmanifest-diff_test",
    "pkg": "./pkg/catalogmanifest",
    "tests": [
      "TestManagedEmptyRestoredBaseline",
      "TestManagedPlanOperationBinding",
      "TestManagedStructuralValidationDoesNotEvaluate",
      "TestManagedIncarnationAndConfirmationUnit",
      "TestResolveManagedPlan",
      "TestManagedDeletionConsentAndBlockedReferences",
      "TestManagedMetadataProvenance",
      "TestManagedPlanDigestAndTampering",
      "TestManagedRecreatedRemovalBlocked",
      "TestManagedPreservedLocalPriceCurrencyBlocked",
      "TestManagedCorruptBaselineRejected",
      "TestManagedCanonicalIdentitiesAndValues",
      "TestManagedBlockedDigestOrderingAndResolution",
      "TestManagedCurrencyAndExpressionsGrouped",
      "TestManagedThreeWayDiff",
      "TestManagedSourceSafety"
    ],
    "matrix": False
  },
  {
    "name": "internal-catalogtransport-transport_test",
    "pkg": "./internal/catalogtransport",
    "tests": [
      "TestCatalogTransportFixedTLSAndEnvelope",
      "TestCatalogTransportRejectsUnsafeDestinationsAndRebinding",
      "TestCatalogTransportRejectsWireFailures",
      "TestCatalogTransportDecompressionLimitAndTruncation",
      "TestCatalogTransportDeadlineAndCancellation",
      "TestCatalogTransportConcurrentRequestCancellation",
      "TestCatalogTransportActualFifteenSecondTimeout",
      "TestCatalogTransportDuplicateHeaders",
      "TestCatalogTransportPreservesPricesAndRejectsInvalidExpression",
      "TestCatalogTransportBodyCancellationAndCorruptGzip"
    ],
    "matrix": False
  },
  {
    "name": "internal-catalogsync-catalogsync_test",
    "pkg": "./internal/catalogsync",
    "tests": [
      "TestExportIncludesCompleteCatalogAndOnlyPricingOptions",
      "TestExportRejectsModelWithMissingVendor",
      "TestBuildPlanPreservesTargetOnlyCatalogAndReconcilesManagedPricing",
      "TestBuildPlanDigestIsDeterministic",
      "TestApplyRequiresCurrentConfirmationAndCreatesBackupBeforeMutation",
      "TestApplyRollsBackWhenBackupCannotBeWritten",
      "TestApplyRollsBackWhenBackupCannotBeSynced",
      "TestManagedExportCompletePricingContract",
      "TestManagedExportSnapshotIntegrity",
      "TestManagedExportCanonicalIdentityAndMetadata",
      "TestManagedExportCanonicalJSON"
    ],
    "matrix": False
  },
  {
    "name": "export-pg",
    "pkg": "./internal/catalogsync",
    "tests": [
      "TestManagedExportDatabaseMatrix"
    ],
    "matrix": True
  },
  {
    "name": "cmd-catalog-sync-main_test",
    "pkg": "./cmd/catalog-sync",
    "tests": [
      "TestRunHelpListsSafeThreeStepWorkflow",
      "TestRunRejectsMissingDSNEnvironmentVariable",
      "TestRunApplyRequiresConfirmationBeforeConnecting",
      "TestRunPlanDoesNotModifyTargetDatabase",
      "TestRunDatabaseErrorDoesNotExposePassword"
    ],
    "matrix": False
  }
]

def validate_dsn(dsn):
    try:
        url = urlsplit(dsn)
        query = parse_qs(url.query, strict_parsing=True, keep_blank_values=True)
        valid = (url.scheme in ("postgres", "postgresql")
                 and url.hostname in ("127.0.0.1", "::1")
                 and bool(url.username) and bool(url.path.strip("/"))
                 and not url.fragment and query == {"sslmode": ["disable"]})
    except ValueError:
        valid = False
    if not valid:
        raise ValueError("required loopback TEST_POSTGRES_DSN URL with sslmode=disable")

def validate_regression_inventory(groups, required):
    selected = {name for group in groups for name in group["tests"]}
    missing = sorted(set(required) - selected)
    if missing:
        raise ValueError("required CI regression missing from PostgreSQL acceptance: " + ", ".join(missing))

def verify_events(events, required, matrix=False):
    passed = {e.get("Test") for e in events if e.get("Action") == "pass" and e.get("Test")}
    for event in events:
        if event.get("Action") in ("skip", "fail"):
            # Only structural diagnostics are public: never echo SQL, DSNs or assertion values.
            test = event.get("Test")
            context = set()
            if test:
                for output in events:
                    if output.get("Action") == "output" and output.get("Test") == test:
                        context.update(re.findall(r"\b[A-Za-z0-9_]+_test\.go:[0-9]+\b|\bSQLSTATE [0-9A-Z]{5}\b",
                                                  output.get("Output", "")))
            detail = "; " + ", ".join(sorted(context)[:8]) if context else ""
            raise ValueError("catalog test failed or skipped: " + event.get("Test", event.get("Package", "")) + detail)
        if event.get("Action") == "run" and any(part in ("sqlite", "mysql") for part in event.get("Test", "").split("/")):
            raise ValueError("non-PostgreSQL subtest selected")
    for name in required:
        if name not in passed:
            raise ValueError("required test did not pass: " + name)
        if matrix and not any(x == name + "/postgres" or x.startswith(name + "/postgres/") for x in passed):
            raise ValueError("required PostgreSQL subtest did not pass: " + name)
    if not required or not passed:
        raise ValueError("empty selection cannot establish catalog acceptance")
    return len(passed)

def run_gate(mode):
    validate_dsn(os.environ.get("TEST_POSTGRES_DSN", ""))
    env = dict(os.environ, CATALOG_SYNC_POSTGRES_ONLY="1", TEST_SECURITY_DIALECT="postgres", GOFLAGS="", GOWORK="off",
               TEST_CATALOG_SYNC_POSTGRES_DSN=os.environ["TEST_POSTGRES_DSN"])
    directory = Path(tempfile.mkdtemp(prefix="catalog-postgres-" + mode + "-"))
    print("Catalog gate logs: " + str(directory), flush=True)
    # Negative tests exercise actual guarded TestMain, never the legacy branch.
    if mode == "normal":
        for name, package, test, override, diagnostic in [
            ("missing-dsn", "./model", "TestCatalogPostgresDSNGuard", {"TEST_POSTGRES_DSN": ""}, "requires a loopback TEST_POSTGRES_DSN"),
            ("invalid-mode", "./model", "TestCatalogPostgresDSNGuard", {"CATALOG_SYNC_POSTGRES_ONLY": "invalid"}, "must be exactly 1"),
            ("host-override", "./model", "TestCatalogPostgresDSNGuard", {"TEST_POSTGRES_DSN": "postgresql://fixture@127.0.0.1/task?sslmode=disable&host=remote.example"}, "requires a loopback TEST_POSTGRES_DSN"),
            ("helper-missing-dsn", "./relay/helper", "TestPerCallPricingUsesBillingIdentity", {"TEST_POSTGRES_DSN": ""}, "requires a loopback TEST_POSTGRES_DSN"),
            ("helper-invalid-mode", "./relay/helper", "TestPerCallPricingUsesBillingIdentity", {"CATALOG_SYNC_POSTGRES_ONLY": "invalid"}, "must be exactly 1"),
            ("helper-host-override", "./relay/helper", "TestPerCallPricingUsesBillingIdentity", {"TEST_POSTGRES_DSN": "postgresql://fixture@127.0.0.1/task?sslmode=disable&host=remote.example"}, "reject ambiguous TEST_POSTGRES_DSN overrides"),
            ("helper-dbname-override", "./relay/helper", "TestHelperPostgresDSNGuard", {"TEST_POSTGRES_DSN": "postgresql://fixture@127.0.0.1/task?sslmode=disable&dbname=task"}, "reject ambiguous TEST_POSTGRES_DSN overrides"),
            ("helper-database-override", "./relay/helper", "TestHelperPostgresDSNGuard", {"TEST_POSTGRES_DSN": "postgresql://fixture@127.0.0.1/task?sslmode=disable&database=task"}, "reject ambiguous TEST_POSTGRES_DSN overrides"),
        ]:
            command = ["go", "test", package, "-run", "^" + test + "$", "-count=1", "-timeout=30s"]
            result = subprocess.run(command, cwd=ROOT, env=dict(env, **override), capture_output=True, text=True)
            output = result.stdout + result.stderr
            (directory / (name + ".log")).write_text(output)
            if result.returncode == 0 or diagnostic not in output:
                raise RuntimeError("PostgreSQL prerequisite did not fail closed: " + name)
    total = 0
    for group in GROUPS:
        selector = "^(" + "|".join(group["tests"]) + ")$" + ("/^postgres$" if group["matrix"] else "")
        command = ["go", "test"]
        if mode == "race":
            command.append("-race")
        if group["pkg"] == "./controller":
            command += ["-tags", "catalogsynctest"]
        command += [group["pkg"], "-run", selector, "-count=1", "-failfast", "-timeout=900s", "-json"]
        print("RUN " + mode + " " + group["name"], flush=True)
        path = directory / (group["name"] + ".jsonl")
        with path.open("w") as output:
            result = subprocess.run(command, cwd=ROOT, env=env, stdout=output, stderr=subprocess.STDOUT)
        (directory / (group["name"] + ".command.json")).write_text(json.dumps(command))
        try:
            events = [json.loads(line) for line in path.read_text().splitlines() if line.startswith("{")]
            count = verify_events(events, group["tests"], group["matrix"])
            if result.returncode:
                raise ValueError("go test returned " + str(result.returncode))
        except ValueError as error:
            print("FAILED " + str(error) + "; full evidence: " + str(path), flush=True)
            return 1
        total += count
        print("PASS " + group["name"] + ": " + str(count) + " tests/subtests; zero skips", flush=True)
    print("PASS mandatory catalog " + mode + ": " + str(total) + " tests/subtests, " + str(len(GROUPS)) + " exact selections", flush=True)
    return 0

class CatalogGateTest(unittest.TestCase):
    def test_pricing_conflict_regressions_are_required_in_nonmatrix_groups(self):
        groups = {group["name"]: group for group in GROUPS}
        for name, package, required in [
            ("model-store-direct", "./model", ["TestCatalogSyncGrokFixedPricingConflict"]),
            ("controller-catalog_sync_http_test", "./controller", [
                "TestCatalogSyncHTTPGrokPricingConflict", "TestCatalogSyncHTTPPricingConflictProjection"]),
        ]:
            with self.subTest(group=name):
                group = groups[name]
                self.assertEqual(group["pkg"], package)
                self.assertFalse(group["matrix"])
                self.assertLessEqual(set(required), set(group["tests"]))

    def test_required_dsn_rejects_redirects_and_omission(self):
        for value in ("", "host=127.0.0.1 dbname=task", "postgresql://u@remote/task?sslmode=disable",
                      "postgresql://u@127.0.0.1/task?sslmode=disable&host=remote",
                      "postgresql://u@127.0.0.1/task?sslmode=disable&host=",
                      "postgresql://u@127.0.0.1/task?sslmode=disable&service=remote",
                      "postgresql://u@127.0.0.1/task?sslmode=disable&port=1"):
            with self.assertRaises(ValueError):
                validate_dsn(value)
        validate_dsn("postgresql://u:p@127.0.0.1:5432/task?sslmode=disable")

    def test_empty_skipped_failed_and_unexecuted_pg_cannot_pass(self):
        passed = [{"Action": "pass", "Test": "TestRequired"}]
        for events in ([], passed + [{"Action": "skip", "Test": "TestRequired/postgres"}],
                       passed + [{"Action": "fail", "Package": "p"}],
                       passed + [{"Action": "run", "Test": "TestRequired/sqlite"}]):
            with self.assertRaises(ValueError):
                verify_events(events, ["TestRequired"], True)
        with self.assertRaises(ValueError):
            verify_events(passed, ["TestMissing"])

    def test_actual_parent_and_pg_child_required_diagnostics_allowed(self):
        events = [{"Action": "output", "Output": "expected killed-driver diagnostic"},
                  {"Action": "pass", "Test": "TestRequired/postgres"},
                  {"Action": "pass", "Test": "TestRequired"}]
        self.assertEqual(verify_events(events, ["TestRequired"], True), 2)

    def test_failure_reports_location_and_sqlstate_without_raw_output(self):
        events = [
            {"Action": "output", "Test": "TestDDL", "Output":
             "Error Trace: /private/path/controller/catalog_sync_test.go:846\n"
             "load models: password=private-token (SQLSTATE 0A000)\n"},
            {"Action": "output", "Test": "TestOther", "Output":
             "other_test.go:99 (SQLSTATE 08000)"},
            {"Action": "fail", "Test": "TestDDL"},
        ]
        with self.assertRaises(ValueError) as failure:
            verify_events(events, ["TestDDL"])
        diagnostic = str(failure.exception)
        self.assertIn("catalog_sync_test.go:846", diagnostic)
        self.assertIn("SQLSTATE 0A000", diagnostic)
        for private in ("private-token", "/private/path", "load models", "other_test.go", "08000"):
            self.assertNotIn(private, diagnostic)

    def test_gate_missing_dsn_exits_before_go(self):
        result = subprocess.run(["bash", str(ROOT / "deploy/tests/catalog_postgres_gate.sh"), "normal"],
                                env=dict(os.environ, TEST_POSTGRES_DSN=""), capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("TEST_POSTGRES_DSN", result.stderr)
        self.assertNotIn("RUN ", result.stdout)

if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--run":
        if sys.argv[2] not in ("normal", "race"):
            raise SystemExit("expected normal or race")
        try:
            raise SystemExit(run_gate(sys.argv[2]))
        except (ValueError, RuntimeError) as error:
            raise SystemExit(str(error))
    unittest.main()
