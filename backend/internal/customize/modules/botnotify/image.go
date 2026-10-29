package botnotify

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// MaxImageBytes bounds one downloaded image. Telegram recompresses photos
// far below this; uncompressed screenshots usually arrive as documents.
const MaxImageBytes = 5 << 20

// MaxImagesPerAnswer bounds the images handed to the model for one answer.
const MaxImagesPerAnswer = 3

var (
	ErrImageTooLarge = errors.New("image too large")
	ErrImageType     = errors.New("unsupported image type")
)

// SniffImage reports the content type of a supported raster image by its
// magic bytes. The type declared by the chat platform is never trusted.
func SniffImage(data []byte) (string, bool) {
	switch ct := http.DetectContentType(data); ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return ct, true
	}
	return "", false
}

// DataURI encodes the image for an OpenAI-compatible image_url part.
func (i Image) DataURI() string {
	return "data:" + i.MIME + ";base64," + base64.StdEncoding.EncodeToString(i.Data)
}

// ParseImageDataURI decodes an admin-uploaded test image with the same size
// and type checks applied to chat attachments.
func ParseImageDataURI(uri string) (Image, error) {
	head, payload, ok := strings.Cut(uri, ",")
	if !ok || !strings.HasPrefix(head, "data:") || !strings.HasSuffix(head, ";base64") {
		return Image{}, ErrImageType
	}
	if base64.StdEncoding.DecodedLen(len(payload)) > MaxImageBytes+3 {
		return Image{}, ErrImageTooLarge
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return Image{}, ErrImageType
	}
	if len(data) > MaxImageBytes {
		return Image{}, ErrImageTooLarge
	}
	mime, ok := SniffImage(data)
	if !ok {
		return Image{}, ErrImageType
	}
	return Image{MIME: mime, Data: data}, nil
}

// downloadImage GETs an image with a hard size limit. Errors never include
// the URL: Telegram file URLs embed the bot token and QQ URLs carry keys.
func downloadImage(ctx context.Context, client *http.Client, rawURL string) (Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Image{}, errors.New("image request invalid")
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Image{}, ctx.Err()
		}
		return Image{}, errors.New("image download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Image{}, fmt.Errorf("image download rejected (HTTP %d)", resp.StatusCode)
	}
	if resp.ContentLength > MaxImageBytes {
		return Image{}, ErrImageTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return Image{}, errors.New("image download interrupted")
	}
	if len(data) > MaxImageBytes {
		return Image{}, ErrImageTooLarge
	}
	mime, ok := SniffImage(data)
	if !ok {
		return Image{}, ErrImageType
	}
	return Image{MIME: mime, Data: data}, nil
}

// qqMediaHosts are the QQ CDN domains attachment URLs point at. Any other
// host is refused so an event can never make the server fetch arbitrary URLs.
var qqMediaHosts = []string{"qq.com", "qq.com.cn", "qpic.cn", "gtimg.cn"}

// qqMediaURL normalizes an attachment URL (QQ sometimes omits the scheme)
// and accepts it only for an allowed QQ media host.
func qqMediaURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	} else if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", false
	}
	if !hostAllowed(u.Hostname(), qqMediaHosts) {
		return "", false
	}
	return u.String(), true
}

func hostAllowed(host string, allowed []string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, a := range allowed {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// newMediaClient returns a client for third-party media downloads: every
// redirect hop must stay on an allowed host, and connections to loopback,
// private or link-local addresses are refused even if DNS points there.
func newMediaClient(allowed []string) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: refusePrivateAddress}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
	}
	return &http.Client{
		Timeout:   40 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || !hostAllowed(req.URL.Hostname(), allowed) {
				return errors.New("redirect refused")
			}
			return nil
		},
	}
}

func refusePrivateAddress(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return errors.New("address refused")
	}
	return nil
}
