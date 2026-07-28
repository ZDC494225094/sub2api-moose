package handler

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeGeminiVideoOperationID(t *testing.T) {
	operationName := "projects/my-project/locations/us-central1/publishers/google/models/veo-3.1-generate-preview/operations/op-123"
	encoded := base64.RawURLEncoding.EncodeToString([]byte(operationName))

	decoded, err := decodeGeminiVideoOperationID(encoded)
	require.NoError(t, err)
	require.Equal(t, operationName, decoded)
	require.Equal(t, "/v1beta/"+operationName, geminiVideoOperationPath(decoded))
}

func TestDecodeGeminiVideoOperationIDRejectsUnsafeNames(t *testing.T) {
	for _, operationName := range []string{
		"",
		"models/veo-3.1",
		"models/veo-3.1/operations/../secret",
		"models/veo-3.1/operations/op-123?alt=media",
		"models/veo-3.1/operations/op-123#fragment",
		`models/veo-3.1/operations\\op-123`,
	} {
		t.Run(operationName, func(t *testing.T) {
			encoded := base64.RawURLEncoding.EncodeToString([]byte(operationName))
			_, err := decodeGeminiVideoOperationID(encoded)
			require.Error(t, err)
		})
	}

	_, err := decodeGeminiVideoOperationID("not-base64!")
	require.Error(t, err)
}

func TestGeminiVideoOperationContentURL(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "AI Studio generated video URI",
			body: `{"response":{"generatedVideos":[{"video":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/video-1:download?alt=media"}}]}}`,
			want: "https://generativelanguage.googleapis.com/v1beta/files/video-1:download?alt=media",
		},
		{
			name: "Vertex generated sample URI",
			body: `{"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"uri":"gs://video-bucket/output/video.mp4"}}]}}}`,
			want: "gs://video-bucket/output/video.mp4",
		},
		{
			name: "Vertex videos GCS URI",
			body: `{"response":{"videos":[{"gcsUri":"gs://video-bucket/output/video.mp4"}]}}`,
			want: "gs://video-bucket/output/video.mp4",
		},
		{
			name: "missing video",
			body: `{"done":true,"response":{}}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, geminiVideoOperationContentURL([]byte(tt.body)))
		})
	}
}
