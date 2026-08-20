package controller

import (
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	type userGroupEntry struct {
		name  string
		ratio interface{}
		desc  string
	}
	entries := make([]userGroupEntry, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		if groupName == "auto" {
			continue
		}
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			entries = append(entries, userGroupEntry{name: groupName, ratio: service.GetUserGroupRatio(userGroup, groupName), desc: desc})
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		entries = append(entries, userGroupEntry{name: "auto", ratio: "自动", desc: setting.GetUsableGroupDescription("auto")})
	}

	metadataByName := make(map[string]struct {
		metadata ratio_setting.GroupMetadata
		order    int
	})
	for order, metadata := range ratio_setting.GetGroupMetadataCopy() {
		metadataByName[metadata.Name] = struct {
			metadata ratio_setting.GroupMetadata
			order    int
		}{metadata: metadata, order: order}
	}
	sort.Slice(entries, func(i, j int) bool {
		left, leftConfigured := metadataByName[entries[i].name]
		right, rightConfigured := metadataByName[entries[j].name]
		if leftConfigured && rightConfigured {
			return left.order < right.order
		}
		if leftConfigured != rightConfigured {
			return leftConfigured
		}
		return entries[i].name < entries[j].name
	})

	usableGroups := make(map[string]map[string]interface{}, len(entries))
	for displayOrder, entry := range entries {
		group := map[string]interface{}{
			"ratio":         entry.ratio,
			"desc":          entry.desc,
			"display_order": displayOrder,
		}
		if configured, ok := metadataByName[entry.name]; ok {
			if configured.metadata.Icon != "" {
				group["icon"] = configured.metadata.Icon
			}
			group["recommendation"] = configured.metadata.Recommendation
		}
		usableGroups[entry.name] = group
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
