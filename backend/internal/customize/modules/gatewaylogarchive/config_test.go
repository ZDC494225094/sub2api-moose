package gatewaylogarchive

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigNormalizeParsesS3URIAndValidates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Prefix = "s3://incoming-04-20260624/logs/"
	require.NoError(t, cfg.Normalize())
	require.Equal(t, "incoming-04-20260624", cfg.Bucket)
	require.Equal(t, "logs", cfg.Prefix)
	require.Equal(t, "logs/2026-06-25/Opus5-2026-06-25.zip", cfg.ObjectKey("2026-06-25", BundleFileName("Opus5", "2026-06-25", 1)))

	cfg = DefaultConfig()
	cfg.Bucket = "incoming-04-20260624"
	require.NoError(t, cfg.Normalize())
	require.Equal(t, "2026-06-25/Opus5-2026-06-25.zip", cfg.ObjectKey("2026-06-25", "Opus5-2026-06-25.zip"))

	for _, bad := range []func(*Config){
		func(c *Config) { c.Bucket = "a"; c.Prefix = "s3://b/x" },
		func(c *Config) { c.Prefix = "a/../b" },
		func(c *Config) { c.Endpoint = "ftp://x" },
		func(c *Config) { c.AccessKeyID = "AK" },
		func(c *Config) { c.Provider = ProviderCompat; c.Bucket = "bucket" },
		func(c *Config) { c.Mode = ModeSelected; c.CaptureEnabled = true },
	} {
		c := DefaultConfig()
		bad(&c)
		require.Error(t, c.Normalize())
	}

	cfg = Config{MaxZipSizeMB: 1, MaxBodyMB: 9999}
	require.NoError(t, cfg.Normalize())
	require.Equal(t, int64(64), cfg.MaxZipSizeMB)
	require.Equal(t, 256, cfg.MaxBodyMB)
	require.Equal(t, 1800, cfg.SessionIdleSeconds)
}

func TestModelKeySelectionAndAliases(t *testing.T) {
	cfg := Config{Mode: ModeSelected, Rules: []ModelRule{{Pattern: "claude-opus-5*", Alias: "Opus5"}, {Pattern: "gpt-6.1-sol"}}}
	require.NoError(t, cfg.Normalize())
	key, ok := cfg.ModelKey("Claude-Opus-5-20261001")
	require.True(t, ok)
	require.Equal(t, "Opus5", key)
	key, ok = cfg.ModelKey("gpt-6.1-sol")
	require.True(t, ok)
	require.Equal(t, "gpt-6.1-sol", key)
	_, ok = cfg.ModelKey("gemini-3-pro")
	require.False(t, ok, "selected mode skips unmatched models")

	cfg.Mode = ModeAll
	key, ok = cfg.ModelKey("vendor/model:latest")
	require.True(t, ok)
	require.Equal(t, "vendor-model-latest", key)
	key, _ = cfg.ModelKey("")
	require.Equal(t, "unknown", key)

	require.True(t, wildcardMatch("*sonnet*", "claude-sonnet-5"))
	require.False(t, wildcardMatch("gpt-*-mini", "gpt-6"))
	require.Equal(t, "Opus5-2026-06-25-part002.zip", BundleFileName("Opus5", "2026-06-25", 2))
	require.Equal(t, "数据_000012.json", EntryName(12))
}
