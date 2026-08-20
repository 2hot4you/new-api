package model

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const marketplaceOrderLockName = "marketplace_display_order"

type marketplaceOrderLock struct {
	Name    string `gorm:"primaryKey;size:64"`
	Version int64  `gorm:"not null;default:0"`
}

func (marketplaceOrderLock) TableName() string {
	return "marketplace_order_locks"
}

var (
	ErrMarketplaceOrderInvalid  = errors.New("marketplace display order is invalid")
	ErrMarketplaceOrderConflict = errors.New("marketplace display order does not match current items")
)

func InitializeMarketplaceDisplayOrders(db *gorm.DB) error {
	return withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
		if err := initializeModelDisplayOrders(tx); err != nil {
			return err
		}
		return initializeVendorDisplayOrders(tx)
	})
}

func ensureMarketplaceOrderLock(db *gorm.DB) error {
	if !db.Migrator().HasTable(&marketplaceOrderLock{}) {
		if err := db.AutoMigrate(&marketplaceOrderLock{}); err != nil {
			return err
		}
	}
	var count int64
	if err := db.Model(&marketplaceOrderLock{}).
		Where("name = ?", marketplaceOrderLockName).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&marketplaceOrderLock{
		Name: marketplaceOrderLockName,
	}).Error
}

func acquireMarketplaceOrderLock(tx *gorm.DB) error {
	result := tx.Model(&marketplaceOrderLock{}).
		Where("name = ?", marketplaceOrderLockName).
		UpdateColumn("version", gorm.Expr("version + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("marketplace display-order lock row is missing")
	}
	var row marketplaceOrderLock
	return lockForUpdate(tx).
		Where("name = ?", marketplaceOrderLockName).
		First(&row).Error
}

func withMarketplaceOrderTransaction(db *gorm.DB, operation func(tx *gorm.DB) error) error {
	if err := ensureMarketplaceOrderLock(db); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := acquireMarketplaceOrderLock(tx); err != nil {
			return err
		}
		return operation(tx)
	})
}

func initializeModelDisplayOrders(tx *gorm.DB) error {
	var models []Model
	if err := lockForUpdate(tx).
		Select("id", "model_name", "release_date", "display_order").
		Find(&models).Error; err != nil {
		return err
	}

	used := make(map[int]struct{}, len(models))
	unset := make([]Model, 0, len(models))
	for _, item := range models {
		if item.DisplayOrder > 0 {
			used[item.DisplayOrder] = struct{}{}
			continue
		}
		unset = append(unset, item)
	}
	sort.Slice(unset, func(i, j int) bool {
		leftDate, leftErr := time.Parse("2006-01-02", unset[i].ReleaseDate)
		rightDate, rightErr := time.Parse("2006-01-02", unset[j].ReleaseDate)
		switch {
		case leftErr == nil && rightErr != nil:
			return true
		case leftErr != nil && rightErr == nil:
			return false
		case leftErr == nil && rightErr == nil && !leftDate.Equal(rightDate):
			return leftDate.After(rightDate)
		case unset[i].ModelName != unset[j].ModelName:
			return unset[i].ModelName < unset[j].ModelName
		default:
			return unset[i].Id < unset[j].Id
		}
	})
	return assignUnusedModelDisplayOrders(tx, unset, used)
}

func assignUnusedModelDisplayOrders(tx *gorm.DB, models []Model, used map[int]struct{}) error {
	next := 1
	for _, item := range models {
		for {
			if _, exists := used[next]; !exists {
				break
			}
			next++
		}
		if err := tx.Model(&Model{}).
			Where("id = ? AND display_order <= 0", item.Id).
			UpdateColumn("display_order", next).Error; err != nil {
			return err
		}
		used[next] = struct{}{}
		next++
	}
	return nil
}

func initializeVendorDisplayOrders(tx *gorm.DB) error {
	var vendors []Vendor
	if err := lockForUpdate(tx).
		Select("id", "display_order").
		Order("id ASC").
		Find(&vendors).Error; err != nil {
		return err
	}

	used := make(map[int]struct{}, len(vendors))
	unset := make([]Vendor, 0, len(vendors))
	for _, item := range vendors {
		if item.DisplayOrder > 0 {
			used[item.DisplayOrder] = struct{}{}
			continue
		}
		unset = append(unset, item)
	}
	next := 1
	for _, item := range unset {
		for {
			if _, exists := used[next]; !exists {
				break
			}
			next++
		}
		if err := tx.Model(&Vendor{}).
			Where("id = ? AND display_order <= 0", item.Id).
			UpdateColumn("display_order", next).Error; err != nil {
			return err
		}
		used[next] = struct{}{}
		next++
	}
	return nil
}

func nextMarketplaceDisplayOrder(tx *gorm.DB, entity interface{}) (int, error) {
	var maximum int
	if err := tx.Model(entity).
		Select("COALESCE(MAX(display_order), 0)").
		Scan(&maximum).Error; err != nil {
		return 0, err
	}
	return maximum + 1, nil
}

func GetModelOrderItems() ([]*Model, error) {
	var models []*Model
	err := DB.Order("display_order ASC").Order("id ASC").Find(&models).Error
	return models, err
}

func GetVendorOrderItems() ([]*Vendor, error) {
	var vendors []*Vendor
	err := DB.Order("display_order ASC").Order("id ASC").Find(&vendors).Error
	return vendors, err
}

func ReorderModels(orderedIDs []int) error {
	return reorderModels(DB, orderedIDs)
}

func reorderModels(db *gorm.DB, orderedIDs []int) error {
	return withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
		currentIDs, err := loadAndLockCurrentIDs(tx, "models")
		if err != nil {
			return err
		}
		if err := validateCompleteOrder(orderedIDs, currentIDs); err != nil {
			return err
		}
		for index, id := range orderedIDs {
			if err := tx.Model(&Model{}).Where("id = ?", id).
				UpdateColumn("display_order", index+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func ReorderVendors(orderedIDs []int) error {
	return reorderVendors(DB, orderedIDs)
}

func reorderVendors(db *gorm.DB, orderedIDs []int) error {
	return withMarketplaceOrderTransaction(db, func(tx *gorm.DB) error {
		currentIDs, err := loadAndLockCurrentIDs(tx, "vendors")
		if err != nil {
			return err
		}
		if err := validateCompleteOrder(orderedIDs, currentIDs); err != nil {
			return err
		}
		for index, id := range orderedIDs {
			if err := tx.Model(&Vendor{}).Where("id = ?", id).
				UpdateColumn("display_order", index+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func loadAndLockCurrentIDs(tx *gorm.DB, table string) ([]int, error) {
	var ids []int
	err := lockForUpdate(tx.Table(table)).
		Select("id").
		Where("deleted_at IS NULL").
		Order("display_order ASC").
		Order("id ASC").
		Scan(&ids).Error
	return ids, err
}

func validateCompleteOrder(orderedIDs []int, currentIDs []int) error {
	seen := make(map[int]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		if id <= 0 {
			return ErrMarketplaceOrderInvalid
		}
		if _, exists := seen[id]; exists {
			return ErrMarketplaceOrderInvalid
		}
		seen[id] = struct{}{}
	}
	if len(orderedIDs) != len(currentIDs) {
		return ErrMarketplaceOrderConflict
	}
	for _, id := range currentIDs {
		if _, exists := seen[id]; !exists {
			return ErrMarketplaceOrderConflict
		}
	}
	return nil
}
