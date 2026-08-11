package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/abema/go-mp4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaProbeReadsMP4MetadataAndNormalizesResolutionTier(t *testing.T) {
	for _, tc := range []struct {
		name   string
		height uint16
		want   string
	}{
		{name: "480p", height: 480, want: "480p"},
		{name: "720p cap", height: 1080, want: "720p"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := fixtureMP4(t, 640, tc.height, 7500)
			result, err := ProbeUserVideo(context.Background(), MediaSource{
				DataURL: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(data),
			})
			require.NoError(t, err)
			assert.InDelta(t, 7.5, result.DurationSeconds, 0.001)
			assert.Equal(t, 640, result.Width)
			assert.Equal(t, int(tc.height), result.Height)
			assert.Equal(t, tc.want, result.ResolutionTier)
			assert.Equal(t, "video/mp4", result.MIMEType)
		})
	}
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

func TestMediaProbeRejectsUnsafeRedirectAndOversizedOrInvalidMIMEResponses(t *testing.T) {
	originalClient := mediaProbeHTTPClient
	originalValidateURL := mediaProbeValidateURL
	t.Cleanup(func() {
		mediaProbeHTTPClient = originalClient
		mediaProbeValidateURL = originalValidateURL
	})
	mediaProbeValidateURL = func(string) error { return nil }

	mediaProbeHTTPClient = func() *http.Client {
		return &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://user@example.com/video.mp4"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
			}),
			CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("unsafe redirect") },
		}
	}
	_, err := ProbeUserVideo(context.Background(), MediaSource{URL: "https://example.com/video.mp4"})
	assert.Error(t, err, "an unsafe redirect must not be followed")

	mediaProbeHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader("not a video"))}, nil
		})}
	}
	_, err = ProbeUserVideo(context.Background(), MediaSource{URL: "https://example.com/video.mp4"})
	assert.Error(t, err, "a declared non-MP4 response must fail before parsing")

	mediaProbeHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}, ContentLength: mediaProbeMaxBytes + 1, Body: io.NopCloser(strings.NewReader("body is not read"))}, nil
		})}
	}
	_, err = ProbeUserVideo(context.Background(), MediaSource{URL: "https://example.com/video.mp4"})
	assert.Error(t, err, "an oversized remote body must fail before it is read")
}

func TestMediaProbeHonorsContextCancellation(t *testing.T) {
	originalClient := mediaProbeHTTPClient
	originalValidateURL := mediaProbeValidateURL
	t.Cleanup(func() {
		mediaProbeHTTPClient = originalClient
		mediaProbeValidateURL = originalValidateURL
	})
	mediaProbeValidateURL = func(string) error { return nil }
	mediaProbeHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := ProbeUserVideo(ctx, MediaSource{URL: "https://example.com/video.mp4"})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixtureMP4(t *testing.T, width, height uint16, duration uint32) []byte {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "fixture-*.mp4")
	require.NoError(t, err)
	defer file.Close()
	w := mp4.NewWriter(file)
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeFtyp()})
	require.NoError(t, err)
	_, err = mp4.Marshal(w, &mp4.Ftyp{MajorBrand: [4]byte{'i', 's', 'o', 'm'}}, mp4.Context{})
	require.NoError(t, err)
	_, err = w.EndBox()
	require.NoError(t, err)
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMoov()})
	require.NoError(t, err)
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeMvhd()})
	require.NoError(t, err)
	_, err = mp4.Marshal(w, &mp4.Mvhd{Timescale: 1000, DurationV0: duration}, mp4.Context{})
	require.NoError(t, err)
	_, err = w.EndBox()
	require.NoError(t, err)
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeTrak()})
	require.NoError(t, err)
	_, err = w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeTkhd()})
	require.NoError(t, err)
	_, err = mp4.Marshal(w, &mp4.Tkhd{TrackID: 1, Width: uint32(width) << 16, Height: uint32(height) << 16}, mp4.Context{})
	require.NoError(t, err)
	_, err = w.EndBox()
	require.NoError(t, err)
	_, err = w.EndBox()
	require.NoError(t, err)
	_, err = w.EndBox()
	require.NoError(t, err)
	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	return bytes.Clone(data)
}

func TestMediaProbeRejectsOversizedDataURL(t *testing.T) {
	data := strings.Repeat("A", mediaProbeMaxBytes+1)
	_, err := ProbeUserVideo(context.Background(), MediaSource{DataURL: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte(data))})
	assert.Error(t, err)
}
