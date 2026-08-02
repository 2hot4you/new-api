package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validCOSTestConfig() COSConfig {
	return COSConfig{
		Enabled:             true,
		Bucket:              "molii-assets-1250000000",
		Region:              "ap-guangzhou",
		SecretID:            "AKIDTEST",
		SecretKey:           "SECRETTEST",
		CustomDomain:        "https://assets.molii.co",
		PathPrefix:          "users",
		UploadExpiryMinutes: 30,
		ReadExpiryMinutes:   60,
	}
}

func TestCOSConfigCanValidateCredentialsWhileUploadsAreDisabled(t *testing.T) {
	config := validCOSTestConfig()
	config.Enabled = false

	require.Error(t, config.Validate())
	require.NoError(t, config.ValidateCredentials())
}

func TestCOSConfigRejectsUnsafeEndpointAndPath(t *testing.T) {
	config := validCOSTestConfig()
	config.CustomDomain = "http://assets.molii.co/path"
	require.Error(t, config.ValidateCredentials())

	config = validCOSTestConfig()
	config.PathPrefix = "../private"
	require.Error(t, config.ValidateCredentials())
}
