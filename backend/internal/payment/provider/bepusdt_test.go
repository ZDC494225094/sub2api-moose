package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBepusdtConfigAllowsHTTPAPIAndCallbackURLs(t *testing.T) {
	_, err := NewBepusdt("1", map[string]string{"apiBase": "http://gateway.example", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "http://app.example/webhook"})
	require.NoError(t, err)

	_, err = NewBepusdt("1", map[string]string{"apiBase": "http://127.0.0.1:8080", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "http://127.0.0.1:8090/webhook"})
	require.NoError(t, err)

	_, err = NewBepusdt("1", map[string]string{"apiBase": "ftp://gateway.example", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "http://app.example/webhook"})
	require.ErrorContains(t, err, "HTTP(S)")
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
}

func TestBepusdtCreatePaymentSignsFixedFiatOrderAndUsesHostedCheckout(t *testing.T) {
	const token = "test-api-token"
	var received bepusdtCreateRequest
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/order/create-transaction", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		require.Equal(t, "merchant-order-1", received.OrderID)
		require.Equal(t, 12.5, received.Amount)
		require.Equal(t, "usdt.trc20", received.TradeType)
		require.Equal(t, "CNY", received.Fiat)
		require.Equal(t, "http://app.example/payment/webhook", received.NotifyURL)
		require.Equal(t, "http://app.example/payment/result", received.RedirectURL)
		require.GreaterOrEqual(t, received.Timeout, int64(120))
		require.Equal(t, signBepusdtPayload(received, token), received.Signature)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status_code":200,"message":"success","data":{"trade_id":"gateway-trade-1","order_id":"merchant-order-1","payment_url":"http://cashier.example/pay/checkout/1"}}`))
	}))
	defer server.Close()

	prov, err := NewBepusdt("1", map[string]string{
		"apiBase": server.URL, "apiToken": token, "tradeType": "usdt.trc20", "fiat": "cny",
		"notifyUrl": "http://app.example/payment/webhook",
	})
	require.NoError(t, err)
	require.Equal(t, []payment.PaymentType{payment.TypeBepusdt}, prov.SupportedTypes())
	result, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "merchant-order-1", Amount: "12.50", Subject: "充值", ReturnURL: "http://app.example/payment/result", TimeoutSeconds: 60,
	})
	require.NoError(t, err)
	require.Equal(t, "gateway-trade-1", result.TradeNo)
	require.Equal(t, "http://cashier.example/pay/checkout/1", result.PayURL)
}

func TestBepusdtVerifyNotificationChecksSignatureAndOnlyEmitsPaidEvents(t *testing.T) {
	prov, err := NewBepusdt("1", map[string]string{
		"apiBase": "https://pay.example", "apiToken": "callback-token", "tradeType": "usdt.trc20", "notifyUrl": "https://app.example/webhook",
	})
	require.NoError(t, err)
	paid := map[string]any{
		"trade_id": "gateway-trade-1", "order_id": "merchant-order-1", "amount": 25.5,
		"actual_amount": "3.75", "token": "T123", "block_transaction_id": "chain-hash", "status": 2,
	}
	paid["signature"] = signBepusdtPayload(paid, "callback-token")
	body, err := json.Marshal(paid)
	require.NoError(t, err)
	notification, err := prov.VerifyNotification(context.Background(), string(body), nil)
	require.NoError(t, err)
	require.Equal(t, "merchant-order-1", notification.OrderID)
	require.Equal(t, "gateway-trade-1", notification.TradeNo)
	require.Equal(t, 25.5, notification.Amount)
	require.Equal(t, "chain-hash", notification.Metadata["block_transaction_id"])

	paid["status"] = 1
	paid["signature"] = signBepusdtPayload(paid, "callback-token")
	body, err = json.Marshal(paid)
	require.NoError(t, err)
	notification, err = prov.VerifyNotification(context.Background(), string(body), nil)
	require.NoError(t, err)
	require.Nil(t, notification)

	paid["status"] = 2
	paid["signature"] = "00000000000000000000000000000000"
	body, err = json.Marshal(paid)
	require.NoError(t, err)
	_, err = prov.VerifyNotification(context.Background(), string(body), nil)
	require.ErrorContains(t, err, "signature mismatch")
}

