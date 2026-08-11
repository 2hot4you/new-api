package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abema/go-mp4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixtureMP4Track struct {
	handler  [4]byte
	codec    mp4.BoxType
	width    uint16
	height   uint16
	duration uint32
	matrix   [9]int32
}

var fixtureCodecDefinitions sync.Once

func videoFixtureTrack(codec string, width, height uint16, duration uint32) fixtureMP4Track {
	return fixtureMP4Track{
		handler:  [4]byte{'v', 'i', 'd', 'e'},
		codec:    mp4.StrToBoxType(codec),
		width:    width,
		height:   height,
		duration: duration,
	}
}

func TestMediaProbeUsesUniqueVideoTrackMetadataAndNormalizesTier(t *testing.T) {
	data := fixtureMP4(t, 99_000,
		videoFixtureTrack("avc1", 640, 480, 7_500),
		fixtureMP4Track{handler: [4]byte{'s', 'o', 'u', 'n'}, width: 1920, height: 1080, duration: 88_000},
	)

	result, err := ProbeUserVideo(context.Background(), MediaSource{Data: data, MIMEType: "video/mp4"})

	require.NoError(t, err)
	assert.InDelta(t, 7.5, result.DurationSeconds, 0.001)
	assert.Equal(t, 640, result.Width)
	assert.Equal(t, 480, result.Height)
	assert.Equal(t, "480p", result.ResolutionTier)
	assert.Equal(t, "video/mp4", result.MIMEType)
}

func TestMediaProbeAcceptsSupportedVideoCodecs(t *testing.T) {
	for _, codec := range []string{"avc1", "avc3", "hvc1", "hev1", "av01"} {
		t.Run(codec, func(t *testing.T) {
			result, err := ProbeUserVideo(context.Background(), MediaSource{
				Data: fixtureMP4(t, 99_000, videoFixtureTrack(codec, 1280, 720, 8_700)), MIMEType: "video/mp4",
			})
			require.NoError(t, err)
			assert.Equal(t, "720p", result.ResolutionTier)
		})
	}
}

func TestMediaProbeRejectsMissingMultipleOrUnknownVideoTracks(t *testing.T) {
	tests := []struct {
		name   string
		tracks []fixtureMP4Track
	}{
		{name: "no video", tracks: []fixtureMP4Track{{handler: [4]byte{'s', 'o', 'u', 'n'}, duration: 1_000}}},
		{name: "multiple video", tracks: []fixtureMP4Track{videoFixtureTrack("avc1", 640, 480, 1_000), videoFixtureTrack("hvc1", 1280, 720, 1_000)}},
		{name: "unknown codec", tracks: []fixtureMP4Track{videoFixtureTrack("vp09", 640, 480, 1_000)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ProbeUserVideo(context.Background(), MediaSource{Data: fixtureMP4(t, 99_000, tt.tracks...), MIMEType: "video/mp4"})
			assert.Error(t, err)
		})
	}
}

func TestMediaProbeUsesDisplayDimensionsAfterTrackRotation(t *testing.T) {
	track := videoFixtureTrack("avc1", 1920, 1080, 5_000)
	track.matrix = [9]int32{0, 0x00010000, 0, -0x00010000, 0, 0, 0, 0, 0x40000000}

	result, err := ProbeUserVideo(context.Background(), MediaSource{Data: fixtureMP4(t, 5_000, track), MIMEType: "video/mp4"})

	require.NoError(t, err)
	assert.Equal(t, 1080, result.Width)
	assert.Equal(t, 1920, result.Height)
	assert.Equal(t, "720p", result.ResolutionTier)
}

func TestMediaProbeRejectsUnsafeRemoteSources(t *testing.T) {
	for _, source := range []MediaSource{
		{URL: "http://127.0.0.1/video.mp4"},
		{URL: "https://user@example.com/video.mp4"},
	} {
		_, err := ProbeUserVideo(context.Background(), source)
		assert.Error(t, err)
	}
}

