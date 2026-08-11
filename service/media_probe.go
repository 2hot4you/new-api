package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
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

type UserVideoProber interface {
	ProbeUserVideo(ctx context.Context, source MediaSource) (*MediaProbeResult, error)
}

type UserVideoProbeFunc func(context.Context, MediaSource) (*MediaProbeResult, error)

func (f UserVideoProbeFunc) ProbeUserVideo(ctx context.Context, source MediaSource) (*MediaProbeResult, error) {
	return f(ctx, source)
}

type mediaProbe struct {
	client *http.Client
}

var defaultMediaProbe UserVideoProber = newMediaProbe(newProtectedFetchHTTPClientWithoutProxy(nil, nil, nil))

func newMediaProbe(client *http.Client) *mediaProbe {
	return &mediaProbe{client: client}
}

func ProbeUserVideo(ctx context.Context, source MediaSource) (*MediaProbeResult, error) {
	return defaultMediaProbe.ProbeUserVideo(ctx, source)
}

func (p *mediaProbe) ProbeUserVideo(ctx context.Context, source MediaSource) (*MediaProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, mediaProbeTimeout)
	defer cancel()

	data, mimeType, err := p.readMediaSource(ctx, source)
	if err != nil {
		return nil, err
	}
	if !isMP4MIME(mimeType) {
		return nil, errors.New("video must be MP4")
	}
	return parseMP4Metadata(data, mimeType)
}

func (p *mediaProbe) readMediaSource(ctx context.Context, source MediaSource) ([]byte, string, error) {
	if strings.TrimSpace(source.DataURL) != "" {
		return decodeMP4DataURL(ctx, source.DataURL)
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.URL)), "data:") {
		return decodeMP4DataURL(ctx, source.URL)
	}
	if len(source.Data) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if len(source.Data) > mediaProbeMaxBytes {
			return nil, "", errors.New("video exceeds maximum probe size")
		}
		return bytes.Clone(source.Data), normalizeMediaMIME(source.MIMEType), nil
	}
	return p.fetchRemoteMP4(ctx, source.URL)
}

func decodeMP4DataURL(ctx context.Context, raw string) ([]byte, string, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ",", 2)
	if len(parts) != 2 || !strings.HasPrefix(strings.ToLower(parts[0]), "data:") || !strings.Contains(strings.ToLower(parts[0]), ";base64") {
		return nil, "", errors.New("video data URL must be base64 encoded")
	}
	header := strings.ToLower(parts[0])
	mimeType := normalizeMediaMIME(strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64"))
	if !isMP4MIME(mimeType) {
		return nil, "", errors.New("video data URL must be MP4")
	}
	encoded := strings.TrimSpace(parts[1])
	decodedLen := base64.StdEncoding.DecodedLen(len(encoded))
	if strings.HasSuffix(encoded, "==") {
		decodedLen -= 2
	} else if strings.HasSuffix(encoded, "=") {
		decodedLen--
	}
	if len(encoded) == 0 || len(encoded) > base64.StdEncoding.EncodedLen(mediaProbeMaxBytes) || decodedLen > mediaProbeMaxBytes {
		return nil, "", errors.New("video exceeds maximum probe size")
	}
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded))
	data, err := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, reader: decoder}, mediaProbeMaxBytes+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", ctxErr
		}
		return nil, "", errors.New("video data URL is invalid")
	}
	if len(data) == 0 || len(data) > mediaProbeMaxBytes {
		return nil, "", errors.New("video exceeds maximum probe size")
	}
	return data, mimeType, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if err == nil {
		if ctxErr := r.ctx.Err(); ctxErr != nil {
			return n, ctxErr
		}
	}
	return n, err
}

func (p *mediaProbe) fetchRemoteMP4(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil, "", errors.New("video URL is invalid")
	}
	if strings.ToLower(path.Ext(parsed.Path)) != ".mp4" {
		return nil, "", errors.New("video URL must use an MP4 extension")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", errors.New("video URL is invalid")
	}
	if p == nil || p.client == nil {
		return nil, "", errors.New("video fetch client is unavailable")
	}
	response, err := p.client.Do(request)
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
	data, err := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, reader: response.Body}, mediaProbeMaxBytes+1))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, "", ctxErr
		}
		return nil, "", err
	}
	if len(data) == 0 || len(data) > mediaProbeMaxBytes {
		return nil, "", errors.New("video exceeds maximum probe size")
	}
	return data, mimeType, nil
}

type mp4VideoTrack struct {
	durationSeconds float64
	width           int
	height          int
}

