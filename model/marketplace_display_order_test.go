package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMarketplaceOrderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "marketplace-order.db")+"?_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&marketplaceOrderLock{}, &Model{}, &Vendor{}))
	require.NoError(t, ensureMarketplaceOrderLock(db))
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
	})
	return db
}

func createModels(t *testing.T, db *gorm.DB, models ...Model) []int {
	t.Helper()
	ids := make([]int, 0, len(models))
	for index := range models {
		require.NoError(t, db.Create(&models[index]).Error)
		ids = append(ids, models[index].Id)
	}
	return ids
}

func createVendors(t *testing.T, db *gorm.DB, vendors ...Vendor) []int {
	t.Helper()
	ids := make([]int, 0, len(vendors))
	for index := range vendors {
		require.NoError(t, db.Create(&vendors[index]).Error)
		ids = append(ids, vendors[index].Id)
	}
	return ids
}

func createOrderedModels(t *testing.T, db *gorm.DB, names ...string) []int {
	t.Helper()
	models := make([]Model, 0, len(names))
	for index, name := range names {
		models = append(models, Model{ModelName: name, DisplayOrder: index + 1})
	}
	return createModels(t, db, models...)
}

func createOrderedVendors(t *testing.T, db *gorm.DB, names ...string) []int {
	t.Helper()
	vendors := make([]Vendor, 0, len(names))
	for index, name := range names {
		vendors = append(vendors, Vendor{Name: name, DisplayOrder: index + 1})
	}
	return createVendors(t, db, vendors...)
}

func loadModelOrders(t *testing.T, db *gorm.DB) map[string]int {
	t.Helper()
	var models []Model
	require.NoError(t, db.Find(&models).Error)
	orders := make(map[string]int, len(models))
	for _, item := range models {
		orders[item.ModelName] = item.DisplayOrder
	}
	return orders
}

func loadVendorOrders(t *testing.T, db *gorm.DB) map[string]int {
	t.Helper()
	var vendors []Vendor
	require.NoError(t, db.Find(&vendors).Error)
	orders := make(map[string]int, len(vendors))
	for _, item := range vendors {
		orders[item.Name] = item.DisplayOrder
	}
	return orders
}

func orderedModelIDs(t *testing.T, db *gorm.DB) []int {
	t.Helper()
	var ids []int
	require.NoError(t, db.Model(&Model{}).Order("display_order ASC").Order("id ASC").Pluck("id", &ids).Error)
	return ids
}

func orderedVendorIDs(t *testing.T, db *gorm.DB) []int {
	t.Helper()
	var ids []int
	require.NoError(t, db.Model(&Vendor{}).Order("display_order ASC").Order("id ASC").Pluck("id", &ids).Error)
	return ids
}

func modelIDs(items []*Model) []int {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	return ids
}

func vendorIDs(items []*Vendor) []int {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}
	return ids
}

func TestMarketplaceDisplayOrderSchema(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	assert.True(t, db.Migrator().HasColumn(&Model{}, "display_order"))
	assert.True(t, db.Migrator().HasColumn(&Vendor{}, "display_order"))
	assert.True(t, db.Migrator().HasTable(&marketplaceOrderLock{}))
	var locks int64
	require.NoError(t, db.Model(&marketplaceOrderLock{}).Count(&locks).Error)
	assert.Equal(t, int64(1), locks)
}

func TestInitializeMarketplaceDisplayOrdersBackfillsOnlyUnsetRows(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	createModels(t, db,
		Model{ModelName: "older", ReleaseDate: "2026-01-01"},
		Model{ModelName: "newer", ReleaseDate: "2026-08-01"},
		Model{ModelName: "invalid-b", ReleaseDate: "August 2026"},
		Model{ModelName: "invalid-a"},
		Model{ModelName: "pinned", DisplayOrder: 7},
	)

	require.NoError(t, InitializeMarketplaceDisplayOrders(db))
	first := loadModelOrders(t, db)
	require.NoError(t, InitializeMarketplaceDisplayOrders(db))
	assert.Equal(t, first, loadModelOrders(t, db))
	assert.Less(t, first["newer"], first["older"])
	assert.Less(t, first["older"], first["invalid-a"])
	assert.Less(t, first["invalid-a"], first["invalid-b"])
	assert.Equal(t, 7, first["pinned"])
	for name, order := range first {
		assert.Positivef(t, order, "%s should have a positive display order", name)
	}
}

