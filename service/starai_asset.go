package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/go-redis/redis/v8"
)

var (
	ErrStarAIAssetUnavailable = errors.New("temporary assets require Redis")
	ErrStarAIAssetNotFound    = errors.New("temporary asset not found or expired")
	ErrStarAIAssetForbidden   = errors.New("temporary asset does not belong to this user")
	ErrStarAIAssetExpired     = errors.New("temporary asset has expired upstream")
	ErrStarAIAssetNotReady    = errors.New("temporary asset is not ready upstream")
	ErrStarAIAssetVerify      = errors.New("temporary asset upstream verification failed")
	ErrStarAIAssetAmbiguous   = errors.New("temporary asset ID is shared by multiple users")
)

type StarAIAssetBinding struct {
	ID                    string `json:"id"`
	UpstreamID            string `json:"upstream_id"`
	ChannelID             int    `json:"channel_id,omitempty"`
	ChannelKeyFingerprint string `json:"channel_key_fingerprint,omitempty"`
	UserID                int    `json:"user_id"`
	TokenID               int    `json:"token_id"`
	AssetType             string `json:"asset_type"`
	Name                  string `json:"name"`
	SourceURL             string `json:"source_url"`
	SourceKind            string `json:"source_kind,omitempty"`
	COSKey                string `json:"cos_key,omitempty"`
	FileName              string `json:"file_name,omitempty"`
	ContentType           string `json:"content_type,omitempty"`
	FileSize              int64  `json:"file_size,omitempty"`
	Status                string `json:"status"`
	CreatedAt             int64  `json:"created_at"`
	ExpiresAt             int64  `json:"expires_at"`
	VerifiedAt            int64  `json:"verified_at"`
	ErrorCode             string `json:"error_code,omitempty"`
	ErrorMessage          string `json:"error_message,omitempty"`
}

