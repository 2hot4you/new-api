package model

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupModelCatalogReconcileTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "catalog-reconcile.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&Channel{}, &Ability{}, &Vendor{}, &Model{}); err != nil {
		t.Fatalf("migrate catalog tables: %v", err)
	}
	DB = db
	t.Cleanup(func() { DB = previousDB })
}

func addEnabledCatalogAbility(t *testing.T, modelName string) {
	t.Helper()
	channel := Channel{Name: "catalog", Key: "test", Status: common.ChannelStatusEnabled, Models: modelName, Group: "default"}
	if err := DB.Create(&channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if err := DB.Create(&Ability{Group: "default", Model: modelName, ChannelId: channel.Id, Enabled: true}).Error; err != nil {
		t.Fatalf("create ability: %v", err)
	}
}

func TestReconcileEnabledModelMetadataCreatesUnpublishedDraftOnce(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "deepseek-v4-flash-202605")

	first, err := ReconcileEnabledModelMetadata()
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if first.CreatedModels != 1 || first.CreatedVendors != 0 {
		t.Fatalf("unexpected first summary: %+v", first)
	}

	var entry Model
	if err := DB.Where("model_name = ?", "deepseek-v4-flash-202605").First(&entry).Error; err != nil {
		t.Fatalf("read reconciled model: %v", err)
	}
	if entry.DisplayName != entry.ModelName || entry.Description != "" || entry.ContextLength != 0 || entry.VendorID != 0 || entry.Status != 1 || entry.MarketplaceEnabled {
		t.Fatalf("discovered model must remain an unpublished metadata draft: %+v", entry)
	}

	second, err := ReconcileEnabledModelMetadata()
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if second != (CatalogReconcileSummary{}) {
		t.Fatalf("second reconcile must be a no-op: %+v", second)
	}
}

func TestReconcileEnabledModelMetadataDoesNotBackfillExistingDraft(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "qwen3.5-plus")
	draft := Model{ModelName: "qwen3.5-plus", DisplayName: "Qwen 3.5 Plus", Status: 1, SyncOfficial: 1}
	if err := draft.Insert(); err != nil {
		t.Fatalf("insert draft: %v", err)
	}

	summary, err := ReconcileEnabledModelMetadata()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if summary != (CatalogReconcileSummary{}) {
		t.Fatalf("existing metadata drafts must not be modified: %+v", summary)
	}

	var stored Model
	if err := DB.First(&stored, draft.Id).Error; err != nil {
		t.Fatalf("read draft: %v", err)
	}
	if stored.DisplayName != "Qwen 3.5 Plus" || stored.Description != "" || stored.VendorID != 0 || stored.MarketplaceEnabled {
		t.Fatalf("draft metadata was unexpectedly inferred: %+v", stored)
	}
}

func TestReconcileEnabledModelMetadataPreservesAdminAndTombstones(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "qwen3.5-plus")
	addEnabledCatalogAbility(t, "kimi-k3")

	customVendor := Vendor{Name: "Custom Vendor", Description: "custom", Icon: "Custom", Status: 1}
	if err := customVendor.Insert(); err != nil {
		t.Fatalf("insert custom vendor: %v", err)
	}
	custom := Model{
		ModelName: "qwen3.5-plus", Description: "admin description", Icon: "AdminIcon", VendorID: customVendor.Id,
		Status: 0, SyncOfficial: 0, Capabilities: []string{"admin-capability"}, ContextLength: 42,
	}
	if err := custom.Insert(); err != nil {
		t.Fatalf("insert custom model: %v", err)
	}
	tombstone := Model{ModelName: "kimi-k3", Status: 1, SyncOfficial: 1}
	if err := tombstone.Insert(); err != nil {
		t.Fatalf("insert tombstone: %v", err)
	}
	if err := tombstone.Delete(); err != nil {
		t.Fatalf("delete tombstone: %v", err)
	}

	if _, err := ReconcileEnabledModelMetadata(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var stored Model
	if err := DB.Unscoped().First(&stored, custom.Id).Error; err != nil {
		t.Fatalf("read custom model: %v", err)
	}
	if stored.Description != "admin description" || stored.Icon != "AdminIcon" || stored.VendorID != customVendor.Id ||
		stored.Status != 0 || stored.ContextLength != 42 || len(stored.Capabilities) != 1 || stored.Capabilities[0] != "admin-capability" {
		t.Fatalf("admin metadata was overwritten: %+v", stored)
	}
	var kimiCount int64
	if err := DB.Unscoped().Model(&Model{}).Where("model_name = ?", "kimi-k3").Count(&kimiCount).Error; err != nil {
		t.Fatalf("count tombstones: %v", err)
	}
	if kimiCount != 1 {
		t.Fatalf("soft-deleted model was restored, count=%d", kimiCount)
	}
}

