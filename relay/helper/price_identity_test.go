package helper

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestPerCallPricingUsesBillingIdentity(t *testing.T) {
	oldPrices := ratio_setting.ModelPrice2JSONString()
	oldRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldRatios))
	})
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"client-alias":9,"selected-image":2,"modifier-task":3}`))
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"ratio-task":4}`))
	for _, tc := range []struct {
		name, origin, billing string
		price, ratio          float64
	}{
		{"explicit image mapping", "client-alias", "selected-image", 2, 0},
		{"canonical modifier", "modifier-task@thinking:on", "", 3, 0},
		{"canonical ratio", "ratio-task@thinking:on", "", -1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			info := &relaycommon.RelayInfo{OriginModelName: tc.origin, BillingModelName: tc.billing, UserGroup: "default", UsingGroup: "default", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "routed-provider"}}
			price, err := ModelPriceHelperPerCall(c, info)
			require.NoError(t, err)
			require.Equal(t, tc.price, price.ModelPrice)
			require.Equal(t, tc.ratio, price.ModelRatio)
			require.Equal(t, tc.origin, info.OriginModelName)
			require.Equal(t, "routed-provider", info.UpstreamModelName)
		})
	}
}
