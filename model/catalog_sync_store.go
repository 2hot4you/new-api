package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func LoadCatalogSyncBaselineTx(tx *gorm.DB, state *CatalogSyncState) (catalogmanifest.Baseline, error) {
	base := catalogmanifest.Baseline{SourceID: string(state.SourceID), Generation: state.BaselineGeneration, ObjectVersions: map[string]string{}}
	var rows []CatalogSyncBaseline
	if err := tx.Order("identity_hash").Find(&rows).Error; err != nil {
		return base, err
	}
	for _, row := range rows {
		entry := catalogmanifest.Entry{Kind: row.Kind, Key: string(row.EntryKey), Value: string(row.EntryValue)}
		id := catalogmanifest.EntryID(entry)
		if row.SourceID != state.SourceID || row.Generation != state.BaselineGeneration || row.IdentityHash != fmt.Sprintf("%x", sha256.Sum256([]byte(id))) {
			return base, fmt.Errorf("catalog baseline provenance mismatch")
		}
		base.Entries = append(base.Entries, entry)
		if row.ObjectVersion != "" {
			base.ObjectVersions[id] = row.ObjectVersion
		}
	}
	return base, nil
}

// Persist the full applied source, including original audit provenance. Caller
// must hold the catalog writer and DB anchor inside the successful apply tx.
func SaveCatalogSyncBaselineTx(tx *gorm.DB, source catalogmanifest.Snapshot, objectVersions map[string]string, expectedGeneration int64) (int64, error) {
	return saveCatalogSyncBaselineTx(tx, source, objectVersions, expectedGeneration, false)
}

// structureOnly is reserved for the attested write path: the caller must have
// already checked the full staged attestation under the root transaction fences.
func saveCatalogSyncBaselineTx(tx *gorm.DB, source catalogmanifest.Snapshot, objectVersions map[string]string, expectedGeneration int64, structureOnly bool) (int64, error) {
	validate := catalogmanifest.ValidateSnapshot
	if structureOnly {
		validate = catalogmanifest.ValidateSnapshotStructure
	}
	if err := validate(source); err != nil {
		return 0, err
	}
	var state CatalogSyncState
	if err := tx.First(&state, CatalogSyncStateID).Error; err != nil {
		return 0, err
	}
	if state.BaselineGeneration != expectedGeneration || (state.SourceID != "" && string(state.SourceID) != source.SourceID) {
		return 0, ErrCatalogSyncPlanStale
	}
	generation := expectedGeneration + 1
	rows := make([]CatalogSyncBaseline, 0, len(source.Entries))
	for _, entry := range source.Entries {
		id := catalogmanifest.EntryID(entry)
		token := objectVersions[id]
		if (entry.Kind == catalogmanifest.KindModel || entry.Kind == catalogmanifest.KindVendor) && token == "" {
			return 0, fmt.Errorf("applied catalog object has no incarnation evidence")
		}
		rows = append(rows, CatalogSyncBaseline{IdentityHash: fmt.Sprintf("%x", sha256.Sum256([]byte(id))), SourceID: CatalogSyncText(source.SourceID), Generation: generation, Kind: entry.Kind, EntryKey: CatalogSyncText(entry.Key), EntryValue: CatalogSyncText(entry.Value), ObjectVersion: token})
	}
	result := tx.Model(&CatalogSyncState{}).Where("id = ? AND baseline_generation = ?", CatalogSyncStateID, expectedGeneration).Updates(map[string]any{"source_id": source.SourceID, "baseline_generation": generation})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, ErrCatalogSyncPlanStale
	}
	if err := tx.Where("1 = 1").Delete(&CatalogSyncBaseline{}).Error; err != nil {
		return 0, err
	}
	if len(rows) > 0 {
		if err := tx.CreateInBatches(rows, 100).Error; err != nil {
			return 0, err
		}
	}
	return generation, nil
}

const CatalogSyncStateID = 1

