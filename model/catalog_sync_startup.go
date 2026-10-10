package model

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCatalogInstanceIneligible = errors.New("catalog sync requires an explicitly configured healthy single instance")

// Called only within an authoritative sensitive reference transaction. SHARE
// excludes heartbeat inserts/updates/deletes until commit, including absent-row
// gaps. It does not detect a second process impersonating the same manual name,
// and is not distributed runtime publication or a deployment discovery service.
func catalogSensitiveInstanceTx(tx *gorm.DB) error {
	identity := common.GetNodeIdentity()
	if os.Getenv("CATALOG_SYNC_SINGLE_INSTANCE") != "true" || !common.IsMasterNode || common.StartTime <= 0 ||
		!identity.ManuallyConfigured || identity.ShouldConfigureManually || identity.Source != common.NodeNameSourceManual ||
		!catalogSafeNodeName(identity.Name) {
		return ErrCatalogInstanceIneligible
	}
	if tx.Dialector.Name() != "postgres" {
		return ErrCatalogInstanceIneligible
	}
	stmt := &gorm.Statement{DB: tx}
	if err := stmt.Parse(&SystemInstance{}); err != nil {
		return err
	}
	table := stmt.Quote(clause.Table{Name: stmt.Schema.Table})
	// The parent reference transaction already pinned its trusted search_path.
	// Lock first, then inspect that exact relation, excluding a DDL replacement.
	if err := tx.Exec("LOCK TABLE " + table + " IN SHARE MODE").Error; err != nil {
		return err
	}
	var kind, persistence string
	var rls, forced, inherited bool
	if err := tx.Raw("SELECT c.relkind, c.relpersistence, c.relrowsecurity, c.relforcerowsecurity, EXISTS (SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid) FROM pg_catalog.pg_class c WHERE c.oid=pg_catalog.to_regclass(?)", table).Row().Scan(&kind, &persistence, &rls, &forced, &inherited); err != nil {
		return err
	}
	if kind != "r" || persistence != "p" || rls || forced || inherited {
		return ErrCatalogInstanceIneligible
	}
	var unique bool
	if err := tx.Raw("SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_attribute a ON a.attrelid=i.indrelid AND a.attnum=i.indkey[0] WHERE i.indrelid=pg_catalog.to_regclass(?) AND i.indisprimary AND i.indisvalid AND i.indisready AND i.indnatts=1 AND a.attname='node_name')", table).Scan(&unique).Error; err != nil {
		return err
	}
	if !unique {
		return ErrCatalogInstanceIneligible
	}
	var instances []SystemInstance
	if err := tx.Find(&instances).Error; err != nil {
		return err
	}
	now := time.Now().Unix()
	self := false
	for _, instance := range instances {
		if !catalogSafeNodeName(instance.NodeName) || instance.LastSeenAt <= 0 || instance.LastSeenAt > now {
			return ErrCatalogInstanceIneligible
		}
		if instance.NodeName != identity.Name {
			if now-instance.LastSeenAt <= SystemInstanceStaleAfterSeconds {
				return ErrCatalogInstanceIneligible
			}
			continue
		}
		if self || instance.StartedAt != common.StartTime || instance.StartedAt > instance.LastSeenAt ||
			now-instance.LastSeenAt > SystemInstanceStaleAfterSeconds || instance.CreatedAt <= 0 ||
			instance.CreatedAt > instance.LastSeenAt || instance.UpdatedAt != instance.LastSeenAt {
			return ErrCatalogInstanceIneligible
		}
		// Decode the reporter's versioned identity contract without importing
		// service (which already imports model). Resources are not eligibility.
		var info struct {
			SchemaVersion int                 `json:"schema_version"`
			Node          common.NodeIdentity `json:"node"`
			Role          struct {
				IsMaster bool `json:"is_master"`
			} `json:"role"`
			Runtime struct {
				StartedAt int64 `json:"started_at"`
			} `json:"runtime"`
		}
		if err := common.UnmarshalJsonStr(instance.Info, &info); err != nil || info.SchemaVersion != 1 ||
			info.Node != identity || !info.Role.IsMaster || info.Runtime.StartedAt != common.StartTime {
			return ErrCatalogInstanceIneligible
		}
		self = true
	}
	if !self {
		return ErrCatalogInstanceIneligible
	}
	return nil
}

func catalogSafeNodeName(name string) bool {
	if name == "" || len(name) > 128 || strings.TrimSpace(name) != name {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return name != "." && name != ".."
}
