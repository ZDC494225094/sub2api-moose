package gatewaylogarchive

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// A real SDK request to a closed port: the page used to show only the first
// 160 characters, which ended inside the request URL.
func TestStorageErrorTextPutsSendCauseFirst(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	objects, err := newS3Objects(BucketConfig{Endpoint: "http://" + addr, Bucket: "archive", Region: "us-east-1", ForcePathStyle: true, AccessKeyID: "a", SecretAccessKey: "s"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = objects.(*s3Objects).client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("archive"), Key: aws.String("2026-11-20/very/long/prefix/model-2026-11-20-1.jsonl"), Body: strings.NewReader("x"),
	}, func(o *s3.Options) {
		o.Retryer = retry.AddWithMaxBackoffDelay(retry.AddWithMaxAttempts(o.Retryer, 2), time.Millisecond)
	})
	if err == nil {
		t.Fatal("expected a send error")
	}
	got := storageErrorText(fmt.Errorf("上传 a.jsonl 失败：%w", err))
	want := "上传 a.jsonl 失败：S3 PutObject 请求未送达或无响应（已尝试 2 次）："
	if !strings.HasPrefix(got, want) {
		t.Fatalf("got %q, want prefix %q", got, want)
	}
	if !strings.Contains(got, "refused") && !strings.Contains(got, "actively refused") {
		t.Fatalf("connection cause missing: %q", got)
	}
	if !strings.HasSuffix(got, "（PUT http://"+addr+"）") || strings.Contains(got, "model-2026") {
		t.Fatalf("expected only the endpoint, not the object URL: %q", got)
	}
}

func TestStorageErrorTextKeepsRelayRetryMarkers(t *testing.T) {
	sdk := &smithy.OperationError{ServiceID: "S3", OperationName: "UploadPart", Err: &retry.MaxAttemptsError{Attempt: 3, Err: &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{}}, Err: &smithyhttp.RequestSendError{Err: &url.Error{
			Op: "Put", URL: "https://dest.example.com/b/k.jsonl?partNumber=3&uploadId=abc",
			Err: errors.New("stream error: stream ID 5; PROTOCOL_ERROR; received from peer"),
		}}},
	}}}
	got := storageErrorText(fmt.Errorf("%s：写入目标桶 dest：%w", relayHTTP1Retry, sdk))
	if !retryRelayHTTP1(got) || !relayProtocolError(got) {
		t.Fatalf("relay retry markers lost: %q", got)
	}
	want := relayHTTP1Retry + "：写入目标桶 dest：S3 UploadPart 请求未送达或无响应（已尝试 3 次）：stream error: stream ID 5; PROTOCOL_ERROR; received from peer（PUT https://dest.example.com）"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestStorageErrorTextHTTPStatusAndPlainErrors(t *testing.T) {
	sdk := &smithy.OperationError{ServiceID: "S3", OperationName: "CreateMultipartUpload", Err: &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 403}}, Err: &smithy.GenericAPIError{Code: "AccessDenied", Message: "denied"}},
		RequestID:     "req-1",
	}}
	got := storageErrorText(fmt.Errorf("创建分片上传：%w", sdk))
	if want := "创建分片上传：S3 CreateMultipartUpload 返回 HTTP 403：api error AccessDenied: denied，RequestID req-1"; got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	canceled := &smithy.OperationError{ServiceID: "S3", OperationName: "UploadPart", Err: &smithy.CanceledError{Err: context.DeadlineExceeded}}
	if got, want := storageErrorText(fmt.Errorf("part 3: %w", canceled)), "part 3: S3 UploadPart 请求被中止：context deadline exceeded"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := storageErrorText(errors.New("本地归档文件不存在")); got != "本地归档文件不存在" {
		t.Fatalf("plain error changed: %q", got)
	}
	if storageErrorText(nil) != "" {
		t.Fatal("nil error must be empty")
	}
}
