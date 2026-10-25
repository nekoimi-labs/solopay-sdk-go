package solopay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nekoimi-labs/solopay-sdk-go/internal/signing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := New(Config{
		BaseURL:           server.URL,
		AppID:             "app-test",
		AppSecret:         "secret-test",
		HTTPClient:        server.Client(),
		AllowInsecureHTTP: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	client.nonce = func() (string, error) { return "nonce-test", nil }
	return client
}

func assertSignedRequest(t *testing.T, request *http.Request, body []byte) {
	t.Helper()
	if got := request.Header.Get(headerAppID); got != "app-test" {
		t.Fatalf("X-App-Id = %q", got)
	}
	timestamp, err := strconv.ParseInt(request.Header.Get(headerTimestamp), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	canonical := signing.Canonical(request.Method, request.URL.Path, timestamp, request.Header.Get(headerNonce), body)
	if !signing.VerifyHex([]byte("secret-test"), canonical, request.Header.Get(headerSignature)) {
		t.Fatalf("invalid signature for canonical %q", canonical)
	}
}

func TestCreatePayment(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/open/orders" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		assertSignedRequest(t, request, body)
		if !strings.Contains(string(body), `"amount_cents":"100"`) {
			t.Fatalf("amount was not encoded as a JSON string: %s", body)
		}
		if got := request.Header.Get(headerIdempotencyKey); got != "ORDER-1" {
			t.Fatalf("Idempotency-Key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code":0,
			"message":"success",
			"data":{
				"order_id":"42",
				"platform_trade_no":"202609080001",
				"out_trade_no":"ORDER-1",
				"channel_type":"wxpay",
				"amount_cents":"100",
				"original_amount_cents":"100",
				"discount_cents":"0",
				"match_mode":"confirm_code",
				"currency":"CNY",
				"confirm_code":"123456",
				"expire_at":"2026-09-08 15:30:00",
				"cashier_url":"https://pay.example/cashier/orders/token"
			}
		}`))
	})

	payment, err := client.CreatePayment(context.Background(), CreatePaymentRequest{
		OutTradeNo:  "ORDER-1",
		Channel:     WeChatPay,
		AmountCents: 100,
		Subject:     "Coffee",
		NotifyURL:   "https://merchant.example/notify",
	})
	if err != nil {
		t.Fatal(err)
	}
	if payment.OrderID != 42 || payment.CashierURL == "" || payment.AmountCents != 100 {
		t.Fatalf("unexpected payment: %+v", payment)
	}
	if payment.ExpireAt.Location().String() != "UTC+8" {
		t.Fatalf("expire location = %s", payment.ExpireAt.Location())
	}
	if got, err := payment.PaymentURL(); err != nil || got != payment.CashierURL {
		t.Fatalf("PaymentURL() = %q, %v", got, err)
	}
}

func TestGetPaymentByOutTradeNoDoesNotSignQuery(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/open/orders/by-out-trade-no" || request.URL.Query().Get("out_trade_no") != "ORDER 1" {
			t.Fatalf("unexpected URL: %s", request.URL.String())
		}
		assertSignedRequest(t, request, nil)
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"order_id":"42","platform_trade_no":"1","out_trade_no":"ORDER 1","channel_type":"alipay","amount_cents":"99","original_amount_cents":"100","discount_cents":"1","match_mode":"random_discount","currency":"CNY","status":"paid","paid_at":"2026-09-08T07:30:00Z","expire_at":"2026-09-08 15:30:00","created_at":"2026-09-08 15:00:00"}}`))
	})
	payment, err := client.GetPaymentByOutTradeNo(context.Background(), "ORDER 1")
	if err != nil {
		t.Fatal(err)
	}
	if payment.Status != PaymentPaid || payment.PaidAt == nil || payment.DiscountCents != 1 {
		t.Fatalf("unexpected payment: %+v", payment)
	}
}

func TestAPIError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(headerRequestID, "request-1")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":40909,"message":"idempotency conflict","error":"different request"}`))
	})
	_, err := client.GetPayment(context.Background(), "123")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v", err, err)
	}
	if apiErr.Code != 40909 || apiErr.RequestID != "request-1" || !IsAPIErrorCode(err, 40909) {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}

func TestCreatePaymentValidation(t *testing.T) {
	client := newTestClient(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("request must not be sent")
	})
	_, err := client.CreatePayment(context.Background(), CreatePaymentRequest{Channel: "card", AmountCents: -1})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %T %v", err, err)
	}
}

func TestNewRejectsInsecureBaseURL(t *testing.T) {
	_, err := New(Config{BaseURL: "http://pay.example", AppID: "app", AppSecret: "secret"})
	if err == nil {
		t.Fatal("expected insecure URL error")
	}
}
