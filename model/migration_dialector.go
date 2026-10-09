package model

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// Embed the concrete dialectors to customize schema reconciliation while
// retaining transaction/savepoint and other optional GORM interfaces.
type mysqlMigrationDialector struct{ mysql.Dialector }

func (d mysqlMigrationDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return mysqlSchemaMigrator{d.Dialector.Migrator(db).(mysql.Migrator)}
}

type mysqlSchemaMigrator struct{ mysql.Migrator }

func (m mysqlSchemaMigrator) AddColumn(value any, name string) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		field := stmt.Schema.LookUpField(name)
		if catalogTextWithoutSQLDefault(field) {
			if field.IgnoreMigration {
				return nil
			}
			return m.DB.Exec("ALTER TABLE ? ADD ? ?", m.CurrentTable(stmt), clause.Column{Name: field.DBName}, m.FullDataTypeOf(field)).Error
		}
		return m.Migrator.AddColumn(value, name)
	})
}

func (m mysqlSchemaMigrator) AlterColumn(value any, name string) error {
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		field := stmt.Schema.LookUpField(name)
		if catalogTextWithoutSQLDefault(field) {
			return m.DB.Exec("ALTER TABLE ? MODIFY COLUMN ? ?", m.CurrentTable(stmt), clause.Column{Name: field.DBName}, m.FullDataTypeOf(field)).Error
		}
		return m.Migrator.AlterColumn(value, name)
	})
}

// MySQL 5.7 cannot default TEXT columns. Retain the model's Go/GORM defaults
// for inserts, but omit them from DDL (including newer MySQL deployments).
func catalogTextWithoutSQLDefault(field *schema.Field) bool {
	return field != nil && field.Schema != nil && strings.EqualFold(string(field.DataType), "text") &&
		(field.Schema.Table == "models" || (field.Schema.Table == "channels" && field.DBName == "molii_grok_management_access_token"))
}

func (m mysqlSchemaMigrator) FullDataTypeOf(field *schema.Field) clause.Expr {
	if catalogTextWithoutSQLDefault(field) {
		copy := *field
		copy.HasDefaultValue, copy.DefaultValue, copy.DefaultValueInterface = false, "", nil
		return m.Migrator.FullDataTypeOf(&copy)
	}
	return m.Migrator.FullDataTypeOf(field)
}

func (m mysqlSchemaMigrator) MigrateColumn(value any, field *schema.Field, column gorm.ColumnType) error {
	if catalogTextWithoutSQLDefault(field) {
		copy := *field
		copy.HasDefaultValue, copy.DefaultValue, copy.DefaultValueInterface = false, "", nil
		return m.Migrator.MigrateColumn(value, &copy, column)
	}
	if !field.HasDefaultValue || !strings.EqualFold(column.DatabaseTypeName(), "decimal") {
		return m.Migrator.MigrateColumn(value, field, column)
	}
	stored, ok := column.DefaultValue()
	if !ok {
		return m.Migrator.MigrateColumn(value, field, column)
	}
	storedNumber, storedErr := decimal.NewFromString(stored)
	modelNumber, modelErr := decimal.NewFromString(field.DefaultValue)
	if storedErr != nil || modelErr != nil || !storedNumber.Equal(modelNumber) {
		return m.Migrator.MigrateColumn(value, field, column)
	}
	// MySQL pads decimal defaults (0 -> 0.000000). Skip only the equivalent
	// default comparison, retaining type/size/null checks. AlterColumn still
	// reads the original model, including its default.
	comparisonField := *field
	comparisonField.HasDefaultValue = false
	comparisonField.DefaultValue = ""
	comparisonField.DefaultValueInterface = nil
	return m.Migrator.MigrateColumn(value, &comparisonField, columnWithoutDefault{column})
}

type migrationColumnType interface{ gorm.ColumnType }

type columnWithoutDefault struct{ migrationColumnType }

func (columnWithoutDefault) DefaultValue() (string, bool) { return "", false }

type postgresMigrationDialector struct{ postgres.Dialector }

func (d postgresMigrationDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return postgresSchemaMigrator{d.Dialector.Migrator(db).(postgres.Migrator)}
}

type postgresSchemaMigrator struct{ postgres.Migrator }

func (m postgresSchemaMigrator) MigrateColumn(value any, field *schema.Field, column gorm.ColumnType) error {
	if field.Schema != nil && field.Schema.Table == "models" && field.Serializer != nil && field.HasDefaultValue {
		if stored, ok := column.DefaultValue(); ok && strings.Trim(stored, "'") == strings.Trim(field.DefaultValue, "'") {
			// GORM leaves serializer defaults quoted but PostgreSQL reports their
			// literal value. Keep other schema comparisons while ignoring equality.
			copy := *field
			copy.HasDefaultValue, copy.DefaultValue, copy.DefaultValueInterface = false, "", nil
			return m.Migrator.MigrateColumn(value, &copy, columnWithoutDefault{column})
		}
	}
	if column.DatabaseTypeName() == "bpchar" && strings.HasPrefix(strings.ToLower(string(field.DataType)), "char(") {
		// PostgreSQL reports CHAR(n) as bpchar. Normalize the name, retaining
		// Length() so a real CHAR length change still triggers migration.
		column = charColumnType{column}
	}
	return m.Migrator.MigrateColumn(value, field, column)
}

// Older schemas and database imports can use a different constraint name from
// the current naming strategy. Resolve single-column uniqueness from the
// catalog instead of asking GORM to drop an inferred uni_<table>_<column>.
func (m postgresSchemaMigrator) MigrateColumnUnique(value any, field *schema.Field, column gorm.ColumnType) error {
	unique, ok := column.Unique()
	if !ok || field.PrimaryKey || !unique || field.Unique {
		return m.Migrator.MigrateColumnUnique(value, field, column)
	}
	return m.RunWithValue(value, func(stmt *gorm.Statement) error {
		schemaName, tableName := m.CurrentSchema(stmt, stmt.Table)
		return m.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("LOCK TABLE ? IN ACCESS EXCLUSIVE MODE", m.CurrentTable(stmt)).Error; err != nil {
				return err
			}
			var constraints []string
			if err := tx.Raw(`
SELECT constraint_meta.conname
FROM pg_catalog.pg_constraint AS constraint_meta
JOIN pg_catalog.pg_class AS table_meta ON table_meta.oid = constraint_meta.conrelid
JOIN pg_catalog.pg_namespace AS schema_meta ON schema_meta.oid = table_meta.relnamespace
JOIN pg_catalog.pg_attribute AS attribute_meta
  ON attribute_meta.attrelid = table_meta.oid
 AND attribute_meta.attnum = constraint_meta.conkey[1]
WHERE schema_meta.nspname = ? AND table_meta.relname = ?
  AND constraint_meta.contype = 'u'
  AND cardinality(constraint_meta.conkey) = 1
  AND attribute_meta.attname = ?
ORDER BY constraint_meta.conname`, schemaName, tableName, field.DBName).Scan(&constraints).Error; err != nil {
				return err
			}
			for _, name := range constraints {
				if err := tx.Migrator().DropConstraint(value, name); err != nil {
					return fmt.Errorf("drop obsolete unique constraint %q on %s.%s: %w", name, stmt.Table, field.DBName, err)
				}
			}
			return nil
		})
	})
}

type charColumnType struct{ migrationColumnType }

func (charColumnType) DatabaseTypeName() string { return "char" }
