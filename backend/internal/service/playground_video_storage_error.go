package service

import (
	"context"
	"crypto/x509"
	"errors"
	"net"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Only fixed categories/hints cross the user/log boundary, never SDK messages.
type videoStorageFailure struct {
	category string
	hint     string
	status   int
	retry    bool
}

func classifyVideoStorageFailure(err error) videoStorageFailure {
	f := videoStorageFailure{category: "storage_error", hint: "请检查存储配置及连接", retry: true}
	if errors.Is(err, ErrObjectStorageAssetRecord) {
		f.category, f.hint, f.retry = "asset_record_failed", "文件已上传，但素材记录写入失败，请检查数据库连接及 object_storage_assets 迁移", false
		return f
	}
	if errors.Is(err, context.Canceled) {
		f.category, f.hint, f.retry = "canceled", "上传已取消", false
		return f
	}
	if errors.Is(err, context.DeadlineExceeded) {
		f.category, f.hint, f.retry = "timeout", "上传超时，请检查对象存储网络连接", false
		return f
	}
	var response *smithyhttp.ResponseError
	if errors.As(err, &response) {
		f.status = response.HTTPStatusCode()
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "AccessDenied", "AllAccessDisabled":
			f.category, f.hint, f.retry = "access_denied", "对象存储拒绝写入，请检查密钥的 PutObject 权限及桶策略", false
			return f
		case "InvalidAccessKeyId", "InvalidToken", "ExpiredToken", "SignatureDoesNotMatch", "AuthorizationHeaderMalformed", "RequestTimeTooSkewed":
			f.category, f.hint, f.retry = "authentication", "对象存储认证或签名失败，请检查密钥、区域、端点及服务器时间", false
			return f
		case "NoSuchBucket":
			f.category, f.hint, f.retry = "bucket_not_found", "存储桶不存在，请检查桶名称与端点", false
			return f
		case "SlowDown", "Throttling", "RequestTimeout", "InternalError", "ServiceUnavailable":
			f.category, f.hint = "temporarily_unavailable", "对象存储暂时不可用，请稍后重试"
			return f
		}
	}
	if f.status != 0 {
		f.category, f.retry = "http_error", f.status == 408 || f.status == 429 || f.status >= 500
		switch f.status {
		case 401, 403:
			f.hint = "对象存储拒绝访问，请检查凭证、写入权限及桶策略"
		case 404:
			f.hint = "对象存储地址或存储桶不存在，请检查端点及桶名称"
		default:
			f.hint = "对象存储返回异常状态，请检查端点配置及服务状态"
		}
		return f
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		f.category, f.hint = "dns", "对象存储域名解析失败，请检查端点及服务器 DNS"
		return f
	}
	var cert x509.UnknownAuthorityError
	var host x509.HostnameError
	var invalid x509.CertificateInvalidError
	if errors.As(err, &cert) || errors.As(err, &host) || errors.As(err, &invalid) {
		f.category, f.hint, f.retry = "tls", "对象存储 TLS 证书校验失败，请检查端点及证书", false
		return f
	}
	var network net.Error
	if errors.As(err, &network) {
		f.category, f.hint = "network", "无法连接对象存储，请检查端点、网络及代理配置"
	}
	return f
}
