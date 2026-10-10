package model

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrCatalogCommitUncertain requires an operation-ID lookup, never automatic
// mutation replay. Even an error returned by COMMIT may follow a durable commit.
var ErrCatalogCommitUncertain = errors.New("catalog commit outcome requires verification")

type catalogReferenceFence struct {
	table       string
	columns     []string
	index       string
	namespace   string
	unqualified bool
	oid         uint32
	persistence string
}

// catalogReferenceTransaction owns a REAL root transaction. The caller must
// already hold WithCatalogWriteBarrier and retain it through runtime publication.
// It must not hold a registry pin or acquire OptionMap/registry locks in write:
// use only the supplied pin's immutable facts, and never release that pin. The
// helper holds it across COMMIT/ROLLBACK and releases it before returning, so
// OptionMap publication can safely follow.
//
// Migrations/anchor initialization belong to startup, not this path. No plain
// user-data read occurs before ALL fences. write owns ready/recovery policy,
// reference checks, operation IDs, and mutations; it must use tx exclusively and
// must not commit, roll back, change session state, or issue DDL/savepoints.
// There is no retry. The 15-second bound also applies without a caller deadline.
func catalogReferenceTransaction(ctx context.Context, db *gorm.DB, registry *jsplugin.Registry, write func(*gorm.DB, *CatalogSyncState, *jsplugin.GenerationPin) error) (result error) {
	if db == nil || registry == nil || write == nil {
		return errors.New("catalog reference transaction requires database, registry and callback")
	}
	// Whitelist true pools. DB.DB() alone would unwrap a transaction into its
	// parent pool, silently accepting nested use and committing too early.
	pool := db.Statement.ConnPool
	if prepared, ok := pool.(*gorm.PreparedStmtDB); ok {
		pool = prepared.ConnPool
	}
	sqlDB, ok := pool.(*sql.DB)
	if !ok {
		return errors.New("catalog reference transaction requires a root database pool")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	var pin *jsplugin.GenerationPin
	// Release last, after setting restoration and exact-connection Close. On
	// cancellation database/sql sets Tx.done BEFORE its worker finishes native
	// rollback; ErrTxDone alone is not a cleanup barrier. Conn.Close waits for
	// that transaction's connection ownership to end before the pin may release.
	defer func() { pin.Release() }()
	defer conn.Close()
	discarded := false
	committed := false
	root := db.Session(&gorm.Session{NewDB: true, Context: ctx})
	root.Statement.ConnPool = conn
	engine := root.Dialector.Name()
	options := &sql.TxOptions{}
	switch engine {
	case "postgres":
		options.Isolation = sql.LevelReadCommitted
	case "mysql":
		options.Isolation = sql.LevelRepeatableRead
		var version, comment string
		if err := conn.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment").Scan(&version, &comment); err != nil {
			return err
		}
		// Interim verified boundary; extend only with the actual version matrix.
		if err := catalogMySQLFenceVersion(version, comment); err != nil {
			return err
		}
		if strings.HasPrefix(version, "5.7.") {
			var unsafe int
			if err := conn.QueryRowContext(ctx, "SELECT @@global.innodb_locks_unsafe_for_binlog").Scan(&unsafe); err != nil {
				return err
			}
			if unsafe != 0 {
				return errors.New("catalog reference fences require InnoDB gap locking")
			}
		}
	case "sqlite":
		// Fail busy immediately instead of inheriting an arbitrarily long DSN
		// busy handler which may outlive cancellation. Restore on this exact
		// connection; a failed reset discards it instead of leaking settings.
		var busy int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
			return err
		}
		defer func() {
			if discarded {
				return
			}
			resetCtx, done := context.WithTimeout(context.Background(), time.Second)
			defer done()
			if _, err := conn.ExecContext(resetCtx, "PRAGMA busy_timeout = "+strconv.Itoa(busy)); err != nil {
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
				result = errors.Join(result, fmt.Errorf("restore catalog connection: %w", err))
				if committed {
					result = errors.Join(ErrCatalogCommitUncertain, result)
				}
			}
		}()
		if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
			return err
		}
	default:
		return errors.New("unsupported catalog reference database")
	}
	fences, err := catalogReferenceFenceSchema(root)
	if err != nil {
		return err
	}
	tx := root.Begin(options)
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		err := tx.Statement.ConnPool.(gorm.TxCommitter).Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			discarded = true
			result = errors.Join(result, fmt.Errorf("catalog rollback: %w", err))
			if committed {
				result = errors.Join(ErrCatalogCommitUncertain, result)
			}
		}
	}()
	if engine == "postgres" {
		if err := pinCatalogPostgresNamespaceTx(tx, fences); err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL lock_timeout = '15s'").Error; err != nil {
			return err
		}
		if err := tx.Exec("SET LOCAL statement_timeout = '15s'").Error; err != nil {
			return err
		}
	}
	// Acquire MySQL's anchor metadata lock before writing: otherwise a DDL
	// engine conversion after preflight could make the anchor nontransactional.
	if engine == "mysql" {
		if err := catalogReferenceKeyFence(tx, fences[0]); err != nil {
			return err
		}
	}
	// The early UPDATE is SQLite's write-transaction acquisition even with a
	// DEFERRED DSN. MySQL's UPDATE and FOR UPDATE are current, not snapshot reads.
	anchor := tx.Exec("UPDATE "+fences[0].table+" SET version = version + 1 WHERE name = ?", marketplaceOrderLockName)
	if anchor.Error != nil {
		return anchor.Error
	}
	if anchor.RowsAffected != 1 {
		return errors.New("catalog mutation anchor missing")
	}
	stateLock := "SELECT id FROM " + fences[1].table + " WHERE id = ?"
	if engine != "sqlite" {
		stateLock += " FOR UPDATE"
	}
	var stateID int
	if err := tx.Raw(stateLock, CatalogSyncStateID).Row().Scan(&stateID); err != nil {
		return err
	}
	for i, fence := range fences {
		if engine == "postgres" {
			if err := tx.Exec("LOCK TABLE " + fence.table + " IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
				return err
			}
			continue
		}
		if engine == "mysql" && i == 0 {
			continue
		}
		if err := catalogReferenceKeyFence(tx, fence); err != nil {
			return err
		}
	}
	// Table access now retains metadata locks. Revalidate the schema so a DDL
	// race between preflight and acquisition cannot change the supported engine
	// or index beneath the proof. These reads occur only AFTER all fences.
	verified, err := catalogReferenceFenceSchema(tx)
	if err != nil {
		return err
	}
	for i := range fences {
		if fences[i].table != verified[i].table || fences[i].index != verified[i].index || !slices.Equal(fences[i].columns, verified[i].columns) {
			return errors.New("catalog reference schema changed during acquisition")
		}
	}
	pin, err = registry.TryPinGeneration()
	if err != nil {
		return err
	}
	var state CatalogSyncState
	if err := tx.First(&state, CatalogSyncStateID).Error; err != nil {
		return err
	}
	if err := write(tx, &state, pin); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		// database/sql marks a Tx done even when SQLite COMMIT returns BUSY
		// while its native transaction remains open. Evict/close the connection
		// before releasing the pin; Rollback on that sql.Tx is already too late.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		discarded = true
		return errors.Join(ErrCatalogCommitUncertain, err)
	}
	committed = true
	return nil
}

