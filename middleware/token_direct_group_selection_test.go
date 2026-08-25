package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDirectTokenGroupSelectionTest(t *testing.T) {
	t.Helper()

	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousIsMasterNode := common.IsMasterNode
	previousSQLitePath := common.SQLitePath
	previousSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()

	common.IsMasterNode = false
	common.SQLitePath = filepath.Join(t.TempDir(), "token-groups.db")
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	require.NoError(t, model.InitDB())
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}))
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))

	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.IsMasterNode = previousIsMasterNode
		common.SQLitePath = previousSQLitePath
		if hadSQLDSN {
			require.NoError(t, os.Setenv("SQL_DSN", previousSQLDSN))
		} else {
			require.NoError(t, os.Unsetenv("SQL_DSN"))
		}
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
	})
}

func TestTokenAuthAllowsExplicitOrderedGroupsWithoutConfiguredAutoGroup(t *testing.T) {
	setupDirectTokenGroupSelectionTest(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username: "direct-group-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Quota:    1000,
	}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		UserId:          user.Id,
		Key:             "directgroupkey",
		Name:            "direct groups",
		Status:          common.TokenStatusEnabled,
		ExpiredTime:     -1,
		UnlimitedQuota:  true,
		Group:           "auto",
		CrossGroupRetry: true,
	}
	require.NoError(t, token.SetAutoGroups([]string{"vip", "default"}))
	require.NoError(t, model.DB.Create(token).Error)

	router := gin.New()
	router.GET("/v1/models", TokenAuth(), func(c *gin.Context) {
		groups, ok := common.GetContextKey(c, constant.ContextKeyTokenAutoGroups)
		c.JSON(http.StatusOK, gin.H{"groups": groups, "has_groups": ok})
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-directgroupkey")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"groups":["vip","default"]`)
}

func TestTokenAuthStillRejectsInheritedAutoWithoutConfiguredAutoGroup(t *testing.T) {
	setupDirectTokenGroupSelectionTest(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username: "legacy-auto-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Quota:    1000,
	}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		UserId:          user.Id,
		Key:             "legacyautokey",
		Name:            "legacy auto",
		Status:          common.TokenStatusEnabled,
		ExpiredTime:     -1,
		UnlimitedQuota:  true,
		Group:           "auto",
		CrossGroupRetry: true,
	}
	require.NoError(t, model.DB.Create(token).Error)

	router := gin.New()
	router.GET("/v1/models", TokenAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-legacyautokey")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestTokenAuthRejectsExplicitGroupsAfterPermissionsAreRevoked(t *testing.T) {
	setupDirectTokenGroupSelectionTest(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username: "revoked-group-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Quota:    1000,
	}
	require.NoError(t, model.DB.Create(user).Error)
	token := &model.Token{
		UserId:          user.Id,
		Key:             "revokedgroupkey",
		Name:            "revoked groups",
		Status:          common.TokenStatusEnabled,
		ExpiredTime:     -1,
		UnlimitedQuota:  true,
		Group:           "auto",
		CrossGroupRetry: true,
	}
	require.NoError(t, token.SetAutoGroups([]string{"revoked"}))
	require.NoError(t, model.DB.Create(token).Error)

	router := gin.New()
	router.GET("/v1/models", TokenAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer sk-revokedgroupkey")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
}
