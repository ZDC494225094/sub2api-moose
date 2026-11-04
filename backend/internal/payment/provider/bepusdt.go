package provider

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	bepusdtHTTPTimeout      = 15 * time.Second
	bepusdtMaxResponse      = 1 << 20
	bepusdtMaxError         = 512
	bepusdtDefaultFiat      = "CNY"
	bepusdtDefaultTradeType = "usdt.trc20"
)

// Bepusdt creates hosted cryptocurrency checkout orders through a BEpusdt instance.
// It only supports fixed fiat-amount payments; BEpusdt's address-exclusive mode
// cannot be reconciled with Sub2API's precomputed order amounts.
type Bepusdt struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

type bepusdtCreateRequest struct {
	OrderID     string  `json:"order_id"`
	Amount      float64 `json:"amount"`
	TradeType   string  `json:"trade_type"`
	Fiat        string  `json:"fiat"`
	Name        string  `json:"name"`
	NotifyURL   string  `json:"notify_url"`
	RedirectURL string  `json:"redirect_url"`
	Timeout     int64   `json:"timeout"`
	Signature   string  `json:"signature"`
}

type bepusdtAPIResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Data       struct {
		TradeID        string `json:"trade_id"`
		OrderID        string `json:"order_id"`
		Amount         string `json:"amount"`
		ActualAmount   string `json:"actual_amount"`
		PaymentURL     string `json:"payment_url"`
		ExpirationTime int64  `json:"expiration_time"`
	} `json:"data"`
}

func NewBepusdt(instanceID string, config map[string]string) (*Bepusdt, error) {
	for _, key := range []string{"apiBase", "apiToken", "notifyUrl"} {
		if strings.TrimSpace(config[key]) == "" {
			return nil, bepusdtConfigError("missing required field: " + key)
		}
	}
	base, err := normalizeBepusdtAPIBase(config["apiBase"])
	if err != nil {
		return nil, bepusdtConfigError(err.Error())
	}
	tradeType := strings.TrimSpace(config["tradeType"])
	if tradeType == "" {
		tradeType = bepusdtDefaultTradeType
	}
	if !validBepusdtTradeType(tradeType) {
		return nil, bepusdtConfigError("tradeType is invalid")
	}
	if err := validateBepusdtCallbackURL(config["notifyUrl"]); err != nil {
		return nil, bepusdtConfigError(err.Error())
	}
	fiat := strings.ToUpper(strings.TrimSpace(config["fiat"]))
	if fiat == "" {
		fiat = bepusdtDefaultFiat
	}
	switch fiat {
	case "CNY", "USD", "EUR", "GBP", "JPY":
	default:
		return nil, bepusdtConfigError("fiat must be CNY, USD, EUR, GBP, or JPY")
	}
	cfg := cloneStringMap(config)
	cfg["apiBase"], cfg["tradeType"], cfg["fiat"] = base, tradeType, fiat
	return &Bepusdt{instanceID: instanceID, config: cfg, httpClient: &http.Client{Timeout: bepusdtHTTPTimeout}}, nil
}

func bepusdtConfigError(message string) error {
	return infraerrors.BadRequest("BEPUSDT_CONFIG_INVALID", message)
}

func normalizeBepusdtAPIBase(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("bepusdt config apiBase must be an HTTP(S) base URL without query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validBepusdtTradeType(raw string) bool {
	switch raw {
	case "usdt.trc20", "usdc.trc20", "tron.trx",
		"usdt.erc20", "usdc.erc20", "ethereum.eth",
		"usdt.polygon", "usdc.polygon", "usdt.bep20", "usdc.bep20", "bsc.bnb",
		"usdt.aptos", "usdc.aptos", "usdt.solana", "usdc.solana",
		"usdt.xlayer", "usdc.xlayer", "usdt.arbitrum", "usdc.arbitrum",
		"usdc.base", "usdt.plasma", "usdt.ton", "ton.gram":
		return true
	default:
		return false
	}
}

func (b *Bepusdt) Name() string        { return "BEpusdt" }
func (b *Bepusdt) ProviderKey() string { return payment.TypeBepusdt }
func (b *Bepusdt) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeBepusdt}
}