func TestInitializeMarketplaceDisplayOrdersBackfillsVendorsByID(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	createVendors(t, db,
		Vendor{Name: "first"},
		Vendor{Name: "pinned", DisplayOrder: 1},
		Vendor{Name: "third"},
	)

	require.NoError(t, InitializeMarketplaceDisplayOrders(db))
	orders := loadVendorOrders(t, db)
	assert.Equal(t, 1, orders["pinned"])
	assert.Equal(t, 2, orders["first"])
	assert.Equal(t, 3, orders["third"])
}

func TestMarketplaceDisplayOrderSequentialCreatesAppendAfterMaximum(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	createModels(t, db, Model{ModelName: "pinned-model", DisplayOrder: 7})
	createVendors(t, db, Vendor{Name: "pinned-vendor", DisplayOrder: 9})

	firstModel := &Model{ModelName: "first-model"}
	secondModel := &Model{ModelName: "second-model"}
	firstVendor := &Vendor{Name: "first-vendor"}
	secondVendor := &Vendor{Name: "second-vendor"}
	require.NoError(t, firstModel.Insert())
	require.NoError(t, secondModel.Insert())
	require.NoError(t, firstVendor.Insert())
	require.NoError(t, secondVendor.Insert())

	assert.Equal(t, 8, firstModel.DisplayOrder)
	assert.Equal(t, 9, secondModel.DisplayOrder)
	assert.Equal(t, 10, firstVendor.DisplayOrder)
	assert.Equal(t, 11, secondVendor.DisplayOrder)
}

func TestMarketplaceDisplayOrderMetadataUpdatesPreserveOrdinaryFormOrder(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	modelID := createModels(t, db, Model{ModelName: "model", DisplayOrder: 4})[0]
	vendorID := createVendors(t, db, Vendor{Name: "vendor", DisplayOrder: 6})[0]

	require.NoError(t, (&Model{Id: modelID, ModelName: "model", Description: "edited"}).Update())
	require.NoError(t, (&Vendor{Id: vendorID, Name: "vendor", Description: "edited"}).Update())
	assert.Equal(t, 4, loadModelOrders(t, db)["model"])
	assert.Equal(t, 6, loadVendorOrders(t, db)["vendor"])

	require.NoError(t, (&Model{Id: modelID, ModelName: "model", DisplayOrder: 8}).Update())
	require.NoError(t, (&Vendor{Id: vendorID, Name: "vendor", DisplayOrder: 10}).Update())
	assert.Equal(t, 4, loadModelOrders(t, db)["model"])
	assert.Equal(t, 6, loadVendorOrders(t, db)["vendor"])
}

func TestMarketplaceDisplayOrderConcurrentAppendsUseUniquePositions(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	const count = 12
	results := make(chan error, count)
	var start sync.WaitGroup
	start.Add(1)
	for index := 0; index < count; index++ {
		go func(index int) {
			start.Wait()
			results <- insertModel(db, &Model{ModelName: fmt.Sprintf("concurrent-%02d", index)})
		}(index)
	}
	start.Done()
	for index := 0; index < count; index++ {
		require.NoError(t, <-results)
	}

	var orders []int
	require.NoError(t, db.Model(&Model{}).Order("display_order ASC").Pluck("display_order", &orders).Error)
	require.Equal(t, count, len(orders))
	for index, order := range orders {
		assert.Equal(t, index+1, order)
	}
}

