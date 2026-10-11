//go:build unit

package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestS3ImageStorageSaveVideoReference(t *testing.T) {
	data := []byte("video reference payload")
	var received []byte
	var length int64
	var contentType, requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		length, contentType, requestPath = r.ContentLength, r.Header.Get("Content-Type"), r.URL.Path
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	for _, public := range []string{"", "https://assets.example.test"} {
		storage, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{
			Endpoint: server.URL, Region: "auto", AccessKeyID: "test", SecretAccessKey: "test",
			ForcePathStyle: true, Bucket: "images", PublicBaseURL: public, PresignExpiry: 24,
		})
		require.NoError(t, err)
		result, err := storage.Save(context.Background(), "images/video-inputs/test-0.png", "image/png", data)
		require.NoError(t, err)
		require.Equal(t, data, received)
		require.Equal(t, int64(len(data)), length)
		require.Equal(t, "image/png", contentType)
		require.Equal(t, "/images/images/video-inputs/test-0.png", requestPath)
		parsed, err := url.Parse(result)
		require.NoError(t, err)
		if public == "" {
			require.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
		} else {
			require.Equal(t, public+"/images/video-inputs/test-0.png", result)
		}
	}
}