func (b *Bepusdt) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	if strings.TrimSpace(req.OrderID) == "" || strings.TrimSpace(b.config["notifyUrl"]) == "" || strings.TrimSpace(req.ReturnURL) == "" {
		return nil, fmt.Errorf("bepusdt create payment: order id, notify URL, and return URL are required")
	}
	if err := validateBepusdtCallbackURL(b.config["notifyUrl"]); err != nil {
		return nil, err
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(req.Amount), 64)
	if err != nil || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, fmt.Errorf("bepusdt create payment: invalid fixed fiat amount %q", req.Amount)
	}
	payload := bepusdtCreateRequest{
		OrderID: req.OrderID, Amount: amount, TradeType: b.config["tradeType"], Fiat: b.config["fiat"],
		Name: req.Subject, NotifyURL: b.config["notifyUrl"], RedirectURL: req.ReturnURL,
		Timeout: req.TimeoutSeconds,
	}
	if strings.TrimSpace(payload.NotifyURL) == "" {
		return nil, fmt.Errorf("bepusdt create payment: notifyUrl is not configured")
	}
	if payload.Timeout < 120 {
		payload.Timeout = 120
	}
	payload.Signature = signBepusdtPayload(payload, b.config["apiToken"])
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bepusdt create payment: encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config["apiBase"]+"/api/v1/order/create-transaction", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bepusdt create payment: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bepusdt create payment: request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, bepusdtMaxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("bepusdt create payment: read response: %w", err)
	}
	if len(responseBody) > bepusdtMaxResponse {
		return nil, fmt.Errorf("bepusdt create payment: response exceeds size limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bepusdt create payment: HTTP %d: %s", resp.StatusCode, truncateBepusdtError(responseBody))
	}
	var result bepusdtAPIResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("bepusdt create payment: invalid response: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt create payment: gateway rejected order: %s", truncateBepusdtError([]byte(result.Message)))
	}
	paymentURL, err := validateBepusdtPaymentURL(result.Data.PaymentURL)
	if err != nil {
		return nil, err
	}
	if result.Data.OrderID != "" && result.Data.OrderID != req.OrderID {
		return nil, fmt.Errorf("bepusdt create payment: returned order id mismatch")
	}
	if strings.TrimSpace(result.Data.TradeID) == "" {
		return nil, fmt.Errorf("bepusdt create payment: response is missing trade id")
	}
	return &payment.CreatePaymentResponse{TradeNo: strings.TrimSpace(result.Data.TradeID), PayURL: paymentURL}, nil
}

func validateBepusdtPaymentURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", fmt.Errorf("bepusdt create payment: gateway returned an invalid HTTP(S) payment URL")
	}
	return parsed.String(), nil
}

func validateBepusdtCallbackURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("bepusdt config notifyUrl must be a valid HTTP(S) URL")
	}
	return nil
}

// signBepusdtPayload follows BEpusdt's sorted, nonempty key=value MD5 signing rule.
func signBepusdtPayload(value any, token string) string {
	encoded, _ := json.Marshal(value)
	var fields map[string]any
	_ = json.Unmarshal(encoded, &fields)
	keys := make([]string, 0, len(fields))
	for key, val := range fields {
		if key != "signature" && val != nil && val != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for _, key := range keys {
		if canonical.Len() > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(key)
		canonical.WriteByte('=')
		switch val := fields[key].(type) {
		case string:
			canonical.WriteString(val)
		case float64:
			canonical.WriteString(fmt.Sprint(val))
		default:
			canonical.WriteString(fmt.Sprint(val))
		}
	}
	sum := md5.Sum([]byte(canonical.String() + token))
	return hex.EncodeToString(sum[:])
}

func verifyBepusdtSignature(rawBody, token string) (map[string]json.RawMessage, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
		return nil, fmt.Errorf("bepusdt notification: invalid JSON: %w", err)
	}
	var signature string
	if err := json.Unmarshal(payload["signature"], &signature); err != nil || strings.TrimSpace(signature) == "" {
		return nil, fmt.Errorf("bepusdt notification: missing signature")
	}
	canonical, err := canonicalizeBepusdtRawPayload(payload)
	if err != nil {
		return nil, err
	}
	sum := md5.Sum([]byte(canonical + token))
	expected := hex.EncodeToString(sum[:])
	if len(signature) != len(expected) || subtle.ConstantTimeCompare([]byte(strings.ToLower(signature)), []byte(expected)) != 1 {
		return nil, fmt.Errorf("bepusdt notification: signature mismatch")
	}
	return payload, nil
}

