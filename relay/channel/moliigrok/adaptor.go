package moliigrok

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const validatedImageCountContextKey = "molii_grok_validated_image_count"

var allowedAspectRatios = map[string]struct{}{
	"1:1": {}, "16:9": {}, "9:16": {}, "4:3": {}, "3:4": {}, "3:2": {}, "2:3": {},
	"2:1": {}, "1:2": {}, "19.5:9": {}, "9:19.5": {}, "20:9": {}, "9:20": {}, "auto": {},
}

var allowedImageModels = map[string]struct{}{
	"grok-imagine-image":         {},
	"grok-imagine-image-quality": {},
}

// Adaptor is independent from the official xAI channel. The embedded OpenAI
// adaptor only supplies protocol methods that this image-only provider does
// not use; provider-specific image conversion is implemented in this package.
type Adaptor struct {
	openai.Adaptor
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
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		return nil, errors.New("image edits are not supported by Molii Grok Imagine API")
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
	aspectRatio := strings.TrimSpace(raw.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	if _, ok := allowedAspectRatios[aspectRatio]; !ok {
		return nil, errors.New("aspect_ratio is not supported")
	}

	c.Set(validatedImageCountContextKey, n)
	return imageRequestPayload{
		Model:       upstreamModel,
		Prompt:      prompt,
		AspectRatio: aspectRatio,
		Resolution:  resolution,
		N:           n,
	}, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	}
	defer service.CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	var upstream imageResponsePayload
	if err := common.Unmarshal(body, &upstream); err != nil {
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API request failed"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	actualCount := len(upstream.Data)
	validatedCount := c.GetInt(validatedImageCountContextKey)
	if validatedCount <= 0 {
		validatedCount = 1
	}
	if actualCount < 1 || actualCount > 4 || actualCount > validatedCount {
		return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API returned an invalid image result"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}

	data := make([]dto.ImageData, 0, actualCount)
	for _, item := range upstream.Data {
		if strings.TrimSpace(item.URL) == "" {
			return nil, types.NewOpenAIError(errors.New("Molii Grok Imagine API returned an invalid image result"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		data = append(data, dto.ImageData{Url: item.URL})
	}
	if info != nil && info.PriceData.UsePrice {
		info.PriceData.AddOtherRatio("n", float64(actualCount))
	}
	c.JSON(http.StatusOK, dto.ImageResponse{Data: data})
	return &dto.Usage{}, nil
}

func (a *Adaptor) SanitizeImageRequestError(err error) *types.NewAPIError {
	return types.NewOpenAIError(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
}

func (a *Adaptor) SanitizeImageError(statusCode int, _ []byte) *types.NewAPIError {
	if statusCode < http.StatusBadRequest || statusCode > http.StatusNetworkAuthenticationRequired {
		statusCode = http.StatusBadGateway
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