func catalogReferenceKeyFence(tx *gorm.DB, fence catalogReferenceFence) error {
	query := "SELECT " + strings.Join(fence.columns, ", ") + " FROM " + fence.table
	if tx.Dialector.Name() == "mysql" {
		query += " FORCE INDEX (" + fence.index + ") ORDER BY " + strings.Join(fence.columns, ", ") + " FOR UPDATE"
	}
	rows, err := tx.Raw(query).Rows()
	if err != nil {
		return err
	}
	values := make([]any, len(fence.columns))
	for i := range values {
		values[i] = new(any)
	}
	for rows.Next() {
		if err := rows.Scan(values...); err != nil {
			return errors.Join(err, rows.Close())
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if tx.Dialector.Name() == "mysql" {
		// SHOW CREATE resolves the actual relation, including temporary shadows
		// invisible to information_schema.tables. It does not open an MVCC data
		// snapshot. The preceding locking scan retains this relation's MDL.
		var name, definition string
		if err := tx.Raw("SHOW CREATE TABLE "+fence.table).Row().Scan(&name, &definition); err != nil {
			return err
		}
		if !strings.HasPrefix(definition, "CREATE TABLE ") || !strings.Contains(definition, "\n) ENGINE=InnoDB ") {
			return errors.New("catalog reference fence requires a persistent InnoDB table")
		}
	}
	return nil
}

// Resolve only trusted model names/keys through the same GORM schema used by
// subsequent readers. No source, credentials, task JSON or option values are
// selected by the fence scans. All rows and insertion gaps are covered.
func catalogReferenceFenceSchema(db *gorm.DB) ([]catalogReferenceFence, error) {
	return catalogRelationSchema(db, []any{&marketplaceOrderLock{}, &CatalogSyncState{}, &Ability{}, &Channel{}, &Midjourney{}, &Model{}, &Option{}, &SystemTask{}, &TaskPlugin{}, &Task{}, &Vendor{}, &CatalogSyncPlan{}, &CatalogSyncBaseline{}, &CatalogSyncOperation{}})
}

func catalogRelationSchema(db *gorm.DB, models []any) ([]catalogReferenceFence, error) {
	indexVisibility := "IS_VISIBLE"
	if db.Dialector.Name() == "mysql" {
		var version string
		if err := db.Raw("SELECT VERSION()").Row().Scan(&version); err != nil {
			return nil, err
		}
		if strings.HasPrefix(version, "5.7.") {
			indexVisibility = "'YES'"
		}
	}
	fences := make([]catalogReferenceFence, 0, len(models))
	for _, model := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			return nil, err
		}
		name := stmt.Schema.Table
		fence := catalogReferenceFence{table: stmt.Quote(clause.Table{Name: name})}
		keys := make([]string, 0, len(stmt.Schema.PrimaryFields))
		for _, field := range stmt.Schema.PrimaryFields {
			keys = append(keys, field.DBName)
			fence.columns = append(fence.columns, stmt.Quote(clause.Column{Name: field.DBName}))
		}
		if len(keys) == 0 {
			return nil, errors.New("catalog reference table has no known key")
		}
		switch db.Dialector.Name() {
		case "mysql":
			parts := strings.Split(name, ".")
			var schemaName string
			if err := db.Raw("SELECT DATABASE()").Row().Scan(&schemaName); err != nil {
				return nil, err
			}
			if len(parts) == 2 {
				schemaName = parts[0]
			} else if len(parts) != 1 {
				return nil, errors.New("unsupported catalog reference table name")
			}
			var engine, tableType string
			if err := db.Raw("SELECT ENGINE, TABLE_TYPE FROM information_schema.tables WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", schemaName, parts[len(parts)-1]).Row().Scan(&engine, &tableType); err != nil {
				return nil, err
			}
			if engine != "InnoDB" || tableType != "BASE TABLE" {
				return nil, errors.New("catalog reference table must be InnoDB")
			}
			var partitions int
			if err := db.Raw("SELECT count(*) FROM information_schema.partitions WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND PARTITION_NAME IS NOT NULL", schemaName, parts[len(parts)-1]).Row().Scan(&partitions); err != nil {
				return nil, err
			}
			if partitions != 0 {
				return nil, errors.New("partitioned catalog reference tables are not verified")
			}
			var indexes []struct {
				IndexName  string
				ColumnName string
				NonUnique  int
				SubPart    sql.NullInt64
				IndexType  string
				IsVisible  string
			}
			if err := db.Raw("SELECT INDEX_NAME AS index_name, COLUMN_NAME AS column_name, NON_UNIQUE AS non_unique, SUB_PART AS sub_part, INDEX_TYPE AS index_type, "+indexVisibility+" AS is_visible FROM information_schema.statistics WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX", schemaName, parts[len(parts)-1]).Scan(&indexes).Error; err != nil {
				return nil, err
			}
			for i := 0; i < len(indexes); {
				indexName := indexes[i].IndexName
				var columns []string
				valid := true
				for i < len(indexes) && indexes[i].IndexName == indexName {
					index := indexes[i]
					valid = valid && index.NonUnique == 0 && !index.SubPart.Valid && index.IndexType == "BTREE" && index.IsVisible == "YES"
					columns = append(columns, index.ColumnName)
					i++
				}
				if valid && slices.Equal(columns, keys) {
					fence.index = stmt.Quote(clause.Column{Name: indexName})
					break
				}
			}
			if fence.index == "" {
				return nil, fmt.Errorf("catalog reference table %s needs a complete unique key index", fence.table)
			}
		case "postgres":
			var kind, persistence, namespace, relation string
			var rowSecurity, forceRowSecurity, inheritance bool
			if err := db.Raw("SELECT c.relkind, c.relpersistence, n.nspname, c.relname, c.relrowsecurity, c.relforcerowsecurity, EXISTS (SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid = c.oid OR i.inhparent = c.oid), c.oid FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace WHERE c.oid = pg_catalog.to_regclass(?)", fence.table).Row().Scan(&kind, &persistence, &namespace, &relation, &rowSecurity, &forceRowSecurity, &inheritance, &fence.oid); err != nil {
				return nil, err
			}
			if kind != "r" || persistence == "t" || rowSecurity || forceRowSecurity || inheritance {
				return nil, errors.New("catalog reference relation must be an ordinary non-RLS persistent table")
			}
			fence.table = stmt.Quote(clause.Column{Name: namespace}) + "." + stmt.Quote(clause.Column{Name: relation})
			fence.namespace, fence.unqualified = namespace, !strings.Contains(name, ".")
			fence.persistence = persistence
		case "sqlite":
			parts := strings.Split(name, ".")
			if len(parts) > 2 || (len(parts) == 2 && parts[0] != "main") {
				return nil, errors.New("catalog references must use the main SQLite database")
			}
			var kind, definition string
			if err := db.Raw("SELECT type, sql FROM main.sqlite_master WHERE name = ?", parts[len(parts)-1]).Row().Scan(&kind, &definition); err != nil {
				return nil, err
			}
			var temporary int
			if err := db.Raw("SELECT count(*) FROM sqlite_temp_master WHERE name = ?", parts[len(parts)-1]).Row().Scan(&temporary); err != nil {
				return nil, err
			}
			if kind != "table" || temporary != 0 || !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(definition)), "CREATE TABLE ") {
				return nil, errors.New("catalog reference relation must be a main SQLite table")
			}
		}
		fences = append(fences, fence)
	}
	return fences, nil
}

