package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
)

// ForwardGeminiVideoOperation polls a previously selected account's Veo operation.
// Model discovery uses the upstream ForwardListModels / ForwardGetModel paths.
func (s *GeminiMessagesCompatService) ForwardGeminiVideoOperation(ctx context.Context, account *Account, path string) (*UpstreamHTTPResult, error) {
	if account == nil {
		return nil, errors.New("account is nil")
	}
	// Vertex operation names can contain ten segments, exceeding the generic
	// Responses suffix limit. Validate this resource locally without widening
	// the shared URL guard used by model discovery or Responses forwarding.
	operationName, isOperation, err := geminiOperationNameFromGETPath(path)
	if err != nil {
		return nil, err
	}
	if !isOperation {
		return nil, errors.New("invalid Gemini operation path")
	}
	path = "/v1beta/" + operationName
	fullURL := ""
	if account.Type == AccountTypeServiceAccount {
		fullURL, err = buildVertexGeminiOperationURL(account.VertexProjectID(), operationName)
		if err != nil {
			return nil, err
		}
	} else {
		baseURL := account.GetGeminiBaseURL(geminicli.AIStudioBaseURL)
		normalizedBaseURL, validateErr := s.validateUpstreamBaseURL(baseURL)
		if validateErr != nil {
			return nil, validateErr
		}
		fullURL = strings.TrimRight(normalizedBaseURL, "/") + path
	}

	var proxyURL string
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	switch account.Type {
	case AccountTypeAPIKey:
		apiKey := strings.TrimSpace(account.GetCredential("api_key"))
		if apiKey == "" {
			return nil, errors.New("gemini api_key not configured")
		}
		req.Header.Set("x-goog-api-key", apiKey)
	case AccountTypeServiceAccount:
		if err := setVertexGeminiAuth(ctx, req, account, s.tokenProvider); err != nil {
			return nil, err
		}
	case AccountTypeOAuth:
		if s.tokenProvider == nil {
			return nil, errors.New("gemini token provider not configured")
		}
		accessToken, err := s.tokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
	default:
		return nil, fmt.Errorf("unsupported account type: %s", account.Type)
	}

	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	wwwAuthenticate := resp.Header.Get("Www-Authenticate")
	filteredHeaders := responseheaders.FilterHeaders(resp.Header, s.responseHeaderFilter)
	if wwwAuthenticate != "" {
		filteredHeaders.Set("Www-Authenticate", wwwAuthenticate)
	}
	return &UpstreamHTTPResult{
		StatusCode: resp.StatusCode,
		Headers:    filteredHeaders,
		Body:       body,
	}, nil
}

func geminiOperationNameFromGETPath(path string) (string, bool, error) {
	const prefix = "/v1beta/"
	if !strings.HasPrefix(path, prefix) {
		return "", false, nil
	}
	escapedSegments := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(path) > 2048 || len(escapedSegments) > 10 {
		return "", false, errors.New("invalid Gemini operation path")
	}
	segments := make([]string, len(escapedSegments))
	isOperation := false
	for index, escapedSegment := range escapedSegments {
		segment, err := url.PathUnescape(escapedSegment)
		if err != nil || !isSafeUpstreamPathSegment(segment) {
			return "", false, errors.New("invalid Gemini operation path")
		}
		segments[index] = segment
		isOperation = isOperation || segment == "operations"
	}
	if !isOperation {
		return "", false, nil
	}
	if len(segments) < 2 || segments[len(segments)-2] != "operations" {
		return "", false, errors.New("invalid Gemini operation path")
	}
	return strings.Join(segments, "/"), true, nil
}

// ForwardAIStudioVideoContent streams the media URI returned by a completed
// Veo operation while keeping the upstream credential on the server.
func (s *GeminiMessagesCompatService) ForwardAIStudioVideoContent(ctx context.Context, c *gin.Context, account *Account, rawURL string) error {
	if account == nil {
		return errors.New("account is nil")
	}
	contentURL, err := s.validateGeminiVideoContentURL(account, rawURL)
	if err != nil {
		return err
	}

	requestCtx := WithHTTPUpstreamRedirectsDisabled(ctx)
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, contentURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "*/*")
	if c != nil {
		if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" {
			req.Header.Set("Range", rangeHeader)
		}
	}
	switch account.Type {
	case AccountTypeAPIKey:
		apiKey := strings.TrimSpace(account.GetCredential("api_key"))
		if apiKey == "" {
			return errors.New("gemini api_key not configured")
		}
		req.Header.Set("x-goog-api-key", apiKey)
	case AccountTypeServiceAccount:
		if err := setVertexGeminiAuth(ctx, req, account, s.tokenProvider); err != nil {
			return err
		}
	case AccountTypeOAuth:
		if s.tokenProvider == nil {
			return errors.New("gemini token provider not configured")
		}
		accessToken, err := s.tokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
	default:
		return fmt.Errorf("unsupported account type: %s", account.Type)
	}
	account.ApplyHeaderOverrides(req.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return errors.New("gemini video content redirect is not allowed")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
		if message == "" {
			message = fmt.Sprintf("Gemini video content returned status %d", resp.StatusCode)
		}
		return errors.New(message)
	}
	return writeGrokMediaContentResponse(c, resp)
}

func (s *GeminiMessagesCompatService) validateGeminiVideoContentURL(account *Account, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("Gemini operation returned an invalid video URL")
	}
	if strings.EqualFold(parsed.Scheme, "gs") {
		if parsed.Port() != "" || parsed.RawQuery != "" || strings.TrimSpace(parsed.EscapedPath()) == "" {
			return "", errors.New("Gemini operation returned an invalid video URL")
		}
		parsed = &url.URL{
			Scheme:  "https",
			Host:    "storage.googleapis.com",
			Path:    "/" + parsed.Hostname() + parsed.Path,
			RawPath: "/" + url.PathEscape(parsed.Hostname()) + parsed.EscapedPath(),
		}
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", errors.New("Gemini operation returned an invalid video URL")
	}
	baseURL, err := s.validateUpstreamBaseURL(account.GetGeminiBaseURL(geminicli.AIStudioBaseURL))
	if err != nil {
		return "", err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(parsed.Hostname())
	baseHost := strings.ToLower(base.Hostname())
	trustedGoogleHost := host == "generativelanguage.googleapis.com" || host == "storage.googleapis.com" || strings.HasSuffix(host, ".googleapis.com")
	if host != baseHost && !trustedGoogleHost {
		return "", errors.New("Gemini operation returned an unsupported video host")
	}
	if parsed.Port() != "" && parsed.Port() != "443" {
		return "", errors.New("Gemini operation returned an unsupported video port")
	}
	return parsed.String(), nil
}
