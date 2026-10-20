package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func videoInputDataURL() string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}

func TestVideoInputStorageUploadsAllMedia(t *testing.T) {
	for _, gemini := range []bool{false, true} {
		storage := &fakeImageStorage{}
		svc := NewPlaygroundRunService()
		svc.imageStorageResolver = func() (*ImageResultUploader, bool) {
			return NewImageResultUploader(storage, "images/", 0, nil), true
		}
		videos := []PlaygroundRunVideoInput{
			{DataURL: "data:video/mp4;base64,QUJD", Type: "video/mp4"},
			{DataURL: "data:video/webm;base64,REVG", Type: "video/webm"},
		}
		request := PlaygroundRunRequest{Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL(), Type: "image/png"}}, ReferenceVideos: videos,
			ReferenceVideo:  &PlaygroundRunVideoInput{DataURL: videos[0].DataURL},
			ReferenceAudios: []PlaygroundRunVideoInput{{DataURL: "data:audio/mpeg;base64,R0hJ", Type: "audio/mpeg"}},
		}
		if gemini {
			request.Platform = PlatformGemini
		}
		require.NoError(t, svc.prepareVideoInputImages(context.Background(), &request))
		require.Len(t, storage.saved, 4)
		require.True(t, strings.HasSuffix(storage.saved[1].key, ".mp4"))
		require.True(t, strings.HasSuffix(storage.saved[2].key, ".webm"))
		require.True(t, strings.HasSuffix(storage.saved[3].key, ".mp3"))
		require.Equal(t, "video/webm", storage.saved[2].contentType)
		require.Equal(t, "DEF", string(storage.saved[2].data))
		require.Equal(t, request.ReferenceVideos[0].DataURL, request.ReferenceVideo.DataURL)
		require.Equal(t, "data:video/mp4;base64,QUJD", videos[0].DataURL)
		payload, err := playgroundVideoGenerationPayload(request, gemini)
		require.NoError(t, err)
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "data:")
		require.NotContains(t, string(encoded), "blob:")
		require.Contains(t, string(encoded), "https://cdn.test/")
	}
}

func TestVideoInputStorageMediaFailureKeepsCaller(t *testing.T) {
	storage := &fakeImageStorage{}
	svc := NewPlaygroundRunService()
	svc.imageStorageResolver = func() (*ImageResultUploader, bool) { return NewImageResultUploader(storage, "images/", 0, nil), true }
	request := PlaygroundRunRequest{Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}, ReferenceVideos: []PlaygroundRunVideoInput{{DataURL: "data:video/mp4;base64,!!!"}}}
	require.Error(t, svc.prepareVideoInputImages(context.Background(), &request))
	require.Equal(t, videoInputDataURL(), request.Images[0].DataURL)
	require.Equal(t, "data:video/mp4;base64,!!!", request.ReferenceVideos[0].DataURL)
}

func TestVideoInputStorageFramesAndReferences(t *testing.T) {
	storage := &fakeImageStorage{}
	svc := NewPlaygroundRunService()
	svc.imageStorageResolver = func() (*ImageResultUploader, bool) {
		return NewImageResultUploader(storage, "images/", 0, nil), true
	}
	original := []PlaygroundRunImageInput{
		{DataURL: videoInputDataURL(), Frame: "first", Name: "hero.png"},
		{DataURL: "https://existing.test/tail.png", Frame: "last"},
		{DataURL: videoInputDataURL()},
	}
	request := PlaygroundRunRequest{Model: "minimax-h3", Images: original}
	require.NoError(t, svc.prepareVideoInputImages(context.Background(), &request))
	require.Len(t, storage.saved, 1, "duplicate references reuse one upload")
	require.Contains(t, storage.saved[0].key, "images/video-inputs/")
	require.Equal(t, pngBytes, storage.saved[0].data)
	require.Equal(t, "image/png", storage.saved[0].contentType)
	require.Equal(t, videoInputDataURL(), original[0].DataURL, "caller input is not mutated")
	require.Equal(t, "hero.png", request.Images[0].Name)
	payload, err := playgroundVideoGenerationPayload(request, false)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.test/"+storage.saved[0].key, payload["image"])
	require.Equal(t, "https://existing.test/tail.png", payload["image_tail"])
	require.Equal(t, []string{request.Images[0].DataURL}, payload["reference_images"])
}

