package catalogsync

import (
	"context"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/catalogmanifest"
	"gorm.io/gorm"
)

// ExportManaged preserves the CLI/API bridge. The caller supplies a stable
// transaction and barrier; serving exports use model.WithCatalogReadSnapshot.
func ExportManaged(ctx context.Context, tx *gorm.DB, sourceID string) (catalogmanifest.Snapshot, error) {
	return model.ExportManagedCatalogTx(ctx, tx, sourceID)
}
