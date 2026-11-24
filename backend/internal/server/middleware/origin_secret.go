package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http/httpguts"
)

// 回源密钥：CDN 回源时携带固定请求头，源站只接受带正确密钥的请求，
// 防止访客绕过 CDN 直接用「源站 IP:端口」访问。
const (
	OriginSecretEnv           = "ORIGIN_SECRET"
	OriginSecretHeaderEnv     = "ORIGIN_SECRET_HEADER"
	DefaultOriginSecretHeader = "X-Origin-Secret"

	originSecretMinLength = 16
	originSecretMaxValues = 4
	originSecretHealthURL = "/health"
)

// OriginSecretConfig 回源密钥配置。Secrets 为空表示未启用。
type OriginSecretConfig struct {
	Header  string
	secrets [][sha256.Size]byte
	// primary is the first configured secret, kept only so the server's own
	// loopback calls (bot AI, 降智监测) can pass the check.
	primary string
}

// Enabled 是否启用回源密钥校验。
func (c OriginSecretConfig) Enabled() bool {
	return len(c.secrets) > 0
}

// SecretCount 已配置的密钥数量（轮换期间可为多个）。
func (c OriginSecretConfig) SecretCount() int {
	return len(c.secrets)
}

// LoadOriginSecretConfigFromEnv 从 ORIGIN_SECRET / ORIGIN_SECRET_HEADER 读取配置。
func LoadOriginSecretConfigFromEnv() (OriginSecretConfig, error) {
	return ParseOriginSecretConfig(os.Getenv(OriginSecretHeaderEnv), os.Getenv(OriginSecretEnv))
}

// ParseOriginSecretConfig 解析回源密钥配置。
// secrets 以英文逗号分隔，支持「新值,旧值」以便换密钥时不中断；全部为空表示未启用。
// header 为空时使用 X-Origin-Secret。
func ParseOriginSecretConfig(header, secrets string) (OriginSecretConfig, error) {
	var cfg OriginSecretConfig
	for _, raw := range strings.Split(secrets, ",") {
		secret := trimEnvQuotes(raw)
		if secret == "" {
			continue
		}
		if len(secret) < originSecretMinLength {
			return OriginSecretConfig{}, fmt.Errorf("%s: each secret must be at least %d characters", OriginSecretEnv, originSecretMinLength)
		}
		if cfg.primary == "" {
			cfg.primary = secret
		}
		cfg.secrets = append(cfg.secrets, sha256.Sum256([]byte(secret)))
	}
	if !cfg.Enabled() {
		return OriginSecretConfig{}, nil
	}

	header = trimEnvQuotes(header)
	if header == "" {
		header = DefaultOriginSecretHeader
	}
	if !httpguts.ValidHeaderFieldName(header) {
		return OriginSecretConfig{}, fmt.Errorf("%s: invalid HTTP header name %q", OriginSecretHeaderEnv, header)
	}
	cfg.Header = textproto.CanonicalMIMEHeaderKey(header)
	return cfg, nil
}

// OriginSecretLoopbackHeader returns the header that the server's own
// loopback requests must carry when ORIGIN_SECRET is enabled. Without it the
// middleware answers those requests with an empty 403 before any handler runs.
// ok is false when the check is disabled or misconfigured.
func OriginSecretLoopbackHeader() (name, value string, ok bool) {
	cfg, err := LoadOriginSecretConfigFromEnv()
	if err != nil || !cfg.Enabled() {
		return "", "", false
	}
	return cfg.Header, cfg.primary, true
}

// trimEnvQuotes 去掉首尾空白，以及 compose 列表写法中被原样保留的一对引号。
func trimEnvQuotes(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' || first == '\'') && first == last {
			value = strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return value
}

// OriginSecret 校验回源密钥。密钥不匹配时返回空 403 并关闭连接；
// GET/HEAD /health 放行，供容器健康检查使用。校验通过后删除该请求头，
// 避免密钥被转发到上游或写入日志。
func OriginSecret(cfg OriginSecretConfig) gin.HandlerFunc {
	if !cfg.Enabled() {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		values := c.Request.Header.Values(cfg.Header)
		matched := cfg.matches(values)
		c.Request.Header.Del(cfg.Header)
		if matched || isOriginSecretExempt(c.Request) {
			c.Next()
			return
		}
		c.Header("Connection", "close")
		c.AbortWithStatus(http.StatusForbidden)
	}
}

func (c OriginSecretConfig) matches(values []string) bool {
	if len(values) > originSecretMaxValues {
		values = values[:originSecretMaxValues]
	}
	matched := 0
	for _, value := range values {
		sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
		for i := range c.secrets {
			matched |= subtle.ConstantTimeCompare(sum[:], c.secrets[i][:])
		}
	}
	return matched == 1
}

func isOriginSecretExempt(r *http.Request) bool {
	if r == nil || r.URL == nil || r.URL.Path != originSecretHealthURL {
		return false
	}
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}
