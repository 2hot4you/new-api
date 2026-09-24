package controller

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"gorm.io/gorm"
)

// resolveTemporaryAssetChannel keeps old bindings on direct StarAI while new
// assets prefer an enabled reseller channel when one is configured.
func resolveTemporaryAssetChannel(binding *service.StarAIAssetBinding) (*model.Channel, error) {
	channelType := constant.ChannelTypeStarAI
	if binding == nil {
		channel, err := model.GetFirstEnabledChannelByType(constant.ChannelTypeByteDanceSeedance)
		if err == nil {
			return channel, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	} else {
		if binding.ChannelType != 0 {
			channelType = binding.ChannelType
		}
		if channelType != constant.ChannelTypeStarAI && channelType != constant.ChannelTypeByteDanceSeedance {
			return nil, errors.New("unsupported temporary asset channel type")
		}
		if binding.ChannelID > 0 {
			channel, err := model.GetChannelById(binding.ChannelID, true)
			if err == nil {
				if channel.Type != channelType {
					return nil, errors.New("temporary asset channel type mismatch")
				}
				if channel.Status == common.ChannelStatusEnabled {
					return channel, nil
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
	}
	return model.GetFirstEnabledChannelByType(channelType)
}

func doTemporaryAssetRequest(channel *model.Channel, method, path string, body io.Reader) ([]byte, int, error) {
	baseURL := strings.TrimRight(channel.GetBaseURL(), "/")
	if baseURL == "" {
		return nil, 0, errors.New("temporary asset upstream URL is required")
	}
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	req.Header.Set("Content-Type", "application/json")
	client, err := service.GetHttpClientWithProxy(channel.GetSetting().Proxy)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return data, resp.StatusCode, err
}
