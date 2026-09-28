package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	optionPrimaryKeyTmpTable = "options_pk_tmp"
	optionPrimaryKeyLockName = "new_api_options_pk"
	optionPrimaryKeyLockID   = 75820193
	optionLegacyTablePrefix  = "options_legacy_"
)

func migrateOptionPrimaryKey(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate options primary key: database is nil")
	}
	if !db.Migrator().HasTable(&Option{}) {
		return nil
	}
	unique, err := optionsKeyIsUnique(db)
	if err != nil {
		return err
	}
	if unique {
		return nil
	}
	return withOptionPrimaryKeyLock(db, func(locked *gorm.DB) error {
		unique, err := optionsKeyIsUnique(locked)
		if err != nil {
			return err
		}
		if unique {
			return nil
		}
		return repairOptionPrimaryKey(locked)
	})
}

func verifyOptionPrimaryKey(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("verify options primary key: database is nil")
	}
	if !db.Migrator().HasTable(&Option{}) {
		return fmt.Errorf("verify options primary key: options table is missing")
	}
	unique, err := optionsKeyIsUnique(db)
	if err != nil {
		return err
	}
	if !unique {
		return fmt.Errorf("verify options primary key: options.key is not unique")
	}
	return nil
}

func optionsKeyIsUnique(db *gorm.DB) (bool, error) {
	indexes, err := db.Migrator().GetIndexes(&Option{})
	if err != nil {
		return false, fmt.Errorf("inspect options indexes: %w", err)
	}
	for _, index := range indexes {
		if len(index.Columns()) != 1 || index.Columns()[0] != "key" {
			continue
		}
		if unique, ok := index.Unique(); ok && unique {
			return true, nil
		}
		if primary, ok := index.PrimaryKey(); ok && primary {
			return true, nil
		}
	}
	if db.Dialector.Name() != "postgres" {
		return false, nil
	}
	// PostgreSQL GetIndexes omits constraint-backed primary/unique keys.
	var count int64
	if err := db.Raw(`
SELECT count(*)
FROM pg_catalog.pg_constraint AS constraint_meta
JOIN pg_catalog.pg_attribute AS attribute_meta
  ON attribute_meta.attrelid = constraint_meta.conrelid
 AND attribute_meta.attnum = constraint_meta.conkey[1]
WHERE constraint_meta.conrelid = to_regclass('options')
  AND constraint_meta.contype IN ('p', 'u')
  AND cardinality(constraint_meta.conkey) = 1
  AND attribute_meta.attname = 'key'`).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("inspect options constraints: %w", err)
	}
	return count > 0, nil
}

func withOptionPrimaryKeyLock(db *gorm.DB, fn func(*gorm.DB) error) error {
	switch db.Dialector.Name() {
	case "mysql":
		return db.Connection(func(locked *gorm.DB) (err error) {
			locked = locked.Session(&gorm.Session{SkipDefaultTransaction: true})
			var acquired int
			if queryErr := locked.Raw("SELECT GET_LOCK(?, 60)", optionPrimaryKeyLockName).Scan(&acquired).Error; queryErr != nil {
				return fmt.Errorf("lock options table: %w", queryErr)
			}
			if acquired != 1 {
				return fmt.Errorf("lock options table: timeout")
			}
			defer func() {
				if releaseErr := locked.Exec("SELECT RELEASE_LOCK(?)", optionPrimaryKeyLockName).Error; err == nil && releaseErr != nil {
					err = fmt.Errorf("unlock options table: %w", releaseErr)
				}
			}()
			return fn(locked)
		})
	case "postgres":
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", optionPrimaryKeyLockID).Error; err != nil {
				return fmt.Errorf("lock options table: %w", err)
			}
			return fn(tx)
		})
	default:
		return db.Transaction(func(tx *gorm.DB) error {
			return fn(tx)
		})
	}
}

