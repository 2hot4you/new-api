package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

type moliiGrokManagementStatusResponse struct {
	Success bool `json:"success"`
	Data    *struct {
		QuotaPerUnit *float64 `json:"quota_per_unit"`
	} `json:"data"`
}

type moliiGrokManagementSelfResponse struct {
	Success bool `json:"success"`
	Data    *struct {
		Quota *float64 `json:"quota"`
	} `json:"data"`
}

const defaultMoliiGrokManagementRequestTimeout = 15 * time.Second

var moliiGrokManagementRequestTimeout = defaultMoliiGrokManagementRequestTimeout

func effectiveMoliiGrokManagementRequestTimeout() time.Duration {
	if moliiGrokManagementRequestTimeout <= 0 {
		return defaultMoliiGrokManagementRequestTimeout
	}
	return moliiGrokManagementRequestTimeout
}

func moliiGrokManagementBaseURL() (string, error) {
	rawBaseURL := strings.TrimSpace(constant.MoliiGrokNewAPIBaseURL)
	baseURL := strings.TrimRight(rawBaseURL, "/")
	parsedBaseURL, parseErr := url.Parse(rawBaseURL)
	if baseURL == "" || parseErr != nil || parsedBaseURL == nil ||
		(parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https") ||
		parsedBaseURL.Hostname() == "" || parsedBaseURL.User != nil ||
		parsedBaseURL.RawQuery != "" || parsedBaseURL.ForceQuery ||
		parsedBaseURL.Fragment != "" || strings.Contains(rawBaseURL, "#") ||
		parsedBaseURL.Opaque != "" || (parsedBaseURL.Path != "" && parsedBaseURL.Path != "/") {
		return "", errors.New("Molii Grok 管理接口配置无效，请管理员检查: MOLII_GROK_NEW_API_BASE_URL")
	}
	return baseURL, nil
}

func moliiGrokManagementConfig() (string, string, int, error) {
	baseURL, baseURLErr := moliiGrokManagementBaseURL()
	accessToken := strings.TrimSpace(constant.MoliiGrokNewAPIAccessToken)
	userID := constant.MoliiGrokNewAPIUserID
	missing := make([]string, 0, 3)
	if baseURLErr != nil {
		missing = append(missing, "MOLII_GROK_NEW_API_BASE_URL")
	}
	if accessToken == "" {
		missing = append(missing, "MOLII_GROK_NEW_API_ACCESS_TOKEN")
	}
	if userID <= 0 {
		missing = append(missing, "MOLII_GROK_NEW_API_USER_ID")
	}
	if len(missing) > 0 {
		return "", "", 0, fmt.Errorf("Molii Grok 管理接口配置缺失或无效，请管理员检查: %s", strings.Join(missing, ", "))
	}
	return baseURL, accessToken, userID, nil
}

func sanitizeMoliiGrokManagementError(err error, secrets ...string) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return errors.New(message)
}

func getMoliiGrokManagementResponse(channel *model.Channel, requestURL string, headers http.Header, secrets ...string) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(context.Background(), effectiveMoliiGrokManagementRequestTimeout())
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, sanitizeMoliiGrokManagementError(errors.New("Molii Grok 管理请求地址无效"), secrets...)
	}
	request.Header = headers.Clone()
	client, err := service.NewProxyHttpClient(channel.GetSetting().Proxy)
	if err != nil {
		return nil, sanitizeMoliiGrokManagementError(fmt.Errorf("Molii Grok 管理请求代理配置无效: %w", err), secrets...)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, sanitizeMoliiGrokManagementError(fmt.Errorf("Molii Grok 管理请求失败: %w", err), secrets...)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Molii Grok 管理请求返回 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, errors.New("读取 Molii Grok 管理响应失败")
	}
	return body, nil
}

func updateChannelMoliiGrokBalance(channel *model.Channel) (float64, error) {
	if channel == nil {
		return 0, errors.New("Molii Grok 渠道配置缺失")
	}
	baseURL, accessToken, userID, err := moliiGrokManagementConfig()
	if err != nil {
		return 0, err
	}
	secrets := []string{accessToken, channel.Key}

	statusBody, err := getMoliiGrokManagementResponse(channel, baseURL+"/api/status", http.Header{}, secrets...)
	if err != nil {
		return 0, err
	}
	var statusResponse moliiGrokManagementStatusResponse
	if err := common.Unmarshal(statusBody, &statusResponse); err != nil {
		return 0, errors.New("Molii Grok 管理状态响应格式无效")
	}
	if !statusResponse.Success {
		return 0, errors.New("Molii Grok 管理状态请求未成功")
	}
	if statusResponse.Data == nil || statusResponse.Data.QuotaPerUnit == nil {
		return 0, errors.New("Molii Grok 管理状态响应缺少 data.quota_per_unit")
	}
	quotaPerUnit := *statusResponse.Data.QuotaPerUnit
	if math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) || quotaPerUnit <= 0 {
		return 0, errors.New("Molii Grok 管理状态返回无效 quota_per_unit")
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+accessToken)
	headers.Set("User-id", strconv.Itoa(userID))
	selfBody, err := getMoliiGrokManagementResponse(channel, baseURL+"/api/user/self", headers, secrets...)
	if err != nil {
		return 0, err
	}
	var selfResponse moliiGrokManagementSelfResponse
	if err := common.Unmarshal(selfBody, &selfResponse); err != nil {
		return 0, errors.New("Molii Grok 用户余额响应格式无效")
	}
	if !selfResponse.Success {
		return 0, errors.New("Molii Grok 用户余额请求未成功")
	}
	if selfResponse.Data == nil || selfResponse.Data.Quota == nil {
		return 0, errors.New("Molii Grok 用户余额响应缺少 data.quota")
	}
	quota := *selfResponse.Data.Quota
	if math.IsNaN(quota) || math.IsInf(quota, 0) || quota < 0 {
		return 0, errors.New("Molii Grok 用户余额返回无效 quota")
	}

	balance := quota / quotaPerUnit
	if math.IsNaN(balance) || math.IsInf(balance, 0) || balance < 0 {
		return 0, errors.New("Molii Grok 换算余额无效")
	}
	channel.UpdateBalance(balance)
	return balance, nil
}
