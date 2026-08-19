package moliigrok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const validatedImageCountContextKey = "molii_grok_validated_image_count"

const (
	imageBillingOutputPriceContextKey = "molii_grok_image_output_price"
	imageBillingInputPriceContextKey  = "molii_grok_image_input_price"
	imageBillingInputCountContextKey  = "molii_grok_image_input_count"
	imageBillingBasePriceContextKey   = "molii_grok_image_base_price"
)

var allowedAspectRatios = map[string]struct{}{
	"1:1": {}, "16:9": {}, "9:16": {}, "4:3": {}, "3:4": {}, "3:2": {}, "2:3": {},
	"2:1": {}, "1:2": {}, "19.5:9": {}, "9:19.5": {}, "20:9": {}, "9:20": {}, "auto": {},
}

var allowedImageModels = map[string]struct{}{
	"grok-imagine-image":         {},
	"grok-imagine-image-quality": {},
	"grok-imagine-image-2.0":     {},
}

// Adaptor is independent from the official xAI channel. The embedded OpenAI
// adaptor only supplies protocol methods that this image-only provider does
// not use; provider-specific image conversion is implemented in this package.
type Adaptor struct {
	openai.Adaptor
	resolveUserFile func(context.Context, int, string, model.MoliiFileMediaType) (*model.MoliiFile, string, error)
}

func (a *Adaptor) userFileResolver() func(context.Context, int, string, model.MoliiFileMediaType) (*model.MoliiFile, string, error) {
	if a != nil && a.resolveUserFile != nil {
		return a.resolveUserFile
	}
	return service.ResolveUserFile
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, headers *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, headers)
	headers.Set("Authorization", "Bearer "+info.ApiKey)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	return nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info == nil {
		return nil, errors.New("request context is missing")
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, fmt.Errorf("read image request: %w", err)
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil, fmt.Errorf("read image request: %w", err)
	}
	var raw rawImageRequest
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid image request: %w", err)
	}
	modelName := strings.TrimSpace(raw.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(request.Model)
	}
	if _, ok := allowedImageModels[modelName]; !ok {
		return nil, errors.New("model is not supported by Molii Grok Imagine API")
	}
	upstreamModel := strings.TrimSpace(info.UpstreamModelName)
	if upstreamModel == "" {
		upstreamModel = modelName
	}
	if _, ok := allowedImageModels[upstreamModel]; !ok {
		return nil, errors.New("mapped model is not supported by Molii Grok Imagine API")
	}

	prompt := strings.TrimSpace(raw.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(request.Prompt)
	}
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if utf8.RuneCountInString(prompt) > 10000 {
		return nil, errors.New("prompt must not exceed 10000 characters")
	}

	n := 1
	if raw.N != nil {
		n = *raw.N
	}
	if n < 1 || n > 4 {
		return nil, errors.New("n must be an integer between 1 and 4")
	}
	resolution := strings.ToLower(strings.TrimSpace(raw.Resolution))
	if resolution == "" {
		resolution = "1k"
	}
	if resolution != "1k" && resolution != "2k" {
		return nil, errors.New("resolution must be one of 1k or 2k")
	}
	quality, err := normalizeImageQuality(modelName, raw.Quality)
	if err != nil {
		return nil, err
	}
	aspectRatio := strings.TrimSpace(raw.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	if _, ok := allowedAspectRatios[aspectRatio]; !ok {
		return nil, errors.New("aspect_ratio is not supported")
	}

	c.Set(validatedImageCountContextKey, n)
	payload := imageRequestPayload{
		Model:       upstreamModel,
		Prompt:      prompt,
		AspectRatio: aspectRatio,
		Resolution:  resolution,
		Quality:     quality,
		N:           n,
	}
	if info.RelayMode != relayconstant.RelayModeImagesEdits && (len(raw.Image) > 0 || len(raw.Images) > 0) {
		return nil, errors.New("image inputs require /v1/images/edits")
	}
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		media, err := a.normalizeImageMedia(c.Request.Context(), info.UserId, raw.Image, raw.Images)
		if err != nil {
			return nil, err
		}
		if len(media) == 1 {
			payload.Image = &media[0]
		} else {
			payload.Images = media
		}
	}
	return payload, nil
}

