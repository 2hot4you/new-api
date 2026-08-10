package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	grokVideoBillingVersion = 1
	grokVideoModelLegacy    = "grok-imagine-video"
	grokVideoModel15        = "grok-imagine-video-1.5"
	grokVideoEditOperation  = "video_edit"
	imageToVideoOperation   = "image_to_video"
	textToVideoOperation    = "text_to_video"
)

func isMoliiGrokTask(task *model.Task) bool {
	return task != nil && task.Platform == constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeMoliiGrokAIGC))
}

func taskUsesFinalUsageLog(task *model.Task) bool {
	if isStarAITask(task) {
		return true
	}
	return isMoliiGrokTask(task) && task.PrivateData.BillingContext != nil &&
		task.PrivateData.BillingContext.FinalUsageLogOnly
}

func isMoliiGrokFinalUsageTask(task *model.Task) bool {
	return isMoliiGrokTask(task) && task.PrivateData.BillingContext != nil &&
		task.PrivateData.BillingContext.FinalUsageLogOnly
}

func isGrokVideoModel(modelName string) bool {
	return modelName == grokVideoModelLegacy || modelName == grokVideoModel15
}

func validGrokBillingNumber(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validSubmittedGrokVideoBilling(snapshot *model.GrokVideoBillingSnapshot) bool {
	if snapshot == nil || snapshot.Version != grokVideoBillingVersion || !isGrokVideoModel(grokVideoBilledModel(snapshot)) {
		return false
	}
	if snapshot.EstimatedDurationSeconds <= 0 || strings.TrimSpace(snapshot.EstimatedResolution) == "" ||
		!validGrokBillingNumber(snapshot.OutputUnitPrice) ||
		!validGrokBillingNumber(snapshot.ImageInputUnitPrice) ||
		!validGrokBillingNumber(snapshot.VideoInputUnitPrice) ||
		!validGrokBillingNumber(snapshot.GroupRatio) {
		return false
	}
	switch snapshot.Operation {
	case textToVideoOperation:
		return snapshot.InputType == "text"
	case imageToVideoOperation:
		return snapshot.InputType == "image" && snapshot.InputImageCount == 1
	case grokVideoEditOperation:
		return snapshot.InputType == "video" && snapshot.VideoInputBilledSeconds > 0
	default:
		return false
	}
}

func validFinalizedGrokVideoBilling(snapshot *model.GrokVideoBillingSnapshot) bool {
	if !validSubmittedGrokVideoBilling(snapshot) || snapshot.ActualDurationSeconds <= 0 ||
		strings.TrimSpace(snapshot.ActualResolution) == "" ||
		!validGrokBillingNumber(snapshot.OutputCost) ||
		!validGrokBillingNumber(snapshot.ImageInputCost) ||
		!validGrokBillingNumber(snapshot.VideoInputCost) ||
		!validGrokBillingNumber(snapshot.Subtotal) {
		return false
	}
	expectedSubtotal := snapshot.OutputCost + snapshot.ImageInputCost + snapshot.VideoInputCost
	return math.Abs(snapshot.Subtotal-expectedSubtotal) <= 1e-9
}

// ConfigureGrokVideoFinalUsage enables the rollout only when a complete V1
// submission snapshot exists. A nil/invalid snapshot deterministically leaves
// the task on the legacy submit-log path.
func ConfigureGrokVideoFinalUsage(bc *model.TaskBillingContext, snapshot *model.GrokVideoBillingSnapshot, requestPath string) bool {
	if bc == nil || !validSubmittedGrokVideoBilling(snapshot) {
		return false
	}
	bc.FinalUsageLogOnly = true
	bc.RequestPath = requestPath
	bc.GrokVideoBilling = snapshot
	return true
}

// BuildGrokVideoBillingSnapshot captures request parameters and direct unit
// prices at submission time. It intentionally records counts and media types,
// never media references or upstream identifiers.
func BuildGrokVideoBillingSnapshot(c *gin.Context, info *relaycommon.RelayInfo, preConsumedQuota int) *model.GrokVideoBillingSnapshot {
	if c == nil || info == nil {
		return nil
	}
	requestedModel := strings.TrimSpace(info.OriginModelName)
	billedModel := ""
	if info.ChannelMeta != nil {
		billedModel = strings.TrimSpace(info.UpstreamModelName)
	}
	if billedModel == "" {
		billedModel = requestedModel
	}
	if !isGrokVideoModel(billedModel) {
		return nil
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	operation := textToVideoOperation
	inputType := "text"
	inputImageCount := 0
	action := ""
	if info.TaskRelayInfo != nil {
		action = info.Action
	}
	if action == grokVideoEditOperation {
		operation = grokVideoEditOperation
		inputType = "video"
	} else if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
		operation = imageToVideoOperation
		inputType = "image"
		// The adaptor sends only the first image even when Images contains more.
		inputImageCount = 1
	}

	estimatedDuration := float64(info.EstimatedVideoSeconds)
	if operation == grokVideoEditOperation {
		// The edit precharge uses the provider's 8.7-second estimate.
		estimatedDuration = 8.7
	}
	outputPrice, imageInputPrice, videoInputPrice, ok := ratio_setting.GetMoliiGrokVideoPrices(
		billedModel, info.EstimatedVideoResolution,
	)
	if !ok {
		return nil
	}

	snapshot := &model.GrokVideoBillingSnapshot{
		Version:                  grokVideoBillingVersion,
		Model:                    requestedModel,
		RequestedModel:           requestedModel,
		BilledModel:              billedModel,
		Operation:                operation,
		InputType:                inputType,
		RequestedDurationSeconds: float64(req.Duration),
		EstimatedDurationSeconds: estimatedDuration,
		RequestedResolution:      strings.ToLower(strings.TrimSpace(req.Resolution)),
		EstimatedResolution:      strings.ToLower(strings.TrimSpace(info.EstimatedVideoResolution)),
		AspectRatio:              strings.TrimSpace(req.AspectRatio),
		InputImageCount:          inputImageCount,
		OutputUnitPrice:          outputPrice,
		ImageInputUnitPrice:      imageInputPrice,
		VideoInputUnitPrice:      videoInputPrice,
		GroupRatio:               info.PriceData.GroupRatioInfo.GroupRatio,
		FinalCost:                float64(preConsumedQuota) / common.QuotaPerUnit,
	}
	if operation == grokVideoEditOperation {
		snapshot.VideoInputBilledSeconds = estimatedDuration
	}
	calculateGrokVideoBillingCosts(snapshot, estimatedDuration)
	return snapshot
}

func calculateGrokVideoBillingCosts(snapshot *model.GrokVideoBillingSnapshot, outputSeconds float64) {
	if snapshot == nil {
		return
	}
	snapshot.OutputCost = snapshot.OutputUnitPrice * outputSeconds
	snapshot.ImageInputCost = snapshot.ImageInputUnitPrice * float64(snapshot.InputImageCount)
	snapshot.VideoInputCost = snapshot.VideoInputUnitPrice * snapshot.VideoInputBilledSeconds
	snapshot.Subtotal = snapshot.OutputCost + snapshot.ImageInputCost + snapshot.VideoInputCost
}

// finalGrokVideoBilling returns a detached public copy whose total is derived
// from the settled task quota, the authoritative ledger amount.
func finalGrokVideoBilling(task *model.Task) (*model.GrokVideoBillingSnapshot, string) {
	if task == nil || task.PrivateData.BillingContext == nil || task.PrivateData.BillingContext.GrokVideoBilling == nil {
		return nil, "生成视频"
	}
	snapshot := *task.PrivateData.BillingContext.GrokVideoBilling
	if snapshot.Version != grokVideoBillingVersion || !isGrokVideoModel(grokVideoBilledModel(&snapshot)) {
		return nil, "生成视频"
	}
	snapshot.GroupRatio = task.PrivateData.BillingContext.GroupRatio
	snapshot.FinalCost = float64(task.Quota) / common.QuotaPerUnit

	duration := snapshot.ActualDurationSeconds
	if duration <= 0 {
		duration = snapshot.EstimatedDurationSeconds
	}
	resolution := snapshot.ActualResolution
	if resolution == "" {
		resolution = snapshot.EstimatedResolution
	}

	operationName := "文生视频"
	formula := fmt.Sprintf("(¥%.6f × %.3g) × %.4f", snapshot.OutputUnitPrice, duration, snapshot.GroupRatio)
	switch snapshot.Operation {
	case imageToVideoOperation:
		operationName = "图生视频"
		formula = fmt.Sprintf("(¥%.6f × %.3g + ¥%.6f × %d) × %.4f",
			snapshot.OutputUnitPrice, duration, snapshot.ImageInputUnitPrice, snapshot.InputImageCount, snapshot.GroupRatio)
	case grokVideoEditOperation:
		operationName = "视频编辑"
		formula = fmt.Sprintf("(¥%.6f × %.3g + ¥%.6f × %.3g) × %.4f",
			snapshot.OutputUnitPrice, duration, snapshot.VideoInputUnitPrice, snapshot.VideoInputBilledSeconds, snapshot.GroupRatio)
	}
	content := fmt.Sprintf("Grok %s，模型 %s，实际 %s · %.3g 秒，计费 %s = ¥%.6f",
		operationName, grokVideoRequestedModel(&snapshot), resolution, duration, formula, snapshot.FinalCost)
	return &snapshot, content
}

func grokVideoRequestedModel(snapshot *model.GrokVideoBillingSnapshot) string {
	if snapshot != nil && strings.TrimSpace(snapshot.RequestedModel) != "" {
		return snapshot.RequestedModel
	}
	if snapshot == nil {
		return ""
	}
	return snapshot.Model
}

func grokVideoBilledModel(snapshot *model.GrokVideoBillingSnapshot) string {
	if snapshot != nil && strings.TrimSpace(snapshot.BilledModel) != "" {
		return snapshot.BilledModel
	}
	return grokVideoRequestedModel(snapshot)
}
