package gatewaylogarchive

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// storageErrorText rewrites an AWS SDK error so the cause comes first.
// The SDK text ("operation error S3: PutObject, exceeded maximum number of
// attempts, 3, https response error StatusCode: 0, RequestID: , HostID: ,
// request send failed, Put \"http://…/<key>?…\": dial tcp …") puts the only
// actionable part after the full request URL, past what the page displays.
// Wrapping context (e.g. the relayHTTP1Retry prefix) and the transport cause text
// (e.g. HTTP/2 PROTOCOL_ERROR) are kept verbatim; callers match on them.
func storageErrorText(err error) string {
	if err == nil {
		return ""
	}
	full := err.Error()
	var op *smithy.OperationError
	if !errors.As(err, &op) {
		return full
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s", op.Service(), op.Operation())
	var canceled *smithy.CanceledError
	var send *smithyhttp.RequestSendError
	var response *awshttp.ResponseError
	switch {
	case errors.As(err, &canceled):
		b.WriteString(" 请求被中止")
	case errors.As(err, &send):
		b.WriteString(" 请求未送达或无响应")
	case errors.As(err, &response) && response.HTTPStatusCode() != 0:
		fmt.Fprintf(&b, " 返回 HTTP %d", response.HTTPStatusCode())
	default:
		b.WriteString(" 失败")
	}
	var attempts *retry.MaxAttemptsError
	if errors.As(err, &attempts) {
		fmt.Fprintf(&b, "（已尝试 %d 次）", attempts.Attempt)
	}
	b.WriteString("：")
	var target *url.Error
	if errors.As(err, &target) {
		b.WriteString(target.Err.Error())
		if u, parseErr := url.Parse(target.URL); parseErr == nil && u.Host != "" {
			fmt.Fprintf(&b, "（%s %s://%s）", strings.ToUpper(target.Op), u.Scheme, u.Host)
		}
	} else {
		b.WriteString(innermost(op.Err).Error())
	}
	if response != nil && response.ServiceRequestID() != "" {
		fmt.Fprintf(&b, "，RequestID %s", response.ServiceRequestID())
	}
	// Replace only the SDK part; wrapping context before or after it stays.
	return strings.Replace(full, op.Error(), b.String(), 1)
}

func innermost(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}