func canonicalizeBepusdtRawPayload(payload map[string]json.RawMessage) (string, error) {
	keys := make([]string, 0, len(payload))
	values := make(map[string]string, len(payload))
	for key, raw := range payload {
		if key == "signature" {
			continue
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("bepusdt notification: invalid field %s: %w", key, err)
		}
		if value == nil || value == "" {
			continue
		}
		keys = append(keys, key)
		switch v := value.(type) {
		case string:
			values[key] = v
		case float64:
			values[key] = fmt.Sprint(v)
		default:
			values[key] = fmt.Sprint(v)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return strings.Join(parts, "&"), nil
}

func truncateBepusdtError(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > bepusdtMaxError {
		text = text[:bepusdtMaxError] + "..."
	}
	return text
}

func (b *Bepusdt) QueryOrder(_ context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	// BEpusdt's public merchant API has no order-query endpoint. Treat the order
	// as pending; signed callbacks remain authoritative for settlement.
	return &payment.QueryOrderResponse{TradeNo: strings.TrimSpace(tradeNo), Status: payment.ProviderStatusPending}, nil
}

func (b *Bepusdt) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	payload, err := verifyBepusdtSignature(rawBody, b.config["apiToken"])
	if err != nil {
		return nil, err
	}
	var fields struct {
		TradeID            string  `json:"trade_id"`
		OrderID            string  `json:"order_id"`
		Amount             float64 `json:"amount"`
		BlockTransactionID string  `json:"block_transaction_id"`
		Status             int     `json:"status"`
	}
	encoded, _ := json.Marshal(payload)
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, fmt.Errorf("bepusdt notification: invalid fields: %w", err)
	}
	if strings.TrimSpace(fields.OrderID) == "" {
		return nil, fmt.Errorf("bepusdt notification: missing order id")
	}
	if fields.Status != 2 {
		return nil, nil
	}
	if strings.TrimSpace(fields.TradeID) == "" {
		return nil, fmt.Errorf("bepusdt notification: missing trade id")
	}
	if fields.Amount <= 0 || math.IsNaN(fields.Amount) || math.IsInf(fields.Amount, 0) {
		return nil, fmt.Errorf("bepusdt notification: invalid fiat amount")
	}
	return &payment.PaymentNotification{
		TradeNo: strings.TrimSpace(fields.TradeID), OrderID: strings.TrimSpace(fields.OrderID),
		Amount: fields.Amount, Status: payment.ProviderStatusSuccess, RawData: rawBody,
		Metadata: map[string]string{"currency": b.config["fiat"], "block_transaction_id": strings.TrimSpace(fields.BlockTransactionID)},
	}, nil
}

func (b *Bepusdt) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("bepusdt does not support refunds")
}

func (b *Bepusdt) CancelPayment(ctx context.Context, tradeID string) error {
	tradeID = strings.TrimSpace(tradeID)
	if tradeID == "" {
		return fmt.Errorf("bepusdt cancel payment: trade id is required")
	}
	request := struct {
		TradeID   string `json:"trade_id"`
		Signature string `json:"signature"`
	}{TradeID: tradeID}
	request.Signature = signBepusdtPayload(request, b.config["apiToken"])
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("bepusdt cancel payment: encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config["apiBase"]+"/api/v1/order/cancel-transaction", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bepusdt cancel payment: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("bepusdt cancel payment: request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, bepusdtMaxResponse+1))
	if err != nil {
		return fmt.Errorf("bepusdt cancel payment: read response: %w", err)
	}
	if len(responseBody) > bepusdtMaxResponse {
		return fmt.Errorf("bepusdt cancel payment: response exceeds size limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bepusdt cancel payment: HTTP %d: %s", resp.StatusCode, truncateBepusdtError(responseBody))
	}
	var result struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return fmt.Errorf("bepusdt cancel payment: invalid response: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return fmt.Errorf("bepusdt cancel payment: gateway rejected cancellation: %s", truncateBepusdtError([]byte(result.Message)))
	}
	return nil
}
