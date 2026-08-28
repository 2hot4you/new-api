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
	"sort"
	"strings"
	"time"

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
	Data   *starAIAssetVerificationResponse `json:"data,omitempty"`
}

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

func StarAIChannelKeyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}

func starAIAssetKey(id string) string       { return "starai:asset:" + id }
func starAIAssetIndexKey(userID int) string { return fmt.Sprintf("starai:assets:user:%d", userID) }

func SaveStarAIAssetBinding(binding *StarAIAssetBinding) error {
	if !common.RedisEnabled || common.RDB == nil {
		return ErrStarAIAssetUnavailable
	}
	if binding.ID == "" {
		binding.ID = "asset-molii-" + strings.ToLower(common.GetRandomString(24))
	}
	now := time.Now()
	ttl := time.Duration(constant.StarAIAssetTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
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
	pipe.Set(ctx, starAIAssetKey(binding.ID), body, ttl)
	pipe.ZAdd(ctx, starAIAssetIndexKey(binding.UserID), &redis.Z{Score: float64(binding.CreatedAt), Member: binding.ID})
	pipe.Expire(ctx, starAIAssetIndexKey(binding.UserID), ttl+time.Hour)
	if binding.COSKey != "" {
		pipe.ZAdd(ctx, starAICOSCleanupIndexKey, &redis.Z{Score: float64(binding.ExpiresAt), Member: binding.COSKey})
	}
	_, err = pipe.Exec(ctx)
	return err
}

func GetStarAIAssetBinding(id string, userID int) (*StarAIAssetBinding, error) {
	binding, err := GetStarAIAssetBindingForAdmin(id)
	if err != nil {
		return nil, err
	}
	if binding.UserID != userID {
		return nil, ErrStarAIAssetForbidden
	}
	return binding, nil
}

func GetStarAIAssetBindingForAdmin(id string) (*StarAIAssetBinding, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return nil, ErrStarAIAssetUnavailable
	}
	body, err := common.RDB.Get(context.Background(), starAIAssetKey(id)).Bytes()
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
	return &binding, nil
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
	iterator := common.RDB.Scan(ctx, 0, "starai:asset:asset-molii-*", 200).Iterator()
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
	iterator := common.RDB.Scan(ctx, 0, "starai:asset:asset-molii-*", 200).Iterator()
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
	pipe.Del(ctx, starAIAssetKey(binding.ID))
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
	pipe.Del(ctx, starAIAssetKey(binding.ID))
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
	if binding == nil || !common.RedisEnabled || common.RDB == nil {
		return ErrStarAIAssetUnavailable
	}
	binding.Status = NormalizeStarAIAssetStatus(status)
	binding.VerifiedAt = time.Now().Unix()
	ttl, err := common.RDB.TTL(context.Background(), starAIAssetKey(binding.ID)).Result()
	if err != nil || ttl <= 0 {
		return ErrStarAIAssetNotFound
	}
	body, _ := common.Marshal(binding)
	return common.RDB.Set(context.Background(), starAIAssetKey(binding.ID), body, ttl).Err()
}

func UpdateStarAIAssetSourceURL(binding *StarAIAssetBinding, sourceURL string) error {
	if binding == nil || !common.RedisEnabled || common.RDB == nil {
		return ErrStarAIAssetUnavailable
	}
	binding.SourceURL = strings.TrimSpace(sourceURL)
	ttl, err := common.RDB.TTL(context.Background(), starAIAssetKey(binding.ID)).Result()
	if err != nil || ttl <= 0 {
		return ErrStarAIAssetNotFound
	}
	body, err := common.Marshal(binding)
	if err != nil {
		return err
	}
	return common.RDB.Set(context.Background(), starAIAssetKey(binding.ID), body, ttl).Err()
}

func ResolveStarAIAssetURI(ctx context.Context, raw string, userID int, config StarAIAssetVerificationConfig) (string, error) {
	if !strings.HasPrefix(raw, "asset://") {
		return raw, nil
	}
	id := strings.TrimPrefix(raw, "asset://")
	if !strings.HasPrefix(id, "asset-molii-") {
		return "", ErrStarAIAssetForbidden
	}
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
	if err := UpdateStarAIAssetStatus(binding, status); err != nil {
		return "", err
	}
	switch status {
	case "ACTIVE", "SUCCESS":
		return "asset://" + binding.UpstreamID, nil
	case "EXPIRED":
		return "", ErrStarAIAssetExpired
	default:
		return "", fmt.Errorf("%w (status=%s)", ErrStarAIAssetNotReady, status)
	}
}
