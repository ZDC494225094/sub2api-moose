package service

import (
	"os"
	"strings"
	"sync"
	"unicode"
)

// AuditServerNodeEnv 多节点部署时为每台服务器设置的节点名（如 hk-1、us-west），
// 写入审计日志的 server_node，用于区分请求由哪台服务器处理。未设置时回退到主机名。
const AuditServerNodeEnv = "SERVER_NODE_NAME"

const (
	auditServerNodeMaxLen  = 128
	auditRequestHostMaxLen = 255
)

var (
	auditServerNodeOnce  sync.Once
	auditServerNodeValue string
)

// AuditServerNode 返回当前进程的节点名（进程内只解析一次）。
func AuditServerNode() string {
	auditServerNodeOnce.Do(func() {
		auditServerNodeValue = resolveAuditServerNode(os.Getenv(AuditServerNodeEnv), os.Hostname)
	})
	return auditServerNodeValue
}

func resolveAuditServerNode(configured string, hostname func() (string, error)) string {
	if node := SanitizeAuditEndpointText(configured, auditServerNodeMaxLen); node != "" {
		return node
	}
	if hostname != nil {
		if host, err := hostname(); err == nil {
			return SanitizeAuditEndpointText(host, auditServerNodeMaxLen)
		}
	}
	return ""
}

// SanitizeAuditRequestHost 规整请求 Host 头后入库（Host 由客户端提供，仅作溯源参考）。
func SanitizeAuditRequestHost(host string) string {
	return strings.ToLower(SanitizeAuditEndpointText(host, auditRequestHostMaxLen))
}

// SanitizeAuditEndpointText 去除空白与控制字符并按 rune 截断。
func SanitizeAuditEndpointText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
	runes := []rune(value)
	if limit > 0 && len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}