func repairOptionPrimaryKey(db *gorm.DB) error {
	if db.Migrator().HasTable(optionPrimaryKeyTmpTable) {
		if err := db.Migrator().DropTable(optionPrimaryKeyTmpTable); err != nil {
			return fmt.Errorf("drop leftover %s: %w", optionPrimaryKeyTmpTable, err)
		}
	}
	if err := db.Table(optionPrimaryKeyTmpTable).Migrator().CreateTable(&Option{}); err != nil {
		return fmt.Errorf("create %s: %w", optionPrimaryKeyTmpTable, err)
	}
	tmpCreated := true
	defer func() {
		if tmpCreated && db.Migrator().HasTable(optionPrimaryKeyTmpTable) {
			_ = db.Migrator().DropTable(optionPrimaryKeyTmpTable)
		}
	}()
	var rows []Option
	var deduped []Option
	var skippedEmpty int
	backup := fmt.Sprintf("%s%d", optionLegacyTablePrefix, time.Now().UnixNano())
	if err := withOptionTableWriteLock(db, func(locked *gorm.DB) error {
		if err := locked.Find(&rows).Error; err != nil {
			return fmt.Errorf("read options rows: %w", err)
		}
		var dedupeErr error
		deduped, skippedEmpty, dedupeErr = dedupeOptionRows(rows)
		if dedupeErr != nil {
			return dedupeErr
		}
		if len(deduped) > 0 {
			if err := locked.Table(optionPrimaryKeyTmpTable).CreateInBatches(deduped, 100).Error; err != nil {
				return fmt.Errorf("insert rebuilt options: %w", err)
			}
		}
		var written int64
		if err := locked.Table(optionPrimaryKeyTmpTable).Count(&written).Error; err != nil {
			return fmt.Errorf("count rebuilt options: %w", err)
		}
		if int(written) != len(deduped) {
			return fmt.Errorf("options rebuild wrote %d rows, want %d", written, len(deduped))
		}
		if err := swapOptionTables(locked, optionPrimaryKeyTmpTable, backup); err != nil {
			return err
		}
		tmpCreated = false
		return nil
	}); err != nil {
		return err
	}
	unique, err := optionsKeyIsUnique(db)
	if err != nil {
		return err
	}
	if !unique {
		return fmt.Errorf("options table still has no unique key after rebuild")
	}
	common.SysLog(fmt.Sprintf("rebuilt options table with primary key from %d rows into %d keys; previous rows kept in %s", len(rows)-skippedEmpty, len(deduped), backup))
	return nil
}

func dedupeOptionRows(rows []Option) ([]Option, int, error) {
	ordered := append([]Option(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Key == ordered[j].Key {
			return ordered[i].Value < ordered[j].Value
		}
		return ordered[i].Key < ordered[j].Key
	})
	seen := make(map[string]int)
	deduped := make([]Option, 0, len(ordered))
	skippedEmpty := 0
	for _, row := range ordered {
		if row.Key == "" {
			skippedEmpty++
			continue
		}
		if i, ok := seen[row.Key]; ok {
			if deduped[i].Value != row.Value {
				return nil, skippedEmpty, fmt.Errorf("options key %q has conflicting values", row.Key)
			}
			continue
		}
		seen[row.Key] = len(deduped)
		deduped = append(deduped, Option{Key: row.Key, Value: row.Value})
	}
	if skippedEmpty > 0 {
		common.SysError(fmt.Sprintf("skipped %d options rows with empty keys while rebuilding the primary key", skippedEmpty))
	}
	return deduped, skippedEmpty, nil
}

func withOptionTableWriteLock(db *gorm.DB, fn func(*gorm.DB) error) (err error) {
	switch db.Dialector.Name() {
	case "mysql":
		if err := db.Exec("LOCK TABLES `options` WRITE, `" + optionPrimaryKeyTmpTable + "` WRITE").Error; err != nil {
			return fmt.Errorf("lock options for rebuild: %w", err)
		}
		defer func() {
			if unlockErr := db.Exec("UNLOCK TABLES").Error; err == nil && unlockErr != nil {
				err = fmt.Errorf("unlock options after rebuild: %w", unlockErr)
			}
		}()
		return fn(db.Session(&gorm.Session{SkipDefaultTransaction: true}))
	case "postgres":
		if err := db.Exec(`LOCK TABLE "options" IN ACCESS EXCLUSIVE MODE`).Error; err != nil {
			return fmt.Errorf("lock options for rebuild: %w", err)
		}
		return fn(db)
	default:
		if err := db.Exec(`UPDATE "options" SET "value" = "value" WHERE 1 = 0`).Error; err != nil {
			return fmt.Errorf("lock options for rebuild: %w", err)
		}
		return fn(db)
	}
}

func swapOptionTables(db *gorm.DB, tmp, backup string) error {
	if !optionSafeIdent(tmp) || !optionSafeIdent(backup) {
		return fmt.Errorf("unsafe options table name")
	}
	if db.Dialector.Name() == "mysql" {
		sql := "RENAME TABLE `options` TO `" + backup + "`, `" + tmp + "` TO `options`"
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("rename options tables: %w", err)
		}
		return nil
	}
	if err := db.Migrator().RenameTable("options", backup); err != nil {
		return fmt.Errorf("rename options to %s: %w", backup, err)
	}
	if err := db.Migrator().RenameTable(tmp, "options"); err != nil {
		return fmt.Errorf("rename %s to options: %w", tmp, err)
	}
	return nil
}

func optionSafeIdent(name string) bool {
	if name == "" {
		return false
	}
	return !strings.ContainsFunc(name, func(r rune) bool {
		return r != '_' && (r < '0' || r > '9') && (r < 'a' || r > 'z')
	})
}
