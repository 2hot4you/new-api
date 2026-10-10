package service

import (
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingmoney"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	grokVideoBillingVersion     = 2
	grokVideoBillingVersionV1   = 1
	grokVideoModelLegacy        = "grok-imagine-video"
	grokVideoModel15            = "grok-imagine-video-1.5"
	grokVideoEditOperation      = "video_edit"
	grokVideoExtensionOperation = "video_extension"
	referenceToVideoOperation   = "reference_to_video"
	imageToVideoOperation       = "image_to_video"
	textToVideoOperation        = "text_to_video"
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
	if snapshot == nil || (snapshot.Version != grokVideoBillingVersionV1 && snapshot.Version != grokVideoBillingVersion) || !isGrokVideoModel(grokVideoBilledModel(snapshot)) {
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
	case grokVideoExtensionOperation:
		return snapshot.InputType == "video" && snapshot.VideoInputBilledSeconds >= 2 && snapshot.VideoInputBilledSeconds <= 15
	case referenceToVideoOperation:
		return snapshot.InputType == "image" && snapshot.InputImageCount >= 1 && snapshot.InputImageCount <= 7
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
	if (snapshot.Operation == grokVideoEditOperation || snapshot.Operation == grokVideoExtensionOperation) && snapshot.ResolutionSource != relaycommon.GrokVideoResolutionSourceInputProbeV1 {
		return false
	}
	if math.Abs(snapshot.Subtotal-expectedSubtotal) > 1e-9 {
		return false
	}
	if snapshot.Version >= grokVideoBillingVersion {
		moneyContext, err := billingmoney.NewContext(snapshot.SourceCurrency, snapshot.CNYPerUSD)
		if err != nil {
			return false
		}
		amounts, err := moneyContext.Normalize(snapshot.Subtotal)
		if err != nil || math.Abs(snapshot.CostUSD-amounts.CostUSD) > 1e-9 {
			return false
		}
	}
	return true
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
	bc.GrokVideoBilling = snapshot.Clone()
	return true
}

// PrepareGrokVideoBillingSnapshot freezes the complete request and price
// contract before wallet/subscription precharge begins.
func PrepareGrokVideoBillingSnapshot(c *gin.Context, info *relaycommon.RelayInfo, preConsumedQuota int) bool {
	if info == nil {
		return false
	}
	snapshot := BuildGrokVideoBillingSnapshot(c, info, preConsumedQuota)
	if snapshot == nil {
		return false
	}
	info.GrokVideoBilling = snapshot.Clone()
	return true
}

// BuildGrokVideoBillingSnapshot captures request parameters and direct unit
// prices at submission time. It intentionally records counts and media types,
// never media references or upstream identifiers.
func BuildGrokVideoBillingSnapshot(c *gin.Context, info *relaycommon.RelayInfo, _ int) *model.GrokVideoBillingSnapshot {
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
	} else if action == grokVideoExtensionOperation {
		operation = grokVideoExtensionOperation
		inputType = "video"
	} else if action == referenceToVideoOperation {
		operation = referenceToVideoOperation
		inputType = "image"
		inputImageCount = len(req.ReferenceImages)
	} else if strings.TrimSpace(req.Image) != "" || len(req.Images) > 0 {
		operation = imageToVideoOperation
		inputType = "image"
		// The adaptor sends only the first image even when Images contains more.
		inputImageCount = 1
	}

	estimatedDuration := float64(info.EstimatedVideoSeconds)
	if operation == grokVideoEditOperation {
		estimatedDuration = info.InputVideoDurationSeconds
		if estimatedDuration <= 0 || strings.TrimSpace(info.InputVideoResolutionTier) == "" || info.InputVideoResolutionSource != relaycommon.GrokVideoResolutionSourceInputProbeV1 {
			return nil
		}
	} else if operation == grokVideoExtensionOperation {
		if estimatedDuration <= 0 || info.InputVideoDurationSeconds < 2 || info.InputVideoDurationSeconds > 15 ||
			strings.TrimSpace(info.InputVideoResolutionTier) == "" || info.InputVideoResolutionSource != relaycommon.GrokVideoResolutionSourceInputProbeV1 {
			return nil
		}
	}
	var prices ratio_setting.MoliiGrokPriceSetting
	var moneyContext billingmoney.Context
	if selected := info.PricingSelection; selected != nil {
		anchor, found := selected.Model(billedModel)
		if !found {
			return nil
		}
		prices, moneyContext = selected.GrokPrices(), anchor.Money
	} else {
		// Historical direct callers without a selection keep the prior fallback.
		prices = ratio_setting.GetMoliiGrokPriceSettingCopy()
		moneyContext, _, err = model.ResolveBillingMoneyContext(model.DB, billedModel)
		if err != nil {
			return nil
		}
	}
	outputPrice, imageInputPrice, videoInputPrice, ok := prices.VideoPrices(billedModel, info.EstimatedVideoResolution)
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
		SourceCurrency:           string(moneyContext.SourceCurrency),
		CNYPerUSD:                moneyContext.CNYPerUSD,
		GroupRatio:               info.PriceData.GroupRatioInfo.GroupRatio,
	}
	if operation == grokVideoEditOperation {
		snapshot.RequestedDurationSeconds = estimatedDuration
		snapshot.RequestedResolution = strings.ToLower(strings.TrimSpace(info.InputVideoResolutionTier))
		snapshot.EstimatedResolution = snapshot.RequestedResolution
		snapshot.ResolutionSource = info.InputVideoResolutionSource
		snapshot.VideoInputBilledSeconds = estimatedDuration
	} else if operation == grokVideoExtensionOperation {
		snapshot.RequestedResolution = strings.ToLower(strings.TrimSpace(info.InputVideoResolutionTier))
		snapshot.EstimatedResolution = snapshot.RequestedResolution
		snapshot.ResolutionSource = info.InputVideoResolutionSource
		snapshot.VideoInputBilledSeconds = info.InputVideoDurationSeconds
	}
	calculateGrokVideoBillingCosts(snapshot, estimatedDuration)
	amounts, err := moneyContext.Normalize(snapshot.Subtotal)
	if err != nil {
		return nil
	}
	snapshot.CostUSD = amounts.CostUSD
	snapshot.FinalSourceCost = snapshot.Subtotal * snapshot.GroupRatio
	snapshot.FinalCostUSD = snapshot.CostUSD * snapshot.GroupRatio
	snapshot.FinalCost = snapshot.FinalCostUSD
	quota, _ := common.QuotaRoundChecked(snapshot.FinalCostUSD * common.QuotaPerUnit)
	info.PriceData.Quota = quota
	info.PriceData.QuotaToPreConsume = quota
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
	if (snapshot.Version != grokVideoBillingVersionV1 && snapshot.Version != grokVideoBillingVersion) || !isGrokVideoModel(grokVideoBilledModel(&snapshot)) {
		return nil, "生成视频"
	}
	snapshot.GroupRatio = task.PrivateData.BillingContext.GroupRatio
	snapshot.FinalCost = float64(task.Quota) / common.QuotaPerUnit
	if snapshot.Version >= grokVideoBillingVersion {
		snapshot.FinalSourceCost = snapshot.Subtotal * snapshot.GroupRatio
		snapshot.FinalCostUSD = snapshot.FinalCost
	}

	duration := snapshot.ActualDurationSeconds
	if duration <= 0 {
		duration = snapshot.EstimatedDurationSeconds
	}
	resolution := snapshot.ActualResolution
	if resolution == "" {
		resolution = snapshot.EstimatedResolution
	}

	symbol := "¥"
	finalDisplayCost := snapshot.FinalCost
	if snapshot.Version >= grokVideoBillingVersion {
		if snapshot.SourceCurrency != string(billingmoney.CurrencyCNY) {
			symbol = "$"
		}
		finalDisplayCost = snapshot.FinalSourceCost
	}
	operationName := "文生视频"
	formula := fmt.Sprintf("(%s%.6f × %.3g) × %.4f", symbol, snapshot.OutputUnitPrice, duration, snapshot.GroupRatio)
	switch snapshot.Operation {
	case imageToVideoOperation:
		operationName = "图生视频"
		formula = fmt.Sprintf("(%s%.6f × %.3g + %s%.6f × %d) × %.4f",
			symbol, snapshot.OutputUnitPrice, duration, symbol, snapshot.ImageInputUnitPrice, snapshot.InputImageCount, snapshot.GroupRatio)
	case grokVideoEditOperation:
		operationName = "视频编辑"
		formula = fmt.Sprintf("(%s%.6f × %.3g + %s%.6f × %.3g) × %.4f",
			symbol, snapshot.OutputUnitPrice, duration, symbol, snapshot.VideoInputUnitPrice, snapshot.VideoInputBilledSeconds, snapshot.GroupRatio)
	case grokVideoExtensionOperation:
		operationName = "视频延长"
		formula = fmt.Sprintf("(%s%.6f × %.3g + %s%.6f × %.3g) × %.4f",
			symbol, snapshot.OutputUnitPrice, duration, symbol, snapshot.VideoInputUnitPrice, snapshot.VideoInputBilledSeconds, snapshot.GroupRatio)
	case referenceToVideoOperation:
		operationName = "参考图生视频"
		formula = fmt.Sprintf("(%s%.6f × %.3g + %s%.6f × %d) × %.4f",
			symbol, snapshot.OutputUnitPrice, duration, symbol, snapshot.ImageInputUnitPrice, snapshot.InputImageCount, snapshot.GroupRatio)
	}
	content := fmt.Sprintf("Grok %s，模型 %s，实际 %s · %.3g 秒，计费 %s = %s%.6f",
		operationName, grokVideoRequestedModel(&snapshot), resolution, duration, formula, symbol, finalDisplayCost)
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
