package model

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findPricingModel(items []Pricing, modelName string) *Pricing {
	for index := range items {
		if items[index].ModelName == modelName {
			return &items[index]
		}
	}
	return nil
}

func TestPricingReadOnlyOmitsAbilityWithoutPersistedMetadata(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 801, constant.ChannelTypeOpenAI, dtoChannelSettingsEmpty())
	insertPricingEndpointAbility(t, 801, "missing-metadata-model")

	var beforeModels, beforeVendors int64
	require.NoError(t, DB.Model(&Model{}).Count(&beforeModels).Error)
	require.NoError(t, DB.Model(&Vendor{}).Count(&beforeVendors).Error)
	InvalidatePricingCache()
	items := GetPricing()

	var afterModels, afterVendors int64
	require.NoError(t, DB.Model(&Model{}).Count(&afterModels).Error)
	require.NoError(t, DB.Model(&Vendor{}).Count(&afterVendors).Error)
	assert.Nil(t, findPricingModel(items, "missing-metadata-model"))
	assert.Equal(t, beforeModels, afterModels)
	assert.Equal(t, beforeVendors, afterVendors)
}

func dtoChannelSettingsEmpty() dto.ChannelOtherSettings {
	return dto.ChannelOtherSettings{}
}

func completePublishedLLM() Model {
	return Model{
		ModelName:             "marketplace-pricing-model",
		DisplayName:           "Marketplace Pricing Model",
		Description:           "A persisted public model.",
		DescriptionEN:         "An English public model description.",
		Icon:                  "OpenAI",
		Tags:                  "chat",
		Status:                1,
		SyncOfficial:          1,
		MarketplaceEnabled:    true,
		ContextLength:         123_456,
		MaxOutputTokens:       7_890,
		KnowledgeCutoff:       "2025-04",
		ReleaseDate:           "2026-01-02",
		InputModalities:       []string{"text"},
		OutputModalities:      []string{"text"},
		Capabilities:          []string{"streaming", "tools"},
		SupportedParameters:   []string{"stream", "tools"},
		SupportedResolutions:  []string{"1024x1024"},
		SupportedAspectRatios: []string{"1:1"},
		MaxInputImages:        3,
		OutputFormats:         []string{"url"},
		MinDuration:           1,
		MaxDuration:           15,
		ReferenceModalities:   []string{"image"},
		MetadataSource:        "admin",
		MetadataVerifiedAt:    "2026-08-13",
	}
}

func pricingModelIDs() []string {
	items := GetPricing()
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ModelName)
	}
	return ids
}

func pricingModelNames(items []Pricing) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.ModelName)
	}
	return names
}

func pricingVendorNames(items []PricingVendor) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func configureMarketplaceModelPrice(t *testing.T, modelName string) {
	t.Helper()
	originalPrices := ratio_setting.GetModelPriceCopy()
	t.Cleanup(func() {
		payload, err := json.Marshal(originalPrices)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(payload)))
	})
	prices := ratio_setting.GetModelPriceCopy()
	prices[modelName] = 0.42
	payload, err := json.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(payload)))
}

func seedVendorPriceGroupAndEndpoint(t *testing.T, row *Model) {
	t.Helper()
	vendor := Vendor{Name: "Published Vendor", Description: "Published introduction", Icon: "OpenAI", Status: 1}
	require.NoError(t, vendor.Insert())
	row.VendorID = vendor.Id
	require.NoError(t, row.Insert())
	configureMarketplaceModelPrice(t, row.ModelName)
	insertPricingEndpointChannel(t, 805, constant.ChannelTypeOpenAI, dtoChannelSettingsEmpty())
	insertPricingEndpointAbility(t, 805, row.ModelName)
}

func TestPricingRequiresExplicitCompleteMarketplacePublication(t *testing.T) {
	resetPricingEndpointTestTables(t)
	row := completePublishedLLM()
	seedVendorPriceGroupAndEndpoint(t, &row)
	RefreshPricing()
	require.Contains(t, pricingModelIDs(), row.ModelName)

	require.NoError(t, DB.Model(&row).Update("marketplace_enabled", false).Error)
	RefreshPricing()
	require.NotContains(t, pricingModelIDs(), row.ModelName)
}