func TestVideoInputStorageSwitchAndGemini(t *testing.T) {
	storage := &fakeImageStorage{}
	svc := NewPlaygroundRunService()
	enabled, calls := false, 0
	svc.imageStorageResolver = func() (*ImageResultUploader, bool) {
		calls++
		return NewImageResultUploader(storage, "images/", 0, nil), enabled
	}
	for _, active := range []bool{false, true, false} {
		enabled = active
		request := PlaygroundRunRequest{Model: "minimax-h3", Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}}
		require.NoError(t, svc.prepareVideoInputImages(context.Background(), &request))
		if active {
			require.NotEqual(t, videoInputDataURL(), request.Images[0].DataURL)
		} else {
			require.Equal(t, videoInputDataURL(), request.Images[0].DataURL)
		}
	}
	require.Equal(t, 3, calls)
	enabled = true
	for _, model := range []string{"veo-3.1", "gemini-omni-flash"} {
		request := PlaygroundRunRequest{Model: model, Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}}
		require.NoError(t, svc.prepareVideoInputImages(context.Background(), &request))
		require.True(t, strings.HasPrefix(request.Images[0].DataURL, "https://cdn.test/"))
	}
	request := PlaygroundRunRequest{Model: "minimax-h3", Images: []PlaygroundRunImageInput{{DataURL: "https://existing.test/a.png"}}}
	require.NoError(t, svc.prepareVideoInputImages(context.Background(), &request))
	require.Equal(t, 5, calls, "all inline protocols resolve storage; HTTP references are retained")
	require.Len(t, storage.saved, 3)
}

func TestVideoInputStorageRejectsInvalidInputsAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, data, resultURL string
		limit                 int64
		storageErr            error
	}{
		{name: "malformed", data: "data:image/png;base64,!!!"},
		{name: "non-image", data: "data:text/plain;base64,QUJD"},
		{name: "fake-image", data: "data:image/png;base64,QUJD"},
		{name: "too-large", data: videoInputDataURL(), limit: 4},
		{name: "upload-failed", data: videoInputDataURL(), storageErr: errors.New("secret details")},
		{name: "invalid-url", data: videoInputDataURL(), resultURL: "data:image/png;base64,QUJD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := &fakeImageStorage{err: tc.storageErr, url: tc.resultURL}
			svc := NewPlaygroundRunService()
			svc.imageStorageResolver = func() (*ImageResultUploader, bool) {
				return NewImageResultUploader(storage, "images/", tc.limit, nil), true
			}
			request := PlaygroundRunRequest{Model: "minimax-h3", Images: []PlaygroundRunImageInput{{DataURL: tc.data}}}
			err := svc.prepareVideoInputImages(context.Background(), &request)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret details")
			require.Equal(t, tc.data, request.Images[0].DataURL)
		})
	}
}

func TestVideoInputStorageExecuteVideoSubmitsURL(t *testing.T) {
	storage := &fakeImageStorage{url: "https://bucket.test/reference.png?signature=test"}
	svc := NewPlaygroundRunService()
	svc.imageStorageResolver = func() (*ImageResultUploader, bool) {
		return NewImageResultUploader(storage, "images/", 0, nil), true
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload["image"] != storage.url {
			t.Errorf("upstream image = %v, want signed URL", payload["image"])
		}
		w.Header().Set("Content-Type", "application/json")
		// Deliberately end after submission, without a paid generation or download.
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	request := PlaygroundRunRequest{Model: "minimax-h3", APIKey: "test", Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}}
	_, err := svc.executeVideo(context.Background(), "test", request, server.URL, time.Now())
	require.ErrorContains(t, err, "neither a video nor a task id")
	require.Equal(t, 1, calls)
	storage.err = errors.New("upload failed")
	_, err = svc.executeVideo(context.Background(), "test", request, server.URL, time.Now())
	require.ErrorContains(t, err, "转存对象存储失败")
	require.Equal(t, 1, calls, "upload failure must not submit to upstream")
}
