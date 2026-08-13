package model

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupModelCatalogMetadataTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "model-catalog.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := db.AutoMigrate(&Model{}); err != nil {
		t.Fatalf("migrate model: %v", err)
	}
	DB = db
	t.Cleanup(func() { DB = previousDB })
}

func TestModelCatalogMetadataRoundTripAndExplicitClear(t *testing.T) {
	setupModelCatalogMetadataTestDB(t)

	entry := &Model{
		ModelName:          "catalog-model",
		Status:             1,
		SyncOfficial:       1,
		ContextLength:      1_000_000,
		MaxOutputTokens:    65_536,
		KnowledgeCutoff:    "2025-04",
		ReleaseDate:        "2026-02-16",
		InputModalities:    []string{"text", "image"},
		OutputModalities:   []string{"text"},
		Capabilities:       []string{"streaming", "tools"},
		MetadataSource:     "models.dev",
		MetadataVerifiedAt: "2026-08-13",
	}
	if err := entry.Insert(); err != nil {
		t.Fatalf("insert model: %v", err)
	}

	var stored Model
	if err := DB.First(&stored, entry.Id).Error; err != nil {
		t.Fatalf("read model: %v", err)
	}
	if stored.ContextLength != 1_000_000 || stored.MaxOutputTokens != 65_536 {
		t.Fatalf("numeric metadata not persisted: %+v", stored)
	}
	if !reflect.DeepEqual(stored.InputModalities, []string{"text", "image"}) ||
		!reflect.DeepEqual(stored.OutputModalities, []string{"text"}) ||
		!reflect.DeepEqual(stored.Capabilities, []string{"streaming", "tools"}) {
		t.Fatalf("list metadata not persisted: %+v", stored)
	}

	stored.ContextLength = 0
	stored.MaxOutputTokens = 0
	stored.KnowledgeCutoff = ""
	stored.ReleaseDate = ""
	stored.InputModalities = []string{}
	stored.OutputModalities = []string{}
	stored.Capabilities = []string{}
	stored.MetadataSource = ""
	stored.MetadataVerifiedAt = ""
	if err := stored.Update(); err != nil {
		t.Fatalf("clear model metadata: %v", err)
	}

	var cleared Model
	if err := DB.First(&cleared, entry.Id).Error; err != nil {
		t.Fatalf("read cleared model: %v", err)
	}
	if cleared.ContextLength != 0 || cleared.MaxOutputTokens != 0 || cleared.KnowledgeCutoff != "" ||
		cleared.ReleaseDate != "" || len(cleared.InputModalities) != 0 || len(cleared.OutputModalities) != 0 ||
		len(cleared.Capabilities) != 0 || cleared.MetadataSource != "" || cleared.MetadataVerifiedAt != "" {
		t.Fatalf("explicit clear was not persisted: %+v", cleared)
	}
}

func TestNormalizeCatalogMetadata(t *testing.T) {
	entry := &Model{
		KnowledgeCutoff:    " 2025-04 ",
		ReleaseDate:        " 2026-02-16 ",
		InputModalities:    []string{" text ", "image", "text", ""},
		OutputModalities:   []string{" text ", "text"},
		Capabilities:       []string{" tools ", "streaming", "tools"},
		MetadataSource:     " models.dev ",
		MetadataVerifiedAt: " 2026-08-13 ",
	}
	if err := entry.NormalizeCatalogMetadata(); err != nil {
		t.Fatalf("normalize valid metadata: %v", err)
	}
	if !reflect.DeepEqual(entry.InputModalities, []string{"text", "image"}) {
		t.Fatalf("input modalities = %v", entry.InputModalities)
	}
	if !reflect.DeepEqual(entry.OutputModalities, []string{"text"}) {
		t.Fatalf("output modalities = %v", entry.OutputModalities)
	}
	if !reflect.DeepEqual(entry.Capabilities, []string{"tools", "streaming"}) {
		t.Fatalf("capabilities = %v", entry.Capabilities)
	}
	if entry.ReleaseDate != "2026-02-16" || entry.MetadataVerifiedAt != "2026-08-13" || entry.MetadataSource != "models.dev" {
		t.Fatalf("string metadata not trimmed: %+v", entry)
	}
}

func TestNormalizeCatalogMetadataRejectsInvalidValues(t *testing.T) {
	tests := []Model{
		{ContextLength: -1},
		{MaxOutputTokens: -1},
		{ReleaseDate: "2026/02/16"},
		{MetadataVerifiedAt: "13-08-2026"},
		{InputModalities: []string{"imaginary"}},
		{Capabilities: []string{"unknown-capability"}},
	}
	for index := range tests {
		if err := tests[index].NormalizeCatalogMetadata(); err == nil {
			t.Fatalf("case %d should fail validation", index)
		}
	}
}