type StarAIAssetStats struct {
	Total        int            `json:"total"`
	Processing   int            `json:"processing"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	Expired      int            `json:"expired"`
	ExpiringSoon int            `json:"expiring_soon"`
	Users        int            `json:"users"`
	ByType       map[string]int `json:"by_type"`
}

type StarAIAssetVerificationConfig struct {
	ChannelID int
	BaseURL   string
	APIKey    string
	Proxy     string
}

type starAIAssetVerificationResponse struct {
	Status string                           `json:"status"`
	Error  *starAIAssetVerificationError    `json:"error,omitempty"`
	Data   *starAIAssetVerificationResponse `json:"data,omitempty"`
}

type starAIAssetVerificationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var (
	starAIAssetURLPattern    = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
	starAIAssetSecretPattern = regexp.MustCompile(`(?i)(bearer\s+|sk-|(?:api[_-]?key|token|secret|authorization)[=:]\s*)[a-z0-9._-]+`)
	starAIAssetIDPattern     = regexp.MustCompile(`(?i)\b(?:asset|task)-[a-z0-9_-]{8,}\b`)
	starAIBrandPattern       = regexp.MustCompile(`(?i)\bstar[\s_-]*ai\b`)
)

func (r *starAIAssetVerificationResponse) payload() *starAIAssetVerificationResponse {
	if r.Data != nil {
		return r.Data
	}
	return r
}

func NormalizeStarAIAssetStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "READY", "SUCCEEDED", "COMPLETED":
		return "SUCCESS"
	case "FAILURE", "ERROR":
		return "FAILED"
	case "DELETED", "NOT_FOUND", "NOTFOUND", "GONE":
		return "EXPIRED"
	default:
		return strings.ToUpper(strings.TrimSpace(status))
	}
}

func SanitizeStarAIAssetErrorMessage(value string) string {
	value = starAIAssetURLPattern.ReplaceAllString(value, "[URL]")
	value = starAIAssetSecretPattern.ReplaceAllString(value, "[REDACTED]")
	value = starAIAssetIDPattern.ReplaceAllString(value, "[ID]")
	value = common.MaskSensitiveInfo(value)
	value = starAIBrandPattern.ReplaceAllString(value, "Molii Volcengine Imagine API")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 240 {
		value = string(runes[:240]) + "…"
	}
	return strings.TrimSpace(value)
}

func SanitizeStarAIAssetErrorCode(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			return r
		}
		return -1
	}, value)
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func StarAIChannelKeyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

func starAIAssetLegacyKey(id string) string { return "starai:asset:" + id }
func starAIAssetUserKey(userID int, id string) string {
	return fmt.Sprintf("starai:asset:user:%d:%s", userID, id)
}
func starAIAssetIndexKey(userID int) string { return fmt.Sprintf("starai:assets:user:%d", userID) }

func starAIAssetBindingKey(binding *StarAIAssetBinding) string {
	if binding.ID != "" && binding.UpstreamID != "" && binding.ID != binding.UpstreamID {
		return starAIAssetLegacyKey(binding.ID)
	}
	return starAIAssetUserKey(binding.UserID, binding.ID)
}

func SaveStarAIAssetBinding(binding *StarAIAssetBinding) error {
	if !common.RedisEnabled || common.RDB == nil {
		return ErrStarAIAssetUnavailable
	}
	if strings.TrimSpace(binding.UpstreamID) == "" {
		return errors.New("temporary asset upstream ID is required")
	}
	binding.UpstreamID = strings.TrimSpace(binding.UpstreamID)
	binding.ID = binding.UpstreamID
	now := time.Now()
	ttl := time.Duration(constant.StarAIAssetTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = time.Duration(constant.DefaultStarAIAssetTTLHours) * time.Hour
	}
	binding.CreatedAt = now.Unix()
	binding.ExpiresAt = now.Add(ttl).Unix()
	binding.VerifiedAt = now.Unix()
	body, err := common.Marshal(binding)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pipe := common.RDB.TxPipeline()
	pipe.Set(ctx, starAIAssetUserKey(binding.UserID, binding.ID), body, ttl)
	pipe.ZAdd(ctx, starAIAssetIndexKey(binding.UserID), &redis.Z{Score: float64(binding.CreatedAt), Member: binding.ID})
	pipe.Expire(ctx, starAIAssetIndexKey(binding.UserID), ttl+time.Hour)
	if binding.COSKey != "" {
		pipe.ZAdd(ctx, starAICOSCleanupIndexKey, &redis.Z{Score: float64(binding.ExpiresAt), Member: binding.COSKey})
	}
	_, err = pipe.Exec(ctx)
	return err
}

func GetStarAIAssetBinding(id string, userID int) (*StarAIAssetBinding, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAIAssetUnavailable
	}
	body, err := common.RDB.Get(context.Background(), starAIAssetUserKey(userID, id)).Bytes()
	if err == nil {
		var binding StarAIAssetBinding
		if err := common.Unmarshal(body, &binding); err != nil {
			return nil, err
		}
		if binding.UserID != userID {
			return nil, ErrStarAIAssetForbidden
		}
		return &binding, nil
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if !strings.HasPrefix(id, "asset-molii-") {
		return nil, ErrStarAIAssetNotFound
	}
	body, err = common.RDB.Get(context.Background(), starAIAssetLegacyKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrStarAIAssetNotFound
	}
	if err != nil {
		return nil, err
	}
	var binding StarAIAssetBinding
	if err := common.Unmarshal(body, &binding); err != nil {
		return nil, err
	}
	if binding.UserID != userID {
		return nil, ErrStarAIAssetForbidden
	}
	return &binding, nil
}

func GetStarAIAssetBindingForAdmin(id string) (*StarAIAssetBinding, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAIAssetUnavailable
	}
	ctx := context.Background()
	if strings.HasPrefix(id, "asset-molii-") {
		body, err := common.RDB.Get(ctx, starAIAssetLegacyKey(id)).Bytes()
		if err == nil {
			var binding StarAIAssetBinding
			if err := common.Unmarshal(body, &binding); err != nil {
				return nil, err
			}
			return &binding, nil
		}
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
	}
	var found *StarAIAssetBinding
	iterator := common.RDB.Scan(ctx, 0, "starai:asset:user:*", 200).Iterator()
	for iterator.Next(ctx) {
		body, err := common.RDB.Get(ctx, iterator.Val()).Bytes()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var binding StarAIAssetBinding
		if common.Unmarshal(body, &binding) != nil || binding.ID != id {
			continue
		}
		if found != nil {
			return nil, ErrStarAIAssetAmbiguous
		}
		copy := binding
		found = &copy
	}
	if err := iterator.Err(); err != nil {
		return nil, err
	}
	if found == nil {
		return nil, ErrStarAIAssetNotFound
	}
	return found, nil
}

func ListStarAIAssetBindings(userID int) ([]StarAIAssetBinding, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAIAssetUnavailable
	}
	ctx := context.Background()
	ids, err := common.RDB.ZRevRange(ctx, starAIAssetIndexKey(userID), 0, 199).Result()
	if err != nil {
		return nil, err
	}
	items := make([]StarAIAssetBinding, 0, len(ids))
	for _, id := range ids {
		binding, getErr := GetStarAIAssetBinding(id, userID)
		if getErr == nil {
			items = append(items, *binding)
		} else if errors.Is(getErr, ErrStarAIAssetNotFound) {
			_ = common.RDB.ZRem(ctx, starAIAssetIndexKey(userID), id).Err()
		}
	}
	return items, nil
}

func ListAllStarAIAssetBindings() ([]StarAIAssetBinding, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAIAssetUnavailable
	}
	ctx := context.Background()
	items := make([]StarAIAssetBinding, 0)
	iterator := common.RDB.Scan(ctx, 0, "starai:asset:*", 200).Iterator()
	for iterator.Next(ctx) {
		body, err := common.RDB.Get(ctx, iterator.Val()).Bytes()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var binding StarAIAssetBinding
		if common.Unmarshal(body, &binding) == nil {
			items = append(items, binding)
		}
	}
	if err := iterator.Err(); err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	return items, nil
}

func GetStarAIAssetStats() (*StarAIAssetStats, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAIAssetUnavailable
	}
	ctx := context.Background()
	stats := &StarAIAssetStats{ByType: map[string]int{"image": 0, "video": 0, "audio": 0}}
	users := make(map[int]struct{})
	now := time.Now().Unix()
	iterator := common.RDB.Scan(ctx, 0, "starai:asset:*", 200).Iterator()
	for iterator.Next(ctx) {
		body, err := common.RDB.Get(ctx, iterator.Val()).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return nil, err
		}
		var binding StarAIAssetBinding
		if err := common.Unmarshal(body, &binding); err != nil {
			continue
		}
		stats.Total++
		users[binding.UserID] = struct{}{}
		stats.ByType[strings.ToLower(binding.AssetType)]++
		switch strings.ToUpper(binding.Status) {
		case "SUCCESS", "ACTIVE":
			stats.Success++
		case "FAILED":
			stats.Failed++
		case "EXPIRED":
			stats.Expired++
		default:
			stats.Processing++
		}
		if binding.ExpiresAt > now && binding.ExpiresAt <= now+6*60*60 {
			stats.ExpiringSoon++
		}
	}
	if err := iterator.Err(); err != nil {
		return nil, err
	}
	stats.Users = len(users)
	return stats, nil
}

func DeleteStarAIAssetBinding(id string, userID int) error {
	binding, err := GetStarAIAssetBinding(id, userID)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pipe := common.RDB.TxPipeline()
	pipe.Del(ctx, starAIAssetBindingKey(binding))
	pipe.ZRem(ctx, starAIAssetIndexKey(userID), binding.ID)
	_, err = pipe.Exec(ctx)
	if err == nil && binding.COSKey != "" {
		if deleteErr := DeleteStarAICOSObject(ctx, binding.COSKey); deleteErr != nil {
			common.SysError("failed to delete COS object with temporary asset: " + deleteErr.Error())
		}
	}
	return err
}

func DeleteStarAIAssetBindingForAdmin(id string) error {
	binding, err := GetStarAIAssetBindingForAdmin(id)
	if err != nil {
		return err
	}
	ctx := context.Background()
	pipe := common.RDB.TxPipeline()
	pipe.Del(ctx, starAIAssetBindingKey(binding))
	pipe.ZRem(ctx, starAIAssetIndexKey(binding.UserID), binding.ID)
	_, err = pipe.Exec(ctx)
	if err == nil && binding.COSKey != "" {
		if deleteErr := DeleteStarAICOSObject(ctx, binding.COSKey); deleteErr != nil {
			common.SysError("failed to delete COS object with temporary asset: " + deleteErr.Error())
		}
	}
	return err
}

func UpdateStarAIAssetStatus(binding *StarAIAssetBinding, status string) error {
	errorCode, errorMessage := "", ""
	if NormalizeStarAIAssetStatus(status) == "FAILED" {
		errorCode = binding.ErrorCode
		errorMessage = binding.ErrorMessage
	}
	return UpdateStarAIAssetVerification(binding, status, errorCode, errorMessage)
}

func UpdateStarAIAssetVerification(binding *StarAIAssetBinding, status, errorCode, errorMessage string) error {
	if binding == nil || !common.RedisEnabled || common.RDB == nil {
		return ErrStarAIAssetUnavailable
	}
	binding.Status = NormalizeStarAIAssetStatus(status)
	if binding.Status == "FAILED" {
		binding.ErrorCode = SanitizeStarAIAssetErrorCode(errorCode)
		binding.ErrorMessage = SanitizeStarAIAssetErrorMessage(errorMessage)
	} else {
		binding.ErrorCode = ""
		binding.ErrorMessage = ""
	}
	binding.VerifiedAt = time.Now().Unix()
	key := starAIAssetBindingKey(binding)
	ttl, err := common.RDB.TTL(context.Background(), key).Result()
	if err != nil || ttl <= 0 {
		return ErrStarAIAssetNotFound
	}
	body, _ := common.Marshal(binding)
	return common.RDB.Set(context.Background(), key, body, ttl).Err()
}

func UpdateStarAIAssetSourceURL(binding *StarAIAssetBinding, sourceURL string) error {
	if binding == nil || !common.RedisEnabled || common.RDB == nil {
		return ErrStarAIAssetUnavailable
	}
	binding.SourceURL = strings.TrimSpace(sourceURL)
	key := starAIAssetBindingKey(binding)
	ttl, err := common.RDB.TTL(context.Background(), key).Result()
	if err != nil || ttl <= 0 {
		return ErrStarAIAssetNotFound
	}
	body, err := common.Marshal(binding)
	if err != nil {
		return err
	}
	return common.RDB.Set(context.Background(), key, body, ttl).Err()
}

func ResolveStarAIAssetURI(ctx context.Context, raw string, userID int, config StarAIAssetVerificationConfig) (string, error) {
	if !strings.HasPrefix(raw, "asset://") {
		return raw, nil
	}
	id := strings.TrimPrefix(raw, "asset://")
	binding, err := GetStarAIAssetBinding(id, userID)
	if err != nil {
		return "", err
	}
	keyChanged := binding.ChannelKeyFingerprint != "" &&
		binding.ChannelKeyFingerprint != StarAIChannelKeyFingerprint(config.APIKey)
	if config.ChannelID > 0 && (binding.ChannelID != config.ChannelID || keyChanged) {
		if binding.COSKey != "" {
			resolved, resolveErr := GetStarAICOSPreviewURL(ctx, binding.COSKey)
			if resolveErr != nil {
				return "", fmt.Errorf("%w: source URL unavailable", ErrStarAIAssetVerify)
			}
			return resolved, nil
		}
		if sourceURL := strings.TrimSpace(binding.SourceURL); sourceURL != "" {
			return sourceURL, nil
		}
		return "", fmt.Errorf("%w: source URL unavailable", ErrStarAIAssetVerify)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(config.APIKey) == "" {
		return "", ErrStarAIAssetVerify
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/assets/"+url.PathEscape(binding.UpstreamID), nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrStarAIAssetVerify, err)
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client, err := GetHttpClientWithProxy(config.Proxy)
	if err != nil {
		return "", fmt.Errorf("%w: invalid channel proxy", ErrStarAIAssetVerify)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: request failed", ErrStarAIAssetVerify)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return "", fmt.Errorf("%w: response read failed", ErrStarAIAssetVerify)
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		if updateErr := UpdateStarAIAssetStatus(binding, "EXPIRED"); updateErr != nil {
			return "", updateErr
		}
		return "", ErrStarAIAssetExpired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: upstream status %d", ErrStarAIAssetVerify, resp.StatusCode)
	}
	var envelope starAIAssetVerificationResponse
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("%w: invalid response", ErrStarAIAssetVerify)
	}
	status := NormalizeStarAIAssetStatus(envelope.payload().Status)
	if status == "" {
		return "", fmt.Errorf("%w: response status missing", ErrStarAIAssetVerify)
	}
	payload := envelope.payload()
	errorCode, errorMessage := "", ""
	if payload.Error != nil {
		errorCode = payload.Error.Code
		errorMessage = payload.Error.Message
	}
	if err := UpdateStarAIAssetVerification(binding, status, errorCode, errorMessage); err != nil {
		return "", err
	}
	switch status {
	case "ACTIVE", "SUCCESS":
		return "asset://" + binding.UpstreamID, nil
	case "EXPIRED":
		return "", ErrStarAIAssetExpired
	case "FAILED":
		if binding.ErrorMessage != "" {
			return "", fmt.Errorf("%w: %s", ErrStarAIAssetVerify, binding.ErrorMessage)
		}
		return "", fmt.Errorf("%w: status=FAILED", ErrStarAIAssetVerify)
	default:
		return "", fmt.Errorf("%w (status=%s)", ErrStarAIAssetNotReady, status)
	}
}
