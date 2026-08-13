package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
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

func TestPricingUsesPersistedMetadataAndReferencedVendorsOnly(t *testing.T) {
	resetPricingEndpointTestTables(t)
	insertPricingEndpointChannel(t, 802, constant.ChannelTypeOpenAI, dtoChannelSettingsEmpty())
	insertPricingEndpointAbility(t, 802, "catalog-priced-model")

	vendor := Vendor{Name: "Published Vendor", Description: "Published introduction", Icon: "OpenAI", Status: 1}
	require.NoError(t, vendor.Insert())
	unused := Vendor{Name: "Unused Vendor", Description: "Must not be returned", Icon: "Claude.Color", Status: 1}
	require.NoError(t, unused.Insert())
	entry := Model{
		ModelName: "catalog-priced-model", Description: "Persisted model", Icon: "OpenAI", Tags: "chat",
		VendorID: vendor.Id, Status: 1, SyncOfficial: 1, ContextLength: 123_456, MaxOutputTokens: 7_890,
		KnowledgeCutoff: "2025-04", ReleaseDate: "2026-01-02", InputModalities: []string{"text", "image"},
		OutputModalities: []string{"text"}, Capabilities: []string{"streaming", "tools"},
		MetadataSource: "admin", MetadataVerifiedAt: "2026-08-13",
	}
	require.NoError(t, entry.Insert())

	InvalidatePricingCache()
	pricing := findPricingModel(GetPricing(), entry.ModelName)
	require.NotNil(t, pricing)
	assert.Equal(t, entry.Description, pricing.Description)
	assert.Equal(t, entry.ContextLength, pricing.ContextLength)
	assert.Equal(t, entry.MaxOutputTokens, pricing.MaxOutputTokens)
	assert.Equal(t, entry.InputModalities, pricing.InputModalities)
	assert.Equal(t, entry.OutputModalities, pricing.OutputModalities)
	assert.Equal(t, entry.Capabilities, pricing.Capabilities)
	assert.Equal(t, entry.MetadataSource, pricing.MetadataSource)
	assert.Equal(t, entry.MetadataVerifiedAt, pricing.MetadataVerifiedAt)

	vendors := GetVendors()
	require.Len(t, vendors, 1)
	assert.Equal(t, vendor.Id, vendors[0].ID)
	assert.Equal(t, "Published introduction", vendors[0].Description)
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
