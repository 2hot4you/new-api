package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simulate an existing d922 catalog row in the owned fixture database. Adding
// the publication epoch must preserve business data and leave recovery to the
// normal runtime, not manufacture a ready epoch during schema migration.
func TestCatalogPublicationFingerprintMigration(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			db := catalogFenceTestDB(t, engine)
			require.NoError(t, db.Create(&Option{Key: "ModelPrice", Value: `{"migration-priced":2}`}).Error)
			require.NoError(t, db.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Updates(map[string]any{
				"revision": 235, "runtime_revision": 235, "publication_state": "committed_pending_publish",
			}).Error)
			// Only the fresh disposable database loses this column. No service or
			// user database is connected by this fixture.
			require.NoError(t, db.Migrator().DropColumn(&CatalogSyncState{}, "runtime_fingerprint"))
			require.NoError(t, MigrateCatalogSync(db))
			var upgraded CatalogSyncState
			require.NoError(t, db.First(&upgraded, CatalogSyncStateID).Error)
			assert.Equal(t, int64(235), upgraded.Revision)
			assert.Equal(t, int64(235), upgraded.RuntimeRevision)
			assert.Equal(t, "committed_pending_publish", upgraded.PublicationState)
			assert.Empty(t, upgraded.RuntimeFingerprint, "migration cannot attest an unbuilt runtime")
			fingerprint := "sha256:" + strings.Repeat("a", 64)
			require.NoError(t, db.Model(&CatalogSyncState{}).Where("id = ?", CatalogSyncStateID).Update("runtime_fingerprint", fingerprint).Error)
			require.NoError(t, MigrateCatalogSync(db))
			var repeated CatalogSyncState
			require.NoError(t, db.First(&repeated, CatalogSyncStateID).Error)
			upgraded.RuntimeFingerprint = fingerprint
			assert.Equal(t, upgraded, repeated, "repeat migration must retain the exact epoch and state")
			var price Option
			require.NoError(t, db.Where(map[string]any{"key": "ModelPrice"}).First(&price).Error)
			assert.Equal(t, `{"migration-priced":2}`, price.Value)
		})
	}
}
