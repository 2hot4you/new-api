"""PostgreSQL deployment acceptance, with explicit coverage boundaries.

Legacy database packages are not run as complete suites: coverage is limited to
the mandatory PostgreSQL inventory instead of implicit cross-engine fixtures.
Database-independent packages and relaykit keep their complete suites. Never
describe this as the old make test.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import test_catalog_postgres_gate as catalog

ROOT = Path(__file__).resolve().parents[2]
# Audited test bodies have no SQL initialization or implicit database TestMain.
FULL_PACKAGES = [
    "./common",
    "./constant",
    "./dto",
    "./internal/catalogtransport",
    "./logger",
    "./oauth",
    "./pkg/billingexpr",
    "./pkg/billingmoney",
    "./pkg/catalogmanifest",
    "./pkg/jsplugin",
    "./pkg/wsmanager",
    "./relay/channel",
    "./relay/channel/advancedcustom",
    "./relay/channel/ali",
    "./relay/channel/aws",
    "./relay/channel/claude",
    "./relay/channel/codex",
    "./relay/channel/gemini",
    "./relay/channel/minimax",
    "./relay/channel/moonshot",
    "./relay/channel/ollama",
    "./relay/channel/openai",
    "./relay/channel/sub2api",
    "./relay/channel/task/seedanceprotocol",
    "./relay/channel/tencent",
    "./relay/common",
    "./relay/constant",
    "./setting",
    "./setting/brand_setting",
    "./setting/config",
    "./setting/model_setting",
    "./setting/operation_setting",
    "./setting/ratio_setting",
    "./setting/reasoning",
    "./setting/system_setting",
    "./setting/task_pricing_setting",
    "./plugins",
    "./relay/channel/task/bytedanceseedance",
    "./relay/channel/task/moliigrok",
    "./relay/channel/task/starai",
    "./setting/billing_setting"
]
# Top-level failures from run 38047800232. Removing any from mandatory execution
# is a gate failure, even if all remaining tests pass.
FAILED_REGRESSIONS = [
    "TestUpdateOptionAliasBillingExprUsesPluginSchema",
    "TestPreConsumePolicyDatabaseMatrix",
    "TestGetUserModelsExpandsAutoGroupsInConfiguredOrder",
    "TestListModelsIncludesTieredBillingModel",
    "TestModelManagementDatabaseMatrix",
    "TestSharedModelPluginPricingDatabaseMatrix",
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
    "TestCatalogRuntimeModelPricingSave",
    "TestCatalogRuntimeOrdinaryOptions",
    "TestCatalogRuntimeOrdinaryRawNoop",
    "TestCatalogRuntimeManagedFailures",
    "TestCatalogRuntimeManagedFreshRecheck",
    "TestCatalogRuntimeManagedResetAndStrictAdoption",
    "TestCatalogRuntimeManagedUnknownMutation",
    "TestCatalogRuntimePublication",
    "TestCatalogRuntimeFailureRecovery",
    "TestCatalogRuntimeDrift",
    "TestCatalogRuntimeScopeAndAcquisition",
    "TestCatalogRuntimeLostAcknowledgement",
    "TestCatalogRuntimeCacheContention",
    "TestCatalogRuntimeAdvancedCacheContention",
    "TestCatalogRuntimeRebuildSQLContext",
    "TestCatalogRuntimeRecheckAfterRebuild",
    "TestCatalogRuntimeRejectsCorruptOperation",
    "TestCatalogRuntimeAdvancedSettingsReadOnly",
    "TestCatalogRuntimeGroupIndexContention",
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
    "TestCatalogSyncStore",
    "TestCatalogSyncStoreBaselineIncarnation",
    "TestCatalogSyncStoreOptionWritersAndNoop",
    "TestTaskRetryRecomputesInferredBillingAndPreservesOverride",
    "TestTaskSelfUseDefaultDoesNotMaskMappedExpression",
    "TestRelayTaskSubmitTieredUsesFrozenBillingModelCurrency",
    "TestRelayTaskSubmitAliasBillingIdentityAndExprFallback",
    "TestRelayTaskSubmitPerCallBillingIdentity",
    "TestEstimateTaskSubmitReusesBillingWithoutPreconsumingOrCallingUpstream",
    "TestSharedTaskBillingExpressionSelectionAndFrozenSettlement",
    "TestRelayTaskSubmitAcceptsAnySuccessfulUpstreamStatus",
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
    "TestInputPreConsumeMultiplierLegacyAndRequestPrices"
]
# These are excluded from complete-suite execution. Only tests explicitly in
# catalog.GROUPS run; some packages have no entries in that inventory.
LEGACY_DATABASE_PACKAGES = [
    ".", "./cmd/catalog-sync", "./controller", "./e2e", "./internal/catalogsync",
    "./middleware", "./model", "./pkg/perf_metrics", "./relay",
    "./relay/channel/moliigrok", "./relay/channel/task/jsplugin",
    "./relay/helper", "./router", "./service", "./service/authz",
]

def full_suite_commands(mode, packages):
    if mode not in ("normal", "race") or not packages or any(package not in FULL_PACKAGES for package in packages):
        raise ValueError("only audited complete database-independent suites are allowed")
    command = ["go", "test"] + (["-race"] if mode == "race" else [])
    command += ["-count=1", "-timeout=900s", "-json"]
    return [(ROOT, command + packages), (ROOT / "relaykit", command + ["./..."])]


def verify_full_suite(events, expected, no_test_packages=()):
    completed = {event.get("Package") for event in events
                 if event.get("Action") == "pass" and not event.get("Test")}
    tests = {event.get("Package") for event in events
             if event.get("Action") == "pass" and event.get("Test")}
    for event in events:
        empty_package = (event.get("Action") == "skip" and not event.get("Test")
                         and event.get("Package") in no_test_packages)
        if event.get("Action") in ("fail", "skip") and not empty_package:
            raise ValueError("suite failed or skipped: " + event.get("Test", event.get("Package", "")))
        if event.get("Action") == "run" and any(part in ("sqlite", "mysql") for part in event.get("Test", "").split("/")):
            raise ValueError("non-PostgreSQL subtest selected")
    if not expected or not set(expected) <= completed & tests:
        raise ValueError("every required package must pass real tests")
    return sum(event.get("Action") == "pass" and bool(event.get("Test")) for event in events)


def run_ci(mode):
    if mode not in ("normal", "race"):
        raise ValueError("expected normal or race")
    catalog.validate_dsn(os.environ.get("TEST_POSTGRES_DSN", ""))
    if not os.environ.get("TEST_REDIS_DSN"):
        raise ValueError("TEST_REDIS_DSN is required for the complete common suite")
    catalog.validate_regression_inventory(catalog.GROUPS, FAILED_REGRESSIONS)
    env = dict(os.environ, GOWORK="off", GOFLAGS="", CATALOG_SYNC_POSTGRES_ONLY="1")
    directory = Path(tempfile.mkdtemp(prefix="postgres-ci-" + mode + "-"))
    print("PostgreSQL CI evidence: " + str(directory), flush=True)
    print("Not run as complete suites; coverage limited to mandatory PG inventory: " + ", ".join(LEGACY_DATABASE_PACKAGES), flush=True)
    for index, (cwd, command) in enumerate(full_suite_commands(mode, FULL_PACKAGES)):
        packages = FULL_PACKAGES if cwd == ROOT else ["./..."]
        listing = subprocess.run(
            ["go", "list", "-f", "{{.ImportPath}} {{if or .TestGoFiles .XTestGoFiles}}tests{{else}}empty{{end}}"] + packages,
            cwd=cwd, env=env, capture_output=True, text=True, check=True)
        rows = [line.split() for line in listing.stdout.splitlines() if line.strip()]
        expected = [package for package, kind in rows if kind == "tests"]
        empty = [package for package, kind in rows if kind == "empty"]
        path = directory / ("full-suite-" + str(index) + ".jsonl")
        print("RUN " + mode + " complete " + ("root database-independent packages" if cwd == ROOT else "relaykit"), flush=True)
        with path.open("w") as output:
            result = subprocess.run(command, cwd=cwd, env=env, stdout=output, stderr=subprocess.STDOUT)
        (directory / ("full-suite-" + str(index) + ".command.json")).write_text(json.dumps(command))
        events = [json.loads(line) for line in path.read_text().splitlines() if line.startswith("{")]
        try:
            count = verify_full_suite(events, expected, empty)
            if result.returncode:
                raise ValueError("go test returned " + str(result.returncode))
        except ValueError as error:
            print("FAILED " + str(error) + "; evidence: " + str(path), flush=True)
            return 1
        print("PASS complete suite: " + str(len(expected)) + " packages, " + str(count) + " tests/subtests; zero test skips", flush=True)
    return catalog.run_gate(mode)


class PostgresCITest(unittest.TestCase):
    def test_failed_business_regressions_cannot_disappear(self):
        catalog.validate_regression_inventory(catalog.GROUPS, FAILED_REGRESSIONS)
        with self.assertRaisesRegex(ValueError, "required CI regression"):
            catalog.validate_regression_inventory([], FAILED_REGRESSIONS)

    def test_full_suites_reject_legacy_database_packages_and_wildcards(self):
        for package in ("./model", "./service", "./relay/helper", "./...", ".", "./middleware"):
            with self.subTest(package=package), self.assertRaises(ValueError):
                full_suite_commands("normal", [package])
        commands = full_suite_commands("race", FULL_PACKAGES)
        self.assertEqual(len(commands), 2)
        self.assertIn("-race", commands[0][1])
        self.assertNotIn("-run", commands[0][1])
        self.assertEqual(commands[1][0], ROOT / "relaykit")
        self.assertIn("./...", commands[1][1])

    def test_suite_verification_requires_every_package_and_nonempty_tests(self):
        good = [
            {"Action": "run", "Package": "example/p", "Test": "TestActual"},
            {"Action": "pass", "Package": "example/p", "Test": "TestActual"},
            {"Action": "pass", "Package": "example/p"},
        ]
        self.assertEqual(verify_full_suite(good, ["example/p"]), 1)
        for events in ([], good[:-1], good + [{"Action": "skip", "Test": "TestOther"}],
                       good + [{"Action": "fail", "Package": "example/p"}],
                       [{"Action": "pass", "Package": "example/p"}],
                       good + [{"Action": "run", "Test": "TestDB/sqlite"}]):
            with self.assertRaises(ValueError):
                verify_full_suite(events, ["example/p"])
        with self.assertRaises(ValueError):
            verify_full_suite(good, ["example/missing"])
        empty = good + [{"Action": "skip", "Package": "example/no-tests"}]
        self.assertEqual(verify_full_suite(empty, ["example/p"], ["example/no-tests"]), 1)
        with self.assertRaises(ValueError):
            verify_full_suite(empty, ["example/p"])

    def test_entrypoint_missing_dsn_fails_before_go(self):
        result = subprocess.run([sys.executable, str(Path(__file__).resolve()), "--run", "normal"],
                                env=dict(os.environ, TEST_POSTGRES_DSN=""),
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("TEST_POSTGRES_DSN", result.stderr)
        self.assertNotIn("RUN ", result.stdout)


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--run":
        try:
            raise SystemExit(run_ci(sys.argv[2]))
        except (ValueError, subprocess.CalledProcessError) as error:
            raise SystemExit(str(error))
    unittest.main()