var (
	ErrCatalogSyncPlanUnavailable = errors.New("catalog sync plan unavailable or expired")
	ErrCatalogSyncPlanStale       = errors.New("catalog sync plan changed; reload before confirming")
	ErrCatalogPublicationPending  = errors.New("catalog publication pending recovery")
)

// CatalogSyncText accommodates the 10 MiB wire limit; MySQL TEXT is only 64 KiB.
type CatalogSyncText string

func (CatalogSyncText) GormDataType() string { return "string" }
func (CatalogSyncText) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == "mysql" {
		return "longtext"
	}
	return "text"
}

type CatalogSyncState struct {
	ID                 int    `gorm:"primaryKey;autoIncrement:false"`
	Revision           int64  `gorm:"not null"`
	CurrentDigest      string `gorm:"size:71;not null"`
	RuntimeRevision    int64  `gorm:"not null"`
	PublicationState   string `gorm:"size:32;not null"`
	PendingOperationID string `gorm:"size:64;not null"`
	SourceID           CatalogSyncText
	BaselineGeneration int64  `gorm:"not null"`
	IncarnationKey     string `gorm:"size:64;not null" json:"-"`
}

type CatalogSyncPlan struct {
	ID                 string          `gorm:"primaryKey;size:64"`
	Validation         CatalogSyncText `json:"-"`
	Body               CatalogSyncText `gorm:"not null"`
	Digest             string          `gorm:"size:71;not null"`
	TargetRevision     int64           `gorm:"not null"`
	BaselineGeneration int64           `gorm:"not null"`
	ExpiresAt          int64           `gorm:"not null;index"`
}

// The fixed-width index avoids MySQL's key length limit for plugin identities.
type CatalogSyncBaseline struct {
	IdentityHash  string          `gorm:"primaryKey;size:64"`
	SourceID      CatalogSyncText `gorm:"not null"`
	Generation    int64           `gorm:"not null"`
	Kind          string          `gorm:"size:32;not null"`
	EntryKey      CatalogSyncText `gorm:"not null"`
	EntryValue    CatalogSyncText `gorm:"not null"`
	ObjectVersion string          `gorm:"size:64;not null"`
}

type CatalogSyncOperation struct {
	ID        string          `gorm:"primaryKey;size:64"`
	PlanID    string          `gorm:"size:64;not null;uniqueIndex"`
	State     string          `gorm:"size:32;not null"`
	Revision  int64           `gorm:"not null"`
	CreatedAt int64           `gorm:"not null"`
	Backup    CatalogSyncText `gorm:"not null"`
	History   CatalogSyncText `gorm:"not null"`
	Result    CatalogSyncText `gorm:"not null"`
}

func catalogRandomID() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func MigrateCatalogSync(db *gorm.DB) error {
	if err := db.AutoMigrate(&CatalogSyncState{}, &CatalogSyncPlan{}, &CatalogSyncBaseline{}, &CatalogSyncOperation{}); err != nil {
		return err
	}
	return EnsureCatalogMutationLock(db)
}

// Initialize anchors before locking subordinate rows. Missing schema fails
// closed instead of silently disabling catalog coordination.
func EnsureCatalogMutationLock(db *gorm.DB) error {
	if err := ensureMarketplaceOrderLock(db); err != nil {
		return err
	}
	var state CatalogSyncState
	result := db.Where("id = ?", CatalogSyncStateID).Limit(1).Find(&state)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	key, err := catalogRandomID()
	if err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&CatalogSyncState{ID: CatalogSyncStateID, IncarnationKey: key, PublicationState: "ready"}).Error
}

func validCatalogActor(actor catalogmanifest.Actor) bool {
	return actor.UserID > 0 && actor.SessionID != "" && actor.TargetID != "" && actor.AuthVersion > 0 && actor.SessionVersion > 0
}