func parseMP4Metadata(data []byte, mimeType string) (*MediaProbeResult, error) {
	reader := bytes.NewReader(data)
	ftyp, err := mp4.ExtractBoxes(reader, nil, []mp4.BoxPath{{mp4.BoxTypeFtyp()}})
	if err != nil || len(ftyp) != 1 {
		return nil, errors.New("video is not a valid MP4")
	}
	tracks, err := mp4.ExtractBoxes(reader, nil, []mp4.BoxPath{{mp4.BoxTypeMoov(), mp4.BoxTypeTrak()}})
	if err != nil {
		return nil, errors.New("video is not a valid MP4")
	}
	videoTracks := make([]mp4VideoTrack, 0, 1)
	for _, trackBox := range tracks {
		track, isVideo, err := parseMP4Track(reader, trackBox)
		if err != nil {
			return nil, err
		}
		if isVideo {
			videoTracks = append(videoTracks, track)
		}
	}
	if len(videoTracks) != 1 {
		return nil, errors.New("video MP4 must contain exactly one video track")
	}
	track := videoTracks[0]
	if track.durationSeconds <= 0 || track.width <= 0 || track.height <= 0 {
		return nil, errors.New("video MP4 metadata is incomplete")
	}
	tier := "720p"
	if track.height <= 480 {
		tier = "480p"
	}
	return &MediaProbeResult{DurationSeconds: track.durationSeconds, Width: track.width, Height: track.height, ResolutionTier: tier, MIMEType: mimeType}, nil
}

func parseMP4Track(reader *bytes.Reader, trackBox *mp4.BoxInfo) (mp4VideoTrack, bool, error) {
	metadata, err := mp4.ExtractBoxesWithPayload(reader, trackBox, []mp4.BoxPath{
		{mp4.BoxTypeTkhd()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMdhd()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeHdlr()},
		{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd()},
	})
	if err != nil {
		return mp4VideoTrack{}, false, errors.New("video is not a valid MP4")
	}
	var tkhd *mp4.Tkhd
	var mdhd *mp4.Mdhd
	var hdlr *mp4.Hdlr
	var stsd *mp4.Stsd
	for _, box := range metadata {
		switch payload := box.Payload.(type) {
		case *mp4.Tkhd:
			tkhd = payload
		case *mp4.Mdhd:
			mdhd = payload
		case *mp4.Hdlr:
			hdlr = payload
		case *mp4.Stsd:
			stsd = payload
		}
	}
	if hdlr == nil || hdlr.HandlerType != [4]byte{'v', 'i', 'd', 'e'} {
		return mp4VideoTrack{}, false, nil
	}
	if tkhd == nil || mdhd == nil || mdhd.Timescale == 0 || mdhd.GetDuration() == 0 || stsd == nil || stsd.EntryCount != 1 {
		return mp4VideoTrack{}, true, errors.New("video MP4 track metadata is incomplete")
	}
	codecBoxes, err := mp4.ExtractBoxes(reader, trackBox, supportedVideoCodecPaths())
	if err != nil || len(codecBoxes) != 1 {
		return mp4VideoTrack{}, true, errors.New("video MP4 codec is not supported")
	}
	width, height := int(tkhd.GetWidthInt()), int(tkhd.GetHeightInt())
	if trackRotatesQuarterTurn(tkhd.Matrix) {
		width, height = height, width
	}
	return mp4VideoTrack{
		durationSeconds: float64(mdhd.GetDuration()) / float64(mdhd.Timescale),
		width:           width,
		height:          height,
	}, true, nil
}

func supportedVideoCodecPaths() []mp4.BoxPath {
	prefix := mp4.BoxPath{mp4.BoxTypeMdia(), mp4.BoxTypeMinf(), mp4.BoxTypeStbl(), mp4.BoxTypeStsd()}
	codecs := []mp4.BoxType{
		mp4.BoxTypeAvc1(), mp4.StrToBoxType("avc3"), mp4.BoxTypeHvc1(), mp4.BoxTypeHev1(), mp4.BoxTypeAv01(),
	}
	paths := make([]mp4.BoxPath, 0, len(codecs))
	for _, codec := range codecs {
		path := append(append(mp4.BoxPath{}, prefix...), codec)
		paths = append(paths, path)
	}
	return paths
}

func trackRotatesQuarterTurn(matrix [9]int32) bool {
	abs := func(value int32) int64 {
		if value < 0 {
			return -int64(value)
		}
		return int64(value)
	}
	return matrix[0] == 0 && matrix[4] == 0 && abs(matrix[1]) == 0x00010000 && abs(matrix[3]) == 0x00010000
}

func normalizeMediaMIME(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
}

func isMP4MIME(value string) bool {
	return value == "video/mp4" || value == "application/mp4"
}
