package operation_setting

import (
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	COSDefaultPathPrefix          = "users"
	COSDefaultUploadExpiryMinutes = 30
	COSDefaultReadExpiryMinutes   = 60
)

type COSConfig struct {
	Enabled             bool
	Bucket              string
	Region              string
	SecretID            string
	SecretKey           string
	CustomDomain        string
	PathPrefix          string
	UploadExpiryMinutes int
	ReadExpiryMinutes   int
}

func GetCOSConfig() COSConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	uploadExpiry, _ := strconv.Atoi(common.OptionMap["COSUploadExpiryMinutes"])
	if uploadExpiry <= 0 {
		uploadExpiry = COSDefaultUploadExpiryMinutes
	}
	readExpiry, _ := strconv.Atoi(common.OptionMap["COSReadExpiryMinutes"])
	if readExpiry <= 0 {
		readExpiry = COSDefaultReadExpiryMinutes
	}
	prefix := strings.Trim(strings.TrimSpace(common.OptionMap["COSPathPrefix"]), "/")
	if prefix == "" {
		prefix = COSDefaultPathPrefix
	}
	return COSConfig{
		Enabled:             common.OptionMap["COSEnabled"] == "true",
		Bucket:              strings.TrimSpace(common.OptionMap["COSBucket"]),
		Region:              strings.TrimSpace(common.OptionMap["COSRegion"]),
		SecretID:            strings.TrimSpace(common.OptionMap["COSSecretID"]),
		SecretKey:           strings.TrimSpace(common.OptionMap["COSSecretKey"]),
		CustomDomain:        strings.TrimRight(strings.TrimSpace(common.OptionMap["COSCustomDomain"]), "/"),
		PathPrefix:          path.Clean(prefix),
		UploadExpiryMinutes: uploadExpiry,
		ReadExpiryMinutes:   readExpiry,
	}
}

func (config COSConfig) Validate() error {
	if !config.Enabled {
		return errors.New("腾讯云 COS 对象存储未启用")
	}
	return config.ValidateCredentials()
}

// ValidateCredentials validates the stored COS connection independently of the
// upload switch. Existing objects must remain readable and removable after new
// local uploads are disabled, and administrators should be able to test a
// configuration before enabling it.
func (config COSConfig) ValidateCredentials() error {
	if config.Bucket == "" || config.Region == "" || config.SecretID == "" || config.SecretKey == "" {
		return errors.New("腾讯云 COS 的存储桶、地域、SecretId 和 SecretKey 必须完整配置")
	}
	if config.CustomDomain != "" {
		parsed, err := url.Parse(config.CustomDomain)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path != "" {
			return errors.New("COS 自定义源站域名必须是没有路径的 HTTPS 地址")
		}
	}
	if config.PathPrefix == "." || strings.HasPrefix(config.PathPrefix, "..") {
		return errors.New("COS 路径前缀无效")
	}
	if config.UploadExpiryMinutes < 5 || config.UploadExpiryMinutes > 120 {
		return errors.New("COS 上传签名有效期必须在 5 到 120 分钟之间")
	}
	if config.ReadExpiryMinutes < 5 || config.ReadExpiryMinutes > 1440 {
		return errors.New("COS 读取签名有效期必须在 5 到 1440 分钟之间")
	}
	return nil
}