func TestReconcileEnabledModelMetadataCreatesMinimalUnknownModel(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	addEnabledCatalogAbility(t, "unknown-catalog-model")

	if _, err := ReconcileEnabledModelMetadata(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var entry Model
	if err := DB.Where("model_name = ?", "unknown-catalog-model").First(&entry).Error; err != nil {
		t.Fatalf("read unknown model: %v", err)
	}
	if entry.Description != "" || entry.VendorID != 0 || entry.ContextLength != 0 || len(entry.Capabilities) != 0 {
		t.Fatalf("unknown model received invented metadata: %+v", entry)
	}
}

func TestReconcileEnabledModelMetadataIgnoresDisabledChannels(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	channel := Channel{Name: "disabled", Key: "test", Status: common.ChannelStatusManuallyDisabled, Models: "glm-5.2", Group: "default"}
	if err := DB.Create(&channel).Error; err != nil {
		t.Fatalf("create disabled channel: %v", err)
	}
	if err := DB.Create(&Ability{Group: "default", Model: "glm-5.2", ChannelId: channel.Id, Enabled: true}).Error; err != nil {
		t.Fatalf("create ability: %v", err)
	}

	if _, err := ReconcileEnabledModelMetadata(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var count int64
	if err := DB.Model(&Model{}).Count(&count).Error; err != nil {
		t.Fatalf("count models: %v", err)
	}
	if count != 0 {
		t.Fatalf("disabled channel model was reconciled")
	}
}

func TestReconcileEnabledModelMetadataAppendsMarketplaceDisplayOrder(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	first := &Model{ModelName: "existing-one"}
	second := &Model{ModelName: "existing-two"}
	if err := first.Insert(); err != nil {
		t.Fatalf("insert first existing model: %v", err)
	}
	if err := second.Insert(); err != nil {
		t.Fatalf("insert second existing model: %v", err)
	}
	addEnabledCatalogAbility(t, "reconciled-three")

	summary, err := ReconcileEnabledModelMetadata()
	if err != nil {
		t.Fatalf("reconcile enabled model metadata: %v", err)
	}
	if summary.CreatedModels != 1 {
		t.Fatalf("expected one reconciled model, got %+v", summary)
	}
	var reconciled Model
	if err := DB.Where("model_name = ?", "reconciled-three").First(&reconciled).Error; err != nil {
		t.Fatalf("load reconciled model: %v", err)
	}
	if reconciled.DisplayOrder != 3 {
		t.Fatalf("reconciled display order = %d, want 3", reconciled.DisplayOrder)
	}

	if err := ReorderModels([]int{reconciled.Id, second.Id, first.Id}); err != nil {
		t.Fatalf("reorder including reconciled model: %v", err)
	}
	ordered := orderedModelIDs(t, DB)
	want := []int{reconciled.Id, second.Id, first.Id}
	if len(ordered) != len(want) {
		t.Fatalf("ordered model count = %d, want %d", len(ordered), len(want))
	}
	for index := range want {
		if ordered[index] != want[index] {
			t.Fatalf("ordered model ids = %v, want %v", ordered, want)
		}
	}
}

func TestReconcileEnabledModelMetadataSerializesWithReorder(t *testing.T) {
	setupModelCatalogReconcileTestDB(t)
	first := &Model{ModelName: "existing-one"}
	second := &Model{ModelName: "existing-two"}
	if err := first.Insert(); err != nil {
		t.Fatalf("insert first existing model: %v", err)
	}
	if err := second.Insert(); err != nil {
		t.Fatalf("insert second existing model: %v", err)
	}
	addEnabledCatalogAbility(t, "reconciled-three")

	createReached := make(chan struct{})
	releaseCreate := make(chan struct{})
	var pauseOnce sync.Once
	callbackName := "test:pause_reconciled_model_create"
	if err := DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			pauseOnce.Do(func() {
				close(createReached)
				<-releaseCreate
			})
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	t.Cleanup(func() { _ = DB.Callback().Create().Remove(callbackName) })

	reconcileResult := make(chan error, 1)
	go func() {
		_, err := ReconcileEnabledModelMetadata()
		reconcileResult <- err
	}()
	select {
	case <-createReached:
	case <-time.After(5 * time.Second):
		t.Fatal("reconciled create did not reach pause")
	}

	reorderResult := make(chan error, 1)
	go func() { reorderResult <- ReorderModels([]int{second.Id, first.Id}) }()
	select {
	case err := <-reorderResult:
		close(releaseCreate)
		t.Fatalf("reorder returned before reconciled create committed: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(releaseCreate)
	if err := <-reconcileResult; err != nil {
		t.Fatalf("reconcile enabled model metadata: %v", err)
	}
	if err := <-reorderResult; !errors.Is(err, ErrMarketplaceOrderConflict) {
		t.Fatalf("reorder error = %v, want conflict", err)
	}
}