func (a *Adaptor) EstimateImageBilling(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (map[string]float64, error) {
	if info == nil {
		return nil, errors.New("request context is missing")
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	var raw rawImageRequest
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid image request: %w", err)
	}
	modelName := strings.TrimSpace(raw.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(request.Model)
	}
	billedModel := strings.TrimSpace(info.UpstreamModelName)
	if billedModel == "" {
		billedModel = modelName
	}
	resolution := strings.ToLower(strings.TrimSpace(raw.Resolution))
	if resolution == "" {
		resolution = "1k"
	}
	quality, err := normalizeImageQuality(billedModel, raw.Quality)
	if err != nil {
		return nil, err
	}
	aspectRatio := strings.TrimSpace(raw.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	outputPrice, inputPrice, ok := ratio_setting.GetMoliiGrokImagePricesForQuality(billedModel, resolution, quality)
	if !ok {
		return nil, errors.New("Molii Grok image pricing is not configured")
	}
	n := 1
	if raw.N != nil {
		n = *raw.N
	}
	if n < 1 || n > 4 {
		return nil, errors.New("n must be an integer between 1 and 4")
	}
	inputCount := 0
	operation := "generation"
	if info.RelayMode != relayconstant.RelayModeImagesEdits && (len(raw.Image) > 0 || len(raw.Images) > 0) {
		return nil, errors.New("image inputs require /v1/images/edits")
	}
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		operation = "edit"
		media, err := a.normalizeImageMedia(c.Request.Context(), info.UserId, raw.Image, raw.Images)
		if err != nil {
			return nil, err
		}
		inputCount = len(media)
	}
	basePrice, ok := ratio_setting.GetModelPrice(billedModel, false)
	if !ok {
		basePrice, ok = ratio_setting.GetDefaultModelPriceMap()[billedModel]
	}
	if !ok || basePrice <= 0 {
		return nil, errors.New("Molii Grok image pricing anchor is invalid")
	}
	c.Set(imageBillingOutputPriceContextKey, outputPrice)
	c.Set(imageBillingInputPriceContextKey, inputPrice)
	c.Set(imageBillingInputCountContextKey, inputCount)
	c.Set(imageBillingBasePriceContextKey, basePrice)
	cost := outputPrice*float64(n) + inputPrice*float64(inputCount)
	info.GrokImageBilling = &relaycommon.GrokImageBillingSnapshot{
		Version:              1,
		Model:                modelName,
		RequestedModel:       modelName,
		BilledModel:          billedModel,
		Operation:            operation,
		Resolution:           resolution,
		Quality:              quality,
		AspectRatio:          aspectRatio,
		RequestedOutputCount: n,
		OutputCount:          n,
		InputImageCount:      inputCount,
		OutputUnitPrice:      outputPrice,
		InputUnitPrice:       inputPrice,
		OutputCost:           outputPrice * float64(n),
		InputCost:            inputPrice * float64(inputCount),
		Subtotal:             cost,
	}
	return map[string]float64{"molii_grok_direct_cost": cost / basePrice}, nil
}

func normalizeImageQuality(modelName, requested string) (string, error) {
	quality := strings.ToLower(strings.TrimSpace(requested))
	if modelName != "grok-imagine-image-2.0" {
		if quality != "" {
			return "", errors.New("quality is only supported by grok-imagine-image-2.0")
		}
		return "", nil
	}
	if quality == "" {
		return "medium", nil
	}
	if quality != "low" && quality != "medium" {
		return "", errors.New("quality must be one of low or medium")
	}
	return quality, nil
}

func (a *Adaptor) normalizeImageMedia(ctx context.Context, userID int, imageRaw, imagesRaw []byte) ([]imageMediaInput, error) {
	rawItems := make([][]byte, 0, 3)
	if len(imageRaw) > 0 && string(imageRaw) != "null" {
		rawItems = append(rawItems, imageRaw)
	}
	if len(imagesRaw) > 0 && string(imagesRaw) != "null" {
		var items []json.RawMessage
		if err := common.Unmarshal(imagesRaw, &items); err != nil {
			return nil, errors.New("images must be an array")
		}
		for _, item := range items {
			rawItems = append(rawItems, item)
		}
	}
	if len(rawItems) < 1 || len(rawItems) > 3 {
		return nil, errors.New("image edits require between 1 and 3 input images")
	}
	media := make([]imageMediaInput, 0, len(rawItems))
	for _, raw := range rawItems {
		var direct string
		if err := common.Unmarshal(raw, &direct); err == nil && strings.TrimSpace(direct) != "" {
			if isFileIDReference(direct) {
				_, signedURL, err := a.userFileResolver()(ctx, userID, strings.TrimSpace(direct), model.MoliiFileMediaTypeImage)
				if err != nil {
					return nil, err
				}
				media = append(media, imageMediaInput{URL: signedURL, Type: "image_url"})
				continue
			}
			media = append(media, imageMediaInput{URL: strings.TrimSpace(direct), Type: "image_url"})
			continue
		}
		var item struct {
			URL    string `json:"url"`
			FileID string `json:"file_id"`
		}
		if err := common.Unmarshal(raw, &item); err != nil {
			return nil, errors.New("each input image must contain url")
		}
		if strings.TrimSpace(item.URL) != "" && strings.TrimSpace(item.FileID) != "" {
			return nil, errors.New("each input image must contain either url or file_id, not both")
		}
		if fileID := strings.TrimSpace(item.FileID); fileID != "" {
			_, signedURL, err := a.userFileResolver()(ctx, userID, fileID, model.MoliiFileMediaTypeImage)
			if err != nil {
				return nil, err
			}
			media = append(media, imageMediaInput{URL: signedURL, Type: "image_url"})
			continue
		}
		url := strings.TrimSpace(item.URL)
		if url == "" {
			return nil, errors.New("each input image must contain url")
		}
		media = append(media, imageMediaInput{URL: url, Type: "image_url"})
	}
	return media, nil
}

func rawImageRequestContainsFileID(raw rawImageRequest) bool {
	return rawMediaContainsFileID(raw.Image) || rawMediaContainsFileID(raw.Images)
}

func rawMediaContainsFileID(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var direct string
	if err := common.Unmarshal(raw, &direct); err == nil {
		return isFileIDReference(direct)
	}
	var items []json.RawMessage
	if err := common.Unmarshal(raw, &items); err == nil {
		for _, item := range items {
			if rawMediaContainsFileID(item) {
				return true
			}
		}
		return false
	}
	var object map[string]json.RawMessage
	if err := common.Unmarshal(raw, &object); err == nil {
		for key := range object {
			if strings.EqualFold(key, "file_id") {
				return true
			}
		}
	}
	return false
}

func isFileIDReference(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "file_")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	requestContext := context.Background()
	if c != nil && c.Request != nil {
		requestContext = c.Request.Context()
	}
	if resp == nil || resp.Body == nil {
		logGrokImageValidationFailureFields(requestContext, info, "parse_upstream_response", "missing_response_body", "")
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logGrokImageValidationFailureFields(requestContext, info, "parse_upstream_response", "response_body_read_failed", "")
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	var upstream imageResponsePayload
	if err := common.Unmarshal(body, &upstream); err != nil {
		logGrokImageValidationFailureFields(requestContext, info, "parse_upstream_response", "invalid_response_json", "")
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	actualCount := len(upstream.Data)
	validatedCount := c.GetInt(validatedImageCountContextKey)
	if validatedCount <= 0 {
		validatedCount = 1
	}
	if actualCount < 1 || actualCount > 4 || actualCount > validatedCount {
		logGrokImageValidationFailureFields(requestContext, info, "parse_upstream_response", "invalid_image_count", "")
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API returned an invalid image result"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	data := make([]dto.ImageData, 0, actualCount)
	for _, item := range upstream.Data {
		resultURL := strings.TrimSpace(item.URL)
		if resultURL == "" {
			logGrokImageValidationFailureFields(requestContext, info, "validate_source", "missing_source_url", "")
			return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
		if !service.IsTrustedMoliiGrokImageURL(resultURL) {
			logGrokImageValidationFailureFields(requestContext, info, "validate_source", "untrusted_result_url", "")
			return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
		data = append(data, dto.ImageData{
			Url:           resultURL,
			MimeType:      item.MimeType,
			RevisedPrompt: item.RevisedPrompt,
		})
	}
	if info == nil {
		logGrokImageValidationFailureFields(requestContext, nil, "parse_upstream_response", "missing_relay_info", "")
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	if info != nil && info.PriceData.UsePrice {
		outputPrice := c.GetFloat64(imageBillingOutputPriceContextKey)
		inputPrice := c.GetFloat64(imageBillingInputPriceContextKey)
		inputCount := c.GetInt(imageBillingInputCountContextKey)
		basePrice := c.GetFloat64(imageBillingBasePriceContextKey)
		if outputPrice >= 0 && inputPrice >= 0 && basePrice > 0 {
			actualCost := outputPrice*float64(actualCount) + inputPrice*float64(inputCount)
			info.PriceData.AddOtherRatio("molii_grok_direct_cost", actualCost/basePrice)
		}
	}
	if info != nil && info.GrokImageBilling != nil {
		snapshot := info.GrokImageBilling
		snapshot.OutputCount = actualCount
		snapshot.OutputCost = snapshot.OutputUnitPrice * float64(actualCount)
		snapshot.InputCost = snapshot.InputUnitPrice * float64(snapshot.InputImageCount)
		snapshot.Subtotal = snapshot.OutputCost + snapshot.InputCost
	}
	c.JSON(http.StatusOK, dto.ImageResponse{Data: data})
	return &dto.Usage{}, nil
}

func logGrokImageValidationFailureFields(ctx context.Context, info *relaycommon.RelayInfo, stage, errorCategory, sourceHost string) {
	logGrokImageValidationFailureFieldsWithRemoteStatus(ctx, info, stage, errorCategory, sourceHost, 0)
}

func logGrokImageValidationFailureFieldsWithRemoteStatus(ctx context.Context, info *relaycommon.RelayInfo, stage, errorCategory, sourceHost string, remoteStatus int) {
	requestID := ""
	modelName := ""
	userID := 0
	channelID := 0
	if info != nil {
		requestID = strings.TrimSpace(info.RequestId)
		modelName = strings.TrimSpace(info.OriginModelName)
		userID = info.UserId
		channelID = info.GetChannelID()
	}
	logger.LogError(ctx, fmt.Sprintf(
		"event=grok_image_result_validation_failed request_id=%q model=%q user_id=%d channel_id=%d stage=%q error_category=%q remote_status=%d source_host=%q",
		requestID,
		modelName,
		userID,
		channelID,
		stage,
		errorCategory,
		remoteStatus,
		sourceHost,
	))
}

func (a *Adaptor) SanitizeImageRequestError(err error) *types.NewAPIError {
	switch {
	case errors.Is(err, model.ErrMoliiFileNotFound):
		return types.NewOpenAIError(errors.New("file not found"), types.ErrorCode("file_not_found"), http.StatusNotFound, types.ErrOptionWithSkipRetry())
	case errors.Is(err, model.ErrMoliiFileExpired):
		return types.NewOpenAIError(errors.New("file has expired"), types.ErrorCode("file_expired"), http.StatusGone, types.ErrOptionWithSkipRetry())
	case errors.Is(err, service.ErrMoliiFileTypeMismatch):
		return types.NewOpenAIError(errors.New("file is not an image"), types.ErrorCode("file_type_mismatch"), http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	case errors.Is(err, service.ErrMoliiFileServiceUnavailable):
		return types.NewOpenAIError(errors.New("file service is temporarily unavailable"), types.ErrorCode("file_service_unavailable"), http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}
	return types.NewOpenAIError(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}

func (a *Adaptor) SanitizeImageError(statusCode int, responseBody []byte) *types.NewAPIError {
	if statusCode < http.StatusBadRequest || statusCode > http.StatusNetworkAuthenticationRequired {
		statusCode = http.StatusBadGateway
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := common.Unmarshal(responseBody, &envelope); err == nil &&
		strings.EqualFold(strings.TrimSpace(envelope.Error.Code), "imagine:content-moderated") {
		return types.NewOpenAIError(
			errors.New("Image request rejected by content safety policy"),
			types.ErrorCodeContentPolicyViolation,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	return types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponse, statusCode)
}

func (a *Adaptor) SanitizeImageTransportError(_ error) *types.NewAPIError {
	return types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeDoRequestFailed, http.StatusBadGateway)
}

func (a *Adaptor) AllowImageRequestBodyLog() bool { return false }

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