func TestMediaProbeRequiresRemoteMP4ExtensionBeforeFetch(t *testing.T) {
	var calls atomic.Int32
	probe := newMediaProbe(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return mp4HTTPResponse(fixtureMP4(t, 1_000, videoFixtureTrack("avc1", 640, 480, 1_000))), nil
	})})

	_, err := probe.ProbeUserVideo(context.Background(), MediaSource{URL: "https://example.com/video"})

	assert.Error(t, err)
	assert.Zero(t, calls.Load())
}

func TestMediaProbeRejectsOversizedOrInvalidMIMEResponses(t *testing.T) {
	tests := []struct {
		name     string
		response *http.Response
	}{
		{name: "invalid MIME", response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader("not a video"))}},
		{name: "oversized", response: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}, ContentLength: mediaProbeMaxBytes + 1, Body: io.NopCloser(strings.NewReader("body is not read"))}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe := newMediaProbe(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) { return tt.response, nil })})
			_, err := probe.ProbeUserVideo(context.Background(), MediaSource{URL: "https://example.com/video.mp4"})
			assert.Error(t, err)
		})
	}
}

func TestMediaProbeHonorsRemoteContextCancellation(t *testing.T) {
	probe := newMediaProbe(&http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := probe.ProbeUserVideo(ctx, MediaSource{URL: "https://example.com/video.mp4"})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestMediaProbeRejectsOversizedDataURLBeforeDecode(t *testing.T) {
	encoded := strings.Repeat("A", base64.StdEncoding.EncodedLen(mediaProbeMaxBytes)+4)
	_, err := ProbeUserVideo(context.Background(), MediaSource{DataURL: "data:video/mp4;base64," + encoded})
	assert.Error(t, err)
}

func TestMediaProbeDataURLAllowsExactDecodedSizeLimit(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(make([]byte, mediaProbeMaxBytes))
	data, mimeType, err := decodeMP4DataURL(context.Background(), "data:video/mp4;base64,"+encoded)

	require.NoError(t, err)
	assert.Len(t, data, mediaProbeMaxBytes)
	assert.Equal(t, "video/mp4", mimeType)
}

func TestMediaProbeDataURLDecodeHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := fixtureMP4(t, 1_000, videoFixtureTrack("avc1", 640, 480, 1_000))

	_, err := ProbeUserVideo(ctx, MediaSource{DataURL: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(data)})

	assert.ErrorIs(t, err, context.Canceled)
}

func TestMediaProbeRejectsMalformedDataURL(t *testing.T) {
	_, err := ProbeUserVideo(context.Background(), MediaSource{DataURL: "data:video/mp4;base64,%%%%"})
	assert.Error(t, err)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mp4HTTPResponse(data []byte) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": []string{"video/mp4"}},
		ContentLength: int64(len(data)),
		Body:          io.NopCloser(bytes.NewReader(data)),
	}
}

func fixtureMP4(t *testing.T, movieDuration uint32, tracks ...fixtureMP4Track) []byte {
	t.Helper()
	fixtureCodecDefinitions.Do(func() {
		mp4.AddAnyTypeBoxDef(&mp4.VisualSampleEntry{}, mp4.StrToBoxType("avc3"))
	})
	file, err := os.CreateTemp(t.TempDir(), "fixture-*.mp4")
	require.NoError(t, err)
	defer file.Close()
	w := mp4.NewWriter(file)
	startFixtureBox(t, w, mp4.BoxTypeFtyp(), &mp4.Ftyp{MajorBrand: [4]byte{'i', 's', 'o', 'm'}})
	startFixtureContainer(t, w, mp4.BoxTypeMoov())
	startFixtureBox(t, w, mp4.BoxTypeMvhd(), &mp4.Mvhd{Timescale: 1000, DurationV0: movieDuration})
	for index, track := range tracks {
		writeFixtureTrack(t, w, uint32(index+1), track)
	}
	endFixtureBox(t, w)
	startFixtureContainer(t, w, mp4.BoxTypeMdat())
	_, err = w.Write([]byte{0})
	require.NoError(t, err)
	endFixtureBox(t, w)
	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	_, err = mp4.Probe(bytes.NewReader(data))
	require.NoError(t, err, "fixture must be a structurally probeable MP4")
	return data
}