func TestPricingMetadataIntersection(t *testing.T) {
	tests := []struct {
		name            string
		expectPublished bool
		mutate          func(t *testing.T, row *Model)
	}{
		{
			name:            "draft model",
			expectPublished: false,
			mutate: func(t *testing.T, row *Model) {
				require.NoError(t, DB.Model(row).Update("marketplace_enabled", false).Error)
			},
		},
		{
			name:            "incomplete metadata",
			expectPublished: true,
			mutate: func(t *testing.T, row *Model) {
				require.NoError(t, DB.Model(row).Update("display_name", "").Error)
			},
		},
		{
			name:            "disabled model",
			expectPublished: true,
			mutate: func(t *testing.T, row *Model) {
				require.NoError(t, DB.Model(row).Update("status", 0).Error)
			},
		},
		{
			name:            "disabled vendor",
			expectPublished: true,
			mutate: func(t *testing.T, row *Model) {
				require.NoError(t, DB.Model(&Vendor{}).Where("id = ?", row.VendorID).Update("status", 0).Error)
			},
		},
		{
			name:            "missing price",
			expectPublished: true,
			mutate: func(t *testing.T, row *Model) {
				payload, err := json.Marshal(map[string]float64{})
				require.NoError(t, err)
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(payload)))
			},
		},
		{
			name:            "empty groups",
			expectPublished: true,
			mutate: func(t *testing.T, row *Model) {
				require.NoError(t, DB.Where("model = ?", row.ModelName).Delete(&Ability{}).Error)
			},
		},
		{
			name:            "empty endpoints",
			expectPublished: true,
			mutate: func(t *testing.T, row *Model) {
				require.NoError(t, DB.Model(&Channel{}).Where("id = ?", 805).Update("type", constant.ChannelTypeMoliiGrokAIGC).Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetPricingEndpointTestTables(t)
			row := completePublishedLLM()
			seedVendorPriceGroupAndEndpoint(t, &row)
			tt.mutate(t, &row)
			RefreshPricing()
			require.NotContains(t, pricingModelIDs(), row.ModelName)

			var persisted Model
			require.NoError(t, DB.First(&persisted, row.Id).Error)
			require.Equal(t, tt.expectPublished, persisted.MarketplaceEnabled, "temporary blockers must not withdraw publication intent")
		})
	}
}

func TestPricingUsesPersistedMetadataAndReferencedVendorsOnly(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 802, constant.ChannelTypeOpenAI, dtoChannelSettingsEmpty())
	insertPricingEndpointAbility(t, 802, "catalog-priced-model")

	vendor := Vendor{Name: "Published Vendor", Description: "Published introduction", Icon: "OpenAI", Status: 1}
	require.NoError(t, vendor.Insert())
	unused := Vendor{Name: "Unused Vendor", Description: "Must not be returned", Icon: "Claude.Color", Status: 1}
	require.NoError(t, unused.Insert())
	entry := completePublishedLLM()
	entry.ModelName = "catalog-priced-model"
	entry.DisplayName = "Database Display Name"
	entry.Description = "Persisted model"
	entry.VendorID = vendor.Id
	require.NoError(t, entry.Insert())
	configureMarketplaceModelPrice(t, entry.ModelName)

	InvalidatePricingCache()
	pricing := findPricingModel(GetPricing(), entry.ModelName)
	require.NotNil(t, pricing)
	assert.Equal(t, entry.DisplayName, pricing.DisplayName)
	assert.Equal(t, entry.Description, pricing.Description)
	assert.Equal(t, entry.DescriptionEN, pricing.DescriptionEN)
	assert.Equal(t, entry.ContextLength, pricing.ContextLength)
	assert.Equal(t, entry.MaxOutputTokens, pricing.MaxOutputTokens)
	assert.Equal(t, entry.InputModalities, pricing.InputModalities)
	assert.Equal(t, entry.OutputModalities, pricing.OutputModalities)
	assert.Equal(t, entry.Capabilities, pricing.Capabilities)
	assert.Equal(t, entry.SupportedParameters, pricing.SupportedParameters)
	assert.Equal(t, entry.SupportedResolutions, pricing.SupportedResolutions)
	assert.Equal(t, entry.SupportedAspectRatios, pricing.SupportedAspectRatios)
	assert.Equal(t, entry.MaxInputImages, pricing.MaxInputImages)
	assert.Equal(t, entry.OutputFormats, pricing.OutputFormats)
	assert.Equal(t, entry.MinDuration, pricing.MinDuration)
	assert.Equal(t, entry.MaxDuration, pricing.MaxDuration)
	assert.Equal(t, entry.ReferenceModalities, pricing.ReferenceModalities)
	assert.Equal(t, entry.UpdatedTime, pricing.MetadataUpdatedTime)

	vendors := GetVendors()
	require.Len(t, vendors, 1)
	assert.Equal(t, vendor.Id, vendors[0].ID)
	assert.Equal(t, "Published introduction", vendors[0].Description)
}

