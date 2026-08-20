package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserGroupsJoinsMetadataAndAssignsStableDisplayOrder(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	user := &model.User{Id: 401, Username: "group-metadata-user", Group: "default"}
	require.NoError(t, db.Create(user).Error)

	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalMetadata := ratio_setting.GroupMetadata2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(originalMetadata))
	})
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"auto":1,"default":1,"vip":2,"zeta":3}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","zeta":"Zeta","auto":"Auto"}`))
	require.NoError(t, ratio_setting.UpdateGroupMetadataByJSONString(`[
		{"name":"vip","icon":"DeepSeek.Color","recommendation":4},
		{"name":"auto","icon":"OpenAI.Color","recommendation":0}
	]`))

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
	context.Set("id", user.Id)

	GetUserGroups(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                       `json:"success"`
		Data    map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	groups := make(map[string]map[string]any, len(response.Data))
	for name, raw := range response.Data {
		var group map[string]any
		require.NoError(t, common.Unmarshal(raw, &group))
		groups[name] = group
	}

	assert.Equal(t, float64(2), groups["vip"]["ratio"])
	assert.Equal(t, "VIP", groups["vip"]["desc"])
	assert.Equal(t, "DeepSeek.Color", groups["vip"]["icon"])
	assert.Equal(t, float64(4), groups["vip"]["recommendation"])
	assert.Equal(t, float64(0), groups["vip"]["display_order"])

	assert.Equal(t, "自动", groups["auto"]["ratio"])
	assert.Equal(t, "OpenAI.Color", groups["auto"]["icon"])
	assert.Equal(t, float64(0), groups["auto"]["recommendation"])
	assert.Equal(t, float64(1), groups["auto"]["display_order"])
	assert.Equal(t, float64(2), groups["default"]["display_order"])
	assert.Equal(t, float64(3), groups["zeta"]["display_order"])
	_, hasDefaultIcon := groups["default"]["icon"]
	assert.False(t, hasDefaultIcon)
}
