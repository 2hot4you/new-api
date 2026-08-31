package service

import (
	"encoding/json"
	"fmt"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

func BuildGPTImage2LogSnapshot(info *relaycommon.RelayInfo, request *dto.ImageRequest) *relaycommon.GPTImage2LogSnapshot {
	if info == nil || request == nil || info.OriginModelName != "gpt-image-2" {
		return nil
	}
	requestedOutputCount := 1
	if request.N != nil && *request.N > 0 {
		requestedOutputCount = int(*request.N)
	}
	operation := "generation"
	if strings.Contains(strings.ToLower(info.RequestURLPath), "/images/edits") {
		operation = "edit"
	}
	return &relaycommon.GPTImage2LogSnapshot{
		Version:              1,
		Model:                "gpt-image-2",
		Operation:            operation,
		Quality:              valueOrDefault(request.Quality, "auto"),
		Background:           rawStringOrDefault(request.Background, "auto"),
		OutputFormat:         rawStringOrDefault(request.OutputFormat, "png"),
		Moderation:           rawStringOrDefault(request.Moderation, "auto"),
		Size:                 valueOrDefault(request.Size, "auto"),
		User:                 rawStringOrDefault(request.User, ""),
		RequestedOutputCount: requestedOutputCount,
		OutputCount:          requestedOutputCount,
	}
}

func valueOrDefault(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func rawStringOrDefault(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback
	}
	return valueOrDefault(value, fallback)
}

func appendGPTImage2Log(other map[string]interface{}, info *relaycommon.RelayInfo) string {
	if other == nil || info == nil || info.GPTImage2Log == nil || info.OriginModelName != "gpt-image-2" {
		return ""
	}
	snapshot := info.GPTImage2Log
	other["gpt_image_2"] = snapshot
	if info.GPTImage2PreviewAvailable {
		other["gpt_image_2_preview_available"] = true
	}
	operation := "GPT Image 2 图片生成"
	if snapshot.Operation == "edit" {
		operation = "GPT Image 2 图片编辑"
	}
	return strings.Join([]string{
		operation,
		fmt.Sprintf("质量 %s", snapshot.Quality),
		fmt.Sprintf("背景 %s", snapshot.Background),
		fmt.Sprintf("格式 %s", snapshot.OutputFormat),
		fmt.Sprintf("审核 %s", snapshot.Moderation),
		fmt.Sprintf("尺寸 %s", snapshot.Size),
		fmt.Sprintf("输出 %d 张", snapshot.OutputCount),
	}, ", ")
}