func writeFixtureTrack(t *testing.T, w *mp4.Writer, id uint32, track fixtureMP4Track) {
	t.Helper()
	startFixtureContainer(t, w, mp4.BoxTypeTrak())
	startFixtureBox(t, w, mp4.BoxTypeTkhd(), &mp4.Tkhd{TrackID: id, DurationV0: track.duration, Width: uint32(track.width) << 16, Height: uint32(track.height) << 16, Matrix: track.matrix})
	startFixtureContainer(t, w, mp4.BoxTypeMdia())
	startFixtureBox(t, w, mp4.BoxTypeMdhd(), &mp4.Mdhd{Timescale: 1000, DurationV0: track.duration})
	startFixtureBox(t, w, mp4.BoxTypeHdlr(), &mp4.Hdlr{HandlerType: track.handler, Name: "handler"})
	startFixtureContainer(t, w, mp4.BoxTypeMinf())
	startFixtureContainer(t, w, mp4.BoxTypeStbl())
	entryCount := uint32(0)
	if track.codec != (mp4.BoxType{}) {
		entryCount = 1
	}
	_, err := w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStsd()})
	require.NoError(t, err)
	_, err = mp4.Marshal(w, &mp4.Stsd{EntryCount: entryCount}, mp4.Context{})
	require.NoError(t, err)
	if entryCount == 1 {
		entry := &mp4.VisualSampleEntry{SampleEntry: mp4.SampleEntry{AnyTypeBox: mp4.AnyTypeBox{Type: track.codec}, DataReferenceIndex: 1}, Width: track.width, Height: track.height, Horizresolution: 0x00480000, Vertresolution: 0x00480000, FrameCount: 1, Depth: 0x18, PreDefined3: -1}
		startFixtureBox(t, w, track.codec, entry)
	}
	endFixtureBox(t, w)
	startFixtureBox(t, w, mp4.BoxTypeStts(), &mp4.Stts{EntryCount: 1, Entries: []mp4.SttsEntry{{SampleCount: 1, SampleDelta: track.duration}}})
	startFixtureBox(t, w, mp4.BoxTypeStsc(), &mp4.Stsc{EntryCount: 1, Entries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 1, SampleDescriptionIndex: 1}}})
	startFixtureBox(t, w, mp4.BoxTypeStsz(), &mp4.Stsz{SampleCount: 1, EntrySize: []uint32{1}})
	startFixtureBox(t, w, mp4.BoxTypeStco(), &mp4.Stco{EntryCount: 1, ChunkOffset: []uint32{0}})
	endFixtureBox(t, w)
	endFixtureBox(t, w)
	endFixtureBox(t, w)
	endFixtureBox(t, w)
}

func startFixtureContainer(t *testing.T, w *mp4.Writer, boxType mp4.BoxType) {
	t.Helper()
	_, err := w.StartBox(&mp4.BoxInfo{Type: boxType})
	require.NoError(t, err)
}

func startFixtureBox(t *testing.T, w *mp4.Writer, boxType mp4.BoxType, payload mp4.IImmutableBox) {
	t.Helper()
	startFixtureContainer(t, w, boxType)
	_, err := mp4.Marshal(w, payload, mp4.Context{})
	require.NoError(t, err)
	endFixtureBox(t, w)
}

func endFixtureBox(t *testing.T, w *mp4.Writer) {
	t.Helper()
	_, err := w.EndBox()
	require.NoError(t, err)
}