func TestBepusdtQueryRefundAndCancelContracts(t *testing.T) {
	var cancelBody struct {
		TradeID   string `json:"trade_id"`
		Signature string `json:"signature"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/order/cancel-transaction", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&cancelBody))
		require.Equal(t, "gateway-trade-2", cancelBody.TradeID)
		require.Equal(t, signBepusdtPayload(cancelBody, "cancel-token"), cancelBody.Signature)
		_, _ = w.Write([]byte(`{"status_code":200,"message":"success"}`))
	}))
	defer server.Close()
	prov, err := NewBepusdt("1", map[string]string{
		"apiBase": server.URL, "apiToken": "cancel-token", "tradeType": "usdt.trc20", "notifyUrl": "https://app.example/webhook",
	})
	require.NoError(t, err)
	query, err := prov.QueryOrder(context.Background(), "gateway-trade-2")
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusPending, query.Status)
	_, err = prov.Refund(context.Background(), payment.RefundRequest{})
	require.ErrorContains(t, err, "does not support refunds")
	require.NoError(t, prov.CancelPayment(context.Background(), "gateway-trade-2"))
}

func TestBepusdtConfigValidationAndSignatureCanonicalization(t *testing.T) {
	_, err := NewBepusdt("1", map[string]string{"apiBase": "javascript:alert(1)", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "https://app.example/webhook"})
	require.Error(t, err)
	_, err = NewBepusdt("1", map[string]string{"apiBase": "https://gateway.example?token=unsafe", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "https://app.example/webhook"})
	require.Error(t, err)
	_, err = NewBepusdt("1", map[string]string{"apiBase": "https://gateway.example", "apiToken": "x", "tradeType": "bad type", "notifyUrl": "https://app.example/webhook"})
	require.Error(t, err)
	_, err = NewBepusdt("1", map[string]string{"apiBase": "http://gateway.example", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "http://app.example/webhook"})
	require.NoError(t, err)
	_, err = NewBepusdt("1", map[string]string{"apiBase": "https://gateway.example", "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "javascript:alert(1)"})
	require.ErrorContains(t, err, "valid HTTP(S)")

	first := map[string]any{"order_id": "a", "amount": 1.25, "optional": "", "signature": "ignored"}
	second := map[string]any{"signature": "different", "optional": nil, "amount": 1.25, "order_id": "a"}
	require.Equal(t, signBepusdtPayload(first, "secret"), signBepusdtPayload(second, "secret"))
	require.Len(t, signBepusdtPayload(first, "secret"), 32)

	docExample := map[string]any{
		"order_id":     "20220201030210321",
		"amount":       42,
		"notify_url":   "http://example.com/notify",
		"redirect_url": "http://example.com/redirect",
	}
	require.Equal(t, "1cd4b52df5587cfb1968b0c0c6e156cd", signBepusdtPayload(docExample, "epusdt_password_xasddawqe"))
}

func TestBepusdtDefaultTimeoutUsesProviderMinimum(t *testing.T) {
	var received bepusdtCreateRequest
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status_code":200,"data":{"trade_id":"t1","payment_url":"https://cashier.example/pay"}}`))
	}))
	defer server.Close()
	prov, err := NewBepusdt("1", map[string]string{"apiBase": server.URL, "apiToken": "x", "tradeType": "usdt.trc20", "notifyUrl": "https://app.example/webhook"})
	require.NoError(t, err)
	_, err = prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "o1", Amount: "1", ReturnURL: "https://app.example/return"})
	require.NoError(t, err)
	require.Equal(t, int64(120), received.Timeout)
}