func CreateCatalogSyncPlan(ctx context.Context, source catalogmanifest.Snapshot, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan, error) {
	var plan catalogmanifest.Plan
	if !validCatalogActor(actor) {
		return plan, ErrCatalogSyncPlanUnavailable
	}
	err := WithCatalogWriteBarrier(func() error {
		if err := EnsureCatalogMutationLock(DB.WithContext(ctx)); err != nil {
			return err
		}
		return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			state, err := LockCatalogMutationTx(tx)
			if err != nil {
				return err
			}
			if state.PublicationState != "ready" {
				return ErrCatalogPublicationPending
			}
			target, err := CaptureCatalogTargetTx(ctx, tx, actor.TargetID, state)
			if err != nil {
				return err
			}
			base, err := LoadCatalogSyncBaselineTx(tx, state)
			if err != nil {
				return err
			}
			plan, err = catalogmanifest.BuildPlan(source, target, base, actor, now)
			if err != nil {
				return err
			}
			plan.ID, err = catalogRandomID()
			if err != nil {
				return err
			}
			plan.Digest, err = catalogmanifest.CanonicalPlanDigest(plan)
			if err != nil {
				return err
			}
			body, err := common.Marshal(plan)
			if err != nil {
				return err
			}
			return tx.Create(&CatalogSyncPlan{ID: plan.ID, Body: CatalogSyncText(body), Digest: plan.Digest, TargetRevision: state.Revision, BaselineGeneration: state.BaselineGeneration, ExpiresAt: plan.ExpiresAt}).Error
		})
	})
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	return plan, nil
}

func GetCatalogSyncPlan(ctx context.Context, id string, actor catalogmanifest.Actor) (catalogmanifest.Plan, error) {
	var row CatalogSyncPlan
	if !validCatalogActor(actor) || id == "" {
		return catalogmanifest.Plan{}, ErrCatalogSyncPlanUnavailable
	}
	if err := DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return catalogmanifest.Plan{}, err
	}
	return decodeCatalogSyncPlan(row, actor, time.Now())
}

func decodeCatalogSyncPlan(row CatalogSyncPlan, actor catalogmanifest.Actor, now time.Time) (catalogmanifest.Plan, error) {
	var plan catalogmanifest.Plan
	if err := common.UnmarshalJsonStr(string(row.Body), &plan); err != nil {
		return plan, err
	}
	if !validCatalogActor(actor) || actor != plan.Actor || plan.ExpiresAt <= now.Unix() || row.ExpiresAt != plan.ExpiresAt || row.ID != plan.ID || row.BaselineGeneration != plan.BaselineGeneration {
		return catalogmanifest.Plan{}, ErrCatalogSyncPlanUnavailable
	}
	digest, err := catalogmanifest.CanonicalPlanDigest(plan)
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	if digest != plan.Digest || digest != row.Digest {
		return catalogmanifest.Plan{}, ErrCatalogSyncPlanStale
	}
	return plan, nil
}

// Choices replace earlier choices. ExpectedDigest is a CAS precondition: two
// stale browser tabs cannot silently replace the proof-bound final plan.
func ResolveCatalogSyncPlan(ctx context.Context, id, expectedDigest string, actor catalogmanifest.Actor, choices catalogmanifest.Resolution) (catalogmanifest.Plan, error) {
	var result catalogmanifest.Plan
	err := WithCatalogWriteBarrier(func() error {
		return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var row CatalogSyncPlan
			if err := lockForUpdate(tx).First(&row, "id = ?", id).Error; err != nil {
				return err
			}
			plan, err := decodeCatalogSyncPlan(row, actor, time.Now())
			if err != nil {
				return err
			}
			if expectedDigest != row.Digest {
				return ErrCatalogSyncPlanStale
			}
			result, err = catalogmanifest.ResolvePlan(plan, choices)
			if err != nil {
				return err
			}
			if result.Digest == row.Digest {
				return nil
			}
			body, err := common.Marshal(result)
			if err != nil {
				return err
			}
			updated := tx.Model(&CatalogSyncPlan{}).Where("id = ? AND digest = ?", id, expectedDigest).Updates(map[string]any{"body": CatalogSyncText(body), "digest": result.Digest})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrCatalogSyncPlanStale
			}
			return nil
		})
	})
	if err != nil {
		return catalogmanifest.Plan{}, err
	}
	return result, nil
}
