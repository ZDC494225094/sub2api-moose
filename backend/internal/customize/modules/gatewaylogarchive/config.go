// Package gatewaylogarchive stores complete gateway request/response bodies,
// one JSON document per client session, packs documents of the same model into
// one ZIP per day (rolling at a configurable size) and uploads the ZIPs to
// S3-compatible buckets under <prefix>/<YYYY-MM-DD>/.
package gatewaylogarchive

import (
	"errors"
	"regexp"
	"strings"
)

const ID = "gateway-log-archive"

const (
	ModeAll      = "all"
	ModeSelected = "selected"

	ProviderAWS    = "aws"
	ProviderCompat = "s3-compatible"
)

// ModelRule matches client-written model names with case-insensitive "*"
// wildcards. Alias, when set, becomes the ZIP name so several model variants
// share one archive (fewer, larger ZIPs).
type ModelRule struct {
	Pattern string `json:"pattern"`
	Alias   string `json:"alias"`
}

// Config is the persisted capture configuration. Storage destinations live in
// gateway_log_archive_buckets (see BucketConfig).
type Config struct {
	CaptureEnabled     bool        `json:"capture_enabled"`
	Mode               string      `json:"mode"`
	Rules              []ModelRule `json:"rules"`
	SuccessOnly        bool        `json:"success_only"`
	MaxZipSizeMB       int64       `json:"max_zip_size_mb"`
	SessionIdleSeconds int         `json:"session_idle_seconds"`
	MaxBodyMB          int         `json:"max_body_mb"`
	KeepLocalHours     int         `json:"keep_local_hours"`
	EntryRetentionDays int         `json:"entry_retention_days"`
	MinFreeDiskGB      int         `json:"min_free_disk_gb"`

	// BucketsMigrated marks that the single bucket of earlier versions (the
	// legacy fields below) has been moved into the buckets table.
	BucketsMigrated bool `json:"buckets_migrated,omitempty"`

	// Legacy single-bucket fields: read once for adoption, then cleared.
	Provider       string `json:"provider,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	Region         string `json:"region,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	Prefix         string `json:"prefix,omitempty"`
	ForcePathStyle bool   `json:"force_path_style,omitempty"`
}

func (c *Config) clearLegacyStorage() {
	c.Provider, c.Bucket, c.Region, c.Endpoint, c.Prefix, c.ForcePathStyle = "", "", "", "", "", false
}

// keepStorageState carries the adoption marker and any not yet adopted legacy
// fields from the stored row; requests never change them.
func (c *Config) keepStorageState(stored Config) {
	c.BucketsMigrated = stored.BucketsMigrated
	c.Provider, c.Bucket, c.Region = stored.Provider, stored.Bucket, stored.Region
	c.Endpoint, c.Prefix, c.ForcePathStyle = stored.Endpoint, stored.Prefix, stored.ForcePathStyle
}

func DefaultConfig() Config {
	return Config{
		Mode:               ModeAll,
		Rules:              []ModelRule{},
		SuccessOnly:        true,
		MaxZipSizeMB:       10240,
		SessionIdleSeconds: 1800,
		MaxBodyMB:          64,
		KeepLocalHours:     0,
		EntryRetentionDays: 90,
		MinFreeDiskGB:      5,
	}
}

var bucketPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,254}$`)

func clamp(v, lo, hi, def int64) int64 {
	if v == 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Normalize trims and validates the capture configuration.
func (c *Config) Normalize() error {
	def := DefaultConfig()
	c.Mode = strings.TrimSpace(c.Mode)
	if c.Mode != ModeSelected {
		c.Mode = ModeAll
	}
	if len(c.Rules) > 200 {
		return errors.New("模型规则最多 200 条")
	}
	rules := make([]ModelRule, 0, len(c.Rules))
	for _, r := range c.Rules {
		r.Pattern = strings.TrimSpace(r.Pattern)
		r.Alias = strings.TrimSpace(r.Alias)
		if r.Pattern == "" {
			continue
		}
		if len(r.Pattern) > 200 || len(r.Alias) > 100 {
			return errors.New("模型规则过长")
		}
		if r.Alias != "" {
			r.Alias = SanitizeName(r.Alias)
		}
		rules = append(rules, r)
	}
	c.Rules = rules
	if c.Mode == ModeSelected && len(c.Rules) == 0 && c.CaptureEnabled {
		return errors.New("“指定模型”模式至少需要一条模型规则")
	}
	c.MaxZipSizeMB = clamp(c.MaxZipSizeMB, 64, 1024*1024, def.MaxZipSizeMB)
	c.SessionIdleSeconds = int(clamp(int64(c.SessionIdleSeconds), 30, 86400, int64(def.SessionIdleSeconds)))
	c.MaxBodyMB = int(clamp(int64(c.MaxBodyMB), 1, 256, int64(def.MaxBodyMB)))
	if c.KeepLocalHours < 0 {
		c.KeepLocalHours = 0
	}
	if c.KeepLocalHours > 24*365 {
		c.KeepLocalHours = 24 * 365
	}
	c.EntryRetentionDays = int(clamp(int64(c.EntryRetentionDays), 1, 3650, int64(def.EntryRetentionDays)))
	if c.MinFreeDiskGB < 0 {
		c.MinFreeDiskGB = 0
	}
	if c.MinFreeDiskGB > 10000 {
		c.MinFreeDiskGB = 10000
	}
	return nil
}

// ModelKey returns the archive name for a model and whether it is captured.
func (c Config) ModelKey(model string) (string, bool) {
	model = strings.TrimSpace(model)
	for _, r := range c.Rules {
		if wildcardMatch(r.Pattern, model) {
			if r.Alias != "" {
				return r.Alias, true
			}
			return SanitizeName(model), true
		}
	}
	if c.Mode == ModeSelected {
		return "", false
	}
	return SanitizeName(model), true
}

// wildcardMatch is a case-insensitive glob supporting only "*".
func wildcardMatch(pattern, value string) bool {
	p, v := strings.ToLower(pattern), strings.ToLower(value)
	if !strings.Contains(p, "*") {
		return p == v
	}
	parts := strings.Split(p, "*")
	if !strings.HasPrefix(v, parts[0]) {
		return false
	}
	v = v[len(parts[0]):]
	last := parts[len(parts)-1]
	for _, part := range parts[1 : len(parts)-1] {
		i := strings.Index(v, part)
		if i < 0 {
			return false
		}
		v = v[i+len(part):]
	}
	return strings.HasSuffix(v, last)
}

// SanitizeName keeps letters, digits, '.', '_' and '-' so the name is safe as a
// file name and S3 key segment.
func SanitizeName(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.TrimSpace(s) {
		ok := r == '.' || r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if len(out) > 100 {
		out = out[:100]
	}
	if out == "" {
		return "unknown"
	}
	return out
}

// BundleFileName follows "<Model>-<YYYY-MM-DD>.zip"; later parts of the same
// day add "-partNNN".
func BundleFileName(modelKey, day string, part int) string {
	if part <= 1 {
		return modelKey + "-" + day + ".zip"
	}
	return modelKey + "-" + day + "-part" + pad3(part) + ".zip"
}

func pad3(n int) string {
	s := itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// EntryName is "数据_000001.json".
func EntryName(seq int) string {
	s := itoa(seq)
	for len(s) < 6 {
		s = "0" + s
	}
	return "数据_" + s + ".json"
}