func TestMarketplaceDisplayOrderDirectDeletesUseSharedLock(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	modelID := createModels(t, db, Model{ModelName: "delete-model", DisplayOrder: 1})[0]
	vendorID := createVendors(t, db, Vendor{Name: "delete-vendor", DisplayOrder: 1})[0]
	lockVersion := func() int64 {
		var row marketplaceOrderLock
		require.NoError(t, db.First(&row, "name = ?", marketplaceOrderLockName).Error)
		return row.Version
	}

	before := lockVersion()
	require.NoError(t, db.Delete(&Model{}, modelID).Error)
	assert.Equal(t, before+1, lockVersion())
	require.NoError(t, db.Delete(&Vendor{}, vendorID).Error)
	assert.Equal(t, before+2, lockVersion())
}

func TestMarketplaceDisplayOrderReorderSeesConcurrentCreate(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	ids := createOrderedModels(t, db, "one", "two")

	createReached := make(chan struct{})
	releaseCreate := make(chan struct{})
	var pauseOnce sync.Once
	callbackName := "test:pause_marketplace_model_create"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			pauseOnce.Do(func() {
				close(createReached)
				<-releaseCreate
			})
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })

	createResult := make(chan error, 1)
	go func() {
		createResult <- insertModel(db, &Model{ModelName: "three"})
	}()
	select {
	case <-createReached:
	case <-time.After(5 * time.Second):
		t.Fatal("create did not pause after acquiring the marketplace order lock")
	}

	reorderResult := make(chan error, 1)
	go func() { reorderResult <- reorderModels(db, []int{ids[1], ids[0]}) }()
	select {
	case err := <-reorderResult:
		close(releaseCreate)
		t.Fatalf("reorder returned before the concurrent create committed: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(releaseCreate)
	require.NoError(t, <-createResult)
	require.ErrorIs(t, <-reorderResult, ErrMarketplaceOrderConflict)
}

func openMarketplaceOrderPostgresPair(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MARKETPLACE_ORDER_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("MARKETPLACE_ORDER_POSTGRES_TEST_DSN is not set")
	}
	open := func() *gorm.DB {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
		return db
	}
	first, second := open(), open()
	require.NoError(t, first.AutoMigrate(&marketplaceOrderLock{}, &Model{}, &Vendor{}))
	require.NoError(t, first.Exec("TRUNCATE TABLE models, vendors, marketplace_order_locks RESTART IDENTITY CASCADE").Error)
	require.NoError(t, ensureMarketplaceOrderLock(first))
	return first, second
}

func TestMarketplaceDisplayOrderPostgresConcurrentAppendsUseUniquePositions(t *testing.T) {
	first, second := openMarketplaceOrderPostgresPair(t)
	connections := []*gorm.DB{first, second}
	const count = 16
	results := make(chan error, count)
	var start sync.WaitGroup
	start.Add(1)
	for index := 0; index < count; index++ {
		go func(index int) {
			start.Wait()
			results <- insertModel(connections[index%len(connections)], &Model{ModelName: fmt.Sprintf("postgres-%02d", index)})
		}(index)
	}
	start.Done()
	for index := 0; index < count; index++ {
		require.NoError(t, <-results)
	}

	var orders []int
	require.NoError(t, first.Model(&Model{}).Order("display_order ASC").Pluck("display_order", &orders).Error)
	require.Equal(t, count, len(orders))
	for index, order := range orders {
		assert.Equal(t, index+1, order)
	}
}

