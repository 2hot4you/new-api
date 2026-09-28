package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuotaSettingForkDefaults(t *testing.T) {
	setting := GetQuotaSetting()
	previous := *setting
	t.Cleanup(func() { *setting = previous })

	assert.Zero(t, setting.TrustQuotaUSD, "missing trust quota configuration must keep wallet pre-consume enabled")
	assert.Equal(t, float64(1), setting.PreConsumeMultiplier)
}
