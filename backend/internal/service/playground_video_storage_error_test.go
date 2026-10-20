package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/require"
)

func TestVideoStorageFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		code, category string
		retry          bool
	}{
		{"AccessDenied", "access_denied", false},
		{"SignatureDoesNotMatch", "authentication", false},
		{"NoSuchBucket", "bucket_not_found", false},
		{"SlowDown", "temporarily_unavailable", true},
		{"ServiceUnavailable", "temporarily_unavailable", true},
		{"unknown-provider-secret", "storage_error", true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			err := fmt.Errorf("S3 PutObject: %w", &smithy.GenericAPIError{Code: tc.code, Message: "secret signed URL"})
			f := classifyVideoStorageFailure(err)
			require.Equal(t, tc.category, f.category)
			require.Equal(t, tc.retry, f.retry)
			require.NotContains(t, f.hint, "secret")
		})
	}
	for _, status := range []int{400, 403, 404, 408, 429, 500, 503} {
		err := &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}}, Err: errors.New("secret")}
		f := classifyVideoStorageFailure(err)
		require.Equal(t, status, f.status)
		require.Equal(t, status == 408 || status == 429 || status >= 500, f.retry)
		require.NotContains(t, f.hint, "secret")
	}
	require.Equal(t, "canceled", classifyVideoStorageFailure(context.Canceled).category)
	require.False(t, classifyVideoStorageFailure(context.DeadlineExceeded).retry)
	require.Equal(t, "dns", classifyVideoStorageFailure(&net.DNSError{Err: "secret", Name: "private-endpoint"}).category)
	recordFailure := classifyVideoStorageFailure(fmt.Errorf("%w: secret DB message", ErrObjectStorageAssetRecord))
	require.Equal(t, "asset_record_failed", recordFailure.category)
	require.False(t, recordFailure.retry)
	require.Contains(t, recordFailure.hint, "素材记录写入失败")
	require.NotContains(t, recordFailure.hint, "secret")
}

func TestVideoInputStorageDeniedAndCanceled(t *testing.T) {
	storage := &videoStorageCountingFailure{err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "secret"}}
	svc := NewPlaygroundRunService()
	svc.imageStorageResolver = func() (*ImageResultUploader, bool) { return NewImageResultUploader(storage, "images/", 0, nil), true }
	request := PlaygroundRunRequest{Model: "minimax-h3", Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}}
	err := svc.prepareVideoInputImages(context.Background(), &request)
	require.ErrorContains(t, err, "PutObject")
	require.NotContains(t, err.Error(), "secret")
	require.Equal(t, 1, storage.calls)
	require.Equal(t, videoInputDataURL(), request.Images[0].DataURL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = svc.prepareVideoInputImages(ctx, &request)
	require.ErrorContains(t, err, "取消")
	require.Equal(t, 1, storage.calls, "canceled requests must not upload")
}

type videoStorageCountingFailure struct {
	calls int
	err   error
}

func (s *videoStorageCountingFailure) Save(_ context.Context, _, _ string, _ []byte) (string, error) {
	s.calls++
	return "", s.err
}

func TestVideoInputStorageTransientRetry(t *testing.T) {
	storage := &videoStorageTransient{}
	svc := NewPlaygroundRunService()
	svc.imageStorageResolver = func() (*ImageResultUploader, bool) { return NewImageResultUploader(storage, "images/", 0, nil), true }
	request := PlaygroundRunRequest{Model: "minimax-h3", Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}}
	require.NoError(t, svc.prepareVideoInputImages(context.Background(), &request))
	require.Equal(t, 2, storage.calls)
	require.Equal(t, "https://assets.example.test/reference.png", request.Images[0].DataURL)
	require.Equal(t, storage.keys[0], storage.keys[1], "retries must reuse the object key")
}

type videoStorageTransient struct {
	calls int
	keys  []string
}

func (s *videoStorageTransient) Save(_ context.Context, key, _ string, _ []byte) (string, error) {
	s.calls++
	s.keys = append(s.keys, key)
	if s.calls == 1 {
		return "", &smithy.GenericAPIError{Code: "ServiceUnavailable", Message: "secret"}
	}
	return "https://assets.example.test/reference.png", nil
}