func TestMarketplaceDisplayOrderPostgresReorderSeesConcurrentCreate(t *testing.T) {
	creator, reorderer := openMarketplaceOrderPostgresPair(t)
	one := &Model{ModelName: "one"}
	two := &Model{ModelName: "two"}
	require.NoError(t, insertModel(creator, one))
	require.NoError(t, insertModel(creator, two))

	createReached := make(chan struct{})
	releaseCreate := make(chan struct{})
	var pauseOnce sync.Once
	callbackName := "test:pause_postgres_marketplace_model_create"
	require.NoError(t, creator.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "models" {
			pauseOnce.Do(func() {
				close(createReached)
				<-releaseCreate
			})
		}
	}))
	t.Cleanup(func() { _ = creator.Callback().Create().Remove(callbackName) })

	createResult := make(chan error, 1)
	go func() { createResult <- insertModel(creator, &Model{ModelName: "three"}) }()
	select {
	case <-createReached:
	case <-time.After(5 * time.Second):
		t.Fatal("PostgreSQL create did not pause after acquiring the marketplace order lock")
	}

	reorderResult := make(chan error, 1)
	go func() { reorderResult <- reorderModels(reorderer, []int{two.Id, one.Id}) }()
	select {
	case err := <-reorderResult:
		close(releaseCreate)
		t.Fatalf("PostgreSQL reorder returned before the concurrent create committed: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(releaseCreate)
	require.NoError(t, <-createResult)
	require.ErrorIs(t, <-reorderResult, ErrMarketplaceOrderConflict)
}

func TestReorderModelsRejectsInvalidIDs(t *testing.T) {
	tests := []struct {
		name string
		ids  []int
	}{
		{name: "duplicate", ids: []int{1, 1}},
		{name: "zero", ids: []int{0, 1}},
		{name: "negative", ids: []int{-1, 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newMarketplaceOrderTestDB(t)
			err := ReorderModels(test.ids)
			require.ErrorIs(t, err, ErrMarketplaceOrderInvalid)
		})
	}
}

func TestReorderModelsRejectsIncompleteSetWithoutPartialWrite(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	ids := createOrderedModels(t, db, "one", "two", "three")

	err := ReorderModels([]int{ids[2], ids[0]})
	require.ErrorIs(t, err, ErrMarketplaceOrderConflict)
	assert.Equal(t, ids, orderedModelIDs(t, db))
}

func TestReorderModelsRejectsUnknownIDWithoutPartialWrite(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	ids := createOrderedModels(t, db, "one", "two", "three")

	err := ReorderModels([]int{ids[2], ids[1], ids[0] + 1000})
	require.ErrorIs(t, err, ErrMarketplaceOrderConflict)
	assert.Equal(t, ids, orderedModelIDs(t, db))
}

func TestReorderModelsAppliesCompleteOrderAndExcludesSoftDeletedRows(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	ids := createOrderedModels(t, db, "one", "two", "deleted")
	require.NoError(t, db.Delete(&Model{}, ids[2]).Error)

	require.NoError(t, ReorderModels([]int{ids[1], ids[0]}))
	items, err := GetModelOrderItems()
	require.NoError(t, err)
	assert.Equal(t, []int{ids[1], ids[0]}, modelIDs(items))
}

func TestReorderVendorsAppliesCompleteOrder(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	ids := createOrderedVendors(t, db, "one", "two", "three")

	require.NoError(t, ReorderVendors([]int{ids[2], ids[0], ids[1]}))
	assert.Equal(t, []int{ids[2], ids[0], ids[1]}, orderedVendorIDs(t, db))
	items, err := GetVendorOrderItems()
	require.NoError(t, err)
	assert.Equal(t, []int{ids[2], ids[0], ids[1]}, vendorIDs(items))
}

func TestReorderModelsRollsBackWhenUpdateFails(t *testing.T) {
	db := newMarketplaceOrderTestDB(t)
	ids := createOrderedModels(t, db, "one", "two", "three")
	require.NoError(t, db.Exec(fmt.Sprintf(`
		CREATE TRIGGER fail_model_order_update
		BEFORE UPDATE OF display_order ON models
		WHEN OLD.id = %d
		BEGIN
			SELECT RAISE(ABORT, 'forced display order update failure');
		END`, ids[1])).Error)

	err := ReorderModels([]int{ids[2], ids[1], ids[0]})
	require.Error(t, err)
	assert.Equal(t, ids, orderedModelIDs(t, db))
}
