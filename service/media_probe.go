package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/abema/go-mp4"
)

const (
	mediaProbeMaxBytes = 32 << 20
	mediaProbeTimeout  = 10 * time.Second
)

// MediaSource carries one user-controlled video reference. DataURL takes
// precedence over URL; Data is useful to callers that already own the bytes.
type MediaSource struct {
	URL      string
	DataURL  string
	Data     []byte
	MIMEType string
}

type MediaProbeResult struct {
	DurationSeconds float64
	Width           int
	Height          int
	ResolutionTier  string
	MIMEType        string
}

var (
	mediaProbeHTTPClient  = GetSSRFProtectedHTTPClient
	mediaProbeValidateURL = ValidateSSRFProtectedFetchURL
)

// ProbeUserVideo reads a bounded MP4 source and returns the normalized billing
// metadata. It never uses an unrestricted HTTP client for user-provided URLs.
func ProbeUserVideo(ctx context.Context, source MediaSource) (*MediaProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, mediaProbeTimeout)
	defer cancel()

	data, mimeType, err := readMediaSource(ctx, source)
	if err != nil {
		return nil, err
	}
	if !isMP4MIME(mimeType) {
		return nil, errors.New("video must be MP4")
	}
	return parseMP4Metadata(data, mimeType)
}

func readMediaSource(ctx context.Context, source MediaSource) ([]byte, string, error) {
	if strings.TrimSpace(source.DataURL) != "" {
		return decodeMP4DataURL(source.DataURL)
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.URL)), "data:") {
		return decodeMP4DataURL(source.URL)
	}
	if len(source.Data) > 0 {
		if len(source.Data) > mediaProbeMaxBytes {
			return nil, "", errors.New("video exceeds maximum probe size")
		}
		return bytes.Clone(source.Data), normalizeMediaMIME(source.MIMEType), nil
	}
	return fetchRemoteMP4(ctx, source.URL)
}

func decodeMP4DataURL(raw string) ([]byte, string, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ",", 2)
	if len(parts) != 2 || !strings.HasPrefix(strings.ToLower(parts[0]), "data:") || !strings.Contains(strings.ToLower(parts[0]), ";base64") {
		return nil, "", errors.New("video data URL must be base64 encoded")
	}
	mimeType := normalizeMediaMIME(strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64"))
	if !isMP4MIME(mimeType) {
		return nil, "", errors.New("video data URL must be MP4")
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", errors.New("video data URL is invalid")
	}
	if len(decoded) == 0 || len(decoded) > mediaProbeMaxBytes {
		return nil, "", errors.New("video exceeds maximum probe size")
	}
	return decoded, mimeType, nil
}

func fetchRemoteMP4(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil, "", errors.New("video URL is invalid")
	}
	if err := mediaProbeValidateURL(parsed.String()); err != nil {
		return nil, "", fmt.Errorf("video URL is blocked: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", errors.New("video URL is invalid")
	}
	client := mediaProbeHTTPClient()
	if client == nil {
		return nil, "", errors.New("video fetch client is unavailable")
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", errors.New("video fetch failed")
	}
	mimeType := normalizeMediaMIME(response.Header.Get("Content-Type"))
	if !isMP4MIME(mimeType) {
		return nil, "", errors.New("video response must be MP4")
	}
	if response.ContentLength > mediaProbeMaxBytes {
		return nil, "", errors.New("video exceeds maximum probe size")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, mediaProbeMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > mediaProbeMaxBytes {
		return nil, "", errors.New("video exceeds maximum probe size")
	}
	if ext := strings.ToLower(path.Ext(parsed.Path)); ext != "" && ext != ".mp4" {
		return nil, "", errors.New("video URL must use an MP4 extension")
	}
	return data, mimeType, nil
}

func parseMP4Metadata(data []byte, mimeType string) (*MediaProbeResult, error) {
	reader := bytes.NewReader(data)
	boxes, err := mp4.ExtractBoxesWithPayload(reader, nil, []mp4.BoxPath{
		{mp4.BoxTypeFtyp()},
		{mp4.BoxTypeMoov(), mp4.BoxTypeMvhd()},
		{mp4.BoxTypeMoov(), mp4.BoxTypeTrak(), mp4.BoxTypeTkhd()},
	})
	if err != nil {
		return nil, errors.New("video is not a valid MP4")
	}
	var mvhd *mp4.Mvhd
	width, height := 0, 0
	for _, box := range boxes {
		switch payload := box.Payload.(type) {
		case *mp4.Mvhd:
			mvhd = payload
		case *mp4.Tkhd:
			if candidateWidth, candidateHeight := int(payload.GetWidthInt()), int(payload.GetHeightInt()); candidateWidth > 0 && candidateHeight > 0 {
				width, height = candidateWidth, candidateHeight
			}
		}
	}
	if mvhd == nil || mvhd.Timescale == 0 || mvhd.GetDuration() == 0 || width == 0 || height == 0 {
		return nil, errors.New("video MP4 metadata is incomplete")
	}
	duration := float64(mvhd.GetDuration()) / float64(mvhd.Timescale)
	if duration <= 0 {
		return nil, errors.New("video duration is invalid")
	}
	tier := "720p"
	if height <= 480 {
		tier = "480p"
	}
	return &MediaProbeResult{DurationSeconds: duration, Width: width, Height: height, ResolutionTier: tier, MIMEType: mimeType}, nil
}

func normalizeMediaMIME(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
	return value
}

func isMP4MIME(value string) bool {
	return value == "video/mp4" || value == "application/mp4"
}
