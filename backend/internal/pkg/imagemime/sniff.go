package imagemime

import (
	"bytes"
	"encoding/base64"
	"strings"
	"unicode"
)

// Normalize returns a canonical image media type value.
func Normalize(mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if semicolon := strings.Index(mediaType, ";"); semicolon >= 0 {
		mediaType = strings.TrimSpace(mediaType[:semicolon])
	}
	if mediaType == "image/jpg" {
		return "image/jpeg"
	}
	return mediaType
}

// Sniff detects common image MIME types from magic bytes.
func Sniff(data []byte) string {
	switch {
	case len(data) >= 8 &&
		data[0] == 0x89 &&
		data[1] == 'P' &&
		data[2] == 'N' &&
		data[3] == 'G' &&
		data[4] == '\r' &&
		data[5] == '\n' &&
		data[6] == 0x1a &&
		data[7] == '\n':
		return "image/png"
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	default:
		return ""
	}
}

// SniffBase64Prefix detects common image MIME types from the beginning of a
// base64 payload without decoding the whole image.
func SniffBase64Prefix(encoded string) string {
	const maxChars = 64
	var builder strings.Builder
	builder.Grow(maxChars)
	for _, r := range encoded {
		if unicode.IsSpace(r) {
			continue
		}
		builder.WriteRune(r)
		if builder.Len() >= maxChars {
			break
		}
	}

	prefix := builder.String()
	if prefix == "" {
		return ""
	}
	if rem := len(prefix) % 4; rem != 0 {
		prefix += strings.Repeat("=", 4-rem)
	}
	data, err := base64.StdEncoding.DecodeString(prefix)
	if err != nil {
		return ""
	}
	return Sniff(data)
}
