package solopay

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/nekoimi-labs/solopay-sdk-go/internal/signing"
)

func signedNotificationRequest(t *testing.T, body []byte, timestamp int64, nonce string) *http.Request {
	t.Helper()
	req := &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Path: "/hooks/solopay"},
		Header: make(http.Header),
	}
	canonical := signing.Canonical(req.Method, req.URL.EscapedPath(), timestamp, nonce, body)
	req.Header.Set(headerAppID, "app-test")
	req.Header.Set(headerTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(headerNonce, nonce)
	req.Header.Set(headerSignature, signing.SignHex([]byte("notify-secret"), canonical))
	return req
}

func TestVerifyNotification(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"event":"payment.paid","platform_trade_no":"P1","out_trade_no":"O1","app_id":"app-test","status":"paid","amount_cents":"1000","currency":"CNY","channel_type":"wxpay","paid_at":"2026-09-08T07:30:00Z","notify_extra":{"tenant":"one"}}`)
	req := signedNotificationRequest(t, body, now.Unix(), "nonce-1")
	verifier, err := NewNotifyVerifier(NotifyVerifierConfig{
		Secret:        "notify-secret",
		ExpectedAppID: "app-test",
		ReplayStore:   NewMemoryReplayStore(),
	})
	if err != nil {
		t.Fatal(err)
	}
	verifier.now = func() time.Time { return now }

	notification, err := verifier.Parse(req, body)
	if err != nil {
		t.Fatal(err)
	}
	if notification.Status != PaymentPaid || notification.AmountCents != 1000 || notification.PaidAt == nil {
		t.Fatalf("unexpected notification: %+v", notification)
	}
	if _, err := verifier.Parse(req, body); !errors.Is(err, ErrNotifyReplay) {
		t.Fatalf("second Parse() error = %v, want replay", err)
	}
}

func TestVerifyNotificationRejectsTampering(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"app_id":"app-test","amount_cents":"100"}`)
	req := signedNotificationRequest(t, body, now.Unix(), "nonce-1")
	verifier, _ := NewNotifyVerifier(NotifyVerifierConfig{Secret: "notify-secret"})
	verifier.now = func() time.Time { return now }
	if err := verifier.Verify(req, append(body, ' ')); !errors.Is(err, ErrNotifyInvalidSignature) {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyNotificationRejectsOldTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	body := []byte(`{"app_id":"app-test"}`)
	req := signedNotificationRequest(t, body, now.Add(-10*time.Minute).Unix(), "nonce-1")
	verifier, _ := NewNotifyVerifier(NotifyVerifierConfig{Secret: "notify-secret", MaxClockSkew: 5 * time.Minute})
	verifier.now = func() time.Time { return now }
	if err := verifier.Verify(req, body); !errors.Is(err, ErrNotifyInvalidTimestamp) {
		t.Fatalf("Verify() error = %v", err)
	}
}
