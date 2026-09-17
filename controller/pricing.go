package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type publicPricingGroupMetadata struct {
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

func buildPublicPricingGroupMetadata(usableGroup map[string]string) map[string]publicPricingGroupMetadata {
	metadata := make(map[string]publicPricingGroupMetadata, len(usableGroup))
	for group, description := range usableGroup {
		metadata[group] = publicPricingGroupMetadata{Description: description}
	}
	for _, configured := range ratio_setting.GetGroupMetadataCopy() {
		entry, ok := metadata[configured.Name]
		if !ok {
			continue
		}
		entry.Icon = configured.Icon
		metadata[configured.Name] = entry
	}
	return metadata
}

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func filterPublicPricingGroups(pricing []model.Pricing, selectableGroups map[string]string) map[string]string {
	visibleGroups := make(map[string]struct{})
	allGroupsVisible := false
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			allGroupsVisible = true
		}
		for _, group := range item.EnableGroup {
			visibleGroups[group] = struct{}{}
		}
	}

	groups := make(map[string]string, len(selectableGroups))
	for group, description := range selectableGroups {
		if _, visible := visibleGroups[group]; visible || allGroupsVisible {
			groups[group] = description
		}
	}
	return groups
}

func buildPublicPricingGroupRatios(userGroup string, usableGroups map[string]string) map[string]float64 {
	configuredRatios := ratio_setting.GetGroupRatioCopy()
	groupRatios := make(map[string]float64, len(usableGroups))
	for group := range usableGroups {
		ratio, configured := configuredRatios[group]
		if !configured {
			continue
		}
		if specialRatio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group); ok {
			ratio = specialRatio
		}
		groupRatios[group] = ratio
	}
	return groupRatios
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
		}
	}

	selectableGroups := service.GetUserSelectableGroups(group)
	pricing = filterPricingByUsableGroups(pricing, selectableGroups)
	usableGroup := filterPublicPricingGroups(pricing, selectableGroups)
	groupRatio := buildPublicPricingGroupRatios(group, usableGroup)

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"group_metadata":     buildPublicPricingGroupMetadata(usableGroup),
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