// Pin all unqualified consumers to the same resolved schema. In particular,
// implicit pg_temp precedence must not redirect reads after metadata locks.
func pinCatalogPostgresNamespaceTx(tx *gorm.DB, fences []catalogReferenceFence) error {
	var namespace string
	for _, fence := range fences {
		if !fence.unqualified {
			continue
		}
		if namespace != "" && namespace != fence.namespace {
			return errors.New("catalog references span mixed unqualified PostgreSQL namespaces")
		}
		namespace = fence.namespace
	}
	if namespace == "" {
		return errors.New("catalog reference namespace unavailable")
	}
	return tx.Exec("SET LOCAL search_path TO " + tx.Statement.Quote(clause.Column{Name: namespace}) + ", pg_catalog, pg_temp").Error
}

// A narrow PostgreSQL read transaction, not the target mutation engine. Source
// capture supplies READ ONLY / REPEATABLE READ; receipt lookup supplies READ
// COMMITTED because authoritative auth validation locks user/session rows.
// Resolve before BEGIN, lock before the first snapshot query, then verify OIDs:
// a same-name DDL replacement cannot inherit the preflight completeness proof.
func catalogPostgresReadTransaction(ctx context.Context, models []any, options *sql.TxOptions, read func(*gorm.DB) error) (result error) {
	if DB == nil || DB.Dialector.Name() != "postgres" {
		return errors.New("catalog read proof requires PostgreSQL")
	}
	pool := DB.Statement.ConnPool
	if prepared, ok := pool.(*gorm.PreparedStmtDB); ok {
		pool = prepared.ConnPool
	}
	sqlDB, ok := pool.(*sql.DB)
	if !ok {
		return errors.New("catalog read proof requires a root database pool")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	// Close drains database/sql's asynchronous rollback on context cancellation
	// before the caller releases its source read barrier or returns a receipt.
	defer func() { result = errors.Join(result, conn.Close()) }()
	root := DB.Session(&gorm.Session{NewDB: true, Context: ctx})
	root.Statement.ConnPool = conn
	fences, err := catalogRelationSchema(root, models)
	if err != nil {
		return err
	}
	for _, fence := range fences {
		if fence.persistence != "p" {
			return errors.New("catalog read proof requires permanent relations")
		}
	}
	tx := root.Begin(options)
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if err := tx.Statement.ConnPool.(gorm.TxCommitter).Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			result = errors.Join(result, err)
		}
	}()
	if err := pinCatalogPostgresNamespaceTx(tx, fences); err != nil {
		return err
	}
	if err := tx.Exec("SET LOCAL lock_timeout = '15s'").Error; err != nil {
		return err
	}
	if err := tx.Exec("SET LOCAL statement_timeout = '15s'").Error; err != nil {
		return err
	}
	for _, fence := range fences {
		if err := tx.Exec("LOCK TABLE " + fence.table + " IN ACCESS SHARE MODE").Error; err != nil {
			return err
		}
	}
	verified, err := catalogRelationSchema(tx, models)
	if err != nil {
		return err
	}
	for i, fence := range fences {
		if fence.table != verified[i].table || fence.oid != verified[i].oid || verified[i].persistence != "p" {
			return errors.New("catalog relation changed during read acquisition")
		}
	}
	if err := read(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		return err
	}
	return nil
}

func catalogMySQLFenceVersion(version, comment string) error {
	parts := strings.Split(version, ".")
	if len(parts) == 3 && (comment == "MySQL Community Server - GPL" || comment == "MySQL Community Server (GPL)") {
		patch, err := strconv.Atoi(parts[2])
		if err == nil && ((parts[0] == "5" && parts[1] == "7" && patch >= 44) || (parts[0] == "8" && parts[1] == "0" && patch >= 46) || (parts[0] == "8" && parts[1] == "4" && patch >= 11)) {
			return nil
		}
	}
	return errors.New("catalog reference fences require verified Oracle MySQL 5.7.44+, 8.0.46+ or 8.4.11+ in those series")
}