func TestPricingModelsDisplayOrder(t *testing.T) {
	resetPricingEndpointTestTables(t)
	require.NoError(t, DB.AutoMigrate(&marketplaceOrderLock{}))
	insertPricingEndpointChannel(t, 806, constant.ChannelTypeOpenAI, dtoChannelSettingsEmpty())

	vendorA := Vendor{Name: "Vendor A", Status: 1}
	require.NoError(t, vendorA.Insert())
	vendorB := Vendor{Name: "Vendor B", Status: 1}
	require.NoError(t, vendorB.Insert())
	require.NoError(t, ReorderVendors([]int{vendorB.Id, vendorA.Id}))

	manualFirst := completePublishedLLM()
	manualFirst.ModelName = "manual-first"
	manualFirst.VendorID = vendorB.Id
	manualFirst.ReleaseDate = "2026-01-01"

	manualSecond := completePublishedLLM()
	manualSecond.ModelName = "manual-second"
	manualSecond.VendorID = vendorA.Id
	manualSecond.ReleaseDate = "2026-12-31"
	require.NoError(t, manualSecond.Insert())
	configureMarketplaceModelPrice(t, manualSecond.ModelName)
	insertPricingEndpointAbility(t, 806, manualSecond.ModelName)
	require.NoError(t, manualFirst.Insert())
	configureMarketplaceModelPrice(t, manualFirst.ModelName)
	insertPricingEndpointAbility(t, 806, manualFirst.ModelName)
	require.NoError(t, ReorderModels([]int{manualFirst.Id, manualSecond.Id}))

	RefreshPricing()
	assert.Equal(t,
		[]string{"manual-first", "manual-second"},
		pricingModelNames(GetPricing()),
	)
	assert.Equal(t,
		[]string{"Vendor B", "Vendor A"},
		pricingVendorNames(GetVendors()),
	)

	first := findPricingModel(GetPricing(), manualFirst.ModelName)
	require.NotNil(t, first)
	assert.Equal(t, 1, first.DisplayOrder)
	assert.Equal(t, 1, GetVendors()[0].DisplayOrder)
}

func TestPricingExcludesDisabledModelAndVendor(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 803, constant.ChannelTypeOpenAI, dtoChannelSettingsEmpty())
	insertPricingEndpointAbility(t, 803, "disabled-model")
	insertPricingEndpointAbility(t, 803, "disabled-vendor-model")

	enabledVendor := Vendor{Name: "Enabled Vendor", Status: 1}
	require.NoError(t, enabledVendor.Insert())
	disabledVendor := Vendor{Name: "Disabled Vendor", Status: 2}
	require.NoError(t, disabledVendor.Insert())
	require.NoError(t, (&Model{ModelName: "disabled-model", VendorID: enabledVendor.Id, Status: 0}).Insert())
	require.NoError(t, (&Model{ModelName: "disabled-vendor-model", VendorID: disabledVendor.Id, Status: 1}).Insert())

	InvalidatePricingCache()
	items := GetPricing()
	assert.Nil(t, findPricingModel(items, "disabled-model"))
	assert.Nil(t, findPricingModel(items, "disabled-vendor-model"))
}
